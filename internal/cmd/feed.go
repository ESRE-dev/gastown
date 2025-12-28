package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/workspace"
)

var (
	feedFollow   bool
	feedLimit    int
	feedSince    string
	feedMol      string
	feedType     string
	feedRig      string
	feedNoFollow bool
)

func init() {
	rootCmd.AddCommand(feedCmd)

	feedCmd.Flags().BoolVarP(&feedFollow, "follow", "f", false, "Stream events in real-time (default when no other flags)")
	feedCmd.Flags().BoolVar(&feedNoFollow, "no-follow", false, "Show events once and exit")
	feedCmd.Flags().IntVarP(&feedLimit, "limit", "n", 100, "Maximum number of events to show")
	feedCmd.Flags().StringVar(&feedSince, "since", "", "Show events since duration (e.g., 5m, 1h, 30s)")
	feedCmd.Flags().StringVar(&feedMol, "mol", "", "Filter by molecule/issue ID prefix")
	feedCmd.Flags().StringVar(&feedType, "type", "", "Filter by event type (create, update, delete, comment)")
	feedCmd.Flags().StringVar(&feedRig, "rig", "", "Run from specific rig's beads directory")
}

var feedCmd = &cobra.Command{
	Use:     "feed",
	GroupID: GroupDiag,
	Short:   "Show real-time activity feed from beads",
	Long: `Display a real-time feed of issue and molecule state changes.

This command wraps 'bd activity' to show mutations as they happen,
providing visibility into workflow progress across Gas Town.

By default, streams in follow mode. Use --no-follow to show events once.

Event symbols:
  +  created/bonded  - New issue or molecule created
  →  in_progress     - Work started on an issue
  ✓  completed       - Issue closed or step completed
  ✗  failed          - Step or issue failed
  ⊘  deleted         - Issue removed

Examples:
  gt feed                       # Stream all events (default: --follow)
  gt feed --no-follow           # Show last 100 events and exit
  gt feed --since 1h            # Events from last hour
  gt feed --mol gt-xyz          # Filter by issue prefix
  gt feed --rig gastown         # Use gastown rig's beads`,
	RunE: runFeed,
}

func runFeed(cmd *cobra.Command, args []string) error {
	// Find bd binary
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		return fmt.Errorf("bd not found in PATH: %w", err)
	}

	// Determine working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// If --rig specified, find that rig's beads directory
	if feedRig != "" {
		townRoot, err := workspace.FindFromCwdOrError()
		if err != nil {
			return fmt.Errorf("not in a Gas Town workspace: %w", err)
		}

		// Try common beads locations for the rig
		candidates := []string{
			fmt.Sprintf("%s/%s/mayor/rig", townRoot, feedRig),
			fmt.Sprintf("%s/%s", townRoot, feedRig),
		}

		found := false
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate + "/.beads"); err == nil {
				workDir = candidate
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("rig '%s' not found or has no .beads directory", feedRig)
		}
	}

	// Build bd activity command args
	bdArgs := []string{"bd", "activity"}

	// Default to follow mode unless --no-follow or other display flags set
	shouldFollow := !feedNoFollow
	if feedFollow {
		shouldFollow = true
	}

	if shouldFollow {
		bdArgs = append(bdArgs, "--follow")
	}

	if feedLimit != 100 {
		bdArgs = append(bdArgs, "--limit", fmt.Sprintf("%d", feedLimit))
	}

	if feedSince != "" {
		bdArgs = append(bdArgs, "--since", feedSince)
	}

	if feedMol != "" {
		bdArgs = append(bdArgs, "--mol", feedMol)
	}

	if feedType != "" {
		bdArgs = append(bdArgs, "--type", feedType)
	}

	// Use exec to replace the current process with bd
	// This gives clean signal handling and terminal control
	env := os.Environ()

	// Change to the target directory before exec
	if err := os.Chdir(workDir); err != nil {
		return fmt.Errorf("changing to directory %s: %w", workDir, err)
	}

	return syscall.Exec(bdPath, bdArgs, env)
}
