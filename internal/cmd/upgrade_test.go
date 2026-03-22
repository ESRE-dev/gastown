package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateCLAUDEMD(t *testing.T) {
	content := generateCLAUDEMD()

	// Must contain the Gas Town header
	if content == "" {
		t.Fatal("generateCLAUDEMD returned empty string")
	}
	if content[0:10] != "# Gas Town" {
		t.Errorf("expected content to start with '# Gas Town', got: %q", content[:10])
	}

	// Must contain identity anchoring instructions
	if !contains(content, "Do NOT adopt an identity") {
		t.Error("CLAUDE.md should contain identity anchoring warning")
	}
	if !contains(content, "GT_ROLE") {
		t.Error("CLAUDE.md should reference GT_ROLE environment variable")
	}
}

func TestUpgradeCLAUDEMD_CreatesMissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with a "town root" that has no CLAUDE.md
	upgradeDryRun = false
	upgradeVerbose = false

	result := upgradeCLAUDEMD(tmpDir)

	// 2 changes: CLAUDE.md created + AGENTS.md symlink created
	if result.changed != 2 {
		t.Errorf("expected 2 changes for new CLAUDE.md + AGENTS.md, got %d", result.changed)
	}

	// Verify file was created
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	data, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("CLAUDE.md not created: %v", err)
	}

	expected := generateCLAUDEMD()
	if string(data) != expected {
		t.Error("CLAUDE.md content doesn't match expected template")
	}

	// Verify AGENTS.md was created as a real file with same content
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md not created: %v", err)
	}
	if string(agentsData) != expected {
		t.Error("AGENTS.md content doesn't match CLAUDE.md template")
	}
	// Verify it's not a symlink
	fi, err := os.Lstat(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md Lstat failed: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("AGENTS.md should be a real file, not a symlink")
	}
}

func TestUpgradeCLAUDEMD_UpToDate(t *testing.T) {
	tmpDir := t.TempDir()

	// Write the expected content for both files
	expected := generateCLAUDEMD()
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte(expected), 0644); err != nil {
		t.Fatal(err)
	}
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(expected), 0644); err != nil {
		t.Fatal(err)
	}

	upgradeDryRun = false
	upgradeVerbose = false

	result := upgradeCLAUDEMD(tmpDir)

	if result.changed != 0 {
		t.Errorf("expected 0 changes for up-to-date CLAUDE.md + AGENTS.md, got %d", result.changed)
	}
}

func TestUpgradeCLAUDEMD_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	upgradeDryRun = true
	upgradeVerbose = false

	result := upgradeCLAUDEMD(tmpDir)

	if result.changed != 1 {
		t.Errorf("expected 1 change in dry-run mode, got %d", result.changed)
	}

	// Verify file was NOT created
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	if _, err := os.Stat(claudePath); !os.IsNotExist(err) {
		t.Error("dry-run should not create CLAUDE.md")
	}

	// Reset
	upgradeDryRun = false
}

func TestUpgradeDaemonConfig_CreatesMissing(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mayor directory (required by DaemonPatrolConfigPath)
	mayorDir := filepath.Join(tmpDir, "mayor")
	if err := os.MkdirAll(mayorDir, 0755); err != nil {
		t.Fatal(err)
	}

	upgradeDryRun = false
	upgradeVerbose = false

	result := upgradeDaemonConfig(tmpDir)

	if result.changed != 1 {
		t.Errorf("expected 1 change for new daemon.json, got %d", result.changed)
	}

	// Verify file exists
	daemonPath := filepath.Join(mayorDir, "daemon.json")
	if _, err := os.Stat(daemonPath); err != nil {
		t.Errorf("daemon.json not created: %v", err)
	}
}

func TestUpgradeDaemonConfig_ExistingValid(t *testing.T) {
	tmpDir := t.TempDir()
	mayorDir := filepath.Join(tmpDir, "mayor")
	if err := os.MkdirAll(mayorDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a valid daemon.json
	daemonPath := filepath.Join(mayorDir, "daemon.json")
	content := `{
		"type": "daemon-patrol-config",
		"version": 1,
		"heartbeat": {"enabled": true, "interval": "3m"},
		"patrols": {}
	}`
	if err := os.WriteFile(daemonPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	upgradeDryRun = false
	upgradeVerbose = false

	result := upgradeDaemonConfig(tmpDir)

	if result.changed != 0 {
		t.Errorf("expected 0 changes for existing daemon.json, got %d", result.changed)
	}
}

func TestUpgradeCommandRegistered(t *testing.T) {
	// Verify the upgrade command is registered in rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "upgrade" {
			found = true
			break
		}
	}
	if !found {
		t.Error("upgrade command not registered with rootCmd")
	}
}

func TestUpgradeBeadsExempt(t *testing.T) {
	if !beadsExemptCommands["upgrade"] {
		t.Error("upgrade should be in beadsExemptCommands")
	}
}

func TestUpgradeBranchCheckExempt(t *testing.T) {
	if !branchCheckExemptCommands["upgrade"] {
		t.Error("upgrade should be in branchCheckExemptCommands")
	}
}

// TestUpgradeCLAUDEMD_MigratesAgentsSymlink verifies that an old AGENTS.md
// symlink is replaced by a real file containing the expected CLAUDE.md content.
func TestUpgradeCLAUDEMD_MigratesAgentsSymlink(t *testing.T) {

	tmpDir := t.TempDir()

	// Write CLAUDE.md with stale content so upgrade will rewrite it.
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte("stale content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create AGENTS.md as a symlink pointing to CLAUDE.md.
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.Symlink(claudePath, agentsPath); err != nil {
		t.Fatal(err)
	}

	upgradeDryRun = false
	upgradeVerbose = false
	result := upgradeCLAUDEMD(tmpDir)

	// Expect 2 changes: CLAUDE.md updated + AGENTS.md migrated from symlink.
	if result.changed != 2 {
		t.Errorf("expected 2 changes (CLAUDE.md update + AGENTS.md migrate), got %d", result.changed)
	}

	expected := generateCLAUDEMD()

	// CLAUDE.md must be a real file with expected content.
	claudeData, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("CLAUDE.md unreadable: %v", err)
	}
	if string(claudeData) != expected {
		t.Errorf("CLAUDE.md content mismatch: want %q, got %q", expected, string(claudeData))
	}

	// AGENTS.md must be a real file (not a symlink) with expected content.
	fi, err := os.Lstat(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md Lstat failed: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("AGENTS.md should be a real file after migration, not a symlink")
	}
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md unreadable: %v", err)
	}
	if string(agentsData) != expected {
		t.Errorf("AGENTS.md content mismatch after migrate: want %q, got %q", expected, string(agentsData))
	}
}

// TestUpgradeCLAUDEMD_UpToDate_CreatesAgentsMD verifies that when CLAUDE.md is
// already up-to-date but AGENTS.md is absent, the upgrade creates AGENTS.md.
func TestUpgradeCLAUDEMD_UpToDate_CreatesAgentsMD(t *testing.T) {

	tmpDir := t.TempDir()

	// Write CLAUDE.md with the expected (up-to-date) content but no AGENTS.md.
	expected := generateCLAUDEMD()
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte(expected), 0644); err != nil {
		t.Fatal(err)
	}

	upgradeDryRun = false
	upgradeVerbose = false
	result := upgradeCLAUDEMD(tmpDir)

	// CLAUDE.md was up-to-date (no change for it), but AGENTS.md was created.
	if result.changed != 1 {
		t.Errorf("expected 1 change (AGENTS.md created), got %d", result.changed)
	}

	// AGENTS.md must exist as a real file with expected content.
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	fi, err := os.Lstat(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md not created: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("AGENTS.md should be a real file, not a symlink")
	}
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md unreadable: %v", err)
	}
	if string(agentsData) != expected {
		t.Errorf("AGENTS.md content mismatch: want %q, got %q", expected, string(agentsData))
	}
}

// TestUpgradeCLAUDEMD_DryRun_NoAgentsMDCreated verifies that in dry-run mode
// neither CLAUDE.md nor AGENTS.md is created on disk.
func TestUpgradeCLAUDEMD_DryRun_NoAgentsMDCreated(t *testing.T) {
	tmpDir := t.TempDir()

	upgradeDryRun = true
	upgradeVerbose = false
	defer func() { upgradeDryRun = false }()

	_ = upgradeCLAUDEMD(tmpDir)

	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	if _, err := os.Stat(claudePath); !os.IsNotExist(err) {
		t.Error("dry-run should not create CLAUDE.md on disk")
	}

	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); !os.IsNotExist(err) {
		t.Error("dry-run should not create AGENTS.md on disk")
	}
}

// contains is already declared in mq_test.go in this package,
// so we reuse it here.
