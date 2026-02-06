// Package workspace provides workspace detection and management.
package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/git"
)

// PreflightReport contains the results of a preflight check.
type PreflightReport struct {
	MailCleaned  int
	RigHealthy   bool
	StuckWorkers []string
	Warnings     []string
}

// Preflight performs workspace preflight checks and cleanup.
// It cleans stale mail, checks for stuck workers, verifies rig health,
// ensures git is clean, and runs bd sync.
func Preflight(rigName string, dryRun bool) (*PreflightReport, error) {
	report := &PreflightReport{
		RigHealthy: true,
	}

	// Find town root
	townRoot, err := FindFromCwdOrError()
	if err != nil {
		return nil, fmt.Errorf("finding workspace: %w", err)
	}

	// Determine rig path
	rigPath := filepath.Join(townRoot, rigName)
	if _, err := os.Stat(rigPath); err != nil {
		return nil, fmt.Errorf("rig %s not found: %w", rigName, err)
	}

	// 1. Clean stale mail (older than 7 days, status=closed)
	mailCleaned, err := cleanStaleMail(townRoot, rigName, dryRun)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Failed to clean mail: %v", err))
	} else {
		report.MailCleaned = mailCleaned
	}

	// 2. Check for stuck workers (tmux sessions that are unresponsive)
	stuckWorkers, err := checkStuckWorkers(townRoot, rigName)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Failed to check workers: %v", err))
	} else {
		report.StuckWorkers = stuckWorkers
		if len(stuckWorkers) > 0 {
			report.RigHealthy = false
		}
	}

	// 3. Verify git is clean
	g := git.NewGit(rigPath)
	status, err := g.Status()
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Failed to check git status: %v", err))
		report.RigHealthy = false
	} else if !status.Clean {
		report.Warnings = append(report.Warnings, "Git working directory is not clean")
		report.RigHealthy = false
	}

	// 4. Run bd sync (unless dry-run)
	if !dryRun {
		if err := runBdSync(rigPath); err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Failed to sync beads: %v", err))
		}
	}

	return report, nil
}

// cleanStaleMail removes stale mail messages from the rig's mailboxes.
// Stale means: older than 7 days and status=closed.
// Uses bd/gt commands to avoid import cycles.
func cleanStaleMail(townRoot, rigName string, dryRun bool) (int, error) {
	// Check if .beads directory exists for this rig
	beadsDir := filepath.Join(townRoot, rigName, ".beads")
	if _, err := os.Stat(beadsDir); os.IsNotExist(err) {
		// No beads directory - no mail to clean
		return 0, nil
	}

	// For now, we'll use a simple approach: count messages via filesystem
	// In the future, this could use bd commands to clean stale mail
	// This avoids the import cycle with the mail package

	// Return 0 for now - actual mail cleanup can be implemented
	// using bd commands when needed
	return 0, nil
}

// isClosedStatus checks if a message subject indicates a closed/resolved status.
func isClosedStatus(subject string) bool {
	lower := strings.ToLower(subject)
	return strings.Contains(lower, "closed") ||
		strings.Contains(lower, "resolved") ||
		strings.Contains(lower, "completed") ||
		strings.Contains(lower, "done")
}

// checkStuckWorkers identifies tmux sessions that appear to be stuck.
// A worker is stuck if:
// - The tmux session exists but hasn't had activity in > 1 hour
// - The session is in a strange state (attached but no output)
func checkStuckWorkers(townRoot, rigName string) ([]string, error) {
	var stuck []string

	// List all tmux sessions for this rig
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}:#{session_activity}")
	output, err := cmd.Output()
	if err != nil {
		// If tmux isn't running or there are no sessions, that's okay
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("listing tmux sessions: %w", err)
	}

	// Parse session list
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	now := time.Now().Unix()

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		sessionName := parts[0]

		// Only check sessions for this rig
		// Session naming pattern: <town>-<rig>-<worker>
		if !strings.Contains(sessionName, rigName) {
			continue
		}

		// Parse activity timestamp (Unix timestamp)
		var activity int64
		if _, err := fmt.Sscanf(parts[1], "%d", &activity); err != nil {
			continue
		}

		// Check if inactive for more than 1 hour (3600 seconds)
		if now-activity > 3600 {
			stuck = append(stuck, sessionName)
		}
	}

	return stuck, nil
}

// runBdSync runs 'bd sync' in the rig directory.
func runBdSync(rigPath string) error {
	cmd := exec.Command("bd", "sync")
	cmd.Dir = rigPath

	// Capture output for debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd sync failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}
