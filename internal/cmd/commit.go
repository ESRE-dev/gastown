package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

var (
	commitMessage     string
	commitAll         bool
	commitAmend       bool
	commitNoEdit      bool
	commitAllowEmpty  bool
	commitDryRun      bool
	commitNoTrailers  bool
	commitIncludeMol  bool
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Git commit with agent identity trailers",
	Long: `Create a git commit with agent identity metadata.

This command wraps git commit and automatically adds trailers identifying
the agent that performed the work:

  Executed-By: beads/crew/dave
  Rig: beads
  Role: crew

For polecats:
  Executed-By: beads/polecat/Nux-1766978911613
  Rig: beads
  Role: polecat

If a molecule is attached to your hook, it's also included:
  Molecule: bd-xyz

This enables forensic analysis and audit trails for agent-mediated commits.

Examples:
  gt commit -m "Fix authentication bug"
  gt commit -a -m "Refactor login flow"
  gt commit --no-trailers -m "Manual commit"  # Skip agent trailers
  gt commit --include-mol -m "Work on feature" # Include molecule even if not pinned`,
	RunE: runCommit,
}

func init() {
	commitCmd.Flags().StringVarP(&commitMessage, "message", "m", "", "Commit message")
	commitCmd.Flags().BoolVarP(&commitAll, "all", "a", false, "Stage all modified files")
	commitCmd.Flags().BoolVar(&commitAmend, "amend", false, "Amend the previous commit")
	commitCmd.Flags().BoolVar(&commitNoEdit, "no-edit", false, "Use the selected commit message without editing")
	commitCmd.Flags().BoolVar(&commitAllowEmpty, "allow-empty", false, "Allow empty commit")
	commitCmd.Flags().BoolVarP(&commitDryRun, "dry-run", "n", false, "Show what would be committed")
	commitCmd.Flags().BoolVar(&commitNoTrailers, "no-trailers", false, "Skip adding agent identity trailers")
	commitCmd.Flags().BoolVar(&commitIncludeMol, "include-mol", true, "Include molecule ID if attached")
	rootCmd.AddCommand(commitCmd)
}

// MoleculeStatus represents the output of gt mol status --json
type MoleculeStatus struct {
	HasMolecule bool   `json:"has_molecule"`
	MoleculeID  string `json:"molecule_id,omitempty"`
	Title       string `json:"title,omitempty"`
	Status      string `json:"status,omitempty"`
}

func runCommit(cmd *cobra.Command, args []string) error {
	// Build git commit arguments
	gitArgs := []string{"commit"}

	if commitAll {
		gitArgs = append(gitArgs, "-a")
	}
	if commitAmend {
		gitArgs = append(gitArgs, "--amend")
	}
	if commitNoEdit {
		gitArgs = append(gitArgs, "--no-edit")
	}
	if commitAllowEmpty {
		gitArgs = append(gitArgs, "--allow-empty")
	}
	if commitDryRun {
		gitArgs = append(gitArgs, "--dry-run")
	}

	// Build the commit message with trailers
	message := commitMessage
	if message == "" && !commitAmend && !commitNoEdit {
		return fmt.Errorf("commit message required (-m)")
	}

	// Add agent trailers unless disabled
	if !commitNoTrailers && message != "" {
		trailers := buildAgentTrailers()
		if len(trailers) > 0 {
			message = appendTrailers(message, trailers)
		}
	}

	// Add message to git args
	if message != "" {
		gitArgs = append(gitArgs, "-m", message)
	}

	// Add any extra args passed through
	gitArgs = append(gitArgs, args...)

	// Show what we're doing
	if commitDryRun {
		fmt.Printf("%s Would run: git %s\n", style.Bold.Render("🔍"), strings.Join(gitArgs, " "))
		if !commitNoTrailers {
			trailers := buildAgentTrailers()
			if len(trailers) > 0 {
				fmt.Printf("\n%s Trailers that would be added:\n", style.Bold.Render("📋"))
				for _, t := range trailers {
					fmt.Printf("  %s\n", t)
				}
			}
		}
		return nil
	}

	// Execute git commit
	gitCmd := exec.Command("git", gitArgs...)
	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr
	gitCmd.Stdin = os.Stdin

	return gitCmd.Run()
}

// buildAgentTrailers constructs the trailers for agent identity.
func buildAgentTrailers() []string {
	var trailers []string

	// Get agent identity
	cwd, err := os.Getwd()
	if err != nil {
		return trailers
	}

	townRoot, err := workspace.FindFromCwd()
	if err != nil || townRoot == "" {
		return trailers
	}

	roleInfo, err := GetRoleWithContext(cwd, townRoot)
	if err != nil {
		return trailers
	}

	// Skip if not in an agent context (unknown/human)
	if roleInfo.Role == RoleUnknown || roleInfo.Role == "" {
		return trailers
	}

	// Build Executed-By trailer
	ctx := RoleContext{
		Role:     roleInfo.Role,
		Rig:      roleInfo.Rig,
		Polecat:  roleInfo.Polecat,
		TownRoot: townRoot,
		WorkDir:  cwd,
	}
	identity := buildAgentIdentity(ctx)
	if identity != "" && identity != "overseer" {
		trailers = append(trailers, fmt.Sprintf("Executed-By: %s", identity))
	}

	// Add Rig trailer
	if roleInfo.Rig != "" {
		trailers = append(trailers, fmt.Sprintf("Rig: %s", roleInfo.Rig))
	}

	// Add Role trailer
	if roleInfo.Role != "" {
		trailers = append(trailers, fmt.Sprintf("Role: %s", roleInfo.Role))
	}

	// Check for pinned molecule
	if commitIncludeMol {
		if molID := getPinnedMolecule(); molID != "" {
			trailers = append(trailers, fmt.Sprintf("Molecule: %s", molID))
		}
	}

	return trailers
}

// getPinnedMolecule checks if there's a molecule attached to the agent's hook.
func getPinnedMolecule() string {
	// Try gt mol status --json
	cmd := exec.Command("gt", "mol", "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	var status MoleculeStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return ""
	}

	if status.HasMolecule && status.MoleculeID != "" {
		return status.MoleculeID
	}

	return ""
}

// appendTrailers adds git trailers to a commit message.
// Trailers are separated from the message body by a blank line.
func appendTrailers(message string, trailers []string) string {
	// Trim trailing whitespace from message
	message = strings.TrimRight(message, "\n\r\t ")

	// Add blank line separator and trailers
	var sb strings.Builder
	sb.WriteString(message)
	sb.WriteString("\n\n")
	for _, trailer := range trailers {
		sb.WriteString(trailer)
		sb.WriteString("\n")
	}

	return sb.String()
}
