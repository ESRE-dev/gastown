package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/steveyegge/gastown/internal/workspace"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Gas Town",
	Long: `Start Gas Town by launching the Deacon and Mayor.

The Deacon is the health-check orchestrator that monitors Mayor and Witnesses.
The Mayor is the global coordinator that dispatches work.

Other agents (Witnesses, Refineries, Polecats) are started lazily as needed.

To stop Gas Town, use 'gt deacon stop' and 'gt mayor stop'.`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	// Verify we're in a Gas Town workspace
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	t := tmux.NewTmux()

	fmt.Printf("Starting Gas Town from %s\n\n", style.Dim.Render(townRoot))

	// Start Mayor first (so Deacon sees it as up)
	mayorRunning, _ := t.HasSession(MayorSessionName)
	if mayorRunning {
		fmt.Printf("  %s Mayor already running\n", style.Dim.Render("○"))
	} else {
		fmt.Printf("  %s Starting Mayor...\n", style.Bold.Render("→"))
		if err := startMayorSession(t); err != nil {
			return fmt.Errorf("starting Mayor: %w", err)
		}
		fmt.Printf("  %s Mayor started\n", style.Bold.Render("✓"))
	}

	// Start Deacon (health monitor)
	deaconRunning, _ := t.HasSession(DeaconSessionName)
	if deaconRunning {
		fmt.Printf("  %s Deacon already running\n", style.Dim.Render("○"))
	} else {
		fmt.Printf("  %s Starting Deacon...\n", style.Bold.Render("→"))
		if err := startDeaconSession(t); err != nil {
			return fmt.Errorf("starting Deacon: %w", err)
		}
		fmt.Printf("  %s Deacon started\n", style.Bold.Render("✓"))
	}

	fmt.Println()
	fmt.Printf("%s Gas Town is running\n", style.Bold.Render("✓"))
	fmt.Println()
	fmt.Printf("  Attach to Mayor:  %s\n", style.Dim.Render("gt mayor attach"))
	fmt.Printf("  Attach to Deacon: %s\n", style.Dim.Render("gt deacon attach"))
	fmt.Printf("  Check status:     %s\n", style.Dim.Render("gt status"))

	return nil
}
