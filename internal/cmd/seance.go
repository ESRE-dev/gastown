package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/claude"
	"github.com/steveyegge/gastown/internal/style"
)

var (
	seanceAll    bool
	seanceRole   string
	seanceRig    string
	seanceRecent int
	seanceJSON   bool
)

var seanceCmd = &cobra.Command{
	Use:     "seance",
	GroupID: GroupDiag,
	Short:   "Discover and browse predecessor sessions",
	Long: `Find and resume predecessor Claude Code sessions.

Seance scans Claude Code's session history to find Gas Town sessions.
Sessions are identified by the [GAS TOWN] beacon sent during startup.

Examples:
  gt seance                     # List recent Gas Town sessions
  gt seance --all               # Include non-Gas Town sessions
  gt seance --role crew         # Filter by role type
  gt seance --rig gastown       # Filter by rig
  gt seance --recent 10         # Last 10 sessions
  gt seance --json              # JSON output

Resume a session in Claude Code:
  claude --resume <session-id>

The beacon format parsed:
  [GAS TOWN] gastown/crew/joe • assigned:gt-xyz • 2025-12-30T15:42`,
	RunE: runSeance,
}

func init() {
	seanceCmd.Flags().BoolVarP(&seanceAll, "all", "a", false, "Include non-Gas Town sessions")
	seanceCmd.Flags().StringVar(&seanceRole, "role", "", "Filter by role (crew, polecat, witness, etc.)")
	seanceCmd.Flags().StringVar(&seanceRig, "rig", "", "Filter by rig name")
	seanceCmd.Flags().IntVarP(&seanceRecent, "recent", "n", 20, "Number of recent sessions to show")
	seanceCmd.Flags().BoolVar(&seanceJSON, "json", false, "Output as JSON")

	rootCmd.AddCommand(seanceCmd)
}

func runSeance(cmd *cobra.Command, args []string) error {
	filter := claude.SessionFilter{
		GasTownOnly: !seanceAll,
		Role:        seanceRole,
		Rig:         seanceRig,
		Limit:       seanceRecent,
	}

	sessions, err := claude.DiscoverSessions(filter)
	if err != nil {
		return fmt.Errorf("discovering sessions: %w", err)
	}

	if seanceJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sessions)
	}

	if len(sessions) == 0 {
		if seanceAll {
			fmt.Println("No sessions found.")
		} else {
			fmt.Println("No Gas Town sessions found.")
			fmt.Println(style.Dim.Render("Use --all to include non-Gas Town sessions"))
		}
		return nil
	}

	// Print header
	fmt.Printf("%s\n\n", style.Bold.Render("Claude Code Sessions"))

	// Calculate column widths
	idWidth := 10
	roleWidth := 24
	timeWidth := 16
	topicWidth := 30

	// Header row
	fmt.Printf("%-*s  %-*s  %-*s  %-*s\n",
		idWidth, "ID",
		roleWidth, "ROLE",
		timeWidth, "STARTED",
		topicWidth, "TOPIC")
	fmt.Printf("%s\n", strings.Repeat("─", idWidth+roleWidth+timeWidth+topicWidth+6))

	for _, s := range sessions {
		id := s.ShortID()

		role := s.Role
		if role == "" {
			// Try to infer from path
			role = inferRoleFromPath(s.Path)
		}
		if len(role) > roleWidth {
			role = role[:roleWidth-1] + "…"
		}

		timeStr := s.FormatTime()

		topic := s.Topic
		if topic == "" && s.Summary != "" {
			// Use summary as fallback
			topic = s.Summary
		}
		if len(topic) > topicWidth {
			topic = topic[:topicWidth-1] + "…"
		}

		// Color based on Gas Town status
		if s.IsGasTown {
			fmt.Printf("%-*s  %-*s  %-*s  %-*s\n",
				idWidth, id,
				roleWidth, role,
				timeWidth, timeStr,
				topicWidth, topic)
		} else {
			fmt.Printf("%s  %s  %s  %s\n",
				style.Dim.Render(fmt.Sprintf("%-*s", idWidth, id)),
				style.Dim.Render(fmt.Sprintf("%-*s", roleWidth, role)),
				style.Dim.Render(fmt.Sprintf("%-*s", timeWidth, timeStr)),
				style.Dim.Render(fmt.Sprintf("%-*s", topicWidth, topic)))
		}
	}

	fmt.Printf("\n%s\n", style.Dim.Render("Resume a session: claude --resume <full-session-id>"))

	return nil
}

// inferRoleFromPath attempts to extract a role from the project path.
func inferRoleFromPath(path string) string {
	// Look for patterns like /crew/joe, /polecats/furiosa, /witness, etc.
	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		switch part {
		case "witness", "refinery", "deacon", "mayor":
			return part
		case "crew", "polecats":
			if i+1 < len(parts) {
				// Include the agent name
				return fmt.Sprintf("%s/%s", part, parts[i+1])
			}
			return part
		}
	}

	// Fall back to last path component
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown"
}
