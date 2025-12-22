package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

var doneCmd = &cobra.Command{
	Use:   "done",
	Short: "Signal work ready for merge queue",
	Long: `Signal that your work is complete and ready for the merge queue.

This is a convenience command for polecats that:
1. Submits the current branch to the merge queue
2. Auto-detects issue ID from branch name

Equivalent to: gt mq submit

Examples:
  gt done                  # Submit current branch
  gt done --issue gt-abc   # Explicit issue ID`,
	RunE: runDone,
}

var (
	doneIssue    string
	donePriority int
)

func init() {
	doneCmd.Flags().StringVar(&doneIssue, "issue", "", "Source issue ID (default: parse from branch name)")
	doneCmd.Flags().IntVarP(&donePriority, "priority", "p", -1, "Override priority (0-4, default: inherit from issue)")

	rootCmd.AddCommand(doneCmd)
}

func runDone(cmd *cobra.Command, args []string) error {
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

	// Initialize git for the current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}
	g := git.NewGit(cwd)

	// Get current branch
	branch, err := g.CurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}

	if branch == "main" || branch == "master" {
		return fmt.Errorf("cannot submit main/master branch to merge queue")
	}

	// Parse branch info
	info := parseBranchName(branch)

	// Override with explicit flags
	issueID := doneIssue
	if issueID == "" {
		issueID = info.Issue
	}
	worker := info.Worker

	if issueID == "" {
		return fmt.Errorf("cannot determine source issue from branch '%s'; use --issue to specify", branch)
	}

	// Initialize beads
	bd := beads.New(cwd)

	// Determine target branch (auto-detect integration branch if applicable)
	target := "main"
	autoTarget, err := detectIntegrationBranch(bd, g, issueID)
	if err == nil && autoTarget != "" {
		target = autoTarget
	}

	// Get source issue for priority inheritance
	var priority int
	if donePriority >= 0 {
		priority = donePriority
	} else {
		// Try to inherit from source issue
		sourceIssue, err := bd.Show(issueID)
		if err != nil {
			priority = 2 // Default
		} else {
			priority = sourceIssue.Priority
		}
	}

	// Build title
	title := fmt.Sprintf("Merge: %s", issueID)

	// CRITICAL: Push branch to origin BEFORE creating MR
	// Without this, the worktree can be deleted and the branch lost forever
	fmt.Printf("Pushing branch to origin...\n")
	if err := g.Push("origin", branch, false); err != nil {
		return fmt.Errorf("pushing branch to origin: %w", err)
	}
	fmt.Printf("%s Branch pushed to origin/%s\n", style.Bold.Render("✓"), branch)

	// Build description with MR fields
	mrFields := &beads.MRFields{
		Branch:      branch,
		Target:      target,
		SourceIssue: issueID,
		Worker:      worker,
		Rig:         rigName,
	}
	description := beads.FormatMRFields(mrFields)

	// Create the merge-request issue
	createOpts := beads.CreateOptions{
		Title:       title,
		Type:        "merge-request",
		Priority:    priority,
		Description: description,
	}

	issue, err := bd.Create(createOpts)
	if err != nil {
		return fmt.Errorf("creating merge request: %w", err)
	}

	// Success output
	fmt.Printf("%s Work submitted to merge queue\n", style.Bold.Render("✓"))
	fmt.Printf("  MR ID: %s\n", style.Bold.Render(issue.ID))
	fmt.Printf("  Source: %s\n", branch)
	fmt.Printf("  Target: %s\n", target)
	fmt.Printf("  Issue: %s\n", issueID)
	if worker != "" {
		fmt.Printf("  Worker: %s\n", worker)
	}
	fmt.Printf("  Priority: P%d\n", priority)
	fmt.Println()
	fmt.Printf("%s\n", style.Dim.Render("The Refinery will process your merge request."))

	return nil
}
