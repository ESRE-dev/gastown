// Package monitoring provides agent status tracking and inference for Gas Town.
//
// It monitors agent activity through output patterns and timeout detection,
// inferring status changes automatically while allowing explicit status reports
// from agents themselves or boss commands.
package monitoring

import "time"

// AgentStatus represents the current operational state of an agent.
type AgentStatus string

const (
	// StatusAvailable means the agent is ready to accept work.
	StatusAvailable AgentStatus = "available"

	// StatusWorking means the agent is actively executing a task.
	StatusWorking AgentStatus = "working"

	// StatusThinking means the agent is processing or reasoning.
	StatusThinking AgentStatus = "thinking"

	// StatusBlocked means the agent is blocked on external input or resources.
	StatusBlocked AgentStatus = "blocked"

	// StatusWaiting means the agent is waiting for a dependency or response.
	StatusWaiting AgentStatus = "waiting"

	// StatusReviewing means the agent is reviewing or analyzing work.
	StatusReviewing AgentStatus = "reviewing"

	// StatusIdle means the agent has been inactive for a threshold period.
	StatusIdle AgentStatus = "idle"

	// StatusPaused means the agent is paused intentionally.
	StatusPaused AgentStatus = "paused"

	// StatusError means the agent encountered an error.
	StatusError AgentStatus = "error"

	// StatusOffline means the agent is not running or unreachable.
	StatusOffline AgentStatus = "offline"
)

// StatusSource indicates how a status was determined.
type StatusSource string

const (
	// SourceBoss means the status came from a boss command.
	SourceBoss StatusSource = "boss"

	// SourceSelf means the status was reported by the agent itself.
	SourceSelf StatusSource = "self"

	// SourceInferred means the status was inferred from patterns or timeout.
	SourceInferred StatusSource = "inferred"
)

// StatusReport represents a single status observation for an agent.
type StatusReport struct {
	// AgentID identifies the agent this report is for.
	AgentID string `json:"agent_id"`

	// Status is the agent's current status.
	Status AgentStatus `json:"status"`

	// Source indicates how this status was determined.
	Source StatusSource `json:"source"`

	// Timestamp is when this status was recorded.
	Timestamp time.Time `json:"timestamp"`

	// Message is an optional human-readable context for the status.
	Message string `json:"message,omitempty"`

	// Pattern is the regex pattern that triggered inference (if Source == SourceInferred).
	Pattern string `json:"pattern,omitempty"`
}
