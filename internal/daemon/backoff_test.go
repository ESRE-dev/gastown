package daemon

import (
	"testing"
	"time"
)

func TestDefaultBackoffConfig(t *testing.T) {
	config := DefaultBackoffConfig()

	if config.Strategy != StrategyGeometric {
		t.Errorf("expected strategy Geometric, got %v", config.Strategy)
	}
	if config.BaseInterval != 5*time.Minute {
		t.Errorf("expected base interval 5m, got %v", config.BaseInterval)
	}
	if config.MaxInterval != 30*time.Minute {
		t.Errorf("expected max interval 30m, got %v", config.MaxInterval)
	}
	if config.Factor != 1.5 {
		t.Errorf("expected factor 1.5, got %v", config.Factor)
	}
}

func TestNewAgentBackoff(t *testing.T) {
	config := DefaultBackoffConfig()
	ab := NewAgentBackoff("test-agent", config)

	if ab.AgentID != "test-agent" {
		t.Errorf("expected agent ID 'test-agent', got %s", ab.AgentID)
	}
	if ab.BaseInterval != 5*time.Minute {
		t.Errorf("expected base interval 5m, got %v", ab.BaseInterval)
	}
	if ab.CurrentInterval != 5*time.Minute {
		t.Errorf("expected current interval 5m, got %v", ab.CurrentInterval)
	}
	if ab.ConsecutiveMiss != 0 {
		t.Errorf("expected consecutive miss 0, got %d", ab.ConsecutiveMiss)
	}
}

func TestAgentBackoff_ShouldPoke(t *testing.T) {
	config := &BackoffConfig{
		Strategy:     StrategyGeometric,
		BaseInterval: 100 * time.Millisecond, // Short for testing
		MaxInterval:  1 * time.Second,
		Factor:       1.5,
	}
	ab := NewAgentBackoff("test", config)

	// Should poke immediately (never poked)
	if !ab.ShouldPoke() {
		t.Error("expected ShouldPoke=true for new agent")
	}

	// Record a poke
	ab.RecordPoke()

	// Should not poke immediately after
	if ab.ShouldPoke() {
		t.Error("expected ShouldPoke=false immediately after poke")
	}

	// Wait for interval
	time.Sleep(110 * time.Millisecond)

	// Now should poke again
	if !ab.ShouldPoke() {
		t.Error("expected ShouldPoke=true after interval elapsed")
	}
}

func TestAgentBackoff_GeometricBackoff(t *testing.T) {
	config := &BackoffConfig{
		Strategy:     StrategyGeometric,
		BaseInterval: 100 * time.Millisecond,
		MaxInterval:  1 * time.Second,
		Factor:       1.5,
	}
	ab := NewAgentBackoff("test", config)

	// Initial interval
	if ab.CurrentInterval != 100*time.Millisecond {
		t.Errorf("expected initial interval 100ms, got %v", ab.CurrentInterval)
	}

	// First miss: 100ms * 1.5 = 150ms
	ab.RecordMiss(config)
	if ab.CurrentInterval != 150*time.Millisecond {
		t.Errorf("expected interval 150ms after 1 miss, got %v", ab.CurrentInterval)
	}
	if ab.ConsecutiveMiss != 1 {
		t.Errorf("expected consecutive miss 1, got %d", ab.ConsecutiveMiss)
	}

	// Second miss: 150ms * 1.5 = 225ms
	ab.RecordMiss(config)
	if ab.CurrentInterval != 225*time.Millisecond {
		t.Errorf("expected interval 225ms after 2 misses, got %v", ab.CurrentInterval)
	}

	// Third miss: 225ms * 1.5 = 337.5ms
	ab.RecordMiss(config)
	expected := time.Duration(337500000) // 337.5ms in nanoseconds
	if ab.CurrentInterval != expected {
		t.Errorf("expected interval ~337.5ms after 3 misses, got %v", ab.CurrentInterval)
	}
}

func TestAgentBackoff_ExponentialBackoff(t *testing.T) {
	config := &BackoffConfig{
		Strategy:     StrategyExponential,
		BaseInterval: 100 * time.Millisecond,
		MaxInterval:  1 * time.Second,
		Factor:       2.0, // Ignored for exponential
	}
	ab := NewAgentBackoff("test", config)

	// First miss: 100ms * 2 = 200ms
	ab.RecordMiss(config)
	if ab.CurrentInterval != 200*time.Millisecond {
		t.Errorf("expected interval 200ms after 1 miss, got %v", ab.CurrentInterval)
	}

	// Second miss: 200ms * 2 = 400ms
	ab.RecordMiss(config)
	if ab.CurrentInterval != 400*time.Millisecond {
		t.Errorf("expected interval 400ms after 2 misses, got %v", ab.CurrentInterval)
	}

	// Third miss: 400ms * 2 = 800ms
	ab.RecordMiss(config)
	if ab.CurrentInterval != 800*time.Millisecond {
		t.Errorf("expected interval 800ms after 3 misses, got %v", ab.CurrentInterval)
	}
}

func TestAgentBackoff_FixedStrategy(t *testing.T) {
	config := &BackoffConfig{
		Strategy:     StrategyFixed,
		BaseInterval: 100 * time.Millisecond,
		MaxInterval:  1 * time.Second,
		Factor:       1.5,
	}
	ab := NewAgentBackoff("test", config)

	// Multiple misses should not change interval
	ab.RecordMiss(config)
	ab.RecordMiss(config)
	ab.RecordMiss(config)

	if ab.CurrentInterval != 100*time.Millisecond {
		t.Errorf("expected interval to stay at 100ms with fixed strategy, got %v", ab.CurrentInterval)
	}
	if ab.ConsecutiveMiss != 3 {
		t.Errorf("expected consecutive miss 3, got %d", ab.ConsecutiveMiss)
	}
}

func TestAgentBackoff_MaxInterval(t *testing.T) {
	config := &BackoffConfig{
		Strategy:     StrategyExponential,
		BaseInterval: 100 * time.Millisecond,
		MaxInterval:  500 * time.Millisecond,
		Factor:       2.0,
	}
	ab := NewAgentBackoff("test", config)

	// Keep missing until we hit the cap
	for i := 0; i < 10; i++ {
		ab.RecordMiss(config)
	}

	if ab.CurrentInterval != 500*time.Millisecond {
		t.Errorf("expected interval capped at 500ms, got %v", ab.CurrentInterval)
	}
}

func TestAgentBackoff_RecordActivity(t *testing.T) {
	config := &BackoffConfig{
		Strategy:     StrategyGeometric,
		BaseInterval: 100 * time.Millisecond,
		MaxInterval:  1 * time.Second,
		Factor:       1.5,
	}
	ab := NewAgentBackoff("test", config)

	// Build up some backoff
	ab.RecordMiss(config)
	ab.RecordMiss(config)
	ab.RecordMiss(config)

	if ab.CurrentInterval == 100*time.Millisecond {
		t.Error("expected interval to have increased")
	}
	if ab.ConsecutiveMiss != 3 {
		t.Errorf("expected consecutive miss 3, got %d", ab.ConsecutiveMiss)
	}

	// Record activity - should reset
	ab.RecordActivity()

	if ab.CurrentInterval != 100*time.Millisecond {
		t.Errorf("expected interval reset to 100ms, got %v", ab.CurrentInterval)
	}
	if ab.ConsecutiveMiss != 0 {
		t.Errorf("expected consecutive miss reset to 0, got %d", ab.ConsecutiveMiss)
	}
	if ab.LastActivity.IsZero() {
		t.Error("expected LastActivity to be set")
	}
}

func TestBackoffManager_GetOrCreate(t *testing.T) {
	bm := NewBackoffManager(DefaultBackoffConfig())

	// First call creates
	ab1 := bm.GetOrCreate("agent1")
	if ab1 == nil {
		t.Fatal("expected agent backoff to be created")
	}
	if ab1.AgentID != "agent1" {
		t.Errorf("expected agent ID 'agent1', got %s", ab1.AgentID)
	}

	// Second call returns same instance
	ab2 := bm.GetOrCreate("agent1")
	if ab1 != ab2 {
		t.Error("expected same instance on second call")
	}

	// Different agent creates new instance
	ab3 := bm.GetOrCreate("agent2")
	if ab1 == ab3 {
		t.Error("expected different instance for different agent")
	}
}

func TestBackoffManager_Stats(t *testing.T) {
	config := &BackoffConfig{
		Strategy:     StrategyGeometric,
		BaseInterval: 100 * time.Millisecond,
		MaxInterval:  1 * time.Second,
		Factor:       1.5,
	}
	bm := NewBackoffManager(config)

	// Create some agents with different backoff states
	bm.RecordPoke("agent1")
	bm.RecordMiss("agent1")

	bm.RecordPoke("agent2")
	bm.RecordMiss("agent2")
	bm.RecordMiss("agent2")

	stats := bm.Stats()

	if len(stats) != 2 {
		t.Errorf("expected 2 agents in stats, got %d", len(stats))
	}

	// agent1: 100ms * 1.5 = 150ms
	if stats["agent1"] != 150*time.Millisecond {
		t.Errorf("expected agent1 interval 150ms, got %v", stats["agent1"])
	}

	// agent2: 100ms * 1.5 * 1.5 = 225ms
	if stats["agent2"] != 225*time.Millisecond {
		t.Errorf("expected agent2 interval 225ms, got %v", stats["agent2"])
	}
}

func TestExtractRigName(t *testing.T) {
	tests := []struct {
		session  string
		expected string
	}{
		{"gt-gastown-witness", "gastown"},
		{"gt-myrig-witness", "myrig"},
		{"gt-my-rig-name-witness", "my-rig-name"},
	}

	for _, tc := range tests {
		result := extractRigName(tc.session)
		if result != tc.expected {
			t.Errorf("extractRigName(%q) = %q, expected %q", tc.session, result, tc.expected)
		}
	}
}
