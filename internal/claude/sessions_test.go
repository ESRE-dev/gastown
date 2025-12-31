package claude

import (
	"testing"
)

func TestDecodePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"-Users-stevey-gt-gastown", "/Users/stevey/gt/gastown"},
		{"-Users-stevey-gt-beads-crew-joe", "/Users/stevey/gt/beads/crew/joe"},
		{"-Users-stevey", "/Users/stevey"},
		{"foo-bar", "foo/bar"}, // Edge case: no leading dash
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := decodePath(tt.input)
			if result != tt.expected {
				t.Errorf("decodePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGasTownPattern(t *testing.T) {
	tests := []struct {
		input       string
		shouldMatch bool
		role        string
		topic       string
	}{
		{
			input:       "[GAS TOWN] gastown/polecats/furiosa • ready • 2025-12-30T22:49",
			shouldMatch: true,
			role:        "gastown/polecats/furiosa",
			topic:       "ready",
		},
		{
			input:       "[GAS TOWN] deacon • patrol • 2025-12-30T08:00",
			shouldMatch: true,
			role:        "deacon",
			topic:       "patrol",
		},
		{
			input:       "[GAS TOWN] gastown/crew/gus • assigned:gt-abc12 • 2025-12-30T15:42",
			shouldMatch: true,
			role:        "gastown/crew/gus",
			topic:       "assigned:gt-abc12",
		},
		{
			input:       "Regular message without beacon",
			shouldMatch: false,
		},
		{
			input:       "[GAS TOWN] witness • handoff",
			shouldMatch: true,
			role:        "witness",
			topic:       "handoff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			match := gasTownPattern.FindStringSubmatch(tt.input)
			if tt.shouldMatch && match == nil {
				t.Errorf("Expected match for %q but got none", tt.input)
				return
			}
			if !tt.shouldMatch && match != nil {
				t.Errorf("Expected no match for %q but got %v", tt.input, match)
				return
			}
			if tt.shouldMatch {
				if match[1] != tt.role {
					t.Errorf("Role: got %q, want %q", match[1], tt.role)
				}
				if len(match) > 2 && match[2] != tt.topic {
					// Topic might have trailing space, trim for comparison
					gotTopic := match[2]
					if gotTopic != tt.topic {
						t.Errorf("Topic: got %q, want %q", gotTopic, tt.topic)
					}
				}
			}
		})
	}
}

func TestSessionInfoShortID(t *testing.T) {
	s := SessionInfo{ID: "d6d8475f-94a9-4a66-bfa6-d60126964427"}
	short := s.ShortID()
	if short != "d6d8475f" {
		t.Errorf("ShortID() = %q, want %q", short, "d6d8475f")
	}

	s2 := SessionInfo{ID: "abc"}
	if s2.ShortID() != "abc" {
		t.Errorf("ShortID() for short ID = %q, want %q", s2.ShortID(), "abc")
	}
}

func TestInferRoleFromPath(t *testing.T) {
	s := SessionInfo{Path: "/Users/stevey/gt/gastown/crew/joe"}
	rig := s.RigFromPath()
	if rig != "gastown" {
		t.Errorf("RigFromPath() = %q, want %q", rig, "gastown")
	}

	s2 := SessionInfo{Path: "/Users/stevey/gt/beads/polecats/jade"}
	if s2.RigFromPath() != "beads" {
		t.Errorf("RigFromPath() = %q, want %q", s2.RigFromPath(), "beads")
	}
}
