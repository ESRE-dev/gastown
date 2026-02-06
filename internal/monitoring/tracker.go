package monitoring

import (
	"sync"
	"time"
)

// StatusTracker tracks the current status and history for a single agent.
// It is thread-safe for concurrent reads and writes.
type StatusTracker struct {
	mu             sync.RWMutex
	agentID        string
	currentStatus  AgentStatus
	currentSource  StatusSource
	lastUpdate     time.Time
	lastActivity   time.Time
	history        []StatusReport
	maxHistory     int
	detectorActive bool
}

// NewStatusTracker creates a new StatusTracker for the given agent.
// maxHistory limits the number of historical status reports kept (0 = unlimited).
func NewStatusTracker(agentID string, maxHistory int) *StatusTracker {
	return &StatusTracker{
		agentID:        agentID,
		currentStatus:  StatusOffline,
		currentSource:  SourceInferred,
		lastUpdate:     time.Now(),
		lastActivity:   time.Now(),
		history:        make([]StatusReport, 0),
		maxHistory:     maxHistory,
		detectorActive: true,
	}
}

// UpdateStatus records a new status for the agent.
func (st *StatusTracker) UpdateStatus(status AgentStatus, source StatusSource, message string, pattern string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()

	// Create status report
	report := StatusReport{
		AgentID:   st.agentID,
		Status:    status,
		Source:    source,
		Timestamp: now,
		Message:   message,
		Pattern:   pattern,
	}

	// Update current state
	st.currentStatus = status
	st.currentSource = source
	st.lastUpdate = now

	// Track activity for idle detection (non-idle statuses count as activity)
	if status != StatusIdle && status != StatusOffline && status != StatusPaused {
		st.lastActivity = now
	}

	// Add to history
	st.history = append(st.history, report)

	// Trim history if needed
	if st.maxHistory > 0 && len(st.history) > st.maxHistory {
		st.history = st.history[len(st.history)-st.maxHistory:]
	}
}

// GetStatus returns the agent's current status and when it was last updated.
func (st *StatusTracker) GetStatus() (AgentStatus, time.Time) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.currentStatus, st.lastUpdate
}

// GetStatusReport returns a full status report for the current state.
func (st *StatusTracker) GetStatusReport() StatusReport {
	st.mu.RLock()
	defer st.mu.RUnlock()

	return StatusReport{
		AgentID:   st.agentID,
		Status:    st.currentStatus,
		Source:    st.currentSource,
		Timestamp: st.lastUpdate,
	}
}

// GetHistory returns a copy of the status history.
func (st *StatusTracker) GetHistory() []StatusReport {
	st.mu.RLock()
	defer st.mu.RUnlock()

	history := make([]StatusReport, len(st.history))
	copy(history, st.history)
	return history
}

// GetLastActivity returns when the agent was last active.
func (st *StatusTracker) GetLastActivity() time.Time {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.lastActivity
}

// SetDetectorActive enables or disables automatic status detection for this agent.
func (st *StatusTracker) SetDetectorActive(active bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.detectorActive = active
}

// IsDetectorActive returns whether automatic status detection is enabled.
func (st *StatusTracker) IsDetectorActive() bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.detectorActive
}

// AgentID returns the agent ID this tracker is for.
func (st *StatusTracker) AgentID() string {
	return st.agentID
}

// MultiAgentTracker manages StatusTrackers for multiple agents.
type MultiAgentTracker struct {
	mu       sync.RWMutex
	trackers map[string]*StatusTracker
}

// NewMultiAgentTracker creates a new MultiAgentTracker.
func NewMultiAgentTracker() *MultiAgentTracker {
	return &MultiAgentTracker{
		trackers: make(map[string]*StatusTracker),
	}
}

// GetOrCreate returns the StatusTracker for an agent, creating it if needed.
func (mat *MultiAgentTracker) GetOrCreate(agentID string, maxHistory int) *StatusTracker {
	mat.mu.Lock()
	defer mat.mu.Unlock()

	if tracker, exists := mat.trackers[agentID]; exists {
		return tracker
	}

	tracker := NewStatusTracker(agentID, maxHistory)
	mat.trackers[agentID] = tracker
	return tracker
}

// Get returns the StatusTracker for an agent, or nil if not found.
func (mat *MultiAgentTracker) Get(agentID string) *StatusTracker {
	mat.mu.RLock()
	defer mat.mu.RUnlock()
	return mat.trackers[agentID]
}

// All returns a map of all agent IDs to their current status.
func (mat *MultiAgentTracker) All() map[string]AgentStatus {
	mat.mu.RLock()
	defer mat.mu.RUnlock()

	result := make(map[string]AgentStatus, len(mat.trackers))
	for id, tracker := range mat.trackers {
		status, _ := tracker.GetStatus()
		result[id] = status
	}
	return result
}

// Remove removes the StatusTracker for an agent.
func (mat *MultiAgentTracker) Remove(agentID string) {
	mat.mu.Lock()
	defer mat.mu.Unlock()
	delete(mat.trackers, agentID)
}

// Count returns the number of tracked agents.
func (mat *MultiAgentTracker) Count() int {
	mat.mu.RLock()
	defer mat.mu.RUnlock()
	return len(mat.trackers)
}
