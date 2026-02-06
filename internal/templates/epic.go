package templates

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// IssueTemplate represents an issue within a phase template.
type IssueTemplate struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description,omitempty"`
	Type        string `yaml:"type,omitempty"`        // task, bug, feature, epic
	Priority    int    `yaml:"priority,omitempty"`    // 0-4
	Labels      []string `yaml:"labels,omitempty"`
}

// Phase represents a phase in the epic template.
type Phase struct {
	Name   string          `yaml:"name"`
	Issues []IssueTemplate `yaml:"issues,omitempty"`
}

// EpicTemplate represents a batch work pattern template.
type EpicTemplate struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Phases      []Phase `yaml:"phases"`
}

// IssueData represents an issue to be created from the template.
// This is a simplified version of beads.Issue with only the fields
// needed for creation via beads.CreateOptions.
type IssueData struct {
	Title       string
	Description string
	Type        string
	Priority    int
	Labels      []string
	Phase       string // Which phase this issue belongs to
}

// LoadTemplate parses a YAML template file and returns an EpicTemplate.
func LoadTemplate(path string) (*EpicTemplate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading template file: %w", err)
	}

	var template EpicTemplate
	if err := yaml.Unmarshal(data, &template); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	if err := template.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &template, nil
}

// Validate checks that the template has valid structure and content.
func (t *EpicTemplate) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("template name is required")
	}

	if len(t.Phases) == 0 {
		return fmt.Errorf("template must have at least one phase")
	}

	// Validate each phase
	phaseNames := make(map[string]bool)
	for i, phase := range t.Phases {
		if strings.TrimSpace(phase.Name) == "" {
			return fmt.Errorf("phase %d: name is required", i)
		}

		// Check for duplicate phase names
		if phaseNames[phase.Name] {
			return fmt.Errorf("duplicate phase name: %s", phase.Name)
		}
		phaseNames[phase.Name] = true

		// Validate issues within the phase
		for j, issue := range phase.Issues {
			if strings.TrimSpace(issue.Title) == "" {
				return fmt.Errorf("phase %s, issue %d: title is required", phase.Name, j)
			}

			// Validate issue type if specified
			if issue.Type != "" {
				validTypes := map[string]bool{
					"task":    true,
					"bug":     true,
					"feature": true,
					"epic":    true,
				}
				if !validTypes[issue.Type] {
					return fmt.Errorf("phase %s, issue %d: invalid type %q (must be task, bug, feature, or epic)", phase.Name, j, issue.Type)
				}
			}

			// Validate priority if specified
			if issue.Priority < 0 || issue.Priority > 4 {
				return fmt.Errorf("phase %s, issue %d: priority must be 0-4, got %d", phase.Name, j, issue.Priority)
			}
		}
	}

	return nil
}

// Instantiate creates IssueData structs from the template.
// The epicID parameter is used to tag issues with a phase label indicating
// which epic and phase they belong to (e.g., "epic:grr-123:startup").
//
// Note: This returns IssueData structs, not full beads.Issue structs,
// since those require database IDs and timestamps that will be assigned
// by the bd CLI when the issues are actually created.
func (t *EpicTemplate) Instantiate(epicID string) ([]IssueData, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}

	var issues []IssueData

	for _, phase := range t.Phases {
		for _, issueTemplate := range phase.Issues {
			issue := IssueData{
				Title:       issueTemplate.Title,
				Description: issueTemplate.Description,
				Type:        issueTemplate.Type,
				Priority:    issueTemplate.Priority,
				Labels:      make([]string, 0, len(issueTemplate.Labels)+2),
				Phase:       phase.Name,
			}

			// Default type to "task" if not specified
			if issue.Type == "" {
				issue.Type = "task"
			}

			// Copy template labels
			issue.Labels = append(issue.Labels, issueTemplate.Labels...)

			// Add phase label to track which phase this issue belongs to
			if epicID != "" {
				issue.Labels = append(issue.Labels, fmt.Sprintf("epic:%s:%s", epicID, phase.Name))
			} else {
				issue.Labels = append(issue.Labels, fmt.Sprintf("phase:%s", phase.Name))
			}

			// Add template name as a label
			issue.Labels = append(issue.Labels, fmt.Sprintf("template:%s", t.Name))

			issues = append(issues, issue)
		}
	}

	return issues, nil
}
