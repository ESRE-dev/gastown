package keepalive

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTouchInWorkspace(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Touch the keepalive
	TouchInWorkspace(tmpDir, "gt status")

	// Read back
	state := Read(tmpDir)
	if state == nil {
		t.Fatal("expected state to be non-nil")
	}

	if state.LastCommand != "gt status" {
		t.Errorf("expected last_command 'gt status', got %q", state.LastCommand)
	}

	// Check timestamp is recent
	if time.Since(state.Timestamp) > time.Minute {
		t.Errorf("timestamp too old: %v", state.Timestamp)
	}
}

func TestReadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	state := Read(tmpDir)
	if state != nil {
		t.Error("expected nil state for non-existent file")
	}
}

func TestStateAge(t *testing.T) {
	// Test nil state
	var nilState *State
	if nilState.Age() < 24*time.Hour {
		t.Error("nil state should have very large age")
	}

	// Test fresh state
	freshState := &State{Timestamp: time.Now().Add(-30 * time.Second)}
	if !freshState.IsFresh() {
		t.Error("30-second-old state should be fresh")
	}
	if freshState.IsStale() {
		t.Error("30-second-old state should not be stale")
	}
	if freshState.IsVeryStale() {
		t.Error("30-second-old state should not be very stale")
	}

	// Test stale state (3 minutes)
	staleState := &State{Timestamp: time.Now().Add(-3 * time.Minute)}
	if staleState.IsFresh() {
		t.Error("3-minute-old state should not be fresh")
	}
	if !staleState.IsStale() {
		t.Error("3-minute-old state should be stale")
	}
	if staleState.IsVeryStale() {
		t.Error("3-minute-old state should not be very stale")
	}

	// Test very stale state (10 minutes)
	veryStaleState := &State{Timestamp: time.Now().Add(-10 * time.Minute)}
	if veryStaleState.IsFresh() {
		t.Error("10-minute-old state should not be fresh")
	}
	if veryStaleState.IsStale() {
		t.Error("10-minute-old state should not be stale (it's very stale)")
	}
	if !veryStaleState.IsVeryStale() {
		t.Error("10-minute-old state should be very stale")
	}
}

func TestDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "some", "nested", "workspace")

	// Touch should create .runtime directory
	TouchInWorkspace(workDir, "gt test")

	// Verify directory was created
	runtimeDir := filepath.Join(workDir, ".runtime")
	if _, err := os.Stat(runtimeDir); os.IsNotExist(err) {
		t.Error("expected .runtime directory to be created")
	}
}
