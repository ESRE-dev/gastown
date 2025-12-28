// Package beads provides field parsing utilities for structured issue descriptions.
package beads

import "strings"

// AgentFields holds parsed fields from an agent bead's description.
// Agent beads store their state as key: value lines in the description.
type AgentFields struct {
	RoleType   string // role_type: mayor, deacon, witness, refinery, polecat
	Rig        string // rig: gastown (or null)
	AgentState string // agent_state: idle, running, working, stopped
	HookBead   string // hook_bead: the bead ID on the hook (or null)
	RoleBead   string // role_bead: the role definition bead
}

// ParseAgentFields extracts agent fields from an issue's description.
// Fields are expected as "key: value" lines. Returns nil if no agent fields found.
func ParseAgentFields(issue *Issue) *AgentFields {
	if issue == nil || issue.Description == "" {
		return nil
	}
	return ParseAgentFieldsFromDescription(issue.Description)
}

// ParseAgentFieldsFromDescription extracts agent fields from a description string.
// Returns nil if no agent fields found.
func ParseAgentFieldsFromDescription(description string) *AgentFields {
	if description == "" {
		return nil
	}

	fields := &AgentFields{}
	hasFields := false

	for _, line := range strings.Split(description, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		if value == "" || value == "null" {
			continue
		}

		switch strings.ToLower(key) {
		case "role_type", "role-type", "roletype":
			fields.RoleType = value
			hasFields = true
		case "rig":
			fields.Rig = value
			hasFields = true
		case "agent_state", "agent-state", "agentstate":
			fields.AgentState = value
			hasFields = true
		case "hook_bead", "hook-bead", "hookbead":
			fields.HookBead = value
			hasFields = true
		case "role_bead", "role-bead", "rolebead":
			fields.RoleBead = value
			hasFields = true
		}
	}

	if !hasFields {
		return nil
	}
	return fields
}

// FormatAgentFields formats AgentFields as a string suitable for an issue description.
// Only non-empty fields are included.
func FormatAgentFields(fields *AgentFields) string {
	if fields == nil {
		return ""
	}

	var lines []string

	if fields.RoleType != "" {
		lines = append(lines, "role_type: "+fields.RoleType)
	}
	if fields.Rig != "" {
		lines = append(lines, "rig: "+fields.Rig)
	}
	if fields.AgentState != "" {
		lines = append(lines, "agent_state: "+fields.AgentState)
	}
	if fields.HookBead != "" {
		lines = append(lines, "hook_bead: "+fields.HookBead)
	}
	if fields.RoleBead != "" {
		lines = append(lines, "role_bead: "+fields.RoleBead)
	}

	return strings.Join(lines, "\n")
}

// AttachmentFields holds the attachment info for pinned beads.
// These fields track which molecule is attached to a handoff/pinned bead.
type AttachmentFields struct {
	AttachedMolecule string // Root issue ID of the attached molecule
	AttachedAt       string // ISO 8601 timestamp when attached
	AttachedArgs     string // Natural language args passed via gt sling --args (no-tmux mode)
}

// ParseAttachmentFields extracts attachment fields from an issue's description.
// Fields are expected as "key: value" lines. Returns nil if no attachment fields found.
func ParseAttachmentFields(issue *Issue) *AttachmentFields {
	if issue == nil || issue.Description == "" {
		return nil
	}

	fields := &AttachmentFields{}
	hasFields := false

	for _, line := range strings.Split(issue.Description, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for "key: value" pattern
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		if value == "" {
			continue
		}

		// Map keys to fields (case-insensitive)
		switch strings.ToLower(key) {
		case "attached_molecule", "attached-molecule", "attachedmolecule":
			fields.AttachedMolecule = value
			hasFields = true
		case "attached_at", "attached-at", "attachedat":
			fields.AttachedAt = value
			hasFields = true
		case "attached_args", "attached-args", "attachedargs":
			fields.AttachedArgs = value
			hasFields = true
		}
	}

	if !hasFields {
		return nil
	}
	return fields
}

// FormatAttachmentFields formats AttachmentFields as a string suitable for an issue description.
// Only non-empty fields are included.
func FormatAttachmentFields(fields *AttachmentFields) string {
	if fields == nil {
		return ""
	}

	var lines []string

	if fields.AttachedMolecule != "" {
		lines = append(lines, "attached_molecule: "+fields.AttachedMolecule)
	}
	if fields.AttachedAt != "" {
		lines = append(lines, "attached_at: "+fields.AttachedAt)
	}
	if fields.AttachedArgs != "" {
		lines = append(lines, "attached_args: "+fields.AttachedArgs)
	}

	return strings.Join(lines, "\n")
}

// SetAttachmentFields updates an issue's description with the given attachment fields.
// Existing attachment field lines are replaced; other content is preserved.
// Returns the new description string.
func SetAttachmentFields(issue *Issue, fields *AttachmentFields) string {
	// Known attachment field keys (lowercase)
	attachmentKeys := map[string]bool{
		"attached_molecule": true,
		"attached-molecule": true,
		"attachedmolecule":  true,
		"attached_at":       true,
		"attached-at":       true,
		"attachedat":        true,
		"attached_args":     true,
		"attached-args":     true,
		"attachedargs":      true,
	}

	// Collect non-attachment lines from existing description
	var otherLines []string
	if issue != nil && issue.Description != "" {
		for _, line := range strings.Split(issue.Description, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				// Preserve blank lines in content
				otherLines = append(otherLines, line)
				continue
			}

			// Check if this is an attachment field line
			colonIdx := strings.Index(trimmed, ":")
			if colonIdx == -1 {
				otherLines = append(otherLines, line)
				continue
			}

			key := strings.ToLower(strings.TrimSpace(trimmed[:colonIdx]))
			if !attachmentKeys[key] {
				otherLines = append(otherLines, line)
			}
			// Skip attachment field lines - they'll be replaced
		}
	}

	// Build new description: attachment fields first, then other content
	formatted := FormatAttachmentFields(fields)

	// Trim trailing blank lines from other content
	for len(otherLines) > 0 && strings.TrimSpace(otherLines[len(otherLines)-1]) == "" {
		otherLines = otherLines[:len(otherLines)-1]
	}
	// Trim leading blank lines from other content
	for len(otherLines) > 0 && strings.TrimSpace(otherLines[0]) == "" {
		otherLines = otherLines[1:]
	}

	if formatted == "" {
		return strings.Join(otherLines, "\n")
	}
	if len(otherLines) == 0 {
		return formatted
	}

	return formatted + "\n\n" + strings.Join(otherLines, "\n")
}

// MRFields holds the structured fields for a merge-request issue.
// These fields are stored as key: value lines in the issue description.
type MRFields struct {
	Branch      string // Source branch name (e.g., "polecat/Nux/gt-xyz")
	Target      string // Target branch (e.g., "main" or "integration/gt-epic")
	SourceIssue string // The work item being merged (e.g., "gt-xyz")
	Worker      string // Who did the work
	Rig         string // Which rig
	MergeCommit string // SHA of merge commit (set on close)
	CloseReason string // Reason for closing: merged, rejected, conflict, superseded
}

// ParseMRFields extracts structured merge-request fields from an issue's description.
// Fields are expected as "key: value" lines, with optional prose text mixed in.
// Returns nil if no MR fields are found.
func ParseMRFields(issue *Issue) *MRFields {
	if issue == nil || issue.Description == "" {
		return nil
	}

	fields := &MRFields{}
	hasFields := false

	for _, line := range strings.Split(issue.Description, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for "key: value" pattern
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		if value == "" {
			continue
		}

		// Map keys to fields (case-insensitive)
		switch strings.ToLower(key) {
		case "branch":
			fields.Branch = value
			hasFields = true
		case "target":
			fields.Target = value
			hasFields = true
		case "source_issue", "source-issue", "sourceissue":
			fields.SourceIssue = value
			hasFields = true
		case "worker":
			fields.Worker = value
			hasFields = true
		case "rig":
			fields.Rig = value
			hasFields = true
		case "merge_commit", "merge-commit", "mergecommit":
			fields.MergeCommit = value
			hasFields = true
		case "close_reason", "close-reason", "closereason":
			fields.CloseReason = value
			hasFields = true
		}
	}

	if !hasFields {
		return nil
	}
	return fields
}

// FormatMRFields formats MRFields as a string suitable for an issue description.
// Only non-empty fields are included.
func FormatMRFields(fields *MRFields) string {
	if fields == nil {
		return ""
	}

	var lines []string

	if fields.Branch != "" {
		lines = append(lines, "branch: "+fields.Branch)
	}
	if fields.Target != "" {
		lines = append(lines, "target: "+fields.Target)
	}
	if fields.SourceIssue != "" {
		lines = append(lines, "source_issue: "+fields.SourceIssue)
	}
	if fields.Worker != "" {
		lines = append(lines, "worker: "+fields.Worker)
	}
	if fields.Rig != "" {
		lines = append(lines, "rig: "+fields.Rig)
	}
	if fields.MergeCommit != "" {
		lines = append(lines, "merge_commit: "+fields.MergeCommit)
	}
	if fields.CloseReason != "" {
		lines = append(lines, "close_reason: "+fields.CloseReason)
	}

	return strings.Join(lines, "\n")
}

// SetMRFields updates an issue's description with the given MR fields.
// Existing MR field lines are replaced; other content is preserved.
// Returns the new description string.
func SetMRFields(issue *Issue, fields *MRFields) string {
	if issue == nil {
		return FormatMRFields(fields)
	}

	// Known MR field keys (lowercase)
	mrKeys := map[string]bool{
		"branch":       true,
		"target":       true,
		"source_issue": true,
		"source-issue": true,
		"sourceissue":  true,
		"worker":       true,
		"rig":          true,
		"merge_commit": true,
		"merge-commit": true,
		"mergecommit":  true,
		"close_reason": true,
		"close-reason": true,
		"closereason":  true,
	}

	// Collect non-MR lines from existing description
	var otherLines []string
	if issue.Description != "" {
		for _, line := range strings.Split(issue.Description, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				// Preserve blank lines in content
				otherLines = append(otherLines, line)
				continue
			}

			// Check if this is an MR field line
			colonIdx := strings.Index(trimmed, ":")
			if colonIdx == -1 {
				otherLines = append(otherLines, line)
				continue
			}

			key := strings.ToLower(strings.TrimSpace(trimmed[:colonIdx]))
			if !mrKeys[key] {
				otherLines = append(otherLines, line)
			}
			// Skip MR field lines - they'll be replaced
		}
	}

	// Build new description: MR fields first, then other content
	formatted := FormatMRFields(fields)

	// Trim trailing blank lines from other content
	for len(otherLines) > 0 && strings.TrimSpace(otherLines[len(otherLines)-1]) == "" {
		otherLines = otherLines[:len(otherLines)-1]
	}
	// Trim leading blank lines from other content
	for len(otherLines) > 0 && strings.TrimSpace(otherLines[0]) == "" {
		otherLines = otherLines[1:]
	}

	if formatted == "" {
		return strings.Join(otherLines, "\n")
	}
	if len(otherLines) == 0 {
		return formatted
	}

	return formatted + "\n\n" + strings.Join(otherLines, "\n")
}
