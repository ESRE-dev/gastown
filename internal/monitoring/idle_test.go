package monitoring

import (
	"context"
	"testing"
	"time"
)

func TestIdleDetector_Creation(t *testing.T) {
	mat := NewMultiAgentTracker()
	detector := NewIdleDetector(mat, 30*time.Second, 5*time.Second)

	if detector.GetThreshold() != 30*time.Second {
		t.Errorf("threshold = %v, want %v", detector.GetThreshold(), 30*time.Second)
	}

	if detector.IsRunning() {
		t.Error("detector should not be running after creation")
	}
}

func TestIdleDetector_StartStop(t *testing.T) {
	mat := NewMultiAgentTracker()
	detector := NewIdleDetector(mat, 30*time.Second, 10*time.Millisecond)

	ctx := context.Background()
	detector.Start(ctx)

	// Give it time to start
	time.Sleep(20 * time.Millisecond)

	if !detector.IsRunning() {
		t.Error("detector should be running after Start()")
	}

	detector.Stop()

	if detector.IsRunning() {
		t.Error("detector should not be running after Stop()")
	}
}

func TestIdleDetector_IdleDetection(t *testing.T) {
	mat := NewMultiAgentTracker()
	threshold := 50 * time.Millisecond
	checkInterval := 20 * time.Millisecond
	detector := NewIdleDetector(mat, threshold, checkInterval)

	// Create an agent and set it to working
	tracker := mat.GetOrCreate("test-agent", 100)
	tracker.UpdateStatus(StatusWorking, SourceSelf, "", "")

	// Start detector
	ctx := context.Background()
	detector.Start(ctx)
	defer detector.Stop()

	// Wait for threshold to expire plus check interval
	time.Sleep(threshold + checkInterval + 30*time.Millisecond)

	// Agent should now be idle
	status, _ := tracker.GetStatus()
	if status != StatusIdle {
		t.Errorf("after idle threshold, status = %q, want %q", status, StatusIdle)
	}

	report := tracker.GetStatusReport()
	if report.Source != SourceInferred {
		t.Errorf("idle status source = %q, want %q", report.Source, SourceInferred)
	}
}

func TestIdleDetector_ActivityResetsIdle(t *testing.T) {
	mat := NewMultiAgentTracker()
	threshold := 50 * time.Millisecond
	checkInterval := 20 * time.Millisecond
	detector := NewIdleDetector(mat, threshold, checkInterval)

	// Create an agent
	tracker := mat.GetOrCreate("test-agent", 100)
	tracker.UpdateStatus(StatusWorking, SourceSelf, "", "")

	ctx := context.Background()
	detector.Start(ctx)
	defer detector.Stop()

	// Wait a bit but not long enough to go idle
	time.Sleep(30 * time.Millisecond)

	// Update activity
	tracker.UpdateStatus(StatusThinking, SourceSelf, "", "")

	// Wait past original threshold but not past new activity time
	time.Sleep(40 * time.Millisecond)

	// Should still not be idle
	status, _ := tracker.GetStatus()
	if status == StatusIdle {
		t.Error("agent should not be idle after recent activity")
	}
}

func TestIdleDetector_DoesNotMarkOfflineAgents(t *testing.T) {
	mat := NewMultiAgentTracker()
	threshold := 50 * time.Millisecond
	checkInterval := 20 * time.Millisecond
	detector := NewIdleDetector(mat, threshold, checkInterval)

	// Create an offline agent
	tracker := mat.GetOrCreate("test-agent", 100)
	tracker.UpdateStatus(StatusOffline, SourceBoss, "", "")

	ctx := context.Background()
	detector.Start(ctx)
	defer detector.Stop()

	// Wait past threshold
	time.Sleep(threshold + checkInterval + 30*time.Millisecond)

	// Should still be offline, not idle
	status, _ := tracker.GetStatus()
	if status != StatusOffline {
		t.Errorf("offline agent status = %q, want %q", status, StatusOffline)
	}
}

func TestIdleDetector_RespectsDetectorActive(t *testing.T) {
	mat := NewMultiAgentTracker()
	threshold := 50 * time.Millisecond
	checkInterval := 20 * time.Millisecond
	detector := NewIdleDetector(mat, threshold, checkInterval)

	// Create an agent with detector disabled
	tracker := mat.GetOrCreate("test-agent", 100)
	tracker.UpdateStatus(StatusWorking, SourceSelf, "", "")
	tracker.SetDetectorActive(false)

	ctx := context.Background()
	detector.Start(ctx)
	defer detector.Stop()

	// Wait past threshold
	time.Sleep(threshold + checkInterval + 30*time.Millisecond)

	// Should still be working (not marked idle)
	status, _ := tracker.GetStatus()
	if status != StatusWorking {
		t.Errorf("with detector disabled, status = %q, want %q", status, StatusWorking)
	}
}

func TestIdleDetector_SetThreshold(t *testing.T) {
	mat := NewMultiAgentTracker()
	detector := NewIdleDetector(mat, 30*time.Second, 5*time.Second)

	newThreshold := 60 * time.Second
	detector.SetThreshold(newThreshold)

	if detector.GetThreshold() != newThreshold {
		t.Errorf("after SetThreshold, threshold = %v, want %v", detector.GetThreshold(), newThreshold)
	}
}

func TestIdleDetector_ContextCancellation(t *testing.T) {
	mat := NewMultiAgentTracker()
	detector := NewIdleDetector(mat, 30*time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	detector.Start(ctx)

	// Give it time to start
	time.Sleep(20 * time.Millisecond)

	if !detector.IsRunning() {
		t.Fatal("detector should be running")
	}

	// Cancel context
	cancel()

	// Give it time to stop
	time.Sleep(30 * time.Millisecond)

	if detector.IsRunning() {
		t.Error("detector should stop when context is cancelled")
	}
}

func TestIdleDetector_MultipleAgents(t *testing.T) {
	mat := NewMultiAgentTracker()
	threshold := 100 * time.Millisecond
	checkInterval := 20 * time.Millisecond
	detector := NewIdleDetector(mat, threshold, checkInterval)

	// Create multiple agents with different activity times
	agent1 := mat.GetOrCreate("agent-1", 100)
	agent1.UpdateStatus(StatusWorking, SourceSelf, "", "")

	agent2 := mat.GetOrCreate("agent-2", 100)
	agent2.UpdateStatus(StatusWorking, SourceSelf, "", "")

	ctx := context.Background()
	detector.Start(ctx)
	defer detector.Stop()

	// Wait past threshold, then update agent2 right before it would go idle
	time.Sleep(threshold + checkInterval + 10*time.Millisecond)

	// Agent1 should now be idle
	status1, _ := agent1.GetStatus()
	if status1 != StatusIdle {
		t.Errorf("agent-1 status = %q, want %q", status1, StatusIdle)
	}

	// Update agent2 to reset its idle timer
	agent2.UpdateStatus(StatusThinking, SourceSelf, "", "")

	// Wait a bit but not past threshold
	time.Sleep(30 * time.Millisecond)

	// Agent2 should still be active (thinking)
	status2, _ := agent2.GetStatus()
	if status2 == StatusIdle {
		t.Error("agent-2 should not be idle after recent activity")
	}
	if status2 != StatusThinking {
		t.Errorf("agent-2 status = %q, want %q", status2, StatusThinking)
	}
}

func TestIdleDetector_AlreadyIdle(t *testing.T) {
	mat := NewMultiAgentTracker()
	threshold := 50 * time.Millisecond
	checkInterval := 20 * time.Millisecond
	detector := NewIdleDetector(mat, threshold, checkInterval)

	// Create an agent that's already idle
	tracker := mat.GetOrCreate("test-agent", 100)
	tracker.UpdateStatus(StatusIdle, SourceInferred, "", "")

	ctx := context.Background()
	detector.Start(ctx)
	defer detector.Stop()

	// Record history length
	historyBefore := len(tracker.GetHistory())

	// Wait past threshold
	time.Sleep(threshold + checkInterval + 30*time.Millisecond)

	// Should not create duplicate idle status
	historyAfter := len(tracker.GetHistory())
	if historyAfter != historyBefore {
		t.Error("should not create duplicate idle status for already-idle agent")
	}
}

func TestIdleDetector_StopIdempotent(t *testing.T) {
	mat := NewMultiAgentTracker()
	detector := NewIdleDetector(mat, 30*time.Second, 10*time.Millisecond)

	// Stop without starting
	detector.Stop() // Should not panic

	// Start and stop multiple times
	ctx := context.Background()
	detector.Start(ctx)
	time.Sleep(20 * time.Millisecond)
	detector.Stop()
	detector.Stop() // Should not panic
}
