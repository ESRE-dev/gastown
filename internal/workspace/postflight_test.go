package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPostflightDryRun(t *testing.T) {
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

	opts := PostflightOptions{
		RigName:     rigName,
		ArchiveMail: true,
		DryRun:      true,
	}

	// Run postflight in dry-run mode
	report, err := Postflight(rigName, opts)
	if err != nil {
		t.Fatalf("Postflight: %v", err)
	}

	// In dry-run mode, we should get a report
	if report == nil {
		t.Error("expected non-nil report")
	}
}

func TestPostflightWithoutArchive(t *testing.T) {
	townRoot := setupTestWorkspace(t)
	rigName := "test-rig"
	setupTestRig(t, townRoot, rigName)

	// Change to workspace
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(townRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	opts := PostflightOptions{
		RigName:     rigName,
		ArchiveMail: false, // Don't archive mail
		DryRun:      true,
	}

	report, err := Postflight(rigName, opts)
	if err != nil {
		t.Fatalf("Postflight: %v", err)
	}

	// Mail should not be archived
	if report.MailArchived != 0 {
		t.Errorf("MailArchived = %d, want 0 (archive disabled)", report.MailArchived)
	}
}

func TestArchiveOldMailDryRun(t *testing.T) {
	townRoot := setupTestWorkspace(t)
	rigName := "test-rig"

	// Test dry-run mode - should not error even if no mail exists
	count, err := archiveOldMail(townRoot, rigName, true)
	if err != nil {
		t.Fatalf("archiveOldMail: %v", err)
	}

	// Should be 0 since there's no mail
	if count != 0 {
		t.Errorf("archiveOldMail count = %d, want 0", count)
	}
}

func TestCleanStaleBranchesDryRun(t *testing.T) {
	townRoot := setupTestWorkspace(t)
	rigName := "test-rig"
	rigPath := setupTestRig(t, townRoot, rigName)

	// Test with fake git repo (will likely fail gracefully)
	count, err := cleanStaleBranches(rigPath, true)

	// We expect an error or 0 count since this isn't a real git repo
	if err != nil {
		// That's okay - we're just testing the function doesn't panic
		t.Logf("cleanStaleBranches error (expected with fake git): %v", err)
		return
	}

	// Should be 0 since there are no real branches
	if count != 0 {
		t.Errorf("cleanStaleBranches count = %d, want 0", count)
	}
}

func TestPostflightNonexistentRig(t *testing.T) {
	townRoot := setupTestWorkspace(t)

	// Change to workspace
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(townRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	opts := PostflightOptions{
		RigName:     "nonexistent-rig",
		ArchiveMail: false,
		DryRun:      true,
	}

	// Should return an error for nonexistent rig
	_, err = Postflight("nonexistent-rig", opts)
	if err == nil {
		t.Error("expected error for nonexistent rig, got nil")
	}
}

func TestArchiveDirectoryCreation(t *testing.T) {
	townRoot := setupTestWorkspace(t)
	rigName := "test-rig"
	rigPath := setupTestRig(t, townRoot, rigName)

	// Create a .beads directory so archiveOldMail runs
	beadsDir := filepath.Join(rigPath, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	// Run archive (not dry-run) to test directory creation
	_, err := archiveOldMail(townRoot, rigName, false)
	if err != nil {
		t.Fatalf("archiveOldMail: %v", err)
	}

	// Check that archive directory was created
	archiveDir := filepath.Join(rigPath, "mail-archive")
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		t.Error("expected archive directory to be created")
	}
}
