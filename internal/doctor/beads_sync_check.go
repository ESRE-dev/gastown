package doctor

import (
	"os"
	"path/filepath"
	"strings"
)

// BeadsSyncBranchCheck verifies that rig-level beads have sync-branch configured.
// Rig beads need sync-branch for multi-clone coordination (polecats, crew, etc.)
// Town-level beads should NOT have sync-branch (single clone, commits to main).
type BeadsSyncBranchCheck struct {
	FixableCheck
}

// NewBeadsSyncBranchCheck creates a new sync-branch configuration check.
func NewBeadsSyncBranchCheck() *BeadsSyncBranchCheck {
	return &BeadsSyncBranchCheck{
		FixableCheck: FixableCheck{
			BaseCheck: BaseCheck{
				CheckName:        "beads-sync-branch",
				CheckDescription: "Verify rig beads have sync-branch configured",
			},
		},
	}
}

// Run checks if rig-level beads have sync-branch properly configured.
func (c *BeadsSyncBranchCheck) Run(ctx *CheckContext) *CheckResult {
	// Only check if a rig is specified
	if ctx.RigName == "" {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: "No rig specified, skipping rig beads sync-branch check",
		}
	}

	// Check rig-level beads config
	rigBeadsDir := filepath.Join(ctx.RigPath(), ".beads")
	configPath := filepath.Join(rigBeadsDir, "config.yaml")

	content, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &CheckResult{
				Name:    c.Name(),
				Status:  StatusWarning,
				Message: "No .beads/config.yaml in rig",
				FixHint: "Run 'bd init' in the rig directory",
			}
		}
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusError,
			Message: "Could not read .beads/config.yaml: " + err.Error(),
		}
	}

	// Check for sync-branch setting
	if !strings.Contains(string(content), "sync-branch:") {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusWarning,
			Message: "Rig beads missing sync-branch configuration",
			Details: []string{
				"Rig: " + ctx.RigName,
				"Rig beads need sync-branch for multi-clone coordination",
				"Without this, polecats and crew members can't share beads",
			},
			FixHint: "Run 'gt doctor --fix' or add 'sync-branch: beads-sync' to .beads/config.yaml",
		}
	}

	return &CheckResult{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: "Rig beads sync-branch is configured",
	}
}

// Fix adds sync-branch to the rig beads config.
func (c *BeadsSyncBranchCheck) Fix(ctx *CheckContext) error {
	if ctx.RigName == "" {
		return nil
	}

	rigBeadsDir := filepath.Join(ctx.RigPath(), ".beads")
	configPath := filepath.Join(rigBeadsDir, "config.yaml")

	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Check if already configured
	if strings.Contains(string(content), "sync-branch:") {
		return nil
	}

	// Append sync-branch setting
	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString("sync-branch: beads-sync\n")
	return err
}
