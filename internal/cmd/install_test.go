package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/doltserver"
)

func TestBuildBdInitArgs_AlwaysIncludesServerPort(t *testing.T) {
	townDir := t.TempDir()
	t.Setenv("GT_DOLT_PORT", "")
	t.Setenv("BEADS_DOLT_PORT", "")

	args := buildBdInitArgs(townDir)

	if len(args) != 6 {
		t.Fatalf("expected 6 args, got %d: %v", len(args), args)
	}
	if args[4] != "--server-port" {
		t.Fatalf("expected args[4] = --server-port, got %q", args[4])
	}
	if args[5] != "3307" {
		t.Fatalf("expected default port 3307, got %q", args[5])
	}
}

func TestBuildBdInitArgs_RespectsGTDoltPortEnv(t *testing.T) {
	townDir := t.TempDir()

	t.Setenv("GT_DOLT_PORT", "4400")

	args := buildBdInitArgs(townDir)

	if args[5] != "4400" {
		t.Fatalf("expected port 4400 from GT_DOLT_PORT, got %q", args[5])
	}
}

func TestBuildBdInitArgs_ConfigYAMLTakesPrecedence(t *testing.T) {
	townDir := t.TempDir()
	doltDataDir := filepath.Join(townDir, ".dolt-data")
	if err := os.MkdirAll(doltDataDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configYAML := "listener:\n  host: 127.0.0.1\n  port: 5500\n"
	if err := os.WriteFile(filepath.Join(doltDataDir, "config.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	t.Setenv("GT_DOLT_PORT", "4400")

	args := buildBdInitArgs(townDir)

	if args[5] != "5500" {
		t.Fatalf("expected port 5500 from config.yaml (precedence over env), got %q", args[5])
	}
}

func TestBuildBdInitArgs_PortMatchesDefaultConfig(t *testing.T) {
	townDir := t.TempDir()

	args := buildBdInitArgs(townDir)
	cfg := doltserver.DefaultConfig(townDir)

	if args[5] != strconv.Itoa(cfg.Port) {
		t.Fatalf("port should match DefaultConfig: args=%q, config=%d", args[5], cfg.Port)
	}
}

func TestEnsureBeadsConfigYAML_CreatesWhenMissing(t *testing.T) {
	beadsDir := t.TempDir()

	if err := beads.EnsureConfigYAML(beadsDir, "hq"); err != nil {
		t.Fatalf("EnsureConfigYAML: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(beadsDir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}

	got := string(data)
	want := "prefix: hq\nissue-prefix: hq\ndolt.idle-timeout: \"0\"\n"
	if got != want {
		t.Fatalf("config.yaml = %q, want %q", got, want)
	}
}

func TestEnsureBeadsConfigYAML_RepairsPrefixKeysAndPreservesOtherLines(t *testing.T) {
	beadsDir := t.TempDir()
	path := filepath.Join(beadsDir, "config.yaml")
	original := strings.Join([]string{
		"# existing settings",
		"prefix: wrong",
		"sync-branch: main",
		"issue-prefix: wrong",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	if err := beads.EnsureConfigYAML(beadsDir, "hq"); err != nil {
		t.Fatalf("EnsureConfigYAML: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "prefix: hq\n") {
		t.Fatalf("config.yaml missing repaired prefix: %q", text)
	}
	if !strings.Contains(text, "issue-prefix: hq\n") {
		t.Fatalf("config.yaml missing repaired issue-prefix: %q", text)
	}
	if !strings.Contains(text, "sync-branch: main\n") {
		t.Fatalf("config.yaml should preserve unrelated settings: %q", text)
	}
}

func TestEnsureBeadsConfigYAML_AddsMissingIssuePrefixKey(t *testing.T) {
	beadsDir := t.TempDir()
	path := filepath.Join(beadsDir, "config.yaml")
	if err := os.WriteFile(path, []byte("prefix: hq\n"), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	if err := beads.EnsureConfigYAML(beadsDir, "hq"); err != nil {
		t.Fatalf("EnsureConfigYAML: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "prefix: hq\n") {
		t.Fatalf("config.yaml missing prefix: %q", text)
	}
	if !strings.Contains(text, "issue-prefix: hq\n") {
		t.Fatalf("config.yaml missing issue-prefix: %q", text)
	}
}

// TestCreateTownRootAgentMDs_CreatesRealFiles verifies that in a fresh directory
// both CLAUDE.md and AGENTS.md are created as regular files with identical content.
func TestCreateTownRootAgentMDs_CreatesRealFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	created, err := createTownRootAgentMDs(tmpDir)
	if err != nil {
		t.Fatalf("createTownRootAgentMDs: %v", err)
	}
	if !created {
		t.Error("expected created=true for a fresh directory")
	}

	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")

	claudeData, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("CLAUDE.md not created: %v", err)
	}
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md not created: %v", err)
	}

	if string(claudeData) != string(agentsData) {
		t.Error("CLAUDE.md and AGENTS.md should have identical content")
	}

	// Both must be real files, not symlinks.
	for _, path := range []string{claudePath, agentsPath} {
		fi, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("Lstat %s: %v", path, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s should be a real file, not a symlink", path)
		}
	}
}

// TestCreateTownRootAgentMDs_MigratesSymlink verifies that an existing AGENTS.md
// symlink is replaced by a real file with the same content as CLAUDE.md.
func TestCreateTownRootAgentMDs_MigratesSymlink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Pre-create CLAUDE.md so the function won't create it anew.
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	existingContent := "existing claude content\n"
	if err := os.WriteFile(claudePath, []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create AGENTS.md as a symlink pointing to CLAUDE.md (old behaviour).
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.Symlink(claudePath, agentsPath); err != nil {
		t.Fatal(err)
	}

	created, err := createTownRootAgentMDs(tmpDir)
	if err != nil {
		t.Fatalf("createTownRootAgentMDs: %v", err)
	}
	if !created {
		t.Error("expected created=true when migrating symlink to real file")
	}

	// AGENTS.md must now be a real file, not a symlink.
	fi, err := os.Lstat(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md Lstat: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("AGENTS.md should be a real file after symlink migration")
	}

	// Content should match what the function writes (the embedded template content,
	// not the pre-existing CLAUDE.md content which was set before the function ran).
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	if len(agentsData) == 0 {
		t.Error("AGENTS.md is empty after migration")
	}
}

// TestCreateTownRootAgentMDs_PreservesExistingRealFile verifies that when both
// CLAUDE.md and AGENTS.md already exist as real files they are not overwritten.
func TestCreateTownRootAgentMDs_PreservesExistingRealFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")

	claudeContent := "custom claude content\n"
	agentsContent := "custom agents content\n"

	if err := os.WriteFile(claudePath, []byte(claudeContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agentsPath, []byte(agentsContent), 0644); err != nil {
		t.Fatal(err)
	}

	created, err := createTownRootAgentMDs(tmpDir)
	if err != nil {
		t.Fatalf("createTownRootAgentMDs: %v", err)
	}
	if created {
		t.Error("expected created=false when both files already exist as real files")
	}

	// Existing content must not be overwritten.
	claudeData, _ := os.ReadFile(claudePath)
	if string(claudeData) != claudeContent {
		t.Errorf("CLAUDE.md was overwritten; got %q, want %q", string(claudeData), claudeContent)
	}
	agentsData, _ := os.ReadFile(agentsPath)
	if string(agentsData) != agentsContent {
		t.Errorf("AGENTS.md was overwritten; got %q, want %q", string(agentsData), agentsContent)
	}
}
