package swarm

import (
	"testing"

	"github.com/steveyegge/gastown/internal/rig"
)

func TestNewManager(t *testing.T) {
	r := &rig.Rig{
		Name: "test-rig",
		Path: "/tmp/test-rig",
	}
	m := NewManager(r)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.rig != r {
		t.Error("Manager rig not set correctly")
	}
	if m.workDir != r.Path {
		t.Errorf("workDir = %q, want %q", m.workDir, r.Path)
	}
}

// Note: Most swarm tests require integration with beads.
// See gt-kc7yj.4 for the E2E integration test.
