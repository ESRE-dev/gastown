package rig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/git"
)

func setupTestTown(t *testing.T) (string, *config.RigsConfig) {
	t.Helper()
	root := t.TempDir()

	rigsConfig := &config.RigsConfig{
		Version: 1,
		Rigs:    make(map[string]config.RigEntry),
	}

	return root, rigsConfig
}

func createTestRig(t *testing.T, root, name string) {
	t.Helper()

	rigPath := filepath.Join(root, name)
	if err := os.MkdirAll(rigPath, 0755); err != nil {
		t.Fatalf("mkdir rig: %v", err)
	}

	// Create agent dirs
	for _, dir := range AgentDirs {
		dirPath := filepath.Join(rigPath, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	// Create witness state.json (witnesses don't have clones, just state)
	witnessState := filepath.Join(rigPath, "witness", "state.json")
	if err := os.WriteFile(witnessState, []byte(`{"role":"witness"}`), 0644); err != nil {
		t.Fatalf("write witness state: %v", err)
	}

	// Create some polecats
	polecatsDir := filepath.Join(rigPath, "polecats")
	for _, polecat := range []string{"Toast", "Cheedo"} {
		if err := os.MkdirAll(filepath.Join(polecatsDir, polecat), 0755); err != nil {
			t.Fatalf("mkdir polecat: %v", err)
		}
	}
}

func TestDiscoverRigs(t *testing.T) {
	root, rigsConfig := setupTestTown(t)

	// Create test rig
	createTestRig(t, root, "gastown")
	rigsConfig.Rigs["gastown"] = config.RigEntry{
		GitURL: "git@github.com:test/gastown.git",
	}

	manager := NewManager(root, rigsConfig, git.NewGit(root))

	rigs, err := manager.DiscoverRigs()
	if err != nil {
		t.Fatalf("DiscoverRigs: %v", err)
	}

	if len(rigs) != 1 {
		t.Errorf("rigs count = %d, want 1", len(rigs))
	}

	rig := rigs[0]
	if rig.Name != "gastown" {
		t.Errorf("Name = %q, want gastown", rig.Name)
	}
	if len(rig.Polecats) != 2 {
		t.Errorf("Polecats count = %d, want 2", len(rig.Polecats))
	}
	if !rig.HasWitness {
		t.Error("expected HasWitness = true")
	}
	if !rig.HasRefinery {
		t.Error("expected HasRefinery = true")
	}
}

func TestGetRig(t *testing.T) {
	root, rigsConfig := setupTestTown(t)

	createTestRig(t, root, "test-rig")
	rigsConfig.Rigs["test-rig"] = config.RigEntry{
		GitURL: "git@github.com:test/test-rig.git",
	}

	manager := NewManager(root, rigsConfig, git.NewGit(root))

	rig, err := manager.GetRig("test-rig")
	if err != nil {
		t.Fatalf("GetRig: %v", err)
	}

	if rig.Name != "test-rig" {
		t.Errorf("Name = %q, want test-rig", rig.Name)
	}
}

func TestGetRigNotFound(t *testing.T) {
	root, rigsConfig := setupTestTown(t)
	manager := NewManager(root, rigsConfig, git.NewGit(root))

	_, err := manager.GetRig("nonexistent")
	if err != ErrRigNotFound {
		t.Errorf("GetRig = %v, want ErrRigNotFound", err)
	}
}

func TestRigExists(t *testing.T) {
	root, rigsConfig := setupTestTown(t)
	rigsConfig.Rigs["exists"] = config.RigEntry{}

	manager := NewManager(root, rigsConfig, git.NewGit(root))

	if !manager.RigExists("exists") {
		t.Error("expected RigExists = true for existing rig")
	}
	if manager.RigExists("nonexistent") {
		t.Error("expected RigExists = false for nonexistent rig")
	}
}

func TestRemoveRig(t *testing.T) {
	root, rigsConfig := setupTestTown(t)
	rigsConfig.Rigs["to-remove"] = config.RigEntry{}

	manager := NewManager(root, rigsConfig, git.NewGit(root))

	if err := manager.RemoveRig("to-remove"); err != nil {
		t.Fatalf("RemoveRig: %v", err)
	}

	if manager.RigExists("to-remove") {
		t.Error("rig should not exist after removal")
	}
}

func TestRemoveRigNotFound(t *testing.T) {
	root, rigsConfig := setupTestTown(t)
	manager := NewManager(root, rigsConfig, git.NewGit(root))

	err := manager.RemoveRig("nonexistent")
	if err != ErrRigNotFound {
		t.Errorf("RemoveRig = %v, want ErrRigNotFound", err)
	}
}

func TestListRigNames(t *testing.T) {
	root, rigsConfig := setupTestTown(t)
	rigsConfig.Rigs["rig1"] = config.RigEntry{}
	rigsConfig.Rigs["rig2"] = config.RigEntry{}

	manager := NewManager(root, rigsConfig, git.NewGit(root))

	names := manager.ListRigNames()
	if len(names) != 2 {
		t.Errorf("names count = %d, want 2", len(names))
	}
}

func TestRigSummary(t *testing.T) {
	rig := &Rig{
		Name:        "test",
		Polecats:    []string{"a", "b", "c"},
		HasWitness:  true,
		HasRefinery: false,
	}

	summary := rig.Summary()

	if summary.Name != "test" {
		t.Errorf("Name = %q, want test", summary.Name)
	}
	if summary.PolecatCount != 3 {
		t.Errorf("PolecatCount = %d, want 3", summary.PolecatCount)
	}
	if !summary.HasWitness {
		t.Error("expected HasWitness = true")
	}
	if summary.HasRefinery {
		t.Error("expected HasRefinery = false")
	}
}
