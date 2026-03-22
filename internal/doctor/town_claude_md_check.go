package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/templates"
)

// TownCLAUDEmdCheck verifies the town-root CLAUDE.md is up to date with
// the version embedded in the binary. This is the highest-value migration
// check — behavioral norms for agents come from CLAUDE.md.
//
// The town-root CLAUDE.md (~/gt/CLAUDE.md) is loaded by Claude Code for
// all agents running from within the town git tree (Mayor, Deacon).
// It must contain operational norms (Dolt awareness, communication hygiene,
// nudge-first) that guide agent behavior.
type TownCLAUDEmdCheck struct {
	FixableCheck
	missingSections []templates.TownRootRequiredSection
	fileMissing     bool
}

// NewTownCLAUDEmdCheck creates a new town-root CLAUDE.md version check.
func NewTownCLAUDEmdCheck() *TownCLAUDEmdCheck {
	return &TownCLAUDEmdCheck{
		FixableCheck: FixableCheck{
			BaseCheck: BaseCheck{
				CheckName:        "town-claude-md",
				CheckDescription: "Verify town-root CLAUDE.md is up to date with embedded version",
				CheckCategory:    CategoryConfig,
			},
		},
	}
}

// Run checks the town-root CLAUDE.md for completeness.
func (c *TownCLAUDEmdCheck) Run(ctx *CheckContext) *CheckResult {
	c.missingSections = nil
	c.fileMissing = false

	claudePath := filepath.Join(ctx.TownRoot, "CLAUDE.md")

	// Check if file exists
	data, err := os.ReadFile(claudePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.fileMissing = true
			return &CheckResult{
				Name:    c.Name(),
				Status:  StatusError,
				Message: "Town-root CLAUDE.md is missing",
				FixHint: "Run 'gt doctor --fix' to create it from embedded template",
			}
		}
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusError,
			Message: fmt.Sprintf("Cannot read town-root CLAUDE.md: %v", err),
		}
	}

	content := string(data)

	// Check for required sections
	required := templates.TownRootRequiredSections()
	var missing []templates.TownRootRequiredSection
	var details []string

	for _, section := range required {
		if !strings.Contains(content, section.Heading) {
			missing = append(missing, section)
			details = append(details, fmt.Sprintf("Missing: %s (%s)", section.Name, section.Heading))
		}
	}

	if len(missing) == 0 {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: "Town-root CLAUDE.md has all required sections",
		}
	}

	c.missingSections = missing

	return &CheckResult{
		Name:    c.Name(),
		Status:  StatusWarning,
		Message: fmt.Sprintf("Town-root CLAUDE.md missing %d section(s)", len(missing)),
		Details: details,
		FixHint: "Run 'gt doctor --fix' to add missing sections from embedded template",
	}
}

// Fix updates the town-root CLAUDE.md with missing sections from the
// embedded template while preserving user customizations.
// It also ensures AGENTS.md exists as a real file (not a symlink) with matching content.
func (c *TownCLAUDEmdCheck) Fix(ctx *CheckContext) error {
	claudePath := filepath.Join(ctx.TownRoot, "CLAUDE.md")
	canonical := templates.TownRootCLAUDEmd()

	var finalContent string

	// If file is missing, create it from the canonical template
	if c.fileMissing {
		finalContent = canonical
		if err := os.WriteFile(claudePath, []byte(finalContent), 0644); err != nil {
			return err
		}
	} else if len(c.missingSections) > 0 {
		// File exists but is missing sections — append them
		data, err := os.ReadFile(claudePath)
		if err != nil {
			return fmt.Errorf("reading CLAUDE.md: %w", err)
		}
		current := string(data)

		// Parse canonical content into H2 sections
		canonicalSections := parseH2Sections(canonical)

		// For each missing section, find it in the canonical and append
		var toAppend strings.Builder
		for _, missing := range c.missingSections {
			for _, cs := range canonicalSections {
				if strings.Contains(cs.content, missing.Heading) {
					toAppend.WriteString("\n")
					toAppend.WriteString(cs.content)
					break
				}
			}
		}

		if toAppend.Len() > 0 {
			if !strings.HasSuffix(current, "\n") {
				current += "\n"
			}
			finalContent = current + toAppend.String()
			if err := os.WriteFile(claudePath, []byte(finalContent), 0644); err != nil {
				return err
			}
		}
	}

	// Ensure AGENTS.md is a real file (not a symlink) with matching content.
	// Read whatever CLAUDE.md now contains as the source of truth.
	if finalContent == "" {
		data, err := os.ReadFile(claudePath)
		if err != nil {
			return fmt.Errorf("reading CLAUDE.md for AGENTS.md sync: %w", err)
		}
		finalContent = string(data)
	}

	return syncAgentsMD(ctx.TownRoot, finalContent)
}

// syncAgentsMD ensures AGENTS.md exists as a real file with the given content.
// Replaces old symlinks with real files for OpenCode compatibility.
func syncAgentsMD(townRoot, content string) error {
	agentsPath := filepath.Join(townRoot, "AGENTS.md")

	fi, err := os.Lstat(agentsPath)
	if os.IsNotExist(err) {
		return os.WriteFile(agentsPath, []byte(content), 0644)
	}
	if err != nil {
		return fmt.Errorf("checking AGENTS.md: %w", err)
	}

	// Replace old symlink with real file.
	if fi.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(agentsPath); err != nil {
			return fmt.Errorf("removing AGENTS.md symlink: %w", err)
		}
		return os.WriteFile(agentsPath, []byte(content), 0644)
	}

	// Real file exists — update if content differs.
	existing, err := os.ReadFile(agentsPath)
	if err != nil {
		return fmt.Errorf("reading AGENTS.md: %w", err)
	}
	if string(existing) != content {
		return os.WriteFile(agentsPath, []byte(content), 0644)
	}

	return nil
}

// h2Section represents a section of markdown delimited by H2 headings.
type h2Section struct {
	heading string // The H2 heading line (e.g., "## Dolt Server — Operational Awareness")
	content string // Full section content including the heading and all sub-content
}

// parseH2Sections splits markdown content into sections by H2 headings.
// The preamble (content before the first H2) is returned as a section with
// an empty heading.
func parseH2Sections(content string) []h2Section {
	var sections []h2Section
	lines := strings.Split(content, "\n")

	var currentHeading string
	var currentContent strings.Builder
	inSection := false

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			if inSection || currentContent.Len() > 0 {
				sections = append(sections, h2Section{
					heading: currentHeading,
					content: currentContent.String(),
				})
			}
			// Start new section
			currentHeading = line
			currentContent.Reset()
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
			inSection = true
		} else {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	// Save final section
	if currentContent.Len() > 0 {
		sections = append(sections, h2Section{
			heading: currentHeading,
			content: currentContent.String(),
		})
	}

	return sections
}
