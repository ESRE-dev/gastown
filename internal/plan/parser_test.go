package plan

import (
	"strings"
	"testing"
)

func TestParseMarkdown_BasicStructure(t *testing.T) {
	input := `# My Project Plan

## Phase 1: Setup
- [ ] Initialize database schema
- [ ] Configure auth service

## Phase 2: Implementation
- [ ] Build API endpoints
- [ ] Create frontend components
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	if doc.Title != "My Project Plan" {
		t.Errorf("Expected title 'My Project Plan', got '%s'", doc.Title)
	}

	if len(doc.Sections) != 2 {
		t.Fatalf("Expected 2 sections, got %d", len(doc.Sections))
	}

	// Check Phase 1
	phase1 := doc.Sections[0]
	if phase1.Title != "Phase 1: Setup" {
		t.Errorf("Expected section 'Phase 1: Setup', got '%s'", phase1.Title)
	}
	if phase1.Level != 2 {
		t.Errorf("Expected level 2, got %d", phase1.Level)
	}
	if len(phase1.Items) != 2 {
		t.Errorf("Expected 2 items in Phase 1, got %d", len(phase1.Items))
	}

	// Check first item
	item1 := phase1.Items[0]
	if item1.Text != "Initialize database schema" {
		t.Errorf("Expected item text 'Initialize database schema', got '%s'", item1.Text)
	}
	if !item1.IsCheckbox {
		t.Error("Expected item to be checkbox")
	}
	if item1.Checked {
		t.Error("Expected item to be unchecked")
	}

	// Check Phase 2
	phase2 := doc.Sections[1]
	if phase2.Title != "Phase 2: Implementation" {
		t.Errorf("Expected section 'Phase 2: Implementation', got '%s'", phase2.Title)
	}
	if len(phase2.Items) != 2 {
		t.Errorf("Expected 2 items in Phase 2, got %d", len(phase2.Items))
	}
}

func TestParseMarkdown_CheckedItems(t *testing.T) {
	input := `# Plan
## Tasks
- [x] Completed task
- [ ] Pending task
- [X] Another completed task
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	section := doc.Sections[0]
	if len(section.Items) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(section.Items))
	}

	if !section.Items[0].Checked {
		t.Error("Expected first item to be checked")
	}
	if section.Items[1].Checked {
		t.Error("Expected second item to be unchecked")
	}
	if !section.Items[2].Checked {
		t.Error("Expected third item to be checked")
	}
}

func TestParseMarkdown_NestedItems(t *testing.T) {
	input := `# Plan
## Tasks
- [ ] Parent task
  - [ ] Child task 1
  - [ ] Child task 2
    - [ ] Grandchild task
- [ ] Another parent task
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	section := doc.Sections[0]
	if len(section.Items) != 2 {
		t.Fatalf("Expected 2 top-level items, got %d", len(section.Items))
	}

	// Check first parent
	parent1 := section.Items[0]
	if parent1.Text != "Parent task" {
		t.Errorf("Expected parent text 'Parent task', got '%s'", parent1.Text)
	}
	if len(parent1.Children) != 2 {
		t.Fatalf("Expected 2 children, got %d", len(parent1.Children))
	}

	// Check child
	child1 := parent1.Children[0]
	if child1.Text != "Child task 1" {
		t.Errorf("Expected child text 'Child task 1', got '%s'", child1.Text)
	}

	// Check grandchild
	child2 := parent1.Children[1]
	if len(child2.Children) != 1 {
		t.Fatalf("Expected 1 grandchild, got %d", len(child2.Children))
	}
	if child2.Children[0].Text != "Grandchild task" {
		t.Errorf("Expected grandchild text 'Grandchild task', got '%s'", child2.Children[0].Text)
	}
}

func TestParseMarkdown_NumberedLists(t *testing.T) {
	input := `# Plan
## Steps
1. First step
2. Second step
3. Third step
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	section := doc.Sections[0]
	if len(section.Items) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(section.Items))
	}

	for i, item := range section.Items {
		if !item.IsNumbered {
			t.Errorf("Expected item %d to be numbered", i)
		}
		if item.Number != i+1 {
			t.Errorf("Expected number %d, got %d", i+1, item.Number)
		}
	}
}

func TestParseMarkdown_MixedLists(t *testing.T) {
	input := `# Plan
## Phase 1
- [ ] Setup task
  1. Sub-step one
  2. Sub-step two
- [ ] Another task
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	section := doc.Sections[0]
	if len(section.Items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(section.Items))
	}

	// Check first item has numbered children
	item1 := section.Items[0]
	if len(item1.Children) != 2 {
		t.Fatalf("Expected 2 children, got %d", len(item1.Children))
	}
	if !item1.Children[0].IsNumbered {
		t.Error("Expected child to be numbered")
	}
	if !item1.Children[1].IsNumbered {
		t.Error("Expected child to be numbered")
	}
}

func TestParseMarkdown_EmptyInput(t *testing.T) {
	input := ``

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	if doc.Title != "" {
		t.Errorf("Expected empty title, got '%s'", doc.Title)
	}
	if len(doc.Sections) != 0 {
		t.Errorf("Expected 0 sections, got %d", len(doc.Sections))
	}
}

func TestParseMarkdown_NoTitle(t *testing.T) {
	input := `## Phase 1
- [ ] Task 1
- [ ] Task 2
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	if doc.Title != "" {
		t.Errorf("Expected empty title, got '%s'", doc.Title)
	}
	if len(doc.Sections) != 1 {
		t.Fatalf("Expected 1 section, got %d", len(doc.Sections))
	}
}

func TestParseMarkdown_NestedSections(t *testing.T) {
	input := `# Plan
## Phase 1
### Setup
- [ ] Task 1
### Configuration
- [ ] Task 2
## Phase 2
- [ ] Task 3
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	if len(doc.Sections) != 2 {
		t.Fatalf("Expected 2 top-level sections, got %d", len(doc.Sections))
	}

	phase1 := doc.Sections[0]
	if len(phase1.Children) != 2 {
		t.Fatalf("Expected 2 subsections in Phase 1, got %d", len(phase1.Children))
	}

	if phase1.Children[0].Title != "Setup" {
		t.Errorf("Expected subsection 'Setup', got '%s'", phase1.Children[0].Title)
	}
	if phase1.Children[1].Title != "Configuration" {
		t.Errorf("Expected subsection 'Configuration', got '%s'", phase1.Children[1].Title)
	}
}

func TestParseMarkdown_BulletItems(t *testing.T) {
	input := `# Plan
## Tasks
- Regular bullet item
- Another bullet item
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	section := doc.Sections[0]
	if len(section.Items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(section.Items))
	}

	item1 := section.Items[0]
	if item1.IsCheckbox {
		t.Error("Expected item to not be checkbox")
	}
	if item1.IsNumbered {
		t.Error("Expected item to not be numbered")
	}
	if item1.Text != "Regular bullet item" {
		t.Errorf("Expected text 'Regular bullet item', got '%s'", item1.Text)
	}
}

func TestParseMarkdown_RealWorldExample(t *testing.T) {
	input := `# Microservices Migration Plan

## Phase 1: Setup Infrastructure
- [ ] Set up Kubernetes cluster
  - [ ] Configure namespaces
  - [ ] Set up ingress controller
- [ ] Set up monitoring stack
  - [ ] Deploy Prometheus
  - [ ] Deploy Grafana
  - [ ] Configure alerts

## Phase 2: Service Migration
1. Migrate authentication service
2. Migrate user service
3. Migrate order service

## Phase 3: Testing
- [ ] Integration testing
- [ ] Load testing
- [ ] Security audit

## Phase 4: Deployment
1. Deploy to staging
2. Run smoke tests
3. Deploy to production
4. Monitor for 24 hours
`

	doc, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	if doc.Title != "Microservices Migration Plan" {
		t.Errorf("Expected title 'Microservices Migration Plan', got '%s'", doc.Title)
	}

	if len(doc.Sections) != 4 {
		t.Fatalf("Expected 4 phases, got %d", len(doc.Sections))
	}

	// Check Phase 1
	phase1 := doc.Sections[0]
	if len(phase1.Items) != 2 {
		t.Errorf("Expected 2 items in Phase 1, got %d", len(phase1.Items))
	}
	if len(phase1.Items[0].Children) != 2 {
		t.Errorf("Expected 2 children in first item, got %d", len(phase1.Items[0].Children))
	}
	if len(phase1.Items[1].Children) != 3 {
		t.Errorf("Expected 3 children in second item, got %d", len(phase1.Items[1].Children))
	}

	// Check Phase 2 (numbered)
	phase2 := doc.Sections[1]
	if len(phase2.Items) != 3 {
		t.Errorf("Expected 3 items in Phase 2, got %d", len(phase2.Items))
	}
	for i, item := range phase2.Items {
		if !item.IsNumbered {
			t.Errorf("Expected item %d to be numbered", i)
		}
	}

	// Check Phase 4 (numbered)
	phase4 := doc.Sections[3]
	if len(phase4.Items) != 4 {
		t.Errorf("Expected 4 items in Phase 4, got %d", len(phase4.Items))
	}
	if phase4.Items[3].Text != "Monitor for 24 hours" {
		t.Errorf("Expected last item text 'Monitor for 24 hours', got '%s'", phase4.Items[3].Text)
	}
}
