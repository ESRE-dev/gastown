package plan

import (
	"strings"
	"testing"
)

func TestConvert_BasicStructure(t *testing.T) {
	input := `# My Project

## Phase 1: Setup
- [ ] Task 1
- [ ] Task 2

## Phase 2: Implementation
- [ ] Task 3
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	epic, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	if epic.Title != "My Project" {
		t.Errorf("Expected title 'My Project', got '%s'", epic.Title)
	}

	if len(epic.Children) != 2 {
		t.Fatalf("Expected 2 children, got %d", len(epic.Children))
	}

	// Check Phase 1
	phase1 := epic.Children[0]
	if phase1.Title != "Phase 1: Setup" {
		t.Errorf("Expected title 'Phase 1: Setup', got '%s'", phase1.Title)
	}
	if len(phase1.Children) != 2 {
		t.Errorf("Expected 2 tasks in Phase 1, got %d", len(phase1.Children))
	}
	if len(phase1.DependsOn) != 0 {
		t.Errorf("Expected Phase 1 to have no dependencies, got %v", phase1.DependsOn)
	}

	// Check Phase 2
	phase2 := epic.Children[1]
	if phase2.Title != "Phase 2: Implementation" {
		t.Errorf("Expected title 'Phase 2: Implementation', got '%s'", phase2.Title)
	}
	if len(phase2.DependsOn) != 1 || phase2.DependsOn[0] != 0 {
		t.Errorf("Expected Phase 2 to depend on Phase 1 (index 0), got %v", phase2.DependsOn)
	}
}

func TestConvert_SequentialPhases(t *testing.T) {
	input := `# Project

## Phase 1
- [ ] Task A

## Phase 2
- [ ] Task B

## Phase 3
- [ ] Task C
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	epic, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	if len(epic.Children) != 3 {
		t.Fatalf("Expected 3 phases, got %d", len(epic.Children))
	}

	// Phase 1 has no dependencies
	if len(epic.Children[0].DependsOn) != 0 {
		t.Errorf("Phase 1 should have no dependencies, got %v", epic.Children[0].DependsOn)
	}

	// Phase 2 depends on Phase 1
	if len(epic.Children[1].DependsOn) != 1 || epic.Children[1].DependsOn[0] != 0 {
		t.Errorf("Phase 2 should depend on Phase 1, got %v", epic.Children[1].DependsOn)
	}

	// Phase 3 depends on Phase 2
	if len(epic.Children[2].DependsOn) != 1 || epic.Children[2].DependsOn[0] != 1 {
		t.Errorf("Phase 3 should depend on Phase 2, got %v", epic.Children[2].DependsOn)
	}
}

func TestConvert_NumberedItemsAreSequential(t *testing.T) {
	input := `# Project

## Phase 1
1. First step
2. Second step
3. Third step
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	epic, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	phase1 := epic.Children[0]
	if !phase1.Sequential {
		t.Error("Expected Phase 1 to be sequential (has numbered items)")
	}
}

func TestConvert_CheckboxItemsAreParallel(t *testing.T) {
	input := `# Project

## Phase 1
- [ ] Task 1
- [ ] Task 2
- [ ] Task 3
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	epic, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	phase1 := epic.Children[0]
	if phase1.Sequential {
		t.Error("Expected Phase 1 to be parallel (has checkbox items)")
	}
}

func TestConvert_IssueTypeDetection(t *testing.T) {
	tests := []struct {
		title        string
		expectedType string
	}{
		{"Fix authentication bug", "bug"},
		{"Bug: Login fails", "bug"},
		{"Add new feature", "feature"},
		{"Implement user dashboard", "feature"},
		{"Testing phase", "task"},
		{"QA validation", "task"},
		{"Phase 1: Setup", "feature"},
		{"Regular task", "task"},
	}

	for _, tt := range tests {
		actualType := determineIssueType(tt.title)
		if actualType != tt.expectedType {
			t.Errorf("For title '%s', expected type '%s', got '%s'",
				tt.title, tt.expectedType, actualType)
		}
	}
}

func TestConvert_NestedItems(t *testing.T) {
	input := `# Project

## Phase 1
- [ ] Parent task
  - [ ] Child task 1
  - [ ] Child task 2
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	epic, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	phase1 := epic.Children[0]
	if len(phase1.Children) != 1 {
		t.Fatalf("Expected 1 parent task, got %d", len(phase1.Children))
	}

	parentTask := phase1.Children[0]
	if parentTask.Title != "Parent task" {
		t.Errorf("Expected parent title 'Parent task', got '%s'", parentTask.Title)
	}
	if len(parentTask.Children) != 2 {
		t.Errorf("Expected 2 child tasks, got %d", len(parentTask.Children))
	}
}

func TestConvert_Description(t *testing.T) {
	input := `# Project

## Phase 1: Setup
## Phase 2: Implementation
## Phase 3: Testing
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	epic, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	expectedDesc := "Plan phases:\nPhase 1: Setup\nPhase 2: Implementation\nPhase 3: Testing"
	if epic.Description != expectedDesc {
		t.Errorf("Expected description:\n%s\nGot:\n%s", expectedDesc, epic.Description)
	}
}

func TestConvert_EmptyDocument(t *testing.T) {
	doc := &PlanDocument{
		Title:    "",
		Sections: []PlanSection{},
	}

	epic, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	if epic.Title != "Untitled Plan" {
		t.Errorf("Expected default title 'Untitled Plan', got '%s'", epic.Title)
	}
	if len(epic.Children) != 0 {
		t.Errorf("Expected 0 children, got %d", len(epic.Children))
	}
}

func TestConvert_NilDocument(t *testing.T) {
	_, err := Convert(nil)
	if err == nil {
		t.Error("Expected error for nil document")
	}
}

func TestConvert_RealWorldExample(t *testing.T) {
	input := `# Microservices Migration

## Phase 1: Infrastructure
- [ ] Set up Kubernetes
  - [ ] Configure namespaces
  - [ ] Set up ingress
- [ ] Deploy monitoring
  - [ ] Prometheus
  - [ ] Grafana

## Phase 2: Service Migration
1. Migrate auth service
2. Migrate user service
3. Migrate order service

## Phase 3: Testing
- [ ] Integration tests
- [ ] Load testing
- [ ] Security audit
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	epic, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	if epic.Title != "Microservices Migration" {
		t.Errorf("Expected title 'Microservices Migration', got '%s'", epic.Title)
	}

	if len(epic.Children) != 3 {
		t.Fatalf("Expected 3 phases, got %d", len(epic.Children))
	}

	// Check Phase 1
	phase1 := epic.Children[0]
	if phase1.Title != "Phase 1: Infrastructure" {
		t.Errorf("Expected title 'Phase 1: Infrastructure', got '%s'", phase1.Title)
	}
	if phase1.Sequential {
		t.Error("Phase 1 should be parallel (checkbox items)")
	}
	if len(phase1.Children) != 2 {
		t.Errorf("Expected 2 tasks in Phase 1, got %d", len(phase1.Children))
	}

	// Check nested children
	task1 := phase1.Children[0]
	if len(task1.Children) != 2 {
		t.Errorf("Expected 2 nested tasks under first task, got %d", len(task1.Children))
	}

	// Check Phase 2 is sequential
	phase2 := epic.Children[1]
	if !phase2.Sequential {
		t.Error("Phase 2 should be sequential (numbered items)")
	}
	if len(phase2.Children) != 3 {
		t.Errorf("Expected 3 services in Phase 2, got %d", len(phase2.Children))
	}

	// Check Phase 2 depends on Phase 1
	if len(phase2.DependsOn) != 1 || phase2.DependsOn[0] != 0 {
		t.Errorf("Phase 2 should depend on Phase 1, got %v", phase2.DependsOn)
	}

	// Check Phase 3 depends on Phase 2
	phase3 := epic.Children[2]
	if len(phase3.DependsOn) != 1 || phase3.DependsOn[0] != 1 {
		t.Errorf("Phase 3 should depend on Phase 2, got %v", phase3.DependsOn)
	}
}

func TestConvert_ItemDescription(t *testing.T) {
	input := `# Project

## Phase 1
- [ ] Main task
  - [ ] Sub-task 1
  - [ ] Sub-task 2
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	epic, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	phase1 := epic.Children[0]
	mainTask := phase1.Children[0]

	// Check that sub-tasks appear in description
	if !strings.Contains(mainTask.Description, "Sub-task 1") {
		t.Error("Expected description to contain 'Sub-task 1'")
	}
	if !strings.Contains(mainTask.Description, "Sub-task 2") {
		t.Error("Expected description to contain 'Sub-task 2'")
	}
}
