package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/hooks"
)

// requiredOpenCodeHooks lists the hook identifiers that the gastown.js plugin
// must contain for proper GasTown integration. Each entry is a substring that
// must appear in the file content.
var requiredOpenCodeHooks = []struct {
	pattern     string // Substring to search for in the JS file
	description string // Human-readable description for diagnostics
}{
	{"session.created", "session lifecycle (prime context)"},
	{"session.deleted", "session cleanup (cost recording)"},
	{"session.compacted", "compaction recovery"},
	{"chat.system.transform", "system prompt injection"},
	{"session.compacting", "compaction context injection"},
	{"chat.message", "turn-boundary mail drain"},
	{"tool.execute.before", "tap guard enforcement"},
	{"shell.env", "GT_* env propagation"},
	{"gt costs record", "cost recording command"},
	{"gt prime", "prime context loading"},
}

// OpenCodeSettingsCheck verifies that OpenCode gastown.js plugin files exist
// and contain all required hooks. This is the OpenCode counterpart of
// ClaudeSettingsCheck — it validates the plugin files that OpenCode uses
// instead of Claude's settings.json.
type OpenCodeSettingsCheck struct {
	FixableCheck
	outOfSync []hooks.Target
	issues    []string // per-target issue descriptions
}

// NewOpenCodeSettingsCheck creates a new OpenCode settings validation check.
func NewOpenCodeSettingsCheck() *OpenCodeSettingsCheck {
	return &OpenCodeSettingsCheck{
		FixableCheck: FixableCheck{
			BaseCheck: BaseCheck{
				CheckName:        "opencode-settings",
				CheckDescription: "Verify OpenCode gastown.js plugin files match expected templates",
				CheckCategory:    CategoryConfig,
			},
		},
	}
}

// Run checks all OpenCode plugin locations for existence and content correctness.
func (c *OpenCodeSettingsCheck) Run(ctx *CheckContext) *CheckResult {
	c.outOfSync = nil
	c.issues = nil

	installer := hooks.InstallerForPreset(config.AgentOpenCode)

	targets, err := installer.DiscoverTargets(ctx.TownRoot)
	if err != nil {
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusWarning,
			Message:  fmt.Sprintf("Failed to discover OpenCode targets: %v", err),
			Category: c.Category(),
		}
	}

	if len(targets) == 0 {
		// No OpenCode targets found — this is fine if nobody is using OpenCode.
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusOK,
			Message:  "No OpenCode plugin targets found (OpenCode not in use)",
			Category: c.Category(),
		}
	}

	// Filter targets to only include those where the role directory exists.
	// discoverTargetsForProvider always emits mayor/deacon targets even when
	// those directories are absent, so we skip targets whose role directory
	// hasn't been created yet (no agent running there).
	var activeTargets []hooks.Target
	for _, target := range targets {
		// Role directory is the grandparent of the plugin file:
		//   <role>/.opencode/plugins/gastown.js → <role>
		roleDir := filepath.Dir(filepath.Dir(filepath.Dir(target.Path)))
		if dirExists(roleDir) {
			activeTargets = append(activeTargets, target)
		}
	}

	if len(activeTargets) == 0 {
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusOK,
			Message:  "No active OpenCode plugin targets found",
			Category: c.Category(),
		}
	}

	var details []string

	for _, target := range activeTargets {
		// Check if the file exists.
		data, readErr := os.ReadFile(target.Path)
		if readErr != nil {
			c.outOfSync = append(c.outOfSync, target)
			detail := fmt.Sprintf("%s: missing plugin file", target.DisplayKey())
			details = append(details, detail)
			c.issues = append(c.issues, detail)
			continue
		}

		content := string(data)

		// Check for each required hook pattern.
		var missing []string
		for _, req := range requiredOpenCodeHooks {
			if !strings.Contains(content, req.pattern) {
				missing = append(missing, req.description)
			}
		}

		if len(missing) > 0 {
			c.outOfSync = append(c.outOfSync, target)
			detail := fmt.Sprintf("%s: missing %s", target.DisplayKey(), strings.Join(missing, ", "))
			details = append(details, detail)
			c.issues = append(c.issues, detail)
		}
	}

	if len(c.outOfSync) == 0 {
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusOK,
			Message:  fmt.Sprintf("All %d OpenCode plugin target(s) up to date", len(activeTargets)),
			Category: c.Category(),
		}
	}

	return &CheckResult{
		Name:     c.Name(),
		Status:   StatusError,
		Message:  fmt.Sprintf("%d OpenCode plugin target(s) out of date", len(c.outOfSync)),
		Details:  details,
		FixHint:  "Run 'gt doctor --fix' to regenerate gastown.js plugin files",
		Category: c.Category(),
	}
}

// Fix regenerates out-of-sync OpenCode plugin files using the HookInstaller.
func (c *OpenCodeSettingsCheck) Fix(_ *CheckContext) error {
	if len(c.outOfSync) == 0 {
		return nil
	}

	installer := hooks.InstallerForPreset(config.AgentOpenCode)

	var errs []string
	for _, target := range c.outOfSync {
		result, err := installer.SyncTarget(target, "")
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", target.DisplayKey(), err))
			continue
		}
		switch result {
		case hooks.SyncCreated:
			fmt.Printf("  Created: %s\n", target.Path)
		case hooks.SyncUpdated:
			fmt.Printf("  Updated: %s\n", target.Path)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
