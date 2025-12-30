package swarm

import (
	"testing"

	"github.com/steveyegge/gastown/internal/rig"
)

func TestManagerCreate(t *testing.T) {
	r := &rig.Rig{
		Name: "test-rig",
		Path: "/tmp/test-rig",
	}
	m := NewManager(r)

	swarm, err := m.Create("epic-1", []string{"Toast", "Nux"}, "main")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if swarm.ID != "epic-1" {
		t.Errorf("ID = %q, want %q", swarm.ID, "epic-1")
	}
	if swarm.State != SwarmCreated {
		t.Errorf("State = %q, want %q", swarm.State, SwarmCreated)
	}
	if len(swarm.Workers) != 2 {
		t.Errorf("Workers = %d, want 2", len(swarm.Workers))
	}
}

func TestManagerCreateDuplicate(t *testing.T) {
	r := &rig.Rig{
		Name: "test-rig",
		Path: "/tmp/test-rig",
	}
	m := NewManager(r)

	_, err := m.Create("epic-1", []string{"Toast"}, "main")
	if err != nil {
		t.Fatalf("First Create failed: %v", err)
	}

	_, err = m.Create("epic-1", []string{"Nux"}, "main")
	if err != ErrSwarmExists {
		t.Errorf("Create duplicate = %v, want ErrSwarmExists", err)
	}
}

func TestManagerStateTransitions(t *testing.T) {
	r := &rig.Rig{
		Name: "test-rig",
		Path: "/tmp/test-rig",
	}
	m := NewManager(r)

	swarm, _ := m.Create("epic-1", []string{"Toast"}, "main")

	// Start
	if err := m.Start(swarm.ID); err != nil {
		t.Errorf("Start failed: %v", err)
	}
	s, _ := m.GetSwarm(swarm.ID)
	if s.State != SwarmActive {
		t.Errorf("State after Start = %q, want %q", s.State, SwarmActive)
	}

	// Can't start again
	if err := m.Start(swarm.ID); err == nil {
		t.Error("Start from Active should fail")
	}

	// Transition to Merging
	if err := m.UpdateState(swarm.ID, SwarmMerging); err != nil {
		t.Errorf("UpdateState to Merging failed: %v", err)
	}

	// Transition to Landed
	if err := m.UpdateState(swarm.ID, SwarmLanded); err != nil {
		t.Errorf("UpdateState to Landed failed: %v", err)
	}

	// Can't transition from terminal
	if err := m.UpdateState(swarm.ID, SwarmActive); err == nil {
		t.Error("UpdateState from Landed should fail")
	}
}

func TestManagerCancel(t *testing.T) {
	r := &rig.Rig{
		Name: "test-rig",
		Path: "/tmp/test-rig",
	}
	m := NewManager(r)

	swarm, _ := m.Create("epic-1", []string{"Toast"}, "main")
	_ = m.Start(swarm.ID)

	if err := m.Cancel(swarm.ID, "user requested"); err != nil {
		t.Errorf("Cancel failed: %v", err)
	}

	s, _ := m.GetSwarm(swarm.ID)
	if s.State != SwarmCancelled {
		t.Errorf("State after Cancel = %q, want %q", s.State, SwarmCancelled)
	}
	if s.Error != "user requested" {
		t.Errorf("Error = %q, want %q", s.Error, "user requested")
	}
}

func TestManagerTaskOperations(t *testing.T) {
	r := &rig.Rig{
		Name: "test-rig",
		Path: "/tmp/test-rig",
	}
	m := NewManager(r)

	swarm, _ := m.Create("epic-1", []string{"Toast"}, "main")

	// Manually add tasks (normally loaded from beads)
	swarm.Tasks = []SwarmTask{
		{IssueID: "task-1", Title: "Task 1", State: TaskPending},
		{IssueID: "task-2", Title: "Task 2", State: TaskPending},
	}

	// Get ready tasks
	ready, err := m.GetReadyTasks(swarm.ID)
	if err != nil {
		t.Errorf("GetReadyTasks failed: %v", err)
	}
	if len(ready) != 2 {
		t.Errorf("GetReadyTasks = %d, want 2", len(ready))
	}

	// Assign task
	if err := m.AssignTask(swarm.ID, "task-1", "Toast"); err != nil {
		t.Errorf("AssignTask failed: %v", err)
	}

	// Check assignment
	s, _ := m.GetSwarm(swarm.ID)
	if s.Tasks[0].Assignee != "Toast" {
		t.Errorf("Assignee = %q, want %q", s.Tasks[0].Assignee, "Toast")
	}
	if s.Tasks[0].State != TaskAssigned {
		t.Errorf("State = %q, want %q", s.Tasks[0].State, TaskAssigned)
	}

	// Update state
	if err := m.UpdateTaskState(swarm.ID, "task-1", TaskMerged); err != nil {
		t.Errorf("UpdateTaskState failed: %v", err)
	}
	s, _ = m.GetSwarm(swarm.ID)
	if s.Tasks[0].State != TaskMerged {
		t.Errorf("State = %q, want %q", s.Tasks[0].State, TaskMerged)
	}
	if s.Tasks[0].MergedAt == nil {
		t.Error("MergedAt should be set")
	}
}

func TestManagerIsComplete(t *testing.T) {
	r := &rig.Rig{
		Name: "test-rig",
		Path: "/tmp/test-rig",
	}
	m := NewManager(r)

	swarm, _ := m.Create("epic-1", []string{"Toast"}, "main")
	swarm.Tasks = []SwarmTask{
		{IssueID: "task-1", State: TaskPending},
		{IssueID: "task-2", State: TaskMerged},
	}

	complete, _ := m.IsComplete(swarm.ID)
	if complete {
		t.Error("IsComplete should be false with pending task")
	}

	// Complete the pending task
	_ = m.UpdateTaskState(swarm.ID, "task-1", TaskMerged)
	complete, _ = m.IsComplete(swarm.ID)
	if !complete {
		t.Error("IsComplete should be true when all tasks merged")
	}
}

// TestSwarmE2ELifecycle documents the end-to-end swarm integration test protocol.
// This test documents the manual testing steps that were validated for gt-kc7yj.4.
//
// The test scenario creates a DAG of work:
//
//	     A
//	    / \
//	   B   C
//	    \ /
//	     D
//
// Test Results (verified 2025-12-29):
//
// 1. CREATE EPIC WITH DEPENDENCIES
//
//	bd create --type=epic --title="Test Epic"         → gt-xxxxx
//	bd create --type=task --title="Task A" --parent=gt-xxxxx  → gt-xxxxx.1
//	bd create --type=task --title="Task B" --parent=gt-xxxxx  → gt-xxxxx.2
//	bd create --type=task --title="Task C" --parent=gt-xxxxx  → gt-xxxxx.3
//	bd create --type=task --title="Task D" --parent=gt-xxxxx  → gt-xxxxx.4
//	bd dep add gt-xxxxx.2 gt-xxxxx.1  # B depends on A
//	bd dep add gt-xxxxx.3 gt-xxxxx.1  # C depends on A
//	bd dep add gt-xxxxx.4 gt-xxxxx.2  # D depends on B
//	bd dep add gt-xxxxx.4 gt-xxxxx.3  # D depends on C
//
// 2. VALIDATE SWARM STRUCTURE ✅
//
//	bd swarm validate gt-xxxxx
//	Expected output:
//	  Wave 1: 1 issue (Task A)
//	  Wave 2: 2 issues (Tasks B, C - parallel)
//	  Wave 3: 1 issue (Task D)
//	  Max parallelism: 2
//	  Swarmable: YES
//
// 3. CREATE SWARM MOLECULE ✅
//
//	bd swarm create gt-xxxxx
//	Expected: Creates molecule with mol_type=swarm linked to epic
//
// 4. VERIFY READY FRONT ✅
//
//	bd swarm status gt-xxxxx
//	Expected:
//	  Ready: Task A
//	  Blocked: Tasks B, C, D (with dependency info)
//
// 5. ISSUE COMPLETION ADVANCES FRONT ✅
//
//	bd close gt-xxxxx.1 --reason "Complete"
//	bd swarm status gt-xxxxx
//	Expected:
//	  Completed: Task A
//	  Ready: Tasks B, C (now unblocked)
//	  Blocked: Task D
//
// 6. PARALLEL WORK ✅
//
//	bd close gt-xxxxx.2 gt-xxxxx.3 --reason "Complete"
//	bd swarm status gt-xxxxx
//	Expected:
//	  Completed: Tasks A, B, C
//	  Ready: Task D (now unblocked)
//
// 7. FINAL COMPLETION ✅
//
//	bd close gt-xxxxx.4 --reason "Complete"
//	bd swarm status gt-xxxxx
//	Expected: Progress 4/4 complete (100%)
//
// 8. SWARM AUTO-CLOSE ⚠️
//
//	The swarm and epic remain open after all tasks complete.
//	This is by design - the Witness coordinator is responsible for
//	detecting completion and closing the swarm molecule.
//	Manual close: bd close gt-xxxxx gt-yyyyy --reason "Swarm complete"
//
// KNOWN ISSUES:
//   - gt swarm status/land fail to find issues (filed as gt-594a4)
//   - bd swarm commands work correctly as the underlying implementation
//   - Auto-close requires Witness patrol (not automatic in beads)
func TestSwarmE2ELifecycle(t *testing.T) {
	// This test documents the manual E2E testing protocol.
	// The actual test requires beads infrastructure and is run manually.
	// See the docstring above for the complete test procedure.
	t.Skip("E2E test requires beads infrastructure - see docstring for manual test protocol")
}
