package daemon

import (
	"time"
)

// BackoffStrategy defines how intervals grow.
type BackoffStrategy string

const (
	// StrategyFixed keeps the same interval (no backoff).
	StrategyFixed BackoffStrategy = "fixed"

	// StrategyGeometric multiplies by a factor each miss (1.5x).
	StrategyGeometric BackoffStrategy = "geometric"

	// StrategyExponential doubles interval each miss (2x).
	StrategyExponential BackoffStrategy = "exponential"
)

// BackoffConfig holds backoff configuration.
type BackoffConfig struct {
	// Strategy determines how intervals grow.
	Strategy BackoffStrategy

	// BaseInterval is the starting interval (default 60s).
	BaseInterval time.Duration

	// MaxInterval is the cap on how large intervals can grow (default 10m).
	MaxInterval time.Duration

	// Factor is the multiplier for geometric backoff (default 1.5).
	Factor float64
}

// DefaultBackoffConfig returns sensible defaults.
// Base interval is 5 minutes since deacon rounds may take a while
// (health checks, plugins, syncing clones, complex remediation).
// Max interval is 30 minutes - beyond that, something is likely wrong.
func DefaultBackoffConfig() *BackoffConfig {
	return &BackoffConfig{
		Strategy:     StrategyGeometric,
		BaseInterval: 5 * time.Minute,
		MaxInterval:  30 * time.Minute,
		Factor:       1.5,
	}
}

// AgentBackoff tracks backoff state for a single agent.
type AgentBackoff struct {
	// AgentID identifies the agent (e.g., "mayor", "gastown-witness").
	AgentID string

	// BaseInterval is the starting interval.
	BaseInterval time.Duration

	// CurrentInterval is the current (possibly backed-off) interval.
	CurrentInterval time.Duration

	// MaxInterval caps how large intervals can grow.
	MaxInterval time.Duration

	// ConsecutiveMiss counts pokes with no response.
	ConsecutiveMiss int

	// LastPoke is when we last poked this agent.
	LastPoke time.Time

	// LastActivity is when the agent last showed activity.
	LastActivity time.Time
}

// NewAgentBackoff creates backoff state for an agent.
func NewAgentBackoff(agentID string, config *BackoffConfig) *AgentBackoff {
	if config == nil {
		config = DefaultBackoffConfig()
	}
	return &AgentBackoff{
		AgentID:         agentID,
		BaseInterval:    config.BaseInterval,
		CurrentInterval: config.BaseInterval,
		MaxInterval:     config.MaxInterval,
	}
}

// ShouldPoke returns true if enough time has passed since the last poke.
func (ab *AgentBackoff) ShouldPoke() bool {
	if ab.LastPoke.IsZero() {
		return true // Never poked
	}
	return time.Since(ab.LastPoke) >= ab.CurrentInterval
}

// RecordPoke records that we poked the agent.
func (ab *AgentBackoff) RecordPoke() {
	ab.LastPoke = time.Now()
}

// RecordMiss records that the agent didn't respond since last poke.
// This increases the backoff interval.
func (ab *AgentBackoff) RecordMiss(config *BackoffConfig) {
	ab.ConsecutiveMiss++

	if config == nil {
		config = DefaultBackoffConfig()
	}

	switch config.Strategy {
	case StrategyFixed:
		// No change
	case StrategyGeometric:
		ab.CurrentInterval = time.Duration(float64(ab.CurrentInterval) * config.Factor)
	case StrategyExponential:
		ab.CurrentInterval = ab.CurrentInterval * 2
	}

	// Cap at max interval
	if ab.CurrentInterval > ab.MaxInterval {
		ab.CurrentInterval = ab.MaxInterval
	}
}

// RecordActivity records that the agent showed activity.
// This resets the backoff to the base interval.
func (ab *AgentBackoff) RecordActivity() {
	ab.ConsecutiveMiss = 0
	ab.CurrentInterval = ab.BaseInterval
	ab.LastActivity = time.Now()
}

// BackoffManager tracks backoff state for all agents.
type BackoffManager struct {
	config *BackoffConfig
	agents map[string]*AgentBackoff
}

// NewBackoffManager creates a new backoff manager.
func NewBackoffManager(config *BackoffConfig) *BackoffManager {
	if config == nil {
		config = DefaultBackoffConfig()
	}
	return &BackoffManager{
		config: config,
		agents: make(map[string]*AgentBackoff),
	}
}

// GetOrCreate returns backoff state for an agent, creating if needed.
func (bm *BackoffManager) GetOrCreate(agentID string) *AgentBackoff {
	if ab, ok := bm.agents[agentID]; ok {
		return ab
	}
	ab := NewAgentBackoff(agentID, bm.config)
	bm.agents[agentID] = ab
	return ab
}

// ShouldPoke returns true if we should poke the given agent.
func (bm *BackoffManager) ShouldPoke(agentID string) bool {
	return bm.GetOrCreate(agentID).ShouldPoke()
}

// RecordPoke records that we poked an agent.
func (bm *BackoffManager) RecordPoke(agentID string) {
	bm.GetOrCreate(agentID).RecordPoke()
}

// RecordMiss records that an agent didn't respond.
func (bm *BackoffManager) RecordMiss(agentID string) {
	bm.GetOrCreate(agentID).RecordMiss(bm.config)
}

// RecordActivity records that an agent showed activity.
func (bm *BackoffManager) RecordActivity(agentID string) {
	bm.GetOrCreate(agentID).RecordActivity()
}

// GetInterval returns the current interval for an agent.
func (bm *BackoffManager) GetInterval(agentID string) time.Duration {
	return bm.GetOrCreate(agentID).CurrentInterval
}

// Stats returns a map of agent ID to current interval for logging.
func (bm *BackoffManager) Stats() map[string]time.Duration {
	stats := make(map[string]time.Duration, len(bm.agents))
	for id, ab := range bm.agents {
		stats[id] = ab.CurrentInterval
	}
	return stats
}
