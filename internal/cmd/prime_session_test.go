package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestReadHookSessionID_EnvTakesPriority verifies GT_SESSION_ID env var is
// returned without touching stdin or persisted files.
func TestReadHookSessionID_EnvTakesPriority(t *testing.T) {
	want := "env-session-abc123"
	t.Setenv("GT_SESSION_ID", want)
	t.Setenv("CLAUDE_SESSION_ID", "should-not-use-this")

	id, _ := readHookSessionID()
	if id != want {
		t.Errorf("readHookSessionID() = %q, want %q", id, want)
	}
}

// TestReadHookSessionID_ClaudeSessionIDFallback verifies CLAUDE_SESSION_ID
// is used when GT_SESSION_ID is unset.
func TestReadHookSessionID_ClaudeSessionIDFallback(t *testing.T) {
	want := "claude-session-xyz"
	t.Setenv("GT_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", want)

	id, _ := readHookSessionID()
	if id != want {
		t.Errorf("readHookSessionID() = %q, want %q", id, want)
	}
}

// TestReadHookSessionID_PersistedFileFallback verifies the persisted
// .runtime/session_id file is used when env vars are unset.
func TestReadHookSessionID_PersistedFileFallback(t *testing.T) {
	want := "persisted-session-456"
	t.Setenv("GT_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "")

	// Write a persisted session file in cwd (ReadPersistedSessionID checks cwd first)
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, ".runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := fmt.Sprintf("%s\n%s\n", want, time.Now().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(runtimeDir, "session_id"), []byte(content), 0644); err != nil {
		t.Fatalf("write session_id: %v", err)
	}

	// Change to the temp dir so ReadPersistedSessionID finds it via cwd
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	id, _ := readHookSessionID()
	if id != want {
		t.Errorf("readHookSessionID() = %q, want %q", id, want)
	}
}

// TestReadHookSessionID_SourceFromEnv verifies GT_HOOK_SOURCE env var
// populates the source return value.
func TestReadHookSessionID_SourceFromEnv(t *testing.T) {
	t.Setenv("GT_SESSION_ID", "some-id")
	t.Setenv("GT_HOOK_SOURCE", "compact")

	_, source := readHookSessionID()
	if source != "compact" {
		t.Errorf("source = %q, want %q", source, "compact")
	}
}

// TestReadHookSessionID_PresetSessionIDEnv verifies OPENCODE_SESSION_ID is used
// when GT_AGENT=opencode and GT_SESSION_ID is unset.
func TestReadHookSessionID_PresetSessionIDEnv(t *testing.T) {
	want := "oc-hook-abc"
	t.Setenv("GT_AGENT", "opencode")
	t.Setenv("OPENCODE_SESSION_ID", want)
	t.Setenv("GT_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "")

	id, _ := readHookSessionID()
	if id != want {
		t.Errorf("readHookSessionID() = %q, want %q (OPENCODE_SESSION_ID should be used for opencode agent)", id, want)
	}
}

// TestReadHookSessionID_GTAgentBlocksClaudeFallback verifies that when GT_AGENT=opencode,
// CLAUDE_SESSION_ID is NOT used even as a fallback. The function should auto-generate a UUID.
func TestReadHookSessionID_GTAgentBlocksClaudeFallback(t *testing.T) {
	t.Setenv("GT_AGENT", "opencode")
	t.Setenv("CLAUDE_SESSION_ID", "claude-should-not-use")
	t.Setenv("GT_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "")

	// Change to a temp dir without .runtime/session_id so persisted file doesn't interfere
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	id, _ := readHookSessionID()
	if id == "claude-should-not-use" {
		t.Error("readHookSessionID() returned CLAUDE_SESSION_ID but GT_AGENT=opencode — cross-agent contamination")
	}
	// Should be an auto-generated UUID (36 chars)
	if len(id) != 36 {
		t.Errorf("auto-generated id %q doesn't look like a UUID (len=%d), want 36", id, len(id))
	}
}

// TestReadHookSessionID_GTSessionIDOverridesPreset verifies GT_SESSION_ID takes priority
// over the agent preset's SessionIDEnv.
func TestReadHookSessionID_GTSessionIDOverridesPreset(t *testing.T) {
	want := "direct-id"
	t.Setenv("GT_SESSION_ID", want)
	t.Setenv("GT_AGENT", "opencode")
	t.Setenv("OPENCODE_SESSION_ID", "oc-id")
	t.Setenv("CLAUDE_SESSION_ID", "")

	id, _ := readHookSessionID()
	if id != want {
		t.Errorf("readHookSessionID() = %q, want %q (GT_SESSION_ID should override preset)", id, want)
	}
}

// when no env vars, stdin, or persisted file are available.
func TestReadHookSessionID_AutoGeneratesFallback(t *testing.T) {
	t.Setenv("GT_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "")

	// Use a temp dir with no .runtime/session_id
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	id, _ := readHookSessionID()
	if id == "" {
		t.Error("readHookSessionID() returned empty string, want auto-generated UUID")
	}
	// Should look like a UUID (36 chars with hyphens)
	if len(id) != 36 {
		t.Errorf("auto-generated id %q doesn't look like a UUID (len=%d)", id, len(id))
	}
}
