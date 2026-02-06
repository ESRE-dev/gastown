package monitoring

import (
	"context"
	"sync"
	"time"
)

// IdleDetector monitors agent trackers for idle timeout violations.
// It runs in the background and automatically updates agent status to idle
// when they exceed the configured threshold without activity.
type IdleDetector struct {
	mu           sync.RWMutex
	tracker      *MultiAgentTracker
	threshold    time.Duration
	checkInterval time.Duration
	running      bool
	stopChan     chan struct{}
	stoppedChan  chan struct{}
}

// NewIdleDetector creates a new IdleDetector.
// threshold is the duration of inactivity before marking an agent as idle.
// checkInterval is how often to check for idle agents.
func NewIdleDetector(tracker *MultiAgentTracker, threshold time.Duration, checkInterval time.Duration) *IdleDetector {
	return &IdleDetector{
		tracker:       tracker,
		threshold:     threshold,
		checkInterval: checkInterval,
		stopChan:      make(chan struct{}),
		stoppedChan:   make(chan struct{}),
	}
}

// Start begins monitoring for idle agents in a background goroutine.
// Returns immediately; use Stop() to halt monitoring.
func (id *IdleDetector) Start(ctx context.Context) {
	id.mu.Lock()
	if id.running {
		id.mu.Unlock()
		return
	}
	id.running = true
	id.mu.Unlock()

	go id.run(ctx)
}

// run is the main monitoring loop.
func (id *IdleDetector) run(ctx context.Context) {
	defer close(id.stoppedChan)

	ticker := time.NewTicker(id.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			id.mu.Lock()
			id.running = false
			id.mu.Unlock()
			return

		case <-id.stopChan:
			id.mu.Lock()
			id.running = false
			id.mu.Unlock()
			return

		case <-ticker.C:
			id.checkIdle()
		}
	}
}

// checkIdle examines all tracked agents and marks idle ones.
func (id *IdleDetector) checkIdle() {
	now := time.Now()
	threshold := id.getThreshold()

	// Get all trackers (snapshot to avoid holding lock during updates)
	id.tracker.mu.RLock()
	trackers := make([]*StatusTracker, 0, len(id.tracker.trackers))
	for _, tracker := range id.tracker.trackers {
		trackers = append(trackers, tracker)
	}
	id.tracker.mu.RUnlock()

	// Check each agent
	for _, tracker := range trackers {
		if !tracker.IsDetectorActive() {
			continue
		}

		currentStatus, _ := tracker.GetStatus()
		lastActivity := tracker.GetLastActivity()
		idleDuration := now.Sub(lastActivity)

		// Only mark as idle if currently in a "active" state and idle threshold exceeded
		if currentStatus != StatusIdle && currentStatus != StatusOffline && idleDuration > threshold {
			tracker.UpdateStatus(
				StatusIdle,
				SourceInferred,
				"No activity detected",
				"idle_timeout",
			)
		}
	}
}

// Stop halts idle detection and waits for the monitoring goroutine to exit.
func (id *IdleDetector) Stop() {
	id.mu.Lock()
	if !id.running {
		id.mu.Unlock()
		return
	}
	id.mu.Unlock()

	close(id.stopChan)
	<-id.stoppedChan
}

// SetThreshold updates the idle timeout threshold.
func (id *IdleDetector) SetThreshold(threshold time.Duration) {
	id.mu.Lock()
	defer id.mu.Unlock()
	id.threshold = threshold
}

// GetThreshold returns the current idle timeout threshold.
func (id *IdleDetector) GetThreshold() time.Duration {
	id.mu.RLock()
	defer id.mu.RUnlock()
	return id.threshold
}

// getThreshold is an internal helper that reads threshold without exposing lock.
func (id *IdleDetector) getThreshold() time.Duration {
	id.mu.RLock()
	defer id.mu.RUnlock()
	return id.threshold
}

// IsRunning returns whether the idle detector is currently running.
func (id *IdleDetector) IsRunning() bool {
	id.mu.RLock()
	defer id.mu.RUnlock()
	return id.running
}
