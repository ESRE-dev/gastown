package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/mrqueue"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

var (
	mqMigrateDryRun bool
)

var mqMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate stale merge-request beads to the mrqueue",
	Long: `Migrate existing merge-request beads to the mrqueue.

This command finds merge-request beads that were created before the mrqueue
integration and adds them to the refinery's work queue (.beads/mq/).

Use this to recover stale MRs that the refinery wasn't processing because
they only existed as beads.

Examples:
  gt mq migrate              # Migrate all stale MRs
  gt mq migrate --dry-run    # Preview what would be migrated`,
	RunE: runMqMigrate,
}

func init() {
	mqMigrateCmd.Flags().BoolVar(&mqMigrateDryRun, "dry-run", false, "Preview only, don't actually migrate")
	mqCmd.AddCommand(mqMigrateCmd)
}

func runMqMigrate(cmd *cobra.Command, args []string) error {
	// Find workspace
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Find current rig
	rigName, _, err := findCurrentRig(townRoot)
	if err != nil {
		return err
	}

	// Initialize beads
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}
	bd := beads.New(cwd)

	// Initialize mrqueue
	mq, err := mrqueue.NewFromWorkdir(cwd)
	if err != nil {
		return fmt.Errorf("accessing merge queue: %w", err)
	}

	// Get existing mrqueue entries to avoid duplicates
	existingMRs, err := mq.List()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("listing existing queue: %w", err)
	}
	existingIDs := make(map[string]bool)
	for _, mr := range existingMRs {
		existingIDs[mr.ID] = true
	}

	// List all open merge-request beads
	// Note: beads.List() with default ListOptions filters for P0 only (Priority default is 0)
	// So we use Priority: -1 to indicate no filter
	allIssues, err := bd.List(beads.ListOptions{Priority: -1})
	if err != nil {
		return fmt.Errorf("listing all beads: %w", err)
	}

	// Filter for open merge-requests
	var issues []*beads.Issue
	for _, issue := range allIssues {
		if issue.Type == "merge-request" && issue.Status == "open" {
			issues = append(issues, issue)
		}
	}

	if len(issues) == 0 {
		fmt.Println("No stale merge-request beads found.")
		return nil
	}

	// Filter to only those not already in mrqueue
	var toMigrate []*beads.Issue
	for _, issue := range issues {
		if !existingIDs[issue.ID] {
			toMigrate = append(toMigrate, issue)
		}
	}

	if len(toMigrate) == 0 {
		fmt.Println("All merge-request beads already in mrqueue.")
		return nil
	}

	fmt.Printf("Found %d stale merge-request bead(s) to migrate:\n\n", len(toMigrate))

	migrated := 0
	for _, issue := range toMigrate {
		// Parse MR fields from bead description
		mrFields := beads.ParseMRFields(issue)
		if mrFields == nil {
			fmt.Printf("  %s %s - skipping (no MR fields in description)\n",
				style.Dim.Render("⚠"), issue.ID)
			continue
		}

		// Extract worker from description
		worker := mrFields.Worker
		if worker == "" {
			// Try to extract from branch name
			if strings.HasPrefix(mrFields.Branch, "polecat/") {
				parts := strings.SplitN(mrFields.Branch, "/", 3)
				if len(parts) >= 2 {
					worker = parts[1]
				}
			}
		}

		if mqMigrateDryRun {
			fmt.Printf("  %s %s - %s (branch: %s, target: %s)\n",
				style.Bold.Render("→"), issue.ID, issue.Title,
				mrFields.Branch, mrFields.Target)
		} else {
			// Create mrqueue entry
			mqEntry := &mrqueue.MR{
				ID:          issue.ID,
				Branch:      mrFields.Branch,
				Target:      mrFields.Target,
				SourceIssue: mrFields.SourceIssue,
				Worker:      worker,
				Rig:         rigName,
				Title:       issue.Title,
				Priority:    issue.Priority,
			}

			if err := mq.Submit(mqEntry); err != nil {
				fmt.Printf("  %s %s - failed: %v\n",
					style.Dim.Render("✗"), issue.ID, err)
				continue
			}

			fmt.Printf("  %s %s - %s\n",
				style.Bold.Render("✓"), issue.ID, issue.Title)
			migrated++
		}
	}

	fmt.Println()
	if mqMigrateDryRun {
		fmt.Printf("Dry run: would migrate %d merge-request(s)\n", len(toMigrate))
		fmt.Println("Run without --dry-run to perform migration.")
	} else {
		fmt.Printf("%s Migrated %d merge-request(s) to mrqueue\n",
			style.Bold.Render("✓"), migrated)
		fmt.Println("The refinery will process them on its next poll cycle.")
	}

	return nil
}
