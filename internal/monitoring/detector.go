package monitoring

import (
	"regexp"
	"sync"
)

// Pattern maps a compiled regex to an agent status.
type Pattern struct {
	Regex  *regexp.Regexp
	Status AgentStatus
	Name   string // Human-readable pattern name for debugging
}

// PatternRegistry manages a collection of regex patterns for status inference.
// It is thread-safe and supports dynamic pattern registration.
type PatternRegistry struct {
	mu       sync.RWMutex
	patterns []Pattern
}

// NewPatternRegistry creates a new PatternRegistry with default patterns.
func NewPatternRegistry() *PatternRegistry {
	pr := &PatternRegistry{
		patterns: make([]Pattern, 0),
	}

	// Register default patterns
	pr.registerDefaults()

	return pr
}

// registerDefaults adds the built-in status detection patterns.
func (pr *PatternRegistry) registerDefaults() {
	defaults := []struct {
		name    string
		pattern string
		status  AgentStatus
	}{
		{"thinking_indicator", `(?i)thinking\.{3}`, StatusThinking},
		{"blocked_prefix", `(?i)^BLOCKED:`, StatusBlocked},
		{"error_prefix", `(?i)^ERROR:`, StatusError},
		{"error_indicator", `(?i)\berror\b.*occurred`, StatusError},
		{"waiting_for", `(?i)waiting for`, StatusWaiting},
		{"reviewing", `(?i)reviewing|analyzing`, StatusReviewing},
		{"working", `(?i)working on|executing|processing`, StatusWorking},
	}

	for _, d := range defaults {
		_ = pr.Register(d.name, d.pattern, d.status)
	}
}

// Register adds a new pattern to the registry.
// Returns an error if the regex pattern is invalid.
func (pr *PatternRegistry) Register(name, pattern string, status AgentStatus) error {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	pr.mu.Lock()
	defer pr.mu.Unlock()

	pr.patterns = append(pr.patterns, Pattern{
		Regex:  regex,
		Status: status,
		Name:   name,
	})

	return nil
}

// Detect scans output text for matching patterns.
// Returns the first matching status and pattern name, or empty values if no match.
func (pr *PatternRegistry) Detect(output string) (AgentStatus, string) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	for _, p := range pr.patterns {
		if p.Regex.MatchString(output) {
			return p.Status, p.Name
		}
	}

	return "", ""
}

// Clear removes all patterns from the registry.
func (pr *PatternRegistry) Clear() {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.patterns = make([]Pattern, 0)
}

// Count returns the number of registered patterns.
func (pr *PatternRegistry) Count() int {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return len(pr.patterns)
}
