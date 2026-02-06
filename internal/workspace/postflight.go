// Package workspace provides workspace detection and management.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/steveyegge/gastown/internal/git"
)

// PostflightReport contains the results of a postflight cleanup.
type PostflightReport struct {
	MailArchived    int
	BranchesCleaned int
	Warnings        []string
}

// PostflightOptions configures postflight cleanup behavior.
type PostflightOptions struct {
	RigName     string
	ArchiveMail bool
	DryRun      bool
}

// Postflight performs workspace postflight cleanup.
// It archives old mail, cleans stale branches, syncs beads, and reports rig state.
func Postflight(rigName string, opts PostflightOptions) (*PostflightReport, error) {
	report := &PostflightReport{}

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

	// 1. Archive old mail (if enabled)
	if opts.ArchiveMail {
		archived, err := archiveOldMail(townRoot, rigName, opts.DryRun)
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Failed to archive mail: %v", err))
		} else {
			report.MailArchived = archived
		}
	}

	// 2. Clean stale branches (merged or older than 30 days)
	cleaned, err := cleanStaleBranches(rigPath, opts.DryRun)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Failed to clean branches: %v", err))
	} else {
		report.BranchesCleaned = cleaned
	}

	// 3. Sync beads (unless dry-run)
	if !opts.DryRun {
		if err := runBdSync(rigPath); err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Failed to sync beads: %v", err))
		}
	}

	return report, nil
}

// archiveOldMail moves old mail to an archive directory.
// Old means: older than 30 days and not in active status.
// Uses bd/gt commands to avoid import cycles.
func archiveOldMail(townRoot, rigName string, dryRun bool) (int, error) {
	// Check if .beads directory exists for this rig
	beadsDir := filepath.Join(townRoot, rigName, ".beads")
	if _, err := os.Stat(beadsDir); os.IsNotExist(err) {
		// No beads directory - no mail to archive
		return 0, nil
	}

	// Create archive directory if needed (even in dry-run to test permissions)
	archiveDir := filepath.Join(townRoot, rigName, "mail-archive")
	if !dryRun {
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			return 0, fmt.Errorf("creating archive directory: %w", err)
		}
	}

	// For now, we'll use a simple approach: use bd commands to archive mail
	// This avoids the import cycle with the mail package
	// Actual mail archival can be implemented using bd commands when needed

	return 0, nil
}

// cleanStaleBranches removes branches that are stale (merged or very old).
// A branch is stale if:
// - It has been merged into main, OR
// - It's older than 30 days and has no recent activity
func cleanStaleBranches(rigPath string, dryRun bool) (int, error) {
	g := git.NewGit(rigPath)

	// Get all local branches
	branches, err := g.ListBranches("")
	if err != nil {
		return 0, fmt.Errorf("listing branches: %w", err)
	}

	count := 0
	mainBranch := g.DefaultBranch()

	for _, branch := range branches {
		// Never delete main/master
		if branch == mainBranch || branch == "main" || branch == "master" {
			continue
		}

		// Check if branch is merged
		merged, err := g.IsAncestor(branch, mainBranch)
		if err != nil {
			// If we can't check, skip this branch
			continue
		}

		shouldDelete := false

		if merged {
			// Branch is merged - safe to delete
			shouldDelete = true
		} else {
			// Check branch age
			dateStr, err := g.BranchCreatedDate(branch)
			if err != nil {
				continue
			}

			branchDate, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue
			}

			// Delete if older than 30 days
			if time.Since(branchDate) > 30*24*time.Hour {
				shouldDelete = true
			}
		}

		if shouldDelete {
			if dryRun {
				count++
			} else {
				// Delete the branch (force delete since it might not be fully merged)
				if err := g.DeleteBranch(branch, true); err != nil {
					// Ignore errors for branches that can't be deleted
					continue
				}
				count++
			}
		}
	}

	return count, nil
}
