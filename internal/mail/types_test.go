package mail

import "testing"

func TestAddressToIdentity(t *testing.T) {
	tests := []struct {
		address  string
		expected string
	}{
		// Town-level agents keep trailing slash
		{"mayor", "mayor/"},
		{"mayor/", "mayor/"},
		{"deacon", "deacon/"},
		{"deacon/", "deacon/"},

		// Rig-level agents: crew/ and polecats/ normalized to canonical form
		{"gastown/polecats/Toast", "gastown/Toast"},
		{"gastown/crew/max", "gastown/max"},
		{"gastown/Toast", "gastown/Toast"},         // Already canonical
		{"gastown/max", "gastown/max"},             // Already canonical
		{"gastown/refinery", "gastown/refinery"},
		{"gastown/witness", "gastown/witness"},

		// Rig broadcast (trailing slash removed)
		{"gastown/", "gastown"},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			got := addressToIdentity(tt.address)
			if got != tt.expected {
				t.Errorf("addressToIdentity(%q) = %q, want %q", tt.address, got, tt.expected)
			}
		})
	}
}

func TestIdentityToAddress(t *testing.T) {
	tests := []struct {
		identity string
		expected string
	}{
		// Town-level agents
		{"mayor", "mayor/"},
		{"mayor/", "mayor/"},
		{"deacon", "deacon/"},
		{"deacon/", "deacon/"},

		// Rig-level agents: crew/ and polecats/ normalized
		{"gastown/polecats/Toast", "gastown/Toast"},
		{"gastown/crew/max", "gastown/max"},
		{"gastown/Toast", "gastown/Toast"},  // Already canonical
		{"gastown/refinery", "gastown/refinery"},
		{"gastown/witness", "gastown/witness"},

		// Rig name only (no transformation)
		{"gastown", "gastown"},
	}

	for _, tt := range tests {
		t.Run(tt.identity, func(t *testing.T) {
			got := identityToAddress(tt.identity)
			if got != tt.expected {
				t.Errorf("identityToAddress(%q) = %q, want %q", tt.identity, got, tt.expected)
			}
		})
	}
}
