package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/git"
)

func TestNewSparseCheckoutCheck(t *testing.T) {
	check := NewSparseCheckoutCheck()

	if check.Name() != "sparse-checkout" {
		t.Errorf("expected name 'sparse-checkout', got %q", check.Name())
	}

	if !check.CanFix() {
		t.Error("expected CanFix to return true")
	}
}

func TestSparseCheckoutCheck_NoRigSpecified(t *testing.T) {
	tmpDir := t.TempDir()

	check := NewSparseCheckoutCheck()
	ctx := &CheckContext{TownRoot: tmpDir, RigName: ""}

	result := check.Run(ctx)

	if result.Status != StatusError {
		t.Errorf("expected StatusError when no rig specified, got %v", result.Status)
	}
	if !strings.Contains(result.Message, "No rig specified") {
		t.Errorf("expected message about no rig, got %q", result.Message)
	}
}

func TestSparseCheckoutCheck_NoGitRepos(t *testing.T) {
	tmpDir := t.TempDir()
	rigName := "testrig"
	rigDir := filepath.Join(tmpDir, rigName)
	if err := os.MkdirAll(rigDir, 0755); err != nil {
		t.Fatal(err)
	}

	check := NewSparseCheckoutCheck()
	ctx := &CheckContext{TownRoot: tmpDir, RigName: rigName}

	result := check.Run(ctx)

	// No git repos found = StatusOK (nothing to check)
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK when no git repos, got %v", result.Status)
	}
}

// initGitRepo creates a minimal git repo with an initial commit.
func initGitRepo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = path
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	// Configure user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = path
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config email failed: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = path
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config name failed: %v\n%s", err, out)
	}

	// Create initial commit
	readmePath := filepath.Join(path, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = path
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = path
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}
}

func TestSparseCheckoutCheck_MayorRigMissingSparseCheckout(t *testing.T) {
	tmpDir := t.TempDir()
	rigName := "testrig"
	rigDir := filepath.Join(tmpDir, rigName)

	// Create mayor/rig as a git repo without sparse checkout
	mayorRig := filepath.Join(rigDir, "mayor", "rig")
	initGitRepo(t, mayorRig)

	check := NewSparseCheckoutCheck()
	ctx := &CheckContext{TownRoot: tmpDir, RigName: rigName}

	result := check.Run(ctx)

	if result.Status != StatusError {
		t.Errorf("expected StatusError for missing sparse checkout, got %v", result.Status)
	}
	if !strings.Contains(result.Message, "1 repo(s) missing") {
		t.Errorf("expected message about missing config, got %q", result.Message)
	}
	if len(result.Details) != 1 || !strings.Contains(result.Details[0], "mayor/rig") {
		t.Errorf("expected details to contain mayor/rig, got %v", result.Details)
	}
}

func TestSparseCheckoutCheck_MayorRigConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	rigName := "testrig"
	rigDir := filepath.Join(tmpDir, rigName)

	// Create mayor/rig as a git repo with sparse checkout configured
	mayorRig := filepath.Join(rigDir, "mayor", "rig")
	initGitRepo(t, mayorRig)
	if err := git.ConfigureSparseCheckout(mayorRig); err != nil {
		t.Fatalf("ConfigureSparseCheckout failed: %v", err)
	}

	check := NewSparseCheckoutCheck()
	ctx := &CheckContext{TownRoot: tmpDir, RigName: rigName}

	result := check.Run(ctx)

	if result.Status != StatusOK {
		t.Errorf("expected StatusOK when sparse checkout configured, got %v", result.Status)
	}
}

func TestSparseCheckoutCheck_CrewMissingSparseCheckout(t *testing.T) {
	tmpDir := t.TempDir()
	rigName := "testrig"
	rigDir := filepath.Join(tmpDir, rigName)

	// Create crew/agent1 as a git repo without sparse checkout
	crewAgent := filepath.Join(rigDir, "crew", "agent1")
	initGitRepo(t, crewAgent)

	check := NewSparseCheckoutCheck()
	ctx := &CheckContext{TownRoot: tmpDir, RigName: rigName}

	result := check.Run(ctx)

	if result.Status != StatusError {
		t.Errorf("expected StatusError for missing sparse checkout, got %v", result.Status)
	}
	if len(result.Details) != 1 || !strings.Contains(result.Details[0], "crew/agent1") {
		t.Errorf("expected details to contain crew/agent1, got %v", result.Details)
	}
}

func TestSparseCheckoutCheck_PolecatMissingSparseCheckout(t *testing.T) {
	tmpDir := t.TempDir()
	rigName := "testrig"
	rigDir := filepath.Join(tmpDir, rigName)

	// Create polecats/pc1 as a git repo without sparse checkout
	polecat := filepath.Join(rigDir, "polecats", "pc1")
	initGitRepo(t, polecat)

	check := NewSparseCheckoutCheck()
	ctx := &CheckContext{TownRoot: tmpDir, RigName: rigName}

	result := check.Run(ctx)

	if result.Status != StatusError {
		t.Errorf("expected StatusError for missing sparse checkout, got %v", result.Status)
	}
	if len(result.Details) != 1 || !strings.Contains(result.Details[0], "polecats/pc1") {
		t.Errorf("expected details to contain polecats/pc1, got %v", result.Details)
	}
}

func TestSparseCheckoutCheck_MultipleReposMissing(t *testing.T) {
	tmpDir := t.TempDir()
	rigName := "testrig"
	rigDir := filepath.Join(tmpDir, rigName)

	// Create multiple git repos without sparse checkout
	initGitRepo(t, filepath.Join(rigDir, "mayor", "rig"))
	initGitRepo(t, filepath.Join(rigDir, "crew", "agent1"))
	initGitRepo(t, filepath.Join(rigDir, "polecats", "pc1"))

	check := NewSparseCheckoutCheck()
	ctx := &CheckContext{TownRoot: tmpDir, RigName: rigName}

	result := check.Run(ctx)

	if result.Status != StatusError {
		t.Errorf("expected StatusError for missing sparse checkout, got %v", result.Status)
	}
	if !strings.Contains(result.Message, "3 repo(s) missing") {
		t.Errorf("expected message about 3 missing repos, got %q", result.Message)
	}
	if len(result.Details) != 3 {
		t.Errorf("expected 3 details, got %d", len(result.Details))
	}
}

func TestSparseCheckoutCheck_MixedConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	rigName := "testrig"
	rigDir := filepath.Join(tmpDir, rigName)

	// Create mayor/rig with sparse checkout configured
	mayorRig := filepath.Join(rigDir, "mayor", "rig")
	initGitRepo(t, mayorRig)
	if err := git.ConfigureSparseCheckout(mayorRig); err != nil {
		t.Fatalf("ConfigureSparseCheckout failed: %v", err)
	}

	// Create crew/agent1 WITHOUT sparse checkout
	crewAgent := filepath.Join(rigDir, "crew", "agent1")
	initGitRepo(t, crewAgent)

	check := NewSparseCheckoutCheck()
	ctx := &CheckContext{TownRoot: tmpDir, RigName: rigName}

	result := check.Run(ctx)

	if result.Status != StatusError {
		t.Errorf("expected StatusError for missing sparse checkout, got %v", result.Status)
	}
	if !strings.Contains(result.Message, "1 repo(s) missing") {
		t.Errorf("expected message about 1 missing repo, got %q", result.Message)
	}
	if len(result.Details) != 1 || !strings.Contains(result.Details[0], "crew/agent1") {
		t.Errorf("expected details to contain only crew/agent1, got %v", result.Details)
	}
}

func TestSparseCheckoutCheck_Fix(t *testing.T) {
	tmpDir := t.TempDir()
	rigName := "testrig"
	rigDir := filepath.Join(tmpDir, rigName)

	// Create git repos without sparse checkout
	mayorRig := filepath.Join(rigDir, "mayor", "rig")
	initGitRepo(t, mayorRig)
	crewAgent := filepath.Join(rigDir, "crew", "agent1")
	initGitRepo(t, crewAgent)

	check := NewSparseCheckoutCheck()
	ctx := &CheckContext{TownRoot: tmpDir, RigName: rigName}

	// Verify fix is needed
	result := check.Run(ctx)
	if result.Status != StatusError {
		t.Fatalf("expected StatusError before fix, got %v", result.Status)
	}

	// Apply fix
	if err := check.Fix(ctx); err != nil {
		t.Fatalf("Fix failed: %v", err)
	}

	// Verify sparse checkout is now configured
	if !git.IsSparseCheckoutConfigured(mayorRig) {
		t.Error("expected sparse checkout to be configured for mayor/rig")
	}
	if !git.IsSparseCheckoutConfigured(crewAgent) {
		t.Error("expected sparse checkout to be configured for crew/agent1")
	}

	// Verify check now passes
	result = check.Run(ctx)
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK after fix, got %v", result.Status)
	}
}

func TestSparseCheckoutCheck_FixNoOp(t *testing.T) {
	tmpDir := t.TempDir()
	rigName := "testrig"
	rigDir := filepath.Join(tmpDir, rigName)

	// Create git repo with sparse checkout already configured
	mayorRig := filepath.Join(rigDir, "mayor", "rig")
	initGitRepo(t, mayorRig)
	if err := git.ConfigureSparseCheckout(mayorRig); err != nil {
		t.Fatalf("ConfigureSparseCheckout failed: %v", err)
	}

	check := NewSparseCheckoutCheck()
	ctx := &CheckContext{TownRoot: tmpDir, RigName: rigName}

	// Run check to populate state
	result := check.Run(ctx)
	if result.Status != StatusOK {
		t.Fatalf("expected StatusOK, got %v", result.Status)
	}

	// Fix should be a no-op (no affected repos)
	if err := check.Fix(ctx); err != nil {
		t.Fatalf("Fix failed: %v", err)
	}

	// Still OK
	result = check.Run(ctx)
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK after no-op fix, got %v", result.Status)
	}
}

func TestSparseCheckoutCheck_NonGitDirSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	rigName := "testrig"
	rigDir := filepath.Join(tmpDir, rigName)

	// Create non-git directories (should be skipped)
	if err := os.MkdirAll(filepath.Join(rigDir, "mayor", "rig"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(rigDir, "crew", "agent1"), 0755); err != nil {
		t.Fatal(err)
	}

	check := NewSparseCheckoutCheck()
	ctx := &CheckContext{TownRoot: tmpDir, RigName: rigName}

	result := check.Run(ctx)

	// Non-git dirs are skipped, so StatusOK
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK when no git repos, got %v", result.Status)
	}
}
