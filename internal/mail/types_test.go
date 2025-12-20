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

		// Rig-level agents use dash separator
		{"gastown/Toast", "gastown-Toast"},
		{"gastown/refinery", "gastown-refinery"},
		{"gastown/witness", "gastown-witness"},

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

		// Rig-level agents
		{"gastown-Toast", "gastown/Toast"},
		{"gastown-refinery", "gastown/refinery"},
		{"gastown-witness", "gastown/witness"},

		// Rig name only (adds trailing slash)
		{"gastown", "gastown/"},
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
