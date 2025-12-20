package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/steveyegge/gastown/internal/tmux"
)

// OrphanSessionCheck detects orphaned tmux sessions that don't match
// the expected Gas Town session naming patterns.
type OrphanSessionCheck struct {
	FixableCheck
	orphanSessions []string // Cached during Run for use in Fix
}

// NewOrphanSessionCheck creates a new orphan session check.
func NewOrphanSessionCheck() *OrphanSessionCheck {
	return &OrphanSessionCheck{
		FixableCheck: FixableCheck{
			BaseCheck: BaseCheck{
				CheckName:        "orphan-sessions",
				CheckDescription: "Detect orphaned tmux sessions",
			},
		},
	}
}

// Run checks for orphaned Gas Town tmux sessions.
func (c *OrphanSessionCheck) Run(ctx *CheckContext) *CheckResult {
	t := tmux.NewTmux()

	sessions, err := t.ListSessions()
	if err != nil {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusWarning,
			Message: "Could not list tmux sessions",
			Details: []string{err.Error()},
		}
	}

	if len(sessions) == 0 {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: "No tmux sessions found",
		}
	}

	// Get list of valid rigs
	validRigs := c.getValidRigs(ctx.TownRoot)

	// Check each session
	var orphans []string
	var validCount int

	for _, session := range sessions {
		if session == "" {
			continue
		}

		// Only check gt-* sessions (Gas Town sessions)
		if !strings.HasPrefix(session, "gt-") {
			continue
		}

		if c.isValidSession(session, validRigs) {
			validCount++
		} else {
			orphans = append(orphans, session)
		}
	}

	// Cache orphans for Fix
	c.orphanSessions = orphans

	if len(orphans) == 0 {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: fmt.Sprintf("All %d Gas Town sessions are valid", validCount),
		}
	}

	details := make([]string, len(orphans))
	for i, session := range orphans {
		details[i] = fmt.Sprintf("Orphan: %s", session)
	}

	return &CheckResult{
		Name:    c.Name(),
		Status:  StatusWarning,
		Message: fmt.Sprintf("Found %d orphaned session(s)", len(orphans)),
		Details: details,
		FixHint: "Run 'gt doctor --fix' to kill orphaned sessions",
	}
}

// Fix kills all orphaned sessions.
func (c *OrphanSessionCheck) Fix(ctx *CheckContext) error {
	if len(c.orphanSessions) == 0 {
		return nil
	}

	t := tmux.NewTmux()
	var lastErr error

	for _, session := range c.orphanSessions {
		if err := t.KillSession(session); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// getValidRigs returns a list of valid rig names from the workspace.
func (c *OrphanSessionCheck) getValidRigs(townRoot string) []string {
	var rigs []string

	// Read rigs.json if it exists
	rigsPath := filepath.Join(townRoot, "mayor", "rigs.json")
	if _, err := os.Stat(rigsPath); err == nil {
		// For simplicity, just scan directories at town root that look like rigs
		entries, err := os.ReadDir(townRoot)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() && entry.Name() != "mayor" && entry.Name() != ".beads" && !strings.HasPrefix(entry.Name(), ".") {
					// Check if it looks like a rig (has polecats/ or crew/ directory)
					polecatsDir := filepath.Join(townRoot, entry.Name(), "polecats")
					crewDir := filepath.Join(townRoot, entry.Name(), "crew")
					if _, err := os.Stat(polecatsDir); err == nil {
						rigs = append(rigs, entry.Name())
					} else if _, err := os.Stat(crewDir); err == nil {
						rigs = append(rigs, entry.Name())
					}
				}
			}
		}
	}

	return rigs
}

// isValidSession checks if a session name matches expected Gas Town patterns.
// Valid patterns:
//   - gt-mayor
//   - gt-deacon
//   - gt-<rig>-witness
//   - gt-<rig>-refinery
//   - gt-<rig>-<polecat> (where polecat is any name)
//
// Note: We can't verify polecat names without reading state, so we're permissive.
func (c *OrphanSessionCheck) isValidSession(session string, validRigs []string) bool {
	// gt-mayor is always valid
	if session == "gt-mayor" {
		return true
	}

	// gt-deacon is always valid
	if session == "gt-deacon" {
		return true
	}

	// For rig-specific sessions, extract rig name
	// Pattern: gt-<rig>-<role>
	parts := strings.SplitN(session, "-", 3)
	if len(parts) < 3 {
		// Invalid format - must be gt-<rig>-<something>
		return false
	}

	rigName := parts[1]

	// Check if this rig exists
	rigFound := false
	for _, r := range validRigs {
		if r == rigName {
			rigFound = true
			break
		}
	}

	if !rigFound {
		// Unknown rig - this is an orphan
		return false
	}

	role := parts[2]

	// witness and refinery are valid roles
	if role == "witness" || role == "refinery" {
		return true
	}

	// Any other name is assumed to be a polecat or crew member
	// We can't easily verify without reading state, so accept it
	return true
}

// OrphanProcessCheck detects orphaned Claude/claude-code processes
// that are not associated with a Gas Town tmux session.
type OrphanProcessCheck struct {
	FixableCheck
	orphanPIDs []int // Cached during Run for use in Fix
}

// NewOrphanProcessCheck creates a new orphan process check.
func NewOrphanProcessCheck() *OrphanProcessCheck {
	return &OrphanProcessCheck{
		FixableCheck: FixableCheck{
			BaseCheck: BaseCheck{
				CheckName:        "orphan-processes",
				CheckDescription: "Detect orphaned Claude processes",
			},
		},
	}
}

// Run checks for orphaned Claude processes.
func (c *OrphanProcessCheck) Run(ctx *CheckContext) *CheckResult {
	// Get list of tmux session PIDs
	tmuxPIDs, err := c.getTmuxSessionPIDs()
	if err != nil {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusWarning,
			Message: "Could not get tmux session info",
			Details: []string{err.Error()},
		}
	}

	// Find Claude processes
	claudeProcs, err := c.findClaudeProcesses()
	if err != nil {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusWarning,
			Message: "Could not list Claude processes",
			Details: []string{err.Error()},
		}
	}

	if len(claudeProcs) == 0 {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: "No Claude processes found",
		}
	}

	// Check which Claude processes are orphaned
	var orphans []processInfo
	var validCount int

	for _, proc := range claudeProcs {
		if c.isOrphanProcess(proc, tmuxPIDs) {
			orphans = append(orphans, proc)
		} else {
			validCount++
		}
	}

	// Cache orphan PIDs for Fix
	c.orphanPIDs = make([]int, len(orphans))
	for i, p := range orphans {
		c.orphanPIDs[i] = p.pid
	}

	if len(orphans) == 0 {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: fmt.Sprintf("All %d Claude processes have valid parents", validCount),
		}
	}

	details := make([]string, len(orphans))
	for i, proc := range orphans {
		details[i] = fmt.Sprintf("PID %d: %s (parent: %d)", proc.pid, proc.cmd, proc.ppid)
	}

	return &CheckResult{
		Name:    c.Name(),
		Status:  StatusWarning,
		Message: fmt.Sprintf("Found %d orphaned Claude process(es)", len(orphans)),
		Details: details,
		FixHint: "Run 'gt doctor --fix' to kill orphaned processes",
	}
}

// Fix kills all orphaned processes.
func (c *OrphanProcessCheck) Fix(ctx *CheckContext) error {
	if len(c.orphanPIDs) == 0 {
		return nil
	}

	var lastErr error
	for _, pid := range c.orphanPIDs {
		proc, err := os.FindProcess(pid)
		if err != nil {
			lastErr = err
			continue
		}
		if err := proc.Signal(os.Interrupt); err != nil {
			// Try SIGKILL if SIGINT fails
			if killErr := proc.Kill(); killErr != nil {
				lastErr = killErr
			}
		}
	}

	return lastErr
}

type processInfo struct {
	pid  int
	ppid int
	cmd  string
}

// getTmuxSessionPIDs returns PIDs of all tmux server processes.
func (c *OrphanProcessCheck) getTmuxSessionPIDs() (map[int]bool, error) {
	// Get tmux server PID and all pane PIDs
	pids := make(map[int]bool)

	// Find tmux server process
	out, err := exec.Command("pgrep", "-x", "tmux").Output()
	if err != nil {
		// No tmux server running
		return pids, nil
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		var pid int
		if _, err := fmt.Sscanf(line, "%d", &pid); err == nil {
			pids[pid] = true
		}
	}

	// Also get shell PIDs inside tmux panes
	t := tmux.NewTmux()
	sessions, _ := t.ListSessions()
	for _, session := range sessions {
		// Get pane PIDs for this session
		out, err := exec.Command("tmux", "list-panes", "-t", session, "-F", "#{pane_pid}").Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			var pid int
			if _, err := fmt.Sscanf(line, "%d", &pid); err == nil {
				pids[pid] = true
			}
		}
	}

	return pids, nil
}

// findClaudeProcesses finds all running claude/claude-code processes.
func (c *OrphanProcessCheck) findClaudeProcesses() ([]processInfo, error) {
	var procs []processInfo

	// Use ps to find claude processes
	// Look for both "claude" and "claude-code" in command
	out, err := exec.Command("ps", "-eo", "pid,ppid,comm").Output()
	if err != nil {
		return nil, err
	}

	// Regex to match claude processes
	claudePattern := regexp.MustCompile(`(?i)claude`)

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Check if command contains "claude"
		cmd := strings.Join(fields[2:], " ")
		if !claudePattern.MatchString(cmd) {
			continue
		}

		var pid, ppid int
		if _, err := fmt.Sscanf(fields[0], "%d", &pid); err != nil {
			continue
		}
		if _, err := fmt.Sscanf(fields[1], "%d", &ppid); err != nil {
			continue
		}

		procs = append(procs, processInfo{
			pid:  pid,
			ppid: ppid,
			cmd:  cmd,
		})
	}

	return procs, nil
}

// isOrphanProcess checks if a Claude process is orphaned.
// A process is orphaned if its parent (or ancestor) is not a tmux session.
func (c *OrphanProcessCheck) isOrphanProcess(proc processInfo, tmuxPIDs map[int]bool) bool {
	// Walk up the process tree looking for a tmux parent
	currentPPID := proc.ppid
	visited := make(map[int]bool)

	for currentPPID > 1 && !visited[currentPPID] {
		visited[currentPPID] = true

		// Check if this is a tmux process
		if tmuxPIDs[currentPPID] {
			return false // Has tmux ancestor, not orphaned
		}

		// Get parent's parent
		out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", currentPPID), "-o", "ppid=").Output()
		if err != nil {
			break
		}

		var nextPPID int
		if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &nextPPID); err != nil {
			break
		}
		currentPPID = nextPPID
	}

	return true // No tmux ancestor found
}
