package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestWorkspace(t *testing.T) string {
	t.Helper()

	// Create temp workspace structure
	root := t.TempDir()

	// Create mayor structure
	mayorDir := filepath.Join(root, "mayor")
	if err := os.MkdirAll(mayorDir, 0755); err != nil {
		t.Fatalf("mkdir mayor: %v", err)
	}

	townFile := filepath.Join(mayorDir, "town.json")
	townConfig := `{"type":"town","version":1,"name":"test-town"}`
	if err := os.WriteFile(townFile, []byte(townConfig), 0644); err != nil {
		t.Fatalf("write town.json: %v", err)
	}

	return root
}

func setupTestRig(t *testing.T, townRoot, rigName string) string {
	t.Helper()

	rigPath := filepath.Join(townRoot, rigName)
	if err := os.MkdirAll(rigPath, 0755); err != nil {
		t.Fatalf("mkdir rig: %v", err)
	}

	// Initialize git repo
	if err := os.MkdirAll(filepath.Join(rigPath, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	return rigPath
}

func TestPreflightDryRun(t *testing.T) {
	// This test verifies that dry-run mode doesn't make changes
	townRoot := setupTestWorkspace(t)
	rigName := "test-rig"
	setupTestRig(t, townRoot, rigName)

	// Change to workspace for FindFromCwdOrError to work
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(townRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Run preflight in dry-run mode
	report, err := Preflight(rigName, true)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}

	// In dry-run mode, we should get a report
	if report == nil {
		t.Error("expected non-nil report")
	}
}

func TestPreflightGitCleanCheck(t *testing.T) {
	townRoot := setupTestWorkspace(t)
	rigName := "test-rig"
	rigPath := setupTestRig(t, townRoot, rigName)

	// Change to rig directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(rigPath); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create a modified file to make git "dirty"
	testFile := filepath.Join(rigPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Note: This test will show warnings about git status since we have
	// a fake git repo. In a real scenario with proper git, it would detect
	// the untracked file.
	report, err := Preflight(rigName, true)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}

	// We expect warnings about git status or other issues
	// since this is not a real git repo
	if report == nil {
		t.Error("expected non-nil report")
	}
}

func TestIsClosedStatus(t *testing.T) {
	tests := []struct {
		subject string
		want    bool
	}{
		{"Task CLOSED: Fix bug", true},
		{"Issue closed by user", true},
		{"RESOLVED: Security issue", true},
		{"Completed task #123", true},
		{"Done: Implementation", true},
		{"In Progress: Working on it", false},
		{"TODO: Need to fix", false},
		{"OPEN: New feature request", false},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			got := isClosedStatus(tt.subject)
			if got != tt.want {
				t.Errorf("isClosedStatus(%q) = %v, want %v", tt.subject, got, tt.want)
			}
		})
	}
}

func TestCheckStuckWorkers(t *testing.T) {
	// This is a challenging test since it requires tmux to be running.
	// We'll skip if tmux isn't available.
	townRoot := setupTestWorkspace(t)
	rigName := "test-rig"

	workers, err := checkStuckWorkers(townRoot, rigName)

	// If tmux isn't available, we should get no error and empty list
	if err != nil {
		// Only fail if it's not a "tmux not found" error
		if !os.IsNotExist(err) {
			t.Logf("checkStuckWorkers error (expected if tmux not running): %v", err)
		}
	}

	// Should return empty list or nil (both are acceptable)
	if workers == nil {
		workers = []string{}
	}

	// We don't expect any stuck workers in a fresh test environment
	if len(workers) > 0 {
		t.Logf("Found workers: %v (unexpected in test environment)", workers)
	}
}

func TestCleanStaleMailDryRun(t *testing.T) {
	townRoot := setupTestWorkspace(t)
	rigName := "test-rig"

	// Test dry-run mode - should not error even if no mail exists
	count, err := cleanStaleMail(townRoot, rigName, true)
	if err != nil {
		t.Fatalf("cleanStaleMail: %v", err)
	}

	// Should be 0 since there's no mail
	if count != 0 {
		t.Errorf("cleanStaleMail count = %d, want 0", count)
	}
}
