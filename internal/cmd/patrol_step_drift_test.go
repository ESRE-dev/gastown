package cmd

import (
	"testing"
)

func TestStepDriftCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range patrolCmd.Commands() {
		if cmd.Use == "step-drift [interval]" {
			found = true
			break
		}
	}
	if !found {
		t.Error("step-drift subcommand not registered under patrol")
	}
}

func TestStepDriftCmd_HasFlags(t *testing.T) {
	flags := []string{"agent", "nudge", "threshold", "watch"}
	for _, name := range flags {
		if patrolStepDriftCmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag: --%s", name)
		}
	}
	// Check -w shorthand for --watch
	if patrolStepDriftCmd.Flags().ShorthandLookup("w") == nil {
		t.Error("missing shorthand -w for --watch")
	}
}

func TestStepDriftCmd_ThresholdDefault(t *testing.T) {
	f := patrolStepDriftCmd.Flags().Lookup("threshold")
	if f == nil {
		t.Fatal("threshold flag not found")
	}
	if f.DefValue != "5" {
		t.Errorf("threshold default = %q, want %q", f.DefValue, "5")
	}
}

func TestMatchStep(t *testing.T) {
	statuses := map[string]bool{
		"Load context and start":      true,
		"Set up working branch":       true,
		"Verify tests pass (precheck)": false,
		"Implement the feature":       false,
	}

	tests := []struct {
		name string
		want bool
	}{
		{"Load context", true},
		{"Set up working branch", true},
		{"Verify tests pass", false},
		{"Implement", false},
		{"Self-review", false}, // not present at all
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchStep(tt.name, statuses)
			if got != tt.want {
				t.Errorf("matchStep(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestCountClosedSteps(t *testing.T) {
	// All closed
	all := map[string]bool{
		"Load context":          true,
		"Set up working branch": true,
		"Verify tests pass":     true,
		"Implement":             true,
		"Self-review":           true,
		"Run tests":             true,
		"Clean up":              true,
		"Prepare work":          true,
		"Submit work":           true,
	}
	if got := countClosedSteps(all); got != 9 {
		t.Errorf("countClosedSteps(all) = %d, want 9", got)
	}

	// None closed
	none := map[string]bool{
		"Load context":          false,
		"Set up working branch": false,
	}
	if got := countClosedSteps(none); got != 0 {
		t.Errorf("countClosedSteps(none) = %d, want 0", got)
	}

	// Nil map
	if got := countClosedSteps(nil); got != 0 {
		t.Errorf("countClosedSteps(nil) = %d, want 0", got)
	}
}

func TestRoundTo1(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{12.34, 12.3},
		{0.0, 0.0},
		{5.99, 5.9},
		{100.05, 100.0},
	}
	for _, tt := range tests {
		got := roundTo1(tt.input)
		if got != tt.want {
			t.Errorf("roundTo1(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestStepDriftResult_JSON(t *testing.T) {
	r := StepDriftResult{
		Rig:      "gastown",
		Name:     "alpha",
		Bead:     "gt-abc123",
		Title:    "Test bead",
		State:    "working",
		AgeMin:   12.3,
		Closed:   3,
		Total:    9,
		Drifting: false,
		Nudged:   false,
		Branch:   "polecat-alpha-1234567890",
	}

	if r.Rig != "gastown" {
		t.Errorf("Rig = %q, want %q", r.Rig, "gastown")
	}
	if r.Total != 9 {
		t.Errorf("Total = %d, want 9", r.Total)
	}
	if r.Drifting {
		t.Error("Drifting should be false when closed > 0")
	}
}

func TestStepsOrder(t *testing.T) {
	if len(stepsOrder) != 9 {
		t.Errorf("stepsOrder has %d entries, want 9", len(stepsOrder))
	}
	if stepsOrder[0] != "Load context" {
		t.Errorf("first step = %q, want %q", stepsOrder[0], "Load context")
	}
	if stepsOrder[8] != "Submit work" {
		t.Errorf("last step = %q, want %q", stepsOrder[8], "Submit work")
	}
}
