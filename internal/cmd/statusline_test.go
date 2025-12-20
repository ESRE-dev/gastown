package cmd

import "testing"

func TestExtractRigFromSession(t *testing.T) {
	tests := []struct {
		session string
		want    string
	}{
		// Standard polecat sessions
		{"gt-gastown-slit", "gastown"},
		{"gt-gastown-Toast", "gastown"},
		{"gt-myrig-worker", "myrig"},

		// Crew sessions
		{"gt-gastown-crew-max", "gastown"},
		{"gt-myrig-crew-user", "myrig"},

		// Witness sessions (daemon.go style: gt-<rig>-witness)
		{"gt-gastown-witness", "gastown"},
		{"gt-myrig-witness", "myrig"},

		// Witness sessions (witness.go style: gt-witness-<rig>)
		{"gt-witness-gastown", "gastown"},
		{"gt-witness-myrig", "myrig"},

		// Refinery sessions
		{"gt-gastown-refinery", "gastown"},
		{"gt-myrig-refinery", "myrig"},

		// Edge cases
		{"gt-a-b", "a"},       // minimum valid
		{"gt-ab", ""},         // too short, no worker
		{"gt-", ""},           // invalid
		{"gt", ""},            // invalid
	}

	for _, tt := range tests {
		t.Run(tt.session, func(t *testing.T) {
			got := extractRigFromSession(tt.session)
			if got != tt.want {
				t.Errorf("extractRigFromSession(%q) = %q, want %q", tt.session, got, tt.want)
			}
		})
	}
}

func TestIsPolecatSession(t *testing.T) {
	tests := []struct {
		session string
		want    bool
	}{
		// Polecat sessions (should return true)
		{"gt-gastown-slit", true},
		{"gt-gastown-Toast", true},
		{"gt-myrig-worker", true},
		{"gt-a-b", true},

		// Non-polecat sessions (should return false)
		{"gt-gastown-witness", false},
		{"gt-witness-gastown", false},
		{"gt-gastown-refinery", false},
		{"gt-gastown-crew-max", false},
		{"gt-myrig-crew-user", false},
	}

	for _, tt := range tests {
		t.Run(tt.session, func(t *testing.T) {
			got := isPolecatSession(tt.session)
			if got != tt.want {
				t.Errorf("isPolecatSession(%q) = %v, want %v", tt.session, got, tt.want)
			}
		})
	}
}
