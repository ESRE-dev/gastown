package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/tmux"
)

var recycleCmd = &cobra.Command{
	Use:   "recycle [role]",
	Short: "Hot-reload the current (or specified) agent session",
	Long: `Instantly restart an agent session in place.

This command uses tmux respawn-pane to kill the current session and restart it
with a fresh Claude instance, running the full startup/priming sequence.

When run without arguments, recycles the current session.
When given a role name, recycles that role's session (and switches to it).

Examples:
  gt recycle           # Recycle current session
  gt recycle crew      # Recycle crew session (auto-detect name)
  gt recycle mayor     # Recycle mayor session
  gt recycle witness   # Recycle witness session for current rig

The command executes instantly - no handoff, no manager involved.
Use 'gt handoff' for graceful lifecycle transitions with context preservation.`,
	RunE: runRecycle,
}

var (
	recycleWatch  bool
	recycleDryRun bool
)

func init() {
	recycleCmd.Flags().BoolVarP(&recycleWatch, "watch", "w", true, "Switch to recycled session (for remote recycle)")
	recycleCmd.Flags().BoolVarP(&recycleDryRun, "dry-run", "n", false, "Show what would be done without executing")
	rootCmd.AddCommand(recycleCmd)
}

func runRecycle(cmd *cobra.Command, args []string) error {
	t := tmux.NewTmux()

	// Verify we're in tmux
	if !tmux.IsInsideTmux() {
		return fmt.Errorf("not running in tmux - cannot recycle")
	}

	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		return fmt.Errorf("TMUX_PANE not set - cannot recycle")
	}

	// Get current session name
	currentSession, err := getCurrentTmuxSession()
	if err != nil {
		return fmt.Errorf("getting session name: %w", err)
	}

	// Determine target session
	targetSession := currentSession
	if len(args) > 0 {
		// User specified a role to recycle
		targetSession, err = resolveRoleToSession(args[0])
		if err != nil {
			return fmt.Errorf("resolving role: %w", err)
		}
	}

	// Build the restart command
	restartCmd, err := buildRestartCommand(targetSession)
	if err != nil {
		return err
	}

	// If recycling a different session, we need to find its pane and respawn there
	if targetSession != currentSession {
		return recycleRemoteSession(t, targetSession, restartCmd)
	}

	// Recycling ourselves - print feedback then respawn
	fmt.Printf("%s Recycling %s...\n", style.Bold.Render("♻️"), currentSession)

	// Dry run mode - show what would happen
	if recycleDryRun {
		fmt.Printf("Would execute: tmux respawn-pane -k -t %s %s\n", pane, restartCmd)
		return nil
	}

	// Use exec to respawn the pane - this kills us and restarts
	return t.RespawnPane(pane, restartCmd)
}

// getCurrentTmuxSession returns the current tmux session name.
func getCurrentTmuxSession() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// resolveRoleToSession converts a role name to a tmux session name.
// For roles that need context (crew, witness, refinery), it auto-detects from environment.
func resolveRoleToSession(role string) (string, error) {
	switch strings.ToLower(role) {
	case "mayor", "may":
		return "gt-mayor", nil

	case "deacon", "dea":
		return "gt-deacon", nil

	case "crew":
		// Try to get rig and crew name from environment or cwd
		rig := os.Getenv("GT_RIG")
		crewName := os.Getenv("GT_CREW")
		if rig == "" || crewName == "" {
			// Try to detect from cwd
			detected, err := detectCrewFromCwd()
			if err == nil {
				rig = detected.rigName
				crewName = detected.crewName
			}
		}
		if rig == "" || crewName == "" {
			return "", fmt.Errorf("cannot determine crew identity - run from crew directory or specify GT_RIG/GT_CREW")
		}
		return fmt.Sprintf("gt-%s-crew-%s", rig, crewName), nil

	case "witness", "wit":
		rig := os.Getenv("GT_RIG")
		if rig == "" {
			return "", fmt.Errorf("cannot determine rig - set GT_RIG or run from rig context")
		}
		return fmt.Sprintf("gt-%s-witness", rig), nil

	case "refinery", "ref":
		rig := os.Getenv("GT_RIG")
		if rig == "" {
			return "", fmt.Errorf("cannot determine rig - set GT_RIG or run from rig context")
		}
		return fmt.Sprintf("gt-%s-refinery", rig), nil

	default:
		// Assume it's a direct session name
		return role, nil
	}
}

// buildRestartCommand creates the gt command to restart a session.
func buildRestartCommand(sessionName string) (string, error) {
	switch {
	case sessionName == "gt-mayor":
		return "gt may at", nil

	case sessionName == "gt-deacon":
		return "gt dea at", nil

	case strings.Contains(sessionName, "-crew-"):
		// gt-<rig>-crew-<name>
		// The attach command can auto-detect from cwd, so just use `gt crew at`
		return "gt crew at", nil

	case strings.HasSuffix(sessionName, "-witness"):
		// gt-<rig>-witness
		return "gt wit at", nil

	case strings.HasSuffix(sessionName, "-refinery"):
		// gt-<rig>-refinery
		return "gt ref at", nil

	default:
		return "", fmt.Errorf("unknown session type: %s (try specifying role explicitly)", sessionName)
	}
}

// recycleRemoteSession respawns a different session and optionally switches to it.
func recycleRemoteSession(t *tmux.Tmux, targetSession, restartCmd string) error {
	// Check if target session exists
	exists, err := t.HasSession(targetSession)
	if err != nil {
		return fmt.Errorf("checking session: %w", err)
	}
	if !exists {
		return fmt.Errorf("session '%s' not found - is the agent running?", targetSession)
	}

	// Get the pane ID for the target session
	targetPane, err := getSessionPane(targetSession)
	if err != nil {
		return fmt.Errorf("getting target pane: %w", err)
	}

	fmt.Printf("%s Recycling %s...\n", style.Bold.Render("♻️"), targetSession)

	// Dry run mode
	if recycleDryRun {
		fmt.Printf("Would execute: tmux respawn-pane -k -t %s %s\n", targetPane, restartCmd)
		if recycleWatch {
			fmt.Printf("Would execute: tmux switch-client -t %s\n", targetSession)
		}
		return nil
	}

	// Respawn the remote session's pane
	if err := t.RespawnPane(targetPane, restartCmd); err != nil {
		return fmt.Errorf("respawning pane: %w", err)
	}

	// If --watch, switch to that session
	if recycleWatch {
		fmt.Printf("Switching to %s...\n", targetSession)
		// Use tmux switch-client to move our view to the target session
		if err := exec.Command("tmux", "switch-client", "-t", targetSession).Run(); err != nil {
			// Non-fatal - they can manually switch
			fmt.Printf("Note: Could not auto-switch (use: tmux switch-client -t %s)\n", targetSession)
		}
	}

	return nil
}

// getSessionPane returns the pane identifier for a session's main pane.
func getSessionPane(sessionName string) (string, error) {
	// Get the pane ID for the first pane in the session
	out, err := exec.Command("tmux", "list-panes", "-t", sessionName, "-F", "#{pane_id}").Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("no panes found in session")
	}
	return lines[0], nil
}
