package monitoring

import (
	"testing"
)

func TestPatternRegistry_DefaultPatterns(t *testing.T) {
	pr := NewPatternRegistry()

	tests := []struct {
		name     string
		input    string
		wantStatus AgentStatus
	}{
		{"thinking_indicator", "Thinking...", StatusThinking},
		{"thinking_lowercase", "thinking...", StatusThinking},
		{"blocked_prefix", "BLOCKED: waiting for input", StatusBlocked},
		{"blocked_lowercase", "blocked: waiting for approval", StatusBlocked},
		{"error_prefix", "ERROR: failed to connect", StatusError},
		{"error_occurred", "an error occurred while processing", StatusError},
		{"waiting_for", "waiting for response", StatusWaiting},
		{"reviewing", "reviewing the code", StatusReviewing},
		{"analyzing", "analyzing the results", StatusReviewing},
		{"working_on", "working on the task", StatusWorking},
		{"executing", "executing the command", StatusWorking},
		{"processing", "processing the data", StatusWorking},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, pattern := pr.Detect(tt.input)
			if status != tt.wantStatus {
				t.Errorf("Detect(%q) status = %q, want %q (pattern: %q)", tt.input, status, tt.wantStatus, pattern)
			}
			if status != "" && pattern == "" {
				t.Errorf("Detect(%q) returned status %q but no pattern name", tt.input, status)
			}
		})
	}
}

func TestPatternRegistry_NoMatch(t *testing.T) {
	pr := NewPatternRegistry()

	status, pattern := pr.Detect("Just a regular message")
	if status != "" {
		t.Errorf("expected no match, got status=%q, pattern=%q", status, pattern)
	}
	if pattern != "" {
		t.Errorf("expected empty pattern, got %q", pattern)
	}
}

func TestPatternRegistry_Register(t *testing.T) {
	pr := NewPatternRegistry()
	pr.Clear() // Start fresh

	// Register custom pattern
	err := pr.Register("custom_pattern", `(?i)custom status`, StatusAvailable)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Test detection
	status, pattern := pr.Detect("This is a custom status message")
	if status != StatusAvailable {
		t.Errorf("expected StatusAvailable, got %q", status)
	}
	if pattern != "custom_pattern" {
		t.Errorf("expected pattern name 'custom_pattern', got %q", pattern)
	}
}

func TestPatternRegistry_RegisterInvalidRegex(t *testing.T) {
	pr := NewPatternRegistry()

	// Try to register invalid regex
	err := pr.Register("bad_pattern", `[unclosed bracket`, StatusWorking)
	if err == nil {
		t.Error("expected error for invalid regex, got nil")
	}
}

func TestPatternRegistry_FirstMatchWins(t *testing.T) {
	pr := NewPatternRegistry()
	pr.Clear()

	// Register two patterns that could match the same input
	_ = pr.Register("first", `error`, StatusError)
	_ = pr.Register("second", `error occurred`, StatusBlocked) // More specific, but registered later

	// First pattern should win
	status, pattern := pr.Detect("error occurred")
	if status != StatusError {
		t.Errorf("expected StatusError (first pattern), got %q", status)
	}
	if pattern != "first" {
		t.Errorf("expected pattern 'first', got %q", pattern)
	}
}

func TestPatternRegistry_Clear(t *testing.T) {
	pr := NewPatternRegistry()

	initialCount := pr.Count()
	if initialCount == 0 {
		t.Fatal("expected default patterns, got 0")
	}

	pr.Clear()

	if pr.Count() != 0 {
		t.Errorf("after Clear(), expected 0 patterns, got %d", pr.Count())
	}

	// Should not match anything
	status, _ := pr.Detect("Thinking...")
	if status != "" {
		t.Errorf("after Clear(), expected no matches, got %q", status)
	}
}

func TestPatternRegistry_Count(t *testing.T) {
	pr := NewPatternRegistry()

	// Should have default patterns
	count := pr.Count()
	if count == 0 {
		t.Error("expected non-zero default pattern count")
	}

	// Add one more
	_ = pr.Register("new_pattern", `test`, StatusWorking)
	newCount := pr.Count()
	if newCount != count+1 {
		t.Errorf("after adding pattern, expected count=%d, got %d", count+1, newCount)
	}
}

func TestPatternRegistry_ThreadSafety(t *testing.T) {
	pr := NewPatternRegistry()

	// Concurrent reads and writes
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = pr.Register("pattern", `test\d+`, StatusWorking)
		}
		done <- true
	}()

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				pr.Detect("test123")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 11; i++ {
		<-done
	}
}
