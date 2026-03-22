package agentlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // registers "sqlite" database/sql driver
)

const (
	// openCodeDBName is the SQLite database filename used by OpenCode.
	openCodeDBName = "opencode.db"

	// openCodeDataSubdir is the path under XDG_DATA_HOME (or ~/.local/share)
	// where OpenCode stores its database.
	openCodeDataSubdir = "opencode"
)

// OpenCodeAdapter reads agent conversation events from OpenCode's SQLite database.
//
// OpenCode stores session data at:
//
//	$XDG_DATA_HOME/opencode/opencode.db  (default: ~/.local/share/opencode/opencode.db)
//
// Tables used: project (workDir→project mapping), session (conversation sessions),
// message (conversation turns), part (content blocks within turns).
//
// The adapter:
//  1. Opens the DB read-only
//  2. Finds the project by matching project.worktree to workDir
//  3. Finds the session by OPENCODE_SESSION_ID env or most recent for the project
//  4. Polls the message and part tables for new conversation data
//  5. Maps message/part data to normalized AgentEvents
//  6. Switches to a newer session when one appears (handles agent restarts)
type OpenCodeAdapter struct {
	// DBPath overrides the default OpenCode database path.
	// When empty, the standard XDG path is used. Exposed for testing.
	DBPath string
}

func (a *OpenCodeAdapter) AgentType() string { return "opencode" }

// Watch starts polling OpenCode's SQLite database for conversation events.
// sessionID is the Gas Town tmux session name (used as a log tag).
// workDir is the agent's working directory (matched against project.worktree).
// since filters sessions created before this time; pass zero to disable.
//
// The returned channel is closed when ctx is canceled or a fatal error occurs.
// Like the Claude adapter, Watch automatically switches to newer sessions
// within one poll interval when OpenCode restarts.
func (a *OpenCodeAdapter) Watch(ctx context.Context, sessionID, workDir string, since time.Time) (<-chan AgentEvent, error) {
	dbPath := a.DBPath
	if dbPath == "" {
		var err error
		dbPath, err = openCodeDBPath()
		if err != nil {
			return nil, fmt.Errorf("locating opencode db: %w", err)
		}
	}

	db, err := openCodeOpen(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening opencode db at %s: %w", dbPath, err)
	}

	ch := make(chan AgentEvent, 64)
	go func() {
		defer close(ch)
		defer db.Close()
		a.pollLoop(ctx, db, sessionID, workDir, since, ch)
	}()
	return ch, nil
}

// openCodeDBPath returns the path to OpenCode's SQLite database.
// Respects XDG_DATA_HOME; falls back to ~/.local/share/opencode/opencode.db.
func openCodeDBPath() (string, error) {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home dir: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	dbPath := filepath.Join(dataDir, openCodeDataSubdir, openCodeDBName)
	if _, err := os.Stat(dbPath); err != nil {
		return "", fmt.Errorf("db not found at %s: %w", dbPath, err)
	}
	return dbPath, nil
}

// openCodeOpen opens the OpenCode SQLite database in read-only mode.
// NOTE: We intentionally do NOT set PRAGMA journal_mode=wal here. The connection
// is read-only (mode=ro), so the pragma would silently fail. The writer side
// (OpenCode itself) is responsible for setting WAL mode on the database file.
// A read-only connection automatically inherits the journal mode from the file.
func openCodeOpen(dbPath string) (*sql.DB, error) {
	// Use URI format with mode=ro for read-only access.
	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Verify connectivity.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}

// pollLoop is the main polling goroutine. It resolves the project and session,
// then continuously polls for new messages. It switches to newer sessions
// when they appear (handling OpenCode restarts).
func (a *OpenCodeAdapter) pollLoop(ctx context.Context, db *sql.DB, gtSessionID, workDir string, since time.Time, ch chan<- AgentEvent) {
	envSessionID := os.Getenv("OPENCODE_SESSION_ID")

	var projectID string
	var ocSessionID string
	var lastMsgTime int64 // Unix milliseconds of last processed message

	// Exponential backoff: start at the base poll interval, double on each
	// consecutive miss (no new data / DB not found), cap at maxPollInterval.
	// Reset to base when new data arrives.
	const basePollInterval = 500 * time.Millisecond
	const maxPollInterval = 10 * time.Second
	pollInterval := basePollInterval

	for {
		if ctx.Err() != nil {
			return
		}

		gotData := false

		// Step 1: Resolve project (retry until found or ctx canceled).
		if projectID == "" {
			pid, err := ocFindProject(ctx, db, workDir)
			if err != nil {
				pollInterval = min(pollInterval*2, maxPollInterval)
				select {
				case <-ctx.Done():
					return
				case <-time.After(pollInterval):
				}
				continue
			}
			projectID = pid
		}

		// Step 2: Resolve session (retry until found or ctx canceled).
		if ocSessionID == "" {
			sid, err := ocFindSession(ctx, db, projectID, envSessionID, since)
			if err != nil {
				pollInterval = min(pollInterval*2, maxPollInterval)
				select {
				case <-ctx.Done():
					return
				case <-time.After(pollInterval):
				}
				continue
			}
			ocSessionID = sid
		}

		// Step 3: Poll for new messages.
		events, newLastTime, err := ocPollMessages(ctx, db, ocSessionID, gtSessionID, a.AgentType(), lastMsgTime)
		if err == nil && len(events) > 0 {
			gotData = true
			for _, ev := range events {
				select {
				case ch <- ev:
				case <-ctx.Done():
					return
				}
			}
			lastMsgTime = newLastTime
		}

		// Step 4: Check for newer session (agent restart detection).
		if newerSID, err := ocFindNewerSession(ctx, db, projectID, ocSessionID, since); err == nil && newerSID != "" {
			ocSessionID = newerSID
			lastMsgTime = 0 // read all messages from the new session
			gotData = true
		}

		// Backoff: reset to base interval when we got data, otherwise double.
		if gotData {
			pollInterval = basePollInterval
		} else {
			pollInterval = min(pollInterval*2, maxPollInterval)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

// ── Database queries ──────────────────────────────────────────────────────────

// ocFindProject finds the OpenCode project ID matching workDir.
// workDir is compared against project.worktree after resolving to an absolute path.
func ocFindProject(_ context.Context, db *sql.DB, workDir string) (string, error) {
	abs, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}
	var projectID string
	err = db.QueryRow("SELECT id FROM project WHERE worktree = ?", abs).Scan(&projectID)
	if err != nil {
		return "", fmt.Errorf("no project for worktree %s: %w", abs, err)
	}
	return projectID, nil
}

// ocFindSession finds the OpenCode session to watch.
// If envSessionID is set (from OPENCODE_SESSION_ID), it is used directly.
// Otherwise, the most recent session for the project created at or after since is returned.
func ocFindSession(_ context.Context, db *sql.DB, projectID, envSessionID string, since time.Time) (string, error) {
	if envSessionID != "" {
		// Verify the session exists and belongs to the project.
		var id string
		err := db.QueryRow(
			"SELECT id FROM session WHERE id = ? AND project_id = ?",
			envSessionID, projectID,
		).Scan(&id)
		if err != nil {
			return "", fmt.Errorf("session %s not found for project %s: %w", envSessionID, projectID, err)
		}
		return id, nil
	}

	// Find most recent session for the project, filtered by since.
	var id string
	var err error
	if since.IsZero() {
		err = db.QueryRow(
			"SELECT id FROM session WHERE project_id = ? ORDER BY time_created DESC LIMIT 1",
			projectID,
		).Scan(&id)
	} else {
		sinceMilli := since.UnixMilli()
		err = db.QueryRow(
			"SELECT id FROM session WHERE project_id = ? AND time_created >= ? ORDER BY time_created DESC LIMIT 1",
			projectID, sinceMilli,
		).Scan(&id)
	}
	if err != nil {
		return "", fmt.Errorf("no session found for project %s: %w", projectID, err)
	}
	return id, nil
}

// ocFindNewerSession checks if a session newer than currentSessionID exists.
// Returns the new session ID or empty string if none found.
func ocFindNewerSession(_ context.Context, db *sql.DB, projectID, currentSessionID string, since time.Time) (string, error) {
	var currentCreated int64
	err := db.QueryRow(
		"SELECT time_created FROM session WHERE id = ?",
		currentSessionID,
	).Scan(&currentCreated)
	if err != nil {
		return "", err
	}

	var newerID string
	query := "SELECT id FROM session WHERE project_id = ? AND time_created > ? ORDER BY time_created DESC LIMIT 1"
	err = db.QueryRow(query, projectID, currentCreated).Scan(&newerID)
	if err != nil {
		return "", err // sql.ErrNoRows is the normal case (no newer session)
	}

	// If since is set, only switch if the newer session was created after since.
	if !since.IsZero() {
		var newerCreated int64
		_ = db.QueryRow("SELECT time_created FROM session WHERE id = ?", newerID).Scan(&newerCreated)
		if newerCreated < since.UnixMilli() {
			return "", nil
		}
	}

	return newerID, nil
}

// ocPollMessages queries messages created after afterTime (Unix millis) in the
// given session, fetches their parts, and converts everything to AgentEvents.
// Returns the events and the time_created of the last processed message.
func ocPollMessages(ctx context.Context, db *sql.DB, ocSessionID, gtSessionID, agentType string, afterTime int64) ([]AgentEvent, int64, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, data, time_created FROM message WHERE session_id = ? AND time_created > ? ORDER BY time_created ASC",
		ocSessionID, afterTime,
	)
	if err != nil {
		return nil, afterTime, fmt.Errorf("querying messages: %w", err)
	}
	defer rows.Close()

	var allEvents []AgentEvent
	var latestTime int64

	for rows.Next() {
		var msgID, rawData string
		var timeCreated int64
		if err := rows.Scan(&msgID, &rawData, &timeCreated); err != nil {
			continue
		}

		// Parse message metadata.
		var msgData ocMessageData
		if err := json.Unmarshal([]byte(rawData), &msgData); err != nil {
			continue
		}

		// Fetch parts for this message.
		parts, err := ocFetchParts(ctx, db, msgID)
		if err != nil {
			continue
		}

		// Convert to AgentEvents.
		ts := time.UnixMilli(timeCreated)
		events := ocParseMessage(msgData, parts, gtSessionID, agentType, ocSessionID, ts)
		allEvents = append(allEvents, events...)
		latestTime = timeCreated
	}

	return allEvents, latestTime, rows.Err()
}

// ocFetchParts retrieves all part records for a message, ordered by creation time.
func ocFetchParts(ctx context.Context, db *sql.DB, messageID string) ([]ocPartData, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT data FROM part WHERE message_id = ? ORDER BY time_created ASC",
		messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parts []ocPartData
	for rows.Next() {
		var rawData string
		if err := rows.Scan(&rawData); err != nil {
			continue
		}
		var part ocPartData
		if err := json.Unmarshal([]byte(rawData), &part); err != nil {
			continue
		}
		parts = append(parts, part)
	}
	return parts, rows.Err()
}

// ── JSON structures for OpenCode's SQLite data ───────────────────────────────

// ocMessageData represents the JSON stored in message.data.
type ocMessageData struct {
	Role string `json:"role"` // "assistant" or "user"
	Time struct {
		Created   float64 `json:"created"`   // Unix milliseconds
		Completed float64 `json:"completed"` // Unix milliseconds (assistant only)
	} `json:"time"`
	ModelID    string    `json:"modelID"`    // e.g. "claude-sonnet-4-20250514"
	ProviderID string    `json:"providerID"` // e.g. "anthropic"
	Cost       float64   `json:"cost"`
	Tokens     *ocTokens `json:"tokens"` // non-nil for assistant messages
}

// ocTokens holds OpenCode's token usage counts for an assistant turn.
type ocTokens struct {
	Total     int `json:"total"`
	Input     int `json:"input"`
	Output    int `json:"output"`
	Reasoning int `json:"reasoning"`
	Cache     struct {
		Read  int `json:"read"`
		Write int `json:"write"`
	} `json:"cache"`
}

// ocPartData represents the JSON stored in part.data.
// Fields are a superset of all part types; unused fields are zero/empty.
type ocPartData struct {
	Type string `json:"type"` // "text", "tool", "reasoning", "step-start", etc.

	// Text parts: {"type":"text", "text":"..."}
	Text string `json:"text,omitempty"`

	// Tool parts: {"type":"tool", "tool":"name", "callID":"...", "state":{...}}
	Tool   string       `json:"tool,omitempty"`
	CallID string       `json:"callID,omitempty"`
	State  *ocToolState `json:"state,omitempty"`
}

// ocToolState holds the execution state of a tool invocation.
type ocToolState struct {
	Status string          `json:"status"` // "pending", "running", "completed", "error"
	Input  json.RawMessage `json:"input,omitempty"`
	Output string          `json:"output,omitempty"`
	Title  string          `json:"title,omitempty"`
}

// ── Event conversion ─────────────────────────────────────────────────────────

// ocParseMessage converts an OpenCode message + its parts into AgentEvents.
// This mirrors parseClaudeCodeLine: one event per content part, plus a
// dedicated "usage" event for assistant turns with token data.
func ocParseMessage(msg ocMessageData, parts []ocPartData, gtSessionID, agentType, nativeSessionID string, ts time.Time) []AgentEvent {
	var events []AgentEvent

	for _, p := range parts {
		var eventType, content string
		switch p.Type {
		case "text":
			eventType = "text"
			content = p.Text
		case "reasoning":
			eventType = "thinking"
			content = p.Text
		case "tool":
			if p.State != nil && p.State.Status == "completed" {
				// Emit tool_use with the tool name and input.
				eventType = "tool_use"
				content = p.Tool
				if len(p.State.Input) > 0 {
					content += ": " + string(p.State.Input)
				}
				events = append(events, AgentEvent{
					AgentType:       agentType,
					SessionID:       gtSessionID,
					NativeSessionID: nativeSessionID,
					EventType:       eventType,
					Role:            msg.Role,
					Content:         content,
					Timestamp:       ts,
					ModelID:         msg.ModelID,
					ProviderID:      msg.ProviderID,
				})

				// Also emit tool_result if there is output.
				if p.State.Output != "" {
					events = append(events, AgentEvent{
						AgentType:       agentType,
						SessionID:       gtSessionID,
						NativeSessionID: nativeSessionID,
						EventType:       "tool_result",
						Role:            msg.Role,
						Content:         p.State.Output,
						Timestamp:       ts,
						ModelID:         msg.ModelID,
						ProviderID:      msg.ProviderID,
					})
				}
				continue // already appended
			}
			continue // skip non-completed tools
		default:
			continue // skip step-start, step-finish, compaction, etc.
		}

		if content == "" {
			continue
		}
		events = append(events, AgentEvent{
			AgentType:       agentType,
			SessionID:       gtSessionID,
			NativeSessionID: nativeSessionID,
			EventType:       eventType,
			Role:            msg.Role,
			Content:         content,
			Timestamp:       ts,
			ModelID:         msg.ModelID,
			ProviderID:      msg.ProviderID,
		})
	}

	// Emit a dedicated "usage" event for assistant turns with token data,
	// matching the Claude adapter's pattern.
	if msg.Role == "assistant" && msg.Tokens != nil {
		t := msg.Tokens
		if t.Input > 0 || t.Output > 0 || t.Cache.Read > 0 || t.Cache.Write > 0 {
			events = append(events, AgentEvent{
				AgentType:           agentType,
				SessionID:           gtSessionID,
				NativeSessionID:     nativeSessionID,
				EventType:           "usage",
				Role:                "assistant",
				Timestamp:           ts,
				ModelID:             msg.ModelID,
				ProviderID:          msg.ProviderID,
				InputTokens:         t.Input,
				OutputTokens:        t.Output,
				CacheReadTokens:     t.Cache.Read,
				CacheCreationTokens: t.Cache.Write,
			})
		}
	}

	return events
}

// OpenCodeSessionCost returns the total USD cost for an OpenCode session.
//
// Resolution order:
//  1. OPENCODE_SESSION_ID env var → direct DB query (no workDir needed)
//  2. workDir → resolve project → most recent session → sum costs
//
// The cost is read from OpenCode's per-message JSON data ($.cost field)
// which is computed by OpenCode's LLM provider at inference time.
// This is a one-shot function: it opens the DB, queries, and closes.
func OpenCodeSessionCost(workDir string) (float64, error) {
	return openCodeSessionCost("", workDir)
}

// OpenCodeSessionCostFromDB is like OpenCodeSessionCost but accepts an explicit
// DB path. Exported for testing.
func OpenCodeSessionCostFromDB(dbPath, workDir string) (float64, error) {
	return openCodeSessionCost(dbPath, workDir)
}

func openCodeSessionCost(dbPath, workDir string) (float64, error) {
	if dbPath == "" {
		var err error
		dbPath, err = openCodeDBPath()
		if err != nil {
			return 0, err
		}
	}

	db, err := openCodeOpen(dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	sessionID := os.Getenv("OPENCODE_SESSION_ID")
	if sessionID == "" && workDir != "" {
		ctx := context.Background()
		projectID, err := ocFindProject(ctx, db, workDir)
		if err != nil {
			return 0, fmt.Errorf("resolving project for %s: %w", workDir, err)
		}
		sessionID, err = ocFindSession(ctx, db, projectID, "", time.Time{})
		if err != nil {
			return 0, fmt.Errorf("resolving session: %w", err)
		}
	}
	if sessionID == "" {
		return 0, fmt.Errorf("no session: OPENCODE_SESSION_ID not set and workDir empty")
	}

	var cost float64
	err = db.QueryRow(
		"SELECT COALESCE(SUM(json_extract(data, '$.cost')), 0) FROM message WHERE session_id = ?",
		sessionID,
	).Scan(&cost)
	if err != nil {
		return 0, fmt.Errorf("querying session cost: %w", err)
	}
	return cost, nil
}
