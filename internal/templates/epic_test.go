package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTemplate_Valid(t *testing.T) {
	yaml := `name: basic-batch
description: Simple batch work pattern
phases:
  - name: startup
    issues:
      - title: "Verify workers ready"
        type: task
        priority: 1
  - name: working
  - name: cleanup
    issues:
      - title: "Merge all branches"
      - title: "Clean up workers"
      - title: "Report to Mayor"
`

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "template.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	tmpl, err := LoadTemplate(path)
	if err != nil {
		t.Fatalf("LoadTemplate() error = %v", err)
	}

	if tmpl.Name != "basic-batch" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "basic-batch")
	}
	if tmpl.Description != "Simple batch work pattern" {
		t.Errorf("Description = %q, want %q", tmpl.Description, "Simple batch work pattern")
	}
	if len(tmpl.Phases) != 3 {
		t.Fatalf("len(Phases) = %d, want 3", len(tmpl.Phases))
	}

	// Check first phase
	if tmpl.Phases[0].Name != "startup" {
		t.Errorf("Phases[0].Name = %q, want %q", tmpl.Phases[0].Name, "startup")
	}
	if len(tmpl.Phases[0].Issues) != 1 {
		t.Fatalf("len(Phases[0].Issues) = %d, want 1", len(tmpl.Phases[0].Issues))
	}
	if tmpl.Phases[0].Issues[0].Title != "Verify workers ready" {
		t.Errorf("Issue title = %q, want %q", tmpl.Phases[0].Issues[0].Title, "Verify workers ready")
	}
	if tmpl.Phases[0].Issues[0].Type != "task" {
		t.Errorf("Issue type = %q, want %q", tmpl.Phases[0].Issues[0].Type, "task")
	}
	if tmpl.Phases[0].Issues[0].Priority != 1 {
		t.Errorf("Issue priority = %d, want 1", tmpl.Phases[0].Issues[0].Priority)
	}

	// Check second phase (no issues)
	if tmpl.Phases[1].Name != "working" {
		t.Errorf("Phases[1].Name = %q, want %q", tmpl.Phases[1].Name, "working")
	}
	if len(tmpl.Phases[1].Issues) != 0 {
		t.Errorf("len(Phases[1].Issues) = %d, want 0", len(tmpl.Phases[1].Issues))
	}

	// Check third phase
	if tmpl.Phases[2].Name != "cleanup" {
		t.Errorf("Phases[2].Name = %q, want %q", tmpl.Phases[2].Name, "cleanup")
	}
	if len(tmpl.Phases[2].Issues) != 3 {
		t.Fatalf("len(Phases[2].Issues) = %d, want 3", len(tmpl.Phases[2].Issues))
	}
}

func TestLoadTemplate_InvalidYAML(t *testing.T) {
	yaml := `name: bad-template
description: This is invalid
phases:
  - name: startup
    issues:
      - title: "Unclosed string
`

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadTemplate(path)
	if err == nil {
		t.Fatal("LoadTemplate() expected error for invalid YAML, got nil")
	}
	if !strings.Contains(err.Error(), "parsing YAML") {
		t.Errorf("Error should mention YAML parsing, got: %v", err)
	}
}

func TestLoadTemplate_FileNotFound(t *testing.T) {
	_, err := LoadTemplate("/nonexistent/path/template.yaml")
	if err == nil {
		t.Fatal("LoadTemplate() expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "reading template file") {
		t.Errorf("Error should mention reading file, got: %v", err)
	}
}

func TestValidate_MissingName(t *testing.T) {
	tmpl := &EpicTemplate{
		Name:        "",
		Description: "Test",
		Phases: []Phase{
			{Name: "test", Issues: []IssueTemplate{}},
		},
	}

	err := tmpl.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for missing name, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Error should mention missing name, got: %v", err)
	}
}

func TestValidate_NoPhases(t *testing.T) {
	tmpl := &EpicTemplate{
		Name:        "test",
		Description: "Test",
		Phases:      []Phase{},
	}

	err := tmpl.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for no phases, got nil")
	}
	if !strings.Contains(err.Error(), "at least one phase") {
		t.Errorf("Error should mention missing phases, got: %v", err)
	}
}

func TestValidate_MissingPhaseName(t *testing.T) {
	tmpl := &EpicTemplate{
		Name:        "test",
		Description: "Test",
		Phases: []Phase{
			{Name: "", Issues: []IssueTemplate{}},
		},
	}

	err := tmpl.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for missing phase name, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Error should mention missing phase name, got: %v", err)
	}
}

func TestValidate_DuplicatePhaseName(t *testing.T) {
	tmpl := &EpicTemplate{
		Name:        "test",
		Description: "Test",
		Phases: []Phase{
			{Name: "startup", Issues: []IssueTemplate{}},
			{Name: "startup", Issues: []IssueTemplate{}},
		},
	}

	err := tmpl.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for duplicate phase names, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate phase name") {
		t.Errorf("Error should mention duplicate phase name, got: %v", err)
	}
}

func TestValidate_MissingIssueTitle(t *testing.T) {
	tmpl := &EpicTemplate{
		Name:        "test",
		Description: "Test",
		Phases: []Phase{
			{
				Name: "startup",
				Issues: []IssueTemplate{
					{Title: ""},
				},
			},
		},
	}

	err := tmpl.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for missing issue title, got nil")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("Error should mention missing title, got: %v", err)
	}
}

func TestValidate_InvalidIssueType(t *testing.T) {
	tmpl := &EpicTemplate{
		Name:        "test",
		Description: "Test",
		Phases: []Phase{
			{
				Name: "startup",
				Issues: []IssueTemplate{
					{Title: "Test", Type: "invalid-type"},
				},
			},
		},
	}

	err := tmpl.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for invalid issue type, got nil")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("Error should mention invalid type, got: %v", err)
	}
}

func TestValidate_InvalidPriority(t *testing.T) {
	tests := []struct {
		name     string
		priority int
	}{
		{"negative", -1},
		{"too high", 5},
		{"way too high", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &EpicTemplate{
				Name:        "test",
				Description: "Test",
				Phases: []Phase{
					{
						Name: "startup",
						Issues: []IssueTemplate{
							{Title: "Test", Priority: tt.priority},
						},
					},
				},
			}

			err := tmpl.Validate()
			if err == nil {
				t.Fatalf("Validate() expected error for priority %d, got nil", tt.priority)
			}
			if !strings.Contains(err.Error(), "priority must be 0-4") {
				t.Errorf("Error should mention priority range, got: %v", err)
			}
		})
	}
}

func TestValidate_ValidTypes(t *testing.T) {
	validTypes := []string{"task", "bug", "feature", "epic"}

	for _, typ := range validTypes {
		t.Run(typ, func(t *testing.T) {
			tmpl := &EpicTemplate{
				Name:        "test",
				Description: "Test",
				Phases: []Phase{
					{
						Name: "startup",
						Issues: []IssueTemplate{
							{Title: "Test", Type: typ},
						},
					},
				},
			}

			err := tmpl.Validate()
			if err != nil {
				t.Errorf("Validate() unexpected error for type %q: %v", typ, err)
			}
		})
	}
}

func TestInstantiate(t *testing.T) {
	tmpl := &EpicTemplate{
		Name:        "basic-batch",
		Description: "Simple batch work pattern",
		Phases: []Phase{
			{
				Name: "startup",
				Issues: []IssueTemplate{
					{
						Title:    "Verify workers ready",
						Type:     "task",
						Priority: 1,
						Labels:   []string{"urgent"},
					},
				},
			},
			{
				Name: "working",
			},
			{
				Name: "cleanup",
				Issues: []IssueTemplate{
					{Title: "Merge all branches"},
					{Title: "Clean up workers", Type: "task", Priority: 2},
					{Title: "Report to Mayor", Description: "Send final report"},
				},
			},
		},
	}

	issues, err := tmpl.Instantiate("grr-123")
	if err != nil {
		t.Fatalf("Instantiate() error = %v", err)
	}

	if len(issues) != 4 {
		t.Fatalf("len(issues) = %d, want 4", len(issues))
	}

	// Check first issue
	if issues[0].Title != "Verify workers ready" {
		t.Errorf("issues[0].Title = %q, want %q", issues[0].Title, "Verify workers ready")
	}
	if issues[0].Type != "task" {
		t.Errorf("issues[0].Type = %q, want %q", issues[0].Type, "task")
	}
	if issues[0].Priority != 1 {
		t.Errorf("issues[0].Priority = %d, want 1", issues[0].Priority)
	}
	if issues[0].Phase != "startup" {
		t.Errorf("issues[0].Phase = %q, want %q", issues[0].Phase, "startup")
	}
	// Check labels
	if !containsString(issues[0].Labels, "urgent") {
		t.Error("issues[0].Labels missing 'urgent'")
	}
	if !containsString(issues[0].Labels, "epic:grr-123:startup") {
		t.Error("issues[0].Labels missing 'epic:grr-123:startup'")
	}
	if !containsString(issues[0].Labels, "template:basic-batch") {
		t.Error("issues[0].Labels missing 'template:basic-batch'")
	}

	// Check cleanup issues
	if issues[1].Title != "Merge all branches" {
		t.Errorf("issues[1].Title = %q, want %q", issues[1].Title, "Merge all branches")
	}
	if issues[1].Type != "task" {
		t.Errorf("issues[1].Type = %q, want 'task' (default)", issues[1].Type)
	}
	if issues[1].Phase != "cleanup" {
		t.Errorf("issues[1].Phase = %q, want %q", issues[1].Phase, "cleanup")
	}

	if issues[2].Title != "Clean up workers" {
		t.Errorf("issues[2].Title = %q, want %q", issues[2].Title, "Clean up workers")
	}
	if issues[2].Priority != 2 {
		t.Errorf("issues[2].Priority = %d, want 2", issues[2].Priority)
	}

	if issues[3].Title != "Report to Mayor" {
		t.Errorf("issues[3].Title = %q, want %q", issues[3].Title, "Report to Mayor")
	}
	if issues[3].Description != "Send final report" {
		t.Errorf("issues[3].Description = %q, want %q", issues[3].Description, "Send final report")
	}
}

func TestInstantiate_NoEpicID(t *testing.T) {
	tmpl := &EpicTemplate{
		Name:        "test",
		Description: "Test",
		Phases: []Phase{
			{
				Name: "startup",
				Issues: []IssueTemplate{
					{Title: "Test task"},
				},
			},
		},
	}

	issues, err := tmpl.Instantiate("")
	if err != nil {
		t.Fatalf("Instantiate() error = %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}

	// Should use phase label instead of epic label when no epic ID
	if !containsString(issues[0].Labels, "phase:startup") {
		t.Error("issues[0].Labels missing 'phase:startup'")
	}
	if containsString(issues[0].Labels, "epic:") {
		t.Error("issues[0].Labels should not have epic label when epicID is empty")
	}
}

func TestInstantiate_InvalidTemplate(t *testing.T) {
	tmpl := &EpicTemplate{
		Name:        "",
		Description: "Invalid",
		Phases:      []Phase{},
	}

	_, err := tmpl.Instantiate("grr-123")
	if err == nil {
		t.Fatal("Instantiate() expected error for invalid template, got nil")
	}
}

// Helper function
func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
