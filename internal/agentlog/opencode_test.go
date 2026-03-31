package agentlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// openTestDB creates a temporary file-based SQLite database and creates the
// OpenCode schema. A file-based DB is used instead of :memory: because
// database/sql uses a connection pool, and modernc.org/sqlite in-memory
// databases are per-connection — DDL on one connection is invisible to others.
// Using a file avoids both the visibility issue and connection-pool deadlocks
// from SetMaxOpenConns(1).
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := `
CREATE TABLE project (
	id           TEXT PRIMARY KEY,
	worktree     TEXT,
	name         TEXT,
	time_created INTEGER,
	time_updated INTEGER
);
CREATE TABLE session (
	id               TEXT PRIMARY KEY,
	project_id       TEXT,
	parent_id        TEXT,
	slug             TEXT,
	directory        TEXT,
	title            TEXT,
	version          TEXT,
	time_created     INTEGER,
	time_updated     INTEGER,
	time_compacting  INTEGER,
	time_archived    INTEGER,
	workspace_id     TEXT
);
CREATE TABLE message (
	id           TEXT PRIMARY KEY,
	session_id   TEXT,
	time_created INTEGER,
	time_updated INTEGER,
	data         TEXT
);
CREATE TABLE part (
	id           TEXT PRIMARY KEY,
	message_id   TEXT,
	session_id   TEXT,
	time_created INTEGER,
	time_updated INTEGER,
	data         TEXT
);`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

// insertProject inserts a row into the project table.
func insertProject(t *testing.T, db *sql.DB, id, worktree string, timeCreatedMilli int64) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO project (id, worktree, name, time_created, time_updated) VALUES (?, ?, ?, ?, ?)",
		id, worktree, "test-project", timeCreatedMilli, timeCreatedMilli,
	)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
}

// insertSession inserts a row into the session table.
func insertSession(t *testing.T, db *sql.DB, id, projectID string, timeCreatedMilli int64) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO session (id, project_id, slug, time_created, time_updated) VALUES (?, ?, ?, ?, ?)",
		id, projectID, "test-slug", timeCreatedMilli, timeCreatedMilli,
	)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
}

// insertMessage inserts a row into the message table.
func insertMessage(t *testing.T, db *sql.DB, id, sessionID string, timeCreatedMilli int64, data any) {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal message data: %v", err)
	}
	_, err = db.Exec(
		"INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?)",
		id, sessionID, timeCreatedMilli, timeCreatedMilli, string(raw),
	)
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}
}

// insertPart inserts a row into the part table.
func insertPart(t *testing.T, db *sql.DB, id, messageID, sessionID string, timeCreatedMilli int64, data any) {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal part data: %v", err)
	}
	_, err = db.Exec(
		"INSERT INTO part (id, message_id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?, ?)",
		id, messageID, sessionID, timeCreatedMilli, timeCreatedMilli, string(raw),
	)
	if err != nil {
		t.Fatalf("insert part: %v", err)
	}
}

// makeMsgData builds an ocMessageData-compatible map for marshalling.
func makeMsgData(role string, tokens *ocTokens) map[string]any {
	m := map[string]any{
		"role": role,
		"time": map[string]any{"created": float64(1772646630992)},
	}
	if tokens != nil {
		m["tokens"] = tokens
		m["cost"] = 0.001
	}
	return m
}

// ── ocParseMessage tests ──────────────────────────────────────────────────────

func TestOcParseMessage_TextPart(t *testing.T) {
	t.Parallel()

	msg := ocMessageData{Role: "assistant"}
	parts := []ocPartData{
		{Type: "text", Text: "hello world"},
	}
	ts := time.Unix(1000, 0)

	events := ocParseMessage(msg, parts, "gt-session", "opencode", "oc-sess-1", ts)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.EventType != "text" {
		t.Errorf("EventType = %q, want %q", ev.EventType, "text")
	}
	if ev.Content != "hello world" {
		t.Errorf("Content = %q, want %q", ev.Content, "hello world")
	}
	if ev.Role != "assistant" {
		t.Errorf("Role = %q, want %q", ev.Role, "assistant")
	}
	if ev.SessionID != "gt-session" {
		t.Errorf("SessionID = %q, want %q", ev.SessionID, "gt-session")
	}
	if ev.AgentType != "opencode" {
		t.Errorf("AgentType = %q, want %q", ev.AgentType, "opencode")
	}
	if ev.NativeSessionID != "oc-sess-1" {
		t.Errorf("NativeSessionID = %q, want %q", ev.NativeSessionID, "oc-sess-1")
	}
	if !ev.Timestamp.Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", ev.Timestamp, ts)
	}
}

func TestOcParseMessage_ToolPart(t *testing.T) {
	t.Parallel()

	msg := ocMessageData{Role: "assistant"}
	parts := []ocPartData{
		{
			Type:   "tool",
			Tool:   "bash",
			CallID: "call_123",
			State: &ocToolState{
				Status: "completed",
				Input:  json.RawMessage(`{"command":"ls"}`),
				Output: "file1.txt\nfile2.txt",
			},
		},
	}
	ts := time.Unix(2000, 0)

	events := ocParseMessage(msg, parts, "gt-session", "opencode", "oc-sess-1", ts)

	if len(events) != 2 {
		t.Fatalf("expected 2 events (tool_use + tool_result), got %d", len(events))
	}

	use := events[0]
	if use.EventType != "tool_use" {
		t.Errorf("events[0].EventType = %q, want %q", use.EventType, "tool_use")
	}
	if !strings.HasPrefix(use.Content, "bash") {
		t.Errorf("tool_use content %q should start with tool name 'bash'", use.Content)
	}
	if !strings.Contains(use.Content, `{"command":"ls"}`) {
		t.Errorf("tool_use content %q should contain input JSON", use.Content)
	}

	result := events[1]
	if result.EventType != "tool_result" {
		t.Errorf("events[1].EventType = %q, want %q", result.EventType, "tool_result")
	}
	if result.Content != "file1.txt\nfile2.txt" {
		t.Errorf("tool_result content = %q, want %q", result.Content, "file1.txt\nfile2.txt")
	}
}

func TestOcParseMessage_ReasoningPart(t *testing.T) {
	t.Parallel()

	msg := ocMessageData{Role: "assistant"}
	parts := []ocPartData{
		{Type: "reasoning", Text: "Let me think about this..."},
	}
	ts := time.Unix(3000, 0)

	events := ocParseMessage(msg, parts, "gt-session", "opencode", "oc-sess-1", ts)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.EventType != "thinking" {
		t.Errorf("EventType = %q, want %q", ev.EventType, "thinking")
	}
	if ev.Content != "Let me think about this..." {
		t.Errorf("Content = %q, want %q", ev.Content, "Let me think about this...")
	}
}

func TestOcParseMessage_UsageEvent(t *testing.T) {
	t.Parallel()

	tokens := &ocTokens{
		Total:     100,
		Input:     80,
		Output:    20,
		Reasoning: 0,
	}
	tokens.Cache.Read = 5
	tokens.Cache.Write = 3

	msg := ocMessageData{
		Role:   "assistant",
		Tokens: tokens,
		Cost:   0.001,
	}
	parts := []ocPartData{
		{Type: "text", Text: "done"},
	}
	ts := time.Unix(4000, 0)

	events := ocParseMessage(msg, parts, "gt-session", "opencode", "oc-sess-1", ts)

	// Expect text event + usage event.
	if len(events) != 2 {
		t.Fatalf("expected 2 events (text + usage), got %d", len(events))
	}

	var usageEv *AgentEvent
	for i := range events {
		if events[i].EventType == "usage" {
			usageEv = &events[i]
		}
	}
	if usageEv == nil {
		t.Fatal("no usage event found")
	}
	if usageEv.InputTokens != 80 {
		t.Errorf("InputTokens = %d, want 80", usageEv.InputTokens)
	}
	if usageEv.OutputTokens != 20 {
		t.Errorf("OutputTokens = %d, want 20", usageEv.OutputTokens)
	}
	if usageEv.CacheReadTokens != 5 {
		t.Errorf("CacheReadTokens = %d, want 5", usageEv.CacheReadTokens)
	}
	if usageEv.CacheCreationTokens != 3 {
		t.Errorf("CacheCreationTokens = %d, want 3", usageEv.CacheCreationTokens)
	}
	if usageEv.Role != "assistant" {
		t.Errorf("usage Role = %q, want %q", usageEv.Role, "assistant")
	}
}

func TestOcParseMessage_NoUsageForUser(t *testing.T) {
	t.Parallel()

	// User messages never have token data; even if Tokens is somehow set,
	// the role check prevents emitting a usage event.
	msg := ocMessageData{
		Role: "user",
		// Tokens deliberately nil — mirrors real user messages.
	}
	parts := []ocPartData{
		{Type: "text", Text: "what is 2+2?"},
	}

	events := ocParseMessage(msg, parts, "gt-session", "opencode", "oc-sess-1", time.Now())

	for _, ev := range events {
		if ev.EventType == "usage" {
			t.Errorf("unexpected usage event for user message: %+v", ev)
		}
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event (text only), got %d", len(events))
	}
}

func TestOcParseMessage_SkipsNonCompletedTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
	}{
		{"pending", "pending"},
		{"running", "running"},
		{"error", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg := ocMessageData{Role: "assistant"}
			parts := []ocPartData{
				{
					Type:   "tool",
					Tool:   "bash",
					CallID: "call_999",
					State: &ocToolState{
						Status: tt.status,
						Input:  json.RawMessage(`{"command":"sleep 10"}`),
					},
				},
			}

			events := ocParseMessage(msg, parts, "gt-session", "opencode", "oc-sess-1", time.Now())

			if len(events) != 0 {
				t.Errorf("expected 0 events for status=%q, got %d", tt.status, len(events))
			}
		})
	}
}

func TestOcParseMessage_SkipsEmptyContent(t *testing.T) {
	t.Parallel()

	msg := ocMessageData{Role: "assistant"}
	parts := []ocPartData{
		{Type: "text", Text: ""},      // empty text — must be skipped
		{Type: "reasoning", Text: ""}, // empty reasoning — must be skipped
	}

	events := ocParseMessage(msg, parts, "gt-session", "opencode", "oc-sess-1", time.Now())

	if len(events) != 0 {
		t.Errorf("expected 0 events for empty-content parts, got %d", len(events))
	}
}

func TestOcParseMessage_SkipsUnknownPartTypes(t *testing.T) {
	t.Parallel()

	msg := ocMessageData{Role: "assistant"}
	parts := []ocPartData{
		{Type: "step-start"},
		{Type: "step-finish"},
		{Type: "compaction"},
		{Type: "unknown-future-type"},
	}

	events := ocParseMessage(msg, parts, "gt-session", "opencode", "oc-sess-1", time.Now())

	if len(events) != 0 {
		t.Errorf("expected 0 events for unknown part types, got %d: %+v", len(events), events)
	}
}

func TestOcParseMessage_ToolWithNoOutput(t *testing.T) {
	t.Parallel()

	// Completed tool with empty output should emit only tool_use (no tool_result).
	msg := ocMessageData{Role: "assistant"}
	parts := []ocPartData{
		{
			Type:   "tool",
			Tool:   "bash",
			CallID: "call_456",
			State: &ocToolState{
				Status: "completed",
				Input:  json.RawMessage(`{"command":"echo"}`),
				Output: "", // no output
			},
		},
	}

	events := ocParseMessage(msg, parts, "gt-session", "opencode", "oc-sess-1", time.Now())

	if len(events) != 1 {
		t.Fatalf("expected 1 event (tool_use only), got %d", len(events))
	}
	if events[0].EventType != "tool_use" {
		t.Errorf("EventType = %q, want %q", events[0].EventType, "tool_use")
	}
}

// ── ocFindProject tests ───────────────────────────────────────────────────────

func TestOcFindProject(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	insertProject(t, db, "proj-abc", wd, 1000)
	insertProject(t, db, "proj-xyz", "/other/path", 2000)

	t.Run("finds matching project", func(t *testing.T) {
		t.Parallel()
		id, err := ocFindProject(context.Background(), db, wd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "proj-abc" {
			t.Errorf("project ID = %q, want %q", id, "proj-abc")
		}
	})

	t.Run("error when no match", func(t *testing.T) {
		t.Parallel()
		_, err := ocFindProject(context.Background(), db, "/nonexistent/path")
		if err == nil {
			t.Error("expected error for unknown worktree, got nil")
		}
	})
}

// ── ocFindSession tests ───────────────────────────────────────────────────────

func TestOcFindSession_ByEnvVar(t *testing.T) {
	// Cannot be parallel: modifies environment variable.
	db := openTestDB(t)
	insertProject(t, db, "proj-1", "/work", 1000)
	insertSession(t, db, "sess-explicit", "proj-1", 1_000_000)
	insertSession(t, db, "sess-recent", "proj-1", 2_000_000)

	t.Setenv("OPENCODE_SESSION_ID", "sess-explicit")

	id, err := ocFindSession(context.Background(), db, "proj-1", "sess-explicit", time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "sess-explicit" {
		t.Errorf("session ID = %q, want %q", id, "sess-explicit")
	}
}

func TestOcFindSession_EnvVar_WrongProject(t *testing.T) {
	db := openTestDB(t)
	insertProject(t, db, "proj-1", "/work", 1000)
	insertProject(t, db, "proj-2", "/other", 2000)
	insertSession(t, db, "sess-for-proj2", "proj-2", 1_000_000)

	// Session exists but belongs to proj-2, not proj-1.
	_, err := ocFindSession(context.Background(), db, "proj-1", "sess-for-proj2", time.Time{})
	if err == nil {
		t.Error("expected error when session belongs to different project")
	}
}

func TestOcFindSession_MostRecent(t *testing.T) {
	t.Parallel()

	base := int64(1_000_000)

	t.Run("no since filter picks newest", func(t *testing.T) {
		t.Parallel()
		db := openTestDB(t)
		insertProject(t, db, "proj-1", "/work", 1000)
		insertSession(t, db, "sess-old", "proj-1", base)
		insertSession(t, db, "sess-newer", "proj-1", base+1000)
		insertSession(t, db, "sess-newest", "proj-1", base+2000)

		id, err := ocFindSession(context.Background(), db, "proj-1", "", time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "sess-newest" {
			t.Errorf("session ID = %q, want %q", id, "sess-newest")
		}
	})

	t.Run("since filter excludes old sessions", func(t *testing.T) {
		t.Parallel()
		db := openTestDB(t)
		insertProject(t, db, "proj-1", "/work", 1000)
		insertSession(t, db, "sess-old", "proj-1", base)
		insertSession(t, db, "sess-newer", "proj-1", base+1000)
		insertSession(t, db, "sess-newest", "proj-1", base+2000)

		// since = base+500ms — excludes sess-old, leaves sess-newer and sess-newest
		since := time.UnixMilli(base + 500)
		id, err := ocFindSession(context.Background(), db, "proj-1", "", since)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "sess-newest" {
			t.Errorf("session ID = %q, want %q", id, "sess-newest")
		}
	})

	t.Run("since filter too recent returns error", func(t *testing.T) {
		t.Parallel()
		db := openTestDB(t)
		insertProject(t, db, "proj-1", "/work", 1000)
		insertSession(t, db, "sess-old", "proj-1", base)

		// since is after all sessions — nothing should match
		since := time.UnixMilli(base + 99_999)
		_, err := ocFindSession(context.Background(), db, "proj-1", "", since)
		if err == nil {
			t.Error("expected error when no sessions match since filter")
		}
	})
}

// ── ocFindNewerSession tests ──────────────────────────────────────────────────

func TestOcFindNewerSession(t *testing.T) {
	t.Parallel()

	base := int64(1_000_000)

	t.Run("returns newer session", func(t *testing.T) {
		t.Parallel()
		db := openTestDB(t)
		insertProject(t, db, "proj-1", "/work", 1000)
		insertSession(t, db, "sess-current", "proj-1", base)
		insertSession(t, db, "sess-newer", "proj-1", base+1000)

		id, err := ocFindNewerSession(context.Background(), db, "proj-1", "sess-current", time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "sess-newer" {
			t.Errorf("newer session ID = %q, want %q", id, "sess-newer")
		}
	})

	t.Run("no newer session returns empty string", func(t *testing.T) {
		t.Parallel()
		db := openTestDB(t)
		insertProject(t, db, "proj-1", "/work", 1000)
		insertSession(t, db, "sess-only", "proj-1", base)

		id, err := ocFindNewerSession(context.Background(), db, "proj-1", "sess-only", time.Time{})
		// sql.ErrNoRows → err is non-nil, id is empty; that's the expected "none found" signal.
		if err == nil && id != "" {
			t.Errorf("expected no newer session, got id=%q", id)
		}
		if id != "" {
			t.Errorf("expected empty id, got %q", id)
		}
	})

	t.Run("since filter excludes older newer session", func(t *testing.T) {
		t.Parallel()
		db := openTestDB(t)
		insertProject(t, db, "proj-1", "/work", 1000)
		insertSession(t, db, "sess-current", "proj-1", base)
		insertSession(t, db, "sess-newer", "proj-1", base+1000)

		// sess-newer is at base+1000. since = base+2000 excludes it.
		since := time.UnixMilli(base + 2000)
		id, err := ocFindNewerSession(context.Background(), db, "proj-1", "sess-current", since)
		// Either error (no rows) or empty id means correct behaviour.
		if err == nil && id != "" {
			t.Errorf("expected no session after since filter, got id=%q", id)
		}
	})
}

func TestOcFindNewerSession_CurrentNotFound(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	insertProject(t, db, "proj-1", "/work", 1000)

	// currentSessionID does not exist — should return an error.
	_, err := ocFindNewerSession(context.Background(), db, "proj-1", "nonexistent-session", time.Time{})
	if err == nil {
		t.Error("expected error when current session ID not found in DB")
	}
}

// ── ocPollMessages tests ──────────────────────────────────────────────────────

func TestOcPollMessages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	baseTime := int64(1_774_200_000_000) // milliseconds

	// helper to build a populated DB for poll tests.
	buildDB := func(t *testing.T) *sql.DB {
		t.Helper()
		db := openTestDB(t)
		insertProject(t, db, "proj-1", "/work", 1000)
		insertSession(t, db, "sess-1", "proj-1", 1_000_000)

		// Message 1: assistant text
		msgData1 := makeMsgData("assistant", &ocTokens{Input: 10, Output: 5})
		insertMessage(t, db, "msg-1", "sess-1", baseTime+1000, msgData1)
		insertPart(t, db, "part-1a", "msg-1", "sess-1", baseTime+500, map[string]any{
			"type": "text",
			"text": "Hello from assistant",
		})

		// Message 2: user text (no tokens)
		msgData2 := makeMsgData("user", nil)
		insertMessage(t, db, "msg-2", "sess-1", baseTime+2000, msgData2)
		insertPart(t, db, "part-2a", "msg-2", "sess-1", baseTime+1500, map[string]any{
			"type": "text",
			"text": "Hello from user",
		})

		// Message 3: tool call
		msgData3 := makeMsgData("assistant", &ocTokens{Input: 20, Output: 10})
		insertMessage(t, db, "msg-3", "sess-1", baseTime+3000, msgData3)
		insertPart(t, db, "part-3a", "msg-3", "sess-1", baseTime+2500, map[string]any{
			"type":   "tool",
			"tool":   "bash",
			"callID": "call_001",
			"state": map[string]any{
				"status": "completed",
				"input":  map[string]any{"command": "pwd"},
				"output": "/home/user",
			},
		})
		return db
	}

	t.Run("polls all messages from beginning", func(t *testing.T) {
		t.Parallel()
		db := buildDB(t)

		events, lastTime, err := ocPollMessages(ctx, db, "sess-1", "gt-sess", "opencode", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) == 0 {
			t.Fatal("expected events, got none")
		}
		if lastTime == 0 {
			t.Error("lastTime should be non-zero after polling messages")
		}

		// Count event types.
		typeCounts := make(map[string]int)
		for _, ev := range events {
			typeCounts[ev.EventType]++
		}
		if typeCounts["text"] < 2 {
			t.Errorf("expected at least 2 text events, got %d", typeCounts["text"])
		}
		if typeCounts["tool_use"] != 1 {
			t.Errorf("expected 1 tool_use event, got %d", typeCounts["tool_use"])
		}
		if typeCounts["tool_result"] != 1 {
			t.Errorf("expected 1 tool_result event, got %d", typeCounts["tool_result"])
		}
		if typeCounts["usage"] < 1 {
			t.Errorf("expected at least 1 usage event, got %d", typeCounts["usage"])
		}
	})

	t.Run("afterTime filters older messages", func(t *testing.T) {
		t.Parallel()
		db := buildDB(t)

		// afterTime = baseTime+1000 means msg-1 is excluded (time_created must be > afterTime)
		events, _, err := ocPollMessages(ctx, db, "sess-1", "gt-sess", "opencode", baseTime+1000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, ev := range events {
			if ev.Content == "Hello from assistant" {
				t.Error("msg-1 events should be excluded by afterTime filter")
			}
		}
	})

	t.Run("empty session returns no events", func(t *testing.T) {
		t.Parallel()
		db := buildDB(t)

		events, lastTime, err := ocPollMessages(ctx, db, "nonexistent-session", "gt-sess", "opencode", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) != 0 {
			t.Errorf("expected 0 events for unknown session, got %d", len(events))
		}
		if lastTime != 0 {
			t.Errorf("lastTime should remain 0 for empty result, got %d", lastTime)
		}
	})
}

// ── OpenCodeAdapter.Watch integration test ────────────────────────────────────

func TestOpenCodeAdapter_WatchIntegration(t *testing.T) {
	// Not parallel: creates a temp file DB (not in-memory) so Watch can open it.
	tmpDir := t.TempDir()
	dbPath := fmt.Sprintf("%s/opencode.db", tmpDir)

	// Create and populate the temp DB.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	schema := `
CREATE TABLE project (id TEXT PRIMARY KEY, worktree TEXT, name TEXT, time_created INTEGER, time_updated INTEGER);
CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT, parent_id TEXT, slug TEXT, directory TEXT, title TEXT, version TEXT, time_created INTEGER, time_updated INTEGER, time_compacting INTEGER, time_archived INTEGER, workspace_id TEXT);
CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT);
CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	baseTime := int64(1_774_200_000_000)
	insertProject(t, db, "proj-int", wd, baseTime)
	insertSession(t, db, "sess-int", "proj-int", baseTime)

	msgData := makeMsgData("assistant", &ocTokens{Input: 50, Output: 25})
	insertMessage(t, db, "msg-int-1", "sess-int", baseTime+1000, msgData)
	insertPart(t, db, "part-int-1", "msg-int-1", "sess-int", baseTime+500, map[string]any{
		"type": "text",
		"text": "integration test reply",
	})

	db.Close() // Close so Watch can re-open read-only.

	adapter := &OpenCodeAdapter{DBPath: dbPath}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch, err := adapter.Watch(ctx, "test-gt-session", wd, time.Time{})
	if err != nil {
		t.Fatalf("Watch() error: %v", err)
	}

	var received []AgentEvent
	deadline := time.After(5 * time.Second)
collect:
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				break collect
			}
			received = append(received, ev)
			// Once we have the text event we care about, cancel and drain.
			for _, e := range received {
				if e.EventType == "text" && e.Content == "integration test reply" {
					cancel()
					// Drain until closed.
					for range ch {
					}
					break collect
				}
			}
		case <-deadline:
			cancel()
			for range ch {
			}
			break collect
		}
	}

	found := false
	for _, ev := range received {
		if ev.EventType == "text" && ev.Content == "integration test reply" {
			found = true
			if ev.AgentType != "opencode" {
				t.Errorf("AgentType = %q, want %q", ev.AgentType, "opencode")
			}
			if ev.SessionID != "test-gt-session" {
				t.Errorf("SessionID = %q, want %q", ev.SessionID, "test-gt-session")
			}
			if ev.NativeSessionID != "sess-int" {
				t.Errorf("NativeSessionID = %q, want %q", ev.NativeSessionID, "sess-int")
			}
		}
	}
	if !found {
		t.Errorf("expected text event 'integration test reply' in received events; got %d events: %+v", len(received), received)
	}
}

func TestOpenCodeAdapter_Watch_DBNotFound(t *testing.T) {
	t.Parallel()

	adapter := &OpenCodeAdapter{DBPath: "/nonexistent/path/opencode.db"}
	_, err := adapter.Watch(context.Background(), "sess", "/work", time.Time{})
	if err == nil {
		t.Error("expected error when DB path does not exist, got nil")
	}
}

// ── OpenCodeSessionCost tests ─────────────────────────────────────────────────

// createTestDBFile creates a temporary file-based SQLite DB with the OpenCode
// schema and returns its path. The DB is closed so OpenCodeSessionCostFromDB
// can open its own connection.
func createTestDBFile(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/opencode.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	schema := `
CREATE TABLE project (
	id           TEXT PRIMARY KEY,
	worktree     TEXT,
	name         TEXT,
	time_created INTEGER,
	time_updated INTEGER
);
CREATE TABLE session (
	id               TEXT PRIMARY KEY,
	project_id       TEXT,
	parent_id        TEXT,
	slug             TEXT,
	directory        TEXT,
	title            TEXT,
	version          TEXT,
	time_created     INTEGER,
	time_updated     INTEGER,
	time_compacting  INTEGER,
	time_archived    INTEGER,
	workspace_id     TEXT
);
CREATE TABLE message (
	id           TEXT PRIMARY KEY,
	session_id   TEXT,
	time_created INTEGER,
	time_updated INTEGER,
	data         TEXT
);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	db.Close()
	return dbPath
}

// populateTestCostDB opens the DB at dbPath and inserts project, session, and messages.
func populateTestCostDB(t *testing.T, dbPath, projectID, worktree, sessionID string, costs []float64) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now().UnixMilli()
	_, err = db.Exec(
		"INSERT INTO project (id, worktree, name, time_created, time_updated) VALUES (?, ?, 'test', ?, ?)",
		projectID, worktree, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		"INSERT INTO session (id, project_id, time_created, time_updated) VALUES (?, ?, ?, ?)",
		sessionID, projectID, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	for i, cost := range costs {
		msgData := fmt.Sprintf(`{"role":"assistant","cost":%f,"modelID":"test-model","tokens":{"total":100,"input":80,"output":20,"reasoning":0,"cache":{"read":0,"write":0}}}`, cost)
		_, err = db.Exec(
			"INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?)",
			fmt.Sprintf("msg_%d", i), sessionID, now+int64(i), now+int64(i), msgData,
		)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestOpenCodeSessionCost_SumsMultipleMessages(t *testing.T) {
	t.Parallel()

	dbPath := createTestDBFile(t)
	workDir := "/tmp/test-project-cost"
	populateTestCostDB(t, dbPath, "proj1", workDir, "ses_abc", []float64{0.01, 0.02, 0.03})

	cost, err := OpenCodeSessionCostFromDB(dbPath, workDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Sum should be 0.06
	if cost < 0.059 || cost > 0.061 {
		t.Errorf("expected cost ~0.06, got %f", cost)
	}
}

func TestOpenCodeSessionCost_ZeroCostMessages(t *testing.T) {
	t.Parallel()

	dbPath := createTestDBFile(t)
	workDir := "/tmp/test-project-zero"
	populateTestCostDB(t, dbPath, "proj2", workDir, "ses_def", []float64{0.0, 0.0})

	cost, err := OpenCodeSessionCostFromDB(dbPath, workDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cost != 0.0 {
		t.Errorf("expected cost 0, got %f", cost)
	}
}

func TestOpenCodeSessionCost_NoMessages(t *testing.T) {
	t.Parallel()

	dbPath := createTestDBFile(t)
	workDir := "/tmp/test-project-empty"
	populateTestCostDB(t, dbPath, "proj3", workDir, "ses_ghi", nil)

	cost, err := OpenCodeSessionCostFromDB(dbPath, workDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cost != 0.0 {
		t.Errorf("expected cost 0, got %f", cost)
	}
}

func TestOpenCodeSessionCost_ByEnvSessionID(t *testing.T) {
	// Test that OPENCODE_SESSION_ID takes priority over workDir resolution.
	dbPath := createTestDBFile(t)
	workDir := "/tmp/test-project-env"
	populateTestCostDB(t, dbPath, "proj4", workDir, "ses_jkl", []float64{0.05, 0.10})

	t.Setenv("OPENCODE_SESSION_ID", "ses_jkl")

	cost, err := OpenCodeSessionCostFromDB(dbPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Sum should be 0.15
	if cost < 0.149 || cost > 0.151 {
		t.Errorf("expected cost ~0.15, got %f", cost)
	}
}

func TestOpenCodeSessionCost_NoProjectOrSession(t *testing.T) {
	t.Parallel()

	dbPath := createTestDBFile(t)

	_, err := OpenCodeSessionCostFromDB(dbPath, "/nonexistent/workdir")
	if err == nil {
		t.Error("expected error for nonexistent project, got nil")
	}
}

func TestOpenCodeSessionCost_DBNotFound(t *testing.T) {
	t.Parallel()

	_, err := OpenCodeSessionCostFromDB("/nonexistent/opencode.db", "/work")
	if err == nil {
		t.Error("expected error for nonexistent DB, got nil")
	}
}

// ── openCodeDBPath tests ──────────────────────────────────────────────────────

func TestOpenCodeDBPath(t *testing.T) {
	// Cannot be parallel: subtests use t.Setenv which modifies process env.
	tests := []struct {
		name     string
		files    map[string]time.Duration // filename → age (negative = in the past)
		wantBase string                   // expected basename of returned path
		wantErr  bool
	}{
		{
			name:     "single default db",
			files:    map[string]time.Duration{"opencode.db": 0},
			wantBase: "opencode.db",
		},
		{
			name:     "single channel db",
			files:    map[string]time.Duration{"opencode-dev.db": 0},
			wantBase: "opencode-dev.db",
		},
		{
			name: "multiple channel dbs picks most recent",
			files: map[string]time.Duration{
				"opencode-dev.db":  -2 * time.Hour,
				"opencode-beta.db": -1 * time.Hour,
				"opencode-prod.db": 0,
			},
			wantBase: "opencode-prod.db",
		},
		{
			name:    "no dbs returns error",
			files:   nil,
			wantErr: true,
		},
		{
			name: "channel db takes priority over default",
			files: map[string]time.Duration{
				"opencode.db":     0,
				"opencode-dev.db": 0,
			},
			wantBase: "opencode-dev.db",
		},
		{
			name:     "arbitrary channel name",
			files:    map[string]time.Duration{"opencode-my-custom-channel.db": 0},
			wantBase: "opencode-my-custom-channel.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			ocDir := filepath.Join(tmpDir, openCodeDataSubdir)
			if err := os.MkdirAll(ocDir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}

			now := time.Now()
			for name, age := range tt.files {
				p := filepath.Join(ocDir, name)
				if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
					t.Fatalf("write %s: %v", name, err)
				}
				mtime := now.Add(age)
				if err := os.Chtimes(p, mtime, mtime); err != nil {
					t.Fatalf("chtimes %s: %v", name, err)
				}
			}

			t.Setenv("XDG_DATA_HOME", tmpDir)

			got, err := openCodeDBPath()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if filepath.Base(got) != tt.wantBase {
				t.Errorf("got basename %q, want %q", filepath.Base(got), tt.wantBase)
			}
		})
	}
}

// ── mostRecentFile tests ──────────────────────────────────────────────────────

func TestMostRecentFile(t *testing.T) {
	t.Parallel()

	t.Run("returns most recent", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		old := filepath.Join(tmpDir, "old.db")
		newer := filepath.Join(tmpDir, "newer.db")
		if err := os.WriteFile(old, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(newer, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
		pastTime := time.Now().Add(-time.Hour)
		if err := os.Chtimes(old, pastTime, pastTime); err != nil {
			t.Fatal(err)
		}

		got, err := mostRecentFile([]string{old, newer})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != newer {
			t.Errorf("got %q, want %q", got, newer)
		}
	})

	t.Run("skips unreadable files", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		good := filepath.Join(tmpDir, "good.db")
		if err := os.WriteFile(good, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}

		got, err := mostRecentFile([]string{"/nonexistent/bad.db", good})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != good {
			t.Errorf("got %q, want %q", got, good)
		}
	})

	t.Run("all unreadable returns error", func(t *testing.T) {
		t.Parallel()
		_, err := mostRecentFile([]string{"/nonexistent/a.db", "/nonexistent/b.db"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}
