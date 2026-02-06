package monitoring

import (
	"testing"
	"time"
)

func TestStatusTracker_Creation(t *testing.T) {
	tracker := NewStatusTracker("test-agent", 100)

	if tracker.AgentID() != "test-agent" {
		t.Errorf("AgentID() = %q, want %q", tracker.AgentID(), "test-agent")
	}

	status, _ := tracker.GetStatus()
	if status != StatusOffline {
		t.Errorf("initial status = %q, want %q", status, StatusOffline)
	}

	if !tracker.IsDetectorActive() {
		t.Error("detector should be active by default")
	}
}

func TestStatusTracker_UpdateStatus(t *testing.T) {
	tracker := NewStatusTracker("test-agent", 100)

	// Update to working
	tracker.UpdateStatus(StatusWorking, SourceSelf, "Starting task", "")

	status, updateTime := tracker.GetStatus()
	if status != StatusWorking {
		t.Errorf("status = %q, want %q", status, StatusWorking)
	}

	if time.Since(updateTime) > time.Second {
		t.Error("update time should be recent")
	}
}

func TestStatusTracker_History(t *testing.T) {
	tracker := NewStatusTracker("test-agent", 5)

	// Add several status updates
	statuses := []AgentStatus{
		StatusWorking,
		StatusThinking,
		StatusBlocked,
		StatusWorking,
		StatusIdle,
	}

	for i, status := range statuses {
		tracker.UpdateStatus(status, SourceInferred, "", "")
		time.Sleep(time.Millisecond) // Ensure timestamps differ

		history := tracker.GetHistory()
		expectedLen := i + 1
		if len(history) != expectedLen {
			t.Errorf("after update %d, history length = %d, want %d", i, len(history), expectedLen)
		}
	}

	// Verify history contents
	history := tracker.GetHistory()
	if len(history) != len(statuses) {
		t.Fatalf("history length = %d, want %d", len(history), len(statuses))
	}

	for i, status := range statuses {
		if history[i].Status != status {
			t.Errorf("history[%d].Status = %q, want %q", i, history[i].Status, status)
		}
	}
}

func TestStatusTracker_HistoryLimit(t *testing.T) {
	maxHistory := 3
	tracker := NewStatusTracker("test-agent", maxHistory)

	// Add more updates than the limit
	for i := 0; i < 10; i++ {
		tracker.UpdateStatus(StatusWorking, SourceInferred, "", "")
	}

	history := tracker.GetHistory()
	if len(history) != maxHistory {
		t.Errorf("history length = %d, want %d (max limit)", len(history), maxHistory)
	}
}

func TestStatusTracker_LastActivity(t *testing.T) {
	tracker := NewStatusTracker("test-agent", 100)

	initialActivity := tracker.GetLastActivity()

	time.Sleep(10 * time.Millisecond)

	// Active status should update last activity
	tracker.UpdateStatus(StatusWorking, SourceSelf, "", "")
	newActivity := tracker.GetLastActivity()

	if !newActivity.After(initialActivity) {
		t.Error("last activity should be updated after active status change")
	}

	time.Sleep(10 * time.Millisecond)

	// Idle status should NOT update last activity
	tracker.UpdateStatus(StatusIdle, SourceInferred, "", "")
	idleActivity := tracker.GetLastActivity()

	if !idleActivity.Equal(newActivity) {
		t.Error("last activity should not change for idle status")
	}

	// Offline status should NOT update last activity
	tracker.UpdateStatus(StatusOffline, SourceBoss, "", "")
	offlineActivity := tracker.GetLastActivity()

	if !offlineActivity.Equal(newActivity) {
		t.Error("last activity should not change for offline status")
	}
}

func TestStatusTracker_DetectorActive(t *testing.T) {
	tracker := NewStatusTracker("test-agent", 100)

	if !tracker.IsDetectorActive() {
		t.Error("detector should be active by default")
	}

	tracker.SetDetectorActive(false)
	if tracker.IsDetectorActive() {
		t.Error("detector should be inactive after SetDetectorActive(false)")
	}

	tracker.SetDetectorActive(true)
	if !tracker.IsDetectorActive() {
		t.Error("detector should be active after SetDetectorActive(true)")
	}
}

func TestStatusTracker_GetStatusReport(t *testing.T) {
	tracker := NewStatusTracker("test-agent", 100)
	tracker.UpdateStatus(StatusWorking, SourceSelf, "Processing", "work_pattern")

	report := tracker.GetStatusReport()

	if report.AgentID != "test-agent" {
		t.Errorf("AgentID = %q, want %q", report.AgentID, "test-agent")
	}
	if report.Status != StatusWorking {
		t.Errorf("Status = %q, want %q", report.Status, StatusWorking)
	}
	if report.Source != SourceSelf {
		t.Errorf("Source = %q, want %q", report.Source, SourceSelf)
	}
}

func TestMultiAgentTracker_Creation(t *testing.T) {
	mat := NewMultiAgentTracker()

	if mat.Count() != 0 {
		t.Errorf("new tracker count = %d, want 0", mat.Count())
	}
}

func TestMultiAgentTracker_GetOrCreate(t *testing.T) {
	mat := NewMultiAgentTracker()

	// Create first tracker
	tracker1 := mat.GetOrCreate("agent-1", 100)
	if tracker1 == nil {
		t.Fatal("GetOrCreate returned nil")
	}
	if tracker1.AgentID() != "agent-1" {
		t.Errorf("AgentID = %q, want %q", tracker1.AgentID(), "agent-1")
	}

	// Get same tracker
	tracker2 := mat.GetOrCreate("agent-1", 100)
	if tracker2 != tracker1 {
		t.Error("GetOrCreate should return same tracker instance")
	}

	// Create different tracker
	tracker3 := mat.GetOrCreate("agent-2", 100)
	if tracker3 == tracker1 {
		t.Error("GetOrCreate should return different tracker for different agent")
	}

	if mat.Count() != 2 {
		t.Errorf("count = %d, want 2", mat.Count())
	}
}

func TestMultiAgentTracker_Get(t *testing.T) {
	mat := NewMultiAgentTracker()

	// Non-existent agent
	tracker := mat.Get("nonexistent")
	if tracker != nil {
		t.Error("Get should return nil for nonexistent agent")
	}

	// Create and get
	created := mat.GetOrCreate("agent-1", 100)
	retrieved := mat.Get("agent-1")
	if retrieved != created {
		t.Error("Get should return created tracker")
	}
}

func TestMultiAgentTracker_All(t *testing.T) {
	mat := NewMultiAgentTracker()

	// Create some trackers with different statuses
	mat.GetOrCreate("agent-1", 100).UpdateStatus(StatusWorking, SourceSelf, "", "")
	mat.GetOrCreate("agent-2", 100).UpdateStatus(StatusIdle, SourceInferred, "", "")
	mat.GetOrCreate("agent-3", 100).UpdateStatus(StatusThinking, SourceSelf, "", "")

	all := mat.All()
	if len(all) != 3 {
		t.Errorf("All() returned %d agents, want 3", len(all))
	}

	if all["agent-1"] != StatusWorking {
		t.Errorf("agent-1 status = %q, want %q", all["agent-1"], StatusWorking)
	}
	if all["agent-2"] != StatusIdle {
		t.Errorf("agent-2 status = %q, want %q", all["agent-2"], StatusIdle)
	}
	if all["agent-3"] != StatusThinking {
		t.Errorf("agent-3 status = %q, want %q", all["agent-3"], StatusThinking)
	}
}

func TestMultiAgentTracker_Remove(t *testing.T) {
	mat := NewMultiAgentTracker()

	mat.GetOrCreate("agent-1", 100)
	mat.GetOrCreate("agent-2", 100)

	if mat.Count() != 2 {
		t.Fatalf("count = %d, want 2", mat.Count())
	}

	mat.Remove("agent-1")

	if mat.Count() != 1 {
		t.Errorf("after Remove, count = %d, want 1", mat.Count())
	}

	if mat.Get("agent-1") != nil {
		t.Error("removed agent should not be retrievable")
	}

	if mat.Get("agent-2") == nil {
		t.Error("non-removed agent should still be retrievable")
	}
}

func TestMultiAgentTracker_ThreadSafety(t *testing.T) {
	mat := NewMultiAgentTracker()
	done := make(chan bool)

	// Concurrent operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			agentID := "agent"
			for j := 0; j < 100; j++ {
				tracker := mat.GetOrCreate(agentID, 100)
				tracker.UpdateStatus(StatusWorking, SourceSelf, "", "")
				mat.All()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
