package cmd

import "testing"

func TestMatchesSlingTarget(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		assignee  string
		selfAgent string
		want      bool
	}{
		{
			name:     "exact match",
			target:   "gastown/polecats/toast",
			assignee: "gastown/polecats/toast",
			want:     true,
		},
		{
			name:     "target with trailing slash matches mayor assignee",
			target:   "mayor",
			assignee: "mayor/",
			want:     true,
		},
		{
			name:     "rig namespace target matches existing polecat assignment",
			target:   "gastown",
			assignee: "gastown/polecats/toast",
			want:     true,
		},
		{
			name:     "dog namespace target matches specific dog assignment",
			target:   "deacon/dogs",
			assignee: "deacon/dogs/alpha",
			want:     true,
		},
		{
			name:     "rig name shorthand matches polecat",
			target:   "gastown/toast",
			assignee: "gastown/polecats/toast",
			want:     true,
		},
		{
			name:     "rig name shorthand matches crew",
			target:   "gastown/alex",
			assignee: "gastown/crew/alex",
			want:     true,
		},
		{
			name:      "self target matches self assignee",
			target:    ".",
			assignee:  "gastown/crew/alex",
			selfAgent: "gastown/crew/alex",
			want:      true,
		},
		{
			name:     "different target does not match",
			target:   "gastown/polecats/other",
			assignee: "gastown/polecats/toast",
			want:     false,
		},
		{
			name:     "rig target does not match non-polecat assignee",
			target:   "gastown",
			assignee: "gastown/crew/alex",
			want:     false,
		},
		{
			name:     "empty assignee never matches",
			target:   "gastown/polecats/toast",
			assignee: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesSlingTarget(tt.target, tt.assignee, tt.selfAgent)
			if got != tt.want {
				t.Fatalf("matchesSlingTarget(%q, %q, %q) = %v, want %v",
					tt.target, tt.assignee, tt.selfAgent, got, tt.want)
			}
		})
	}
}
