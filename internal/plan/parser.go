package plan

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var (
	// Header patterns
	headerRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

	// List item patterns
	checkboxRegex    = regexp.MustCompile(`^(\s*)-\s+\[([xX ])\]\s+(.+)$`)
	bulletRegex      = regexp.MustCompile(`^(\s*)-\s+(.+)$`)
	numberedRegex    = regexp.MustCompile(`^(\s*)(\d+)\.\s+(.+)$`)
)

// ParseMarkdown parses a markdown document into a structured PlanDocument.
func ParseMarkdown(reader io.Reader) (*PlanDocument, error) {
	scanner := bufio.NewScanner(reader)
	doc := &PlanDocument{
		Sections: make([]PlanSection, 0),
	}

	var currentSection *PlanSection
	var sectionStack []*PlanSection
	var itemStack []itemStackEntry

	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check for headers
		if matches := headerRegex.FindStringSubmatch(line); matches != nil {
			level := len(matches[1])
			title := strings.TrimSpace(matches[2])

			// First H1 becomes document title
			if level == 1 && doc.Title == "" {
				doc.Title = title
				continue
			}

			// Create new section
			section := PlanSection{
				Title:    title,
				Level:    level,
				Items:    make([]PlanItem, 0),
				Children: make([]PlanSection, 0),
			}

			// Pop stack until we find parent level
			for len(sectionStack) > 0 && sectionStack[len(sectionStack)-1].Level >= level {
				sectionStack = sectionStack[:len(sectionStack)-1]
			}

			// Add section to appropriate parent
			if len(sectionStack) == 0 {
				doc.Sections = append(doc.Sections, section)
				currentSection = &doc.Sections[len(doc.Sections)-1]
			} else {
				parent := sectionStack[len(sectionStack)-1]
				parent.Children = append(parent.Children, section)
				currentSection = &parent.Children[len(parent.Children)-1]
			}

			sectionStack = append(sectionStack, currentSection)
			itemStack = nil // Reset item stack for new section
			continue
		}

		// Parse list items
		item := parseListItem(line)
		if item != nil {
			if currentSection == nil {
				// Items before first header - create default section
				doc.Sections = append(doc.Sections, PlanSection{
					Title: "Tasks",
					Level: 1,
					Items: make([]PlanItem, 0),
				})
				currentSection = &doc.Sections[len(doc.Sections)-1]
			}

			// Handle nesting based on indentation
			addItemToStack(&itemStack, item, currentSection)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return doc, nil
}

type itemStackEntry struct {
	item  *PlanItem
	level int
}

func parseListItem(line string) *PlanItem {
	// Try checkbox pattern
	if matches := checkboxRegex.FindStringSubmatch(line); matches != nil {
		indent := len(matches[1])
		checked := strings.ToLower(matches[2]) == "x"
		text := strings.TrimSpace(matches[3])

		return &PlanItem{
			Text:       text,
			Checked:    checked,
			IsCheckbox: true,
			Level:      indent / 2, // Assume 2-space indent
			Children:   make([]PlanItem, 0),
		}
	}

	// Try numbered list pattern
	if matches := numberedRegex.FindStringSubmatch(line); matches != nil {
		indent := len(matches[1])
		num := 0
		_, _ = fmt.Sscanf(matches[2], "%d", &num)
		text := strings.TrimSpace(matches[3])

		return &PlanItem{
			Text:       text,
			IsNumbered: true,
			Number:     num,
			Level:      indent / 2,
			Children:   make([]PlanItem, 0),
		}
	}

	// Try bullet list pattern
	if matches := bulletRegex.FindStringSubmatch(line); matches != nil {
		indent := len(matches[1])
		text := strings.TrimSpace(matches[2])

		// Skip if it looks like a checkbox (we already checked above)
		if strings.HasPrefix(text, "[ ]") || strings.HasPrefix(text, "[x]") || strings.HasPrefix(text, "[X]") {
			return nil
		}

		return &PlanItem{
			Text:     text,
			Level:    indent / 2,
			Children: make([]PlanItem, 0),
		}
	}

	return nil
}

func addItemToStack(stack *[]itemStackEntry, item *PlanItem, section *PlanSection) {
	// Empty stack - add to section
	if len(*stack) == 0 {
		section.Items = append(section.Items, *item)
		*stack = append(*stack, itemStackEntry{
			item:  &section.Items[len(section.Items)-1],
			level: item.Level,
		})
		return
	}

	// Find parent based on indentation level
	parentIdx := len(*stack) - 1
	for parentIdx >= 0 && (*stack)[parentIdx].level >= item.Level {
		parentIdx--
	}

	if parentIdx < 0 {
		// Top-level item
		section.Items = append(section.Items, *item)
		*stack = []itemStackEntry{{
			item:  &section.Items[len(section.Items)-1],
			level: item.Level,
		}}
	} else {
		// Child item
		parent := (*stack)[parentIdx].item
		parent.Children = append(parent.Children, *item)

		// Trim stack and add new item
		*stack = (*stack)[:parentIdx+1]
		*stack = append(*stack, itemStackEntry{
			item:  &parent.Children[len(parent.Children)-1],
			level: item.Level,
		})
	}
}
