package plan

import (
	"fmt"
	"strings"
)

// Convert transforms a PlanDocument into an EpicPlan structure.
func Convert(doc *PlanDocument) (*EpicPlan, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is nil")
	}

	epic := &EpicPlan{
		Title:       doc.Title,
		Description: "",
		Children:    make([]EpicChild, 0),
		Priority:    2, // Default medium priority
	}

	// If no title, use default
	if epic.Title == "" {
		epic.Title = "Untitled Plan"
	}

	// Build description from section titles
	var descParts []string
	for _, section := range doc.Sections {
		descParts = append(descParts, section.Title)
	}
	if len(descParts) > 0 {
		epic.Description = "Plan phases:\n" + strings.Join(descParts, "\n")
	}

	// Convert sections to children
	// Sections at the same level are sequential (phase dependencies)
	for i, section := range doc.Sections {
		child := convertSection(&section, i)
		if child != nil {
			// Sections depend on previous section (sequential phases)
			if i > 0 {
				child.DependsOn = []int{i - 1}
			}
			epic.Children = append(epic.Children, *child)
		}
	}

	return epic, nil
}

func convertSection(section *PlanSection, index int) *EpicChild {
	if section == nil {
		return nil
	}

	child := &EpicChild{
		Title:       section.Title,
		Description: "",
		Type:        determineIssueType(section.Title),
		Priority:    2, // Default medium priority
		Children:    make([]EpicChild, 0),
		DependsOn:   make([]int, 0),
		Sequential:  false, // Items within a section are parallel by default
	}

	// Build description from items
	var descParts []string
	for _, item := range section.Items {
		descParts = append(descParts, formatItemText(&item))
	}
	if len(descParts) > 0 {
		child.Description = strings.Join(descParts, "\n")
	}

	// Convert direct items to children
	for _, item := range section.Items {
		itemChild := convertItem(&item)
		if itemChild != nil {
			child.Children = append(child.Children, *itemChild)
		}
	}

	// If section has numbered items, they should be sequential
	hasNumbered := false
	for _, item := range section.Items {
		if item.IsNumbered {
			hasNumbered = true
			break
		}
	}
	child.Sequential = hasNumbered

	// Convert subsections recursively
	for i, subsection := range section.Children {
		subChild := convertSection(&subsection, i)
		if subChild != nil {
			child.Children = append(child.Children, *subChild)
		}
	}

	return child
}

func convertItem(item *PlanItem) *EpicChild {
	if item == nil {
		return nil
	}

	child := &EpicChild{
		Title:       item.Text,
		Description: "",
		Type:        "task", // Items are typically tasks
		Priority:    2,
		Children:    make([]EpicChild, 0),
		DependsOn:   make([]int, 0),
		Sequential:  item.IsNumbered, // Numbered items have sequential children
	}

	// Build description from children
	var descParts []string
	for _, childItem := range item.Children {
		descParts = append(descParts, formatItemText(&childItem))
	}
	if len(descParts) > 0 {
		child.Description = strings.Join(descParts, "\n")
	}

	// Convert nested items to children
	for _, childItem := range item.Children {
		nestedChild := convertItem(&childItem)
		if nestedChild != nil {
			child.Children = append(child.Children, *nestedChild)
		}
	}

	return child
}

func formatItemText(item *PlanItem) string {
	prefix := "-"
	if item.IsCheckbox {
		if item.Checked {
			prefix = "- [x]"
		} else {
			prefix = "- [ ]"
		}
	} else if item.IsNumbered {
		prefix = fmt.Sprintf("%d.", item.Number)
	}

	return fmt.Sprintf("%s %s", prefix, item.Text)
}

func determineIssueType(title string) string {
	lower := strings.ToLower(title)

	// Check for common keywords
	if strings.Contains(lower, "bug") || strings.Contains(lower, "fix") {
		return "bug"
	}
	if strings.Contains(lower, "feature") || strings.Contains(lower, "add") || strings.Contains(lower, "implement") {
		return "feature"
	}
	if strings.Contains(lower, "test") || strings.Contains(lower, "qa") {
		return "task"
	}

	// Default to feature for phases/major sections
	if strings.Contains(lower, "phase") || strings.Contains(lower, "stage") || strings.Contains(lower, "step") {
		return "feature"
	}

	return "task"
}
