package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var formulasCmd = &cobra.Command{
	Use:     "formulas",
	Aliases: []string{"formula"},
	GroupID: GroupWork,
	Short:   "List available workflow formulas",
	Long: `List available workflow formulas (molecule templates).

Formulas are reusable workflow templates that can be instantiated via:
  gt sling mol-xxx target    # Pour formula and dispatch

This is a convenience alias for 'bd formula list'.

Examples:
  gt formulas                 # List all formulas
  gt formulas --json          # JSON output`,
	RunE: runFormulas,
}

var formulasJSON bool

func init() {
	formulasCmd.Flags().BoolVar(&formulasJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(formulasCmd)
}

func runFormulas(cmd *cobra.Command, args []string) error {
	bdArgs := []string{"formula", "list"}
	if formulasJSON {
		bdArgs = append(bdArgs, "--json")
	}

	bdCmd := exec.Command("bd", bdArgs...)
	bdCmd.Stdout = os.Stdout
	bdCmd.Stderr = os.Stderr
	return bdCmd.Run()
}
