//go:build !windows

package session

import (
	"testing"
)

func TestBuildAgentLogArgs(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		workDir   string
		since     string
		runID     string
		agentName string
		wantAgent []string // expected ["--agent", value] subsequence; nil if absent
	}{
		{
			name:      "opencode agent",
			sessionID: "sess-1",
			workDir:   "/tmp/work",
			since:     "2025-01-01T00:00:00Z",
			runID:     "run-1",
			agentName: "opencode",
			wantAgent: []string{"--agent", "opencode"},
		},
		{
			name:      "claudecode agent",
			sessionID: "sess-2",
			workDir:   "/tmp/work2",
			since:     "2025-01-01T00:00:00Z",
			runID:     "run-2",
			agentName: "claudecode",
			wantAgent: []string{"--agent", "claudecode"},
		},
		{
			name:      "empty agent preserves backward compat",
			sessionID: "sess-3",
			workDir:   "/tmp/work3",
			since:     "2025-01-01T00:00:00Z",
			runID:     "run-3",
			agentName: "",
			wantAgent: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildAgentLogArgs(tt.sessionID, tt.workDir, tt.since, tt.runID, tt.agentName)

			if tt.wantAgent != nil {
				idx := indexOfSubsequence(args, tt.wantAgent)
				if idx < 0 {
					t.Errorf("args %v missing subsequence %v", args, tt.wantAgent)
				}
			} else {
				// --agent must NOT appear
				for i, a := range args {
					if a == "--agent" {
						t.Errorf("args %v contains unexpected --agent at index %d", args, i)
						break
					}
				}
			}

			// Base flags must always be present.
			if args[0] != "agent-log" {
				t.Errorf("args[0] = %q, want %q", args[0], "agent-log")
			}
			if idx := indexOfSubsequence(args, []string{"--session", tt.sessionID}); idx < 0 {
				t.Errorf("args %v missing --session %s", args, tt.sessionID)
			}
			if idx := indexOfSubsequence(args, []string{"--work-dir", tt.workDir}); idx < 0 {
				t.Errorf("args %v missing --work-dir %s", args, tt.workDir)
			}
		})
	}
}

// indexOfSubsequence returns the index where sub starts in slice, or -1.
func indexOfSubsequence(slice, sub []string) int {
	if len(sub) == 0 || len(sub) > len(slice) {
		return -1
	}
	for i := 0; i <= len(slice)-len(sub); i++ {
		match := true
		for j := range sub {
			if slice[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
