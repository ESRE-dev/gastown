package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

// cycleSession is the --session flag for cycle next/prev commands.
// When run via tmux key binding (run-shell), the session context may not be
// correct, so we pass the session name explicitly via #{session_name} expansion.
var cycleSession string

func init() {
	rootCmd.AddCommand(cycleCmd)
	cycleCmd.AddCommand(cycleNextCmd)
	cycleCmd.AddCommand(cyclePrevCmd)

	cycleNextCmd.Flags().StringVar(&cycleSession, "session", "", "Override current session (used by tmux binding)")
	cyclePrevCmd.Flags().StringVar(&cycleSession, "session", "", "Override current session (used by tmux binding)")
}

var cycleCmd = &cobra.Command{
	Use:   "cycle",
	Short: "Cycle between sessions in the same group",
	Long: `Cycle between related tmux sessions based on the current session type.

Session groups:
- Town sessions: Mayor ↔ Deacon
- Crew sessions: All crew members in the same rig (e.g., gastown/crew/max ↔ gastown/crew/joe)

The appropriate cycling is detected automatically from the session name.`,
}

var cycleNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Switch to next session in group",
	Long: `Switch to the next session in the current group.

This command is typically invoked via the C-b n keybinding. It automatically
detects whether you're in a town-level session (Mayor/Deacon) or a crew session
and cycles within the appropriate group.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cycleToSession(1, cycleSession)
	},
}

var cyclePrevCmd = &cobra.Command{
	Use:   "prev",
	Short: "Switch to previous session in group",
	Long: `Switch to the previous session in the current group.

This command is typically invoked via the C-b p keybinding. It automatically
detects whether you're in a town-level session (Mayor/Deacon) or a crew session
and cycles within the appropriate group.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cycleToSession(-1, cycleSession)
	},
}

// cycleToSession dispatches to the appropriate cycling function based on session type.
// direction: 1 for next, -1 for previous
// sessionOverride: if non-empty, use this instead of detecting current session
func cycleToSession(direction int, sessionOverride string) error {
	session := sessionOverride
	if session == "" {
		var err error
		session, err = getCurrentTmuxSession()
		if err != nil {
			return nil // Not in tmux, nothing to do
		}
	}

	// Check if it's a town-level session
	for _, townSession := range townLevelSessions {
		if session == townSession {
			return cycleTownSession(direction, session)
		}
	}

	// Check if it's a crew session (format: gt-<rig>-crew-<name>)
	if strings.HasPrefix(session, "gt-") && strings.Contains(session, "-crew-") {
		return cycleCrewSession(direction, session)
	}

	// Unknown session type (polecat, witness, refinery) - do nothing
	return nil
}
