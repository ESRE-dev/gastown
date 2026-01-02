// Package cmd provides CLI commands for the gt tool.
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/tmux"
)

var (
	costsJSON   bool
	costsToday  bool
	costsWeek   bool
	costsByRole bool
	costsByRig  bool

	// Record subcommand flags
	recordSession  string
	recordWorkItem string
)

var costsCmd = &cobra.Command{
	Use:     "costs",
	GroupID: GroupDiag,
	Short:   "Show costs for running Claude sessions",
	Long: `Display costs for Claude Code sessions in Gas Town.

By default, shows live costs scraped from running tmux sessions.

Examples:
  gt costs              # Live costs from running sessions
  gt costs --today      # Today's total from ledger
  gt costs --week       # This week's total
  gt costs --by-role    # Breakdown by role (polecat, witness, etc.)
  gt costs --by-rig     # Breakdown by rig
  gt costs --json       # Output as JSON`,
	RunE: runCosts,
}

var costsRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record session cost to ledger (called by Stop hook)",
	Long: `Record the final cost of a session to the cost ledger.

This command is intended to be called from a Claude Code Stop hook.
It captures the final cost from the tmux session and writes it to
~/.gt/costs.jsonl.

Examples:
  gt costs record --session gt-gastown-toast
  gt costs record --session gt-gastown-toast --work-item gt-abc123`,
	RunE: runCostsRecord,
}

func init() {
	rootCmd.AddCommand(costsCmd)
	costsCmd.Flags().BoolVar(&costsJSON, "json", false, "Output as JSON")
	costsCmd.Flags().BoolVar(&costsToday, "today", false, "Show today's total from ledger")
	costsCmd.Flags().BoolVar(&costsWeek, "week", false, "Show this week's total from ledger")
	costsCmd.Flags().BoolVar(&costsByRole, "by-role", false, "Show breakdown by role")
	costsCmd.Flags().BoolVar(&costsByRig, "by-rig", false, "Show breakdown by rig")

	// Add record subcommand
	costsCmd.AddCommand(costsRecordCmd)
	costsRecordCmd.Flags().StringVar(&recordSession, "session", "", "Tmux session name to record")
	costsRecordCmd.Flags().StringVar(&recordWorkItem, "work-item", "", "Work item ID (bead) for attribution")
}

// SessionCost represents cost info for a single session.
type SessionCost struct {
	Session string  `json:"session"`
	Role    string  `json:"role"`
	Rig     string  `json:"rig,omitempty"`
	Worker  string  `json:"worker,omitempty"`
	Cost    float64 `json:"cost_usd"`
	Running bool    `json:"running"`
}

// CostEntry is a ledger entry for historical cost tracking.
type CostEntry struct {
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Rig       string    `json:"rig,omitempty"`
	Worker    string    `json:"worker,omitempty"`
	CostUSD   float64   `json:"cost_usd"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	WorkItem  string    `json:"work_item,omitempty"`
}

// CostsOutput is the JSON output structure.
type CostsOutput struct {
	Sessions []SessionCost      `json:"sessions,omitempty"`
	Total    float64            `json:"total_usd"`
	ByRole   map[string]float64 `json:"by_role,omitempty"`
	ByRig    map[string]float64 `json:"by_rig,omitempty"`
	Period   string             `json:"period,omitempty"`
}

// costRegex matches cost patterns like "$1.23" or "$12.34"
var costRegex = regexp.MustCompile(`\$(\d+\.\d{2})`)

func runCosts(cmd *cobra.Command, args []string) error {
	// If querying ledger, use ledger functions
	if costsToday || costsWeek || costsByRole || costsByRig {
		return runCostsFromLedger()
	}

	// Default: show live costs from running sessions
	return runLiveCosts()
}

func runLiveCosts() error {
	t := tmux.NewTmux()

	// Get all tmux sessions
	sessions, err := t.ListSessions()
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	var costs []SessionCost
	var total float64

	for _, session := range sessions {
		// Only process Gas Town sessions (start with "gt-")
		if !strings.HasPrefix(session, constants.SessionPrefix) {
			continue
		}

		// Parse session name to get role/rig/worker
		role, rig, worker := parseSessionName(session)

		// Capture pane content
		content, err := t.CapturePaneAll(session)
		if err != nil {
			continue // Skip sessions we can't capture
		}

		// Extract cost from content
		cost := extractCost(content)

		// Check if Claude is running
		running := t.IsClaudeRunning(session)

		costs = append(costs, SessionCost{
			Session: session,
			Role:    role,
			Rig:     rig,
			Worker:  worker,
			Cost:    cost,
			Running: running,
		})
		total += cost
	}

	// Sort by session name
	sort.Slice(costs, func(i, j int) bool {
		return costs[i].Session < costs[j].Session
	})

	if costsJSON {
		return outputCostsJSON(CostsOutput{
			Sessions: costs,
			Total:    total,
		})
	}

	return outputCostsHuman(costs, total)
}

func runCostsFromLedger() error {
	ledgerPath := getLedgerPath()
	entries, err := readLedger(ledgerPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println(style.Dim.Render("No cost ledger found. Costs are recorded when sessions end."))
			return nil
		}
		return fmt.Errorf("reading ledger: %w", err)
	}

	// Filter entries by time period
	var filtered []CostEntry
	now := time.Now()

	for _, entry := range entries {
		if costsToday {
			// Today: same day
			if entry.EndedAt.Year() == now.Year() &&
				entry.EndedAt.YearDay() == now.YearDay() {
				filtered = append(filtered, entry)
			}
		} else if costsWeek {
			// This week: within 7 days
			weekAgo := now.AddDate(0, 0, -7)
			if entry.EndedAt.After(weekAgo) {
				filtered = append(filtered, entry)
			}
		} else {
			// No time filter
			filtered = append(filtered, entry)
		}
	}

	// Calculate totals
	var total float64
	byRole := make(map[string]float64)
	byRig := make(map[string]float64)

	for _, entry := range filtered {
		total += entry.CostUSD
		byRole[entry.Role] += entry.CostUSD
		if entry.Rig != "" {
			byRig[entry.Rig] += entry.CostUSD
		}
	}

	// Build output
	output := CostsOutput{
		Total: total,
	}

	if costsByRole {
		output.ByRole = byRole
	}
	if costsByRig {
		output.ByRig = byRig
	}

	// Set period label
	if costsToday {
		output.Period = "today"
	} else if costsWeek {
		output.Period = "this week"
	}

	if costsJSON {
		return outputCostsJSON(output)
	}

	return outputLedgerHuman(output, filtered)
}

// parseSessionName extracts role, rig, and worker from a session name.
// Session names follow the pattern: gt-<rig>-<worker> or gt-<global-agent>
// Examples:
//   - gt-mayor -> role=mayor, rig="", worker="mayor"
//   - gt-deacon -> role=deacon, rig="", worker="deacon"
//   - gt-gastown-toast -> role=polecat, rig=gastown, worker=toast
//   - gt-gastown-witness -> role=witness, rig=gastown, worker=""
//   - gt-gastown-refinery -> role=refinery, rig=gastown, worker=""
//   - gt-gastown-crew-joe -> role=crew, rig=gastown, worker=joe
func parseSessionName(session string) (role, rig, worker string) {
	// Remove gt- prefix
	name := strings.TrimPrefix(session, constants.SessionPrefix)

	// Check for global agents
	switch name {
	case "mayor":
		return constants.RoleMayor, "", "mayor"
	case "deacon":
		return constants.RoleDeacon, "", "deacon"
	}

	// Parse rig-based session: rig-worker or rig-crew-name
	parts := strings.SplitN(name, "-", 3)
	if len(parts) < 2 {
		return "unknown", "", name
	}

	rig = parts[0]
	worker = parts[1]

	// Check for crew pattern: rig-crew-name
	if worker == "crew" && len(parts) >= 3 {
		return constants.RoleCrew, rig, parts[2]
	}

	// Check for special workers
	switch worker {
	case "witness":
		return constants.RoleWitness, rig, ""
	case "refinery":
		return constants.RoleRefinery, rig, ""
	}

	// Default to polecat
	return constants.RolePolecat, rig, worker
}

// extractCost finds the most recent cost value in pane content.
// Claude Code displays cost in the format "$X.XX" in the status area.
func extractCost(content string) float64 {
	matches := costRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return 0.0
	}

	// Get the last (most recent) match
	lastMatch := matches[len(matches)-1]
	if len(lastMatch) < 2 {
		return 0.0
	}

	var cost float64
	_, _ = fmt.Sscanf(lastMatch[1], "%f", &cost)
	return cost
}

// getLedgerPath returns the path to the cost ledger file.
func getLedgerPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gt", "costs.jsonl")
}

// readLedger reads all entries from the cost ledger.
func readLedger(path string) ([]CostEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []CostEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry CostEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // Skip malformed lines
		}
		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// WriteLedgerEntry appends a cost entry to the ledger.
// This is called by the SessionEnd hook handler.
func WriteLedgerEntry(entry CostEntry) error {
	path := getLedgerPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating ledger directory: %w", err)
	}

	// Open file for appending
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening ledger: %w", err)
	}
	defer file.Close()

	// Write JSON line
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling entry: %w", err)
	}

	_, err = file.Write(append(data, '\n'))
	return err
}

func outputCostsJSON(output CostsOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputCostsHuman(costs []SessionCost, total float64) error {
	if len(costs) == 0 {
		fmt.Println(style.Dim.Render("No Gas Town sessions found"))
		return nil
	}

	fmt.Printf("\n%s Live Session Costs\n\n", style.Bold.Render("💰"))

	// Print table header
	fmt.Printf("%-25s %-10s %-15s %10s %8s\n",
		"Session", "Role", "Rig/Worker", "Cost", "Status")
	fmt.Println(strings.Repeat("─", 75))

	// Print each session
	for _, c := range costs {
		statusIcon := style.Success.Render("●")
		if !c.Running {
			statusIcon = style.Dim.Render("○")
		}

		rigWorker := c.Rig
		if c.Worker != "" && c.Worker != c.Rig {
			if rigWorker != "" {
				rigWorker += "/" + c.Worker
			} else {
				rigWorker = c.Worker
			}
		}

		fmt.Printf("%-25s %-10s %-15s %10s %8s\n",
			c.Session,
			c.Role,
			rigWorker,
			fmt.Sprintf("$%.2f", c.Cost),
			statusIcon)
	}

	// Print total
	fmt.Println(strings.Repeat("─", 75))
	fmt.Printf("%s %s\n", style.Bold.Render("Total:"), fmt.Sprintf("$%.2f", total))

	return nil
}

func outputLedgerHuman(output CostsOutput, entries []CostEntry) error {
	periodStr := ""
	if output.Period != "" {
		periodStr = fmt.Sprintf(" (%s)", output.Period)
	}

	fmt.Printf("\n%s Cost Summary%s\n\n", style.Bold.Render("📊"), periodStr)

	// Total
	fmt.Printf("%s $%.2f\n", style.Bold.Render("Total:"), output.Total)

	// By role breakdown
	if output.ByRole != nil && len(output.ByRole) > 0 {
		fmt.Printf("\n%s\n", style.Bold.Render("By Role:"))
		for role, cost := range output.ByRole {
			icon := constants.RoleEmoji(role)
			fmt.Printf("  %s %-12s $%.2f\n", icon, role, cost)
		}
	}

	// By rig breakdown
	if output.ByRig != nil && len(output.ByRig) > 0 {
		fmt.Printf("\n%s\n", style.Bold.Render("By Rig:"))
		for rig, cost := range output.ByRig {
			fmt.Printf("  %-15s $%.2f\n", rig, cost)
		}
	}

	// Session count
	fmt.Printf("\n%s %d sessions\n", style.Dim.Render("Entries:"), len(entries))

	return nil
}

// runCostsRecord captures the final cost from a session and writes to ledger.
// This is called by the Claude Code Stop hook.
func runCostsRecord(cmd *cobra.Command, args []string) error {
	// Get session from flag or try to detect from environment
	session := recordSession
	if session == "" {
		// Try to get from TMUX_PANE or tmux environment
		session = os.Getenv("GT_SESSION")
	}
	if session == "" {
		return fmt.Errorf("--session flag required (or set GT_SESSION env var)")
	}

	t := tmux.NewTmux()

	// Capture pane content
	content, err := t.CapturePaneAll(session)
	if err != nil {
		// Session may already be gone - that's OK, we'll record with zero cost
		content = ""
	}

	// Extract cost
	cost := extractCost(content)

	// Parse session name
	role, rig, worker := parseSessionName(session)

	// Create ledger entry
	entry := CostEntry{
		SessionID: session,
		Role:      role,
		Rig:       rig,
		Worker:    worker,
		CostUSD:   cost,
		StartedAt: time.Time{}, // We don't have start time; could enhance later
		EndedAt:   time.Now(),
		WorkItem:  recordWorkItem,
	}

	// Write to ledger
	if err := WriteLedgerEntry(entry); err != nil {
		return fmt.Errorf("writing ledger: %w", err)
	}

	// Output confirmation (silent if cost is zero and no work item)
	if cost > 0 || recordWorkItem != "" {
		fmt.Printf("%s Recorded $%.2f for %s", style.Success.Render("✓"), cost, session)
		if recordWorkItem != "" {
			fmt.Printf(" (work: %s)", recordWorkItem)
		}
		fmt.Println()
	}

	return nil
}
