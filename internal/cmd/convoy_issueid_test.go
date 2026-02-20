package cmd

import (
	"testing"

	"github.com/steveyegge/gastown/internal/session"
)

func TestLooksLikeIssueID(t *testing.T) {
	originalRegistry := session.DefaultRegistry()
	defer session.SetDefaultRegistry(originalRegistry)

	testRegistry := session.NewPrefixRegistry()
	testRegistry.Register("nx", "nexus")
	testRegistry.Register("rpk", "nrpk")
	testRegistry.Register("longpfx", "longprefix")
	session.SetDefaultRegistry(testRegistry)

	tests := []struct {
		input string
		want  bool
	}{
		{"gt-abc123", true},
		{"bd-xyz789", true},
		{"hq-mayor", true},
		{"nx-def456", true},
		{"rpk-ghi012", true},
		{"longpfx-jkl345", true},
		{"nv-short", true},
		{"ab-min", true},
		{"abcdef-max6", true},
		{"notvalid", false},
		{"no-hyphen-after", true},
		{"A-uppercase", false},
		{"1-number", false},
		{"", false},
		{"-noprefix", false},
		{"a-tooshort", false},
		{"abcdefg-toolong", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := looksLikeIssueID(tc.input)
			if got != tc.want {
				t.Errorf("looksLikeIssueID(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
