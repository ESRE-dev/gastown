package doctor

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/steveyegge/gastown/internal/beads"
)

// DefaultStaleThreshold is the default time after which an attachment is considered stale.
// Attachments with no molecule activity in this duration may indicate stuck work.
const DefaultStaleThreshold = 1 * time.Hour

// StaleAttachmentsCheck detects attached molecules that haven't been updated in too long.
// This may indicate stuck work - a polecat that crashed or got stuck during processing.
type StaleAttachmentsCheck struct {
	BaseCheck
	Threshold time.Duration // Configurable staleness threshold
}

// NewStaleAttachmentsCheck creates a new stale attachments check with the default threshold.
func NewStaleAttachmentsCheck() *StaleAttachmentsCheck {
	return NewStaleAttachmentsCheckWithThreshold(DefaultStaleThreshold)
}

// NewStaleAttachmentsCheckWithThreshold creates a new stale attachments check with a custom threshold.
func NewStaleAttachmentsCheckWithThreshold(threshold time.Duration) *StaleAttachmentsCheck {
	return &StaleAttachmentsCheck{
		BaseCheck: BaseCheck{
			CheckName:        "stale-attachments",
			CheckDescription: "Check for attached molecules that haven't been updated in too long",
		},
		Threshold: threshold,
	}
}

// StaleAttachment represents a single stale attachment finding.
type StaleAttachment struct {
	Rig           string
	PinnedBeadID  string
	PinnedTitle   string
	Assignee      string
	MoleculeID    string
	MoleculeTitle string
	LastUpdated   time.Time
	StaleDuration time.Duration
}

// Run checks for stale attachments across all rigs.
func (c *StaleAttachmentsCheck) Run(ctx *CheckContext) *CheckResult {
	// If a specific rig is specified, only check that one
	var rigsToCheck []string
	if ctx.RigName != "" {
		rigsToCheck = []string{ctx.RigName}
	} else {
		// Discover all rigs
		rigs, err := discoverRigs(ctx.TownRoot)
		if err != nil {
			return &CheckResult{
				Name:    c.Name(),
				Status:  StatusError,
				Message: "Failed to discover rigs",
				Details: []string{err.Error()},
			}
		}
		rigsToCheck = rigs
	}

	if len(rigsToCheck) == 0 {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: "No rigs configured",
		}
	}

	// Find stale attachments across all rigs
	var staleAttachments []StaleAttachment
	var checkedCount int
	cutoff := time.Now().Add(-c.Threshold)

	for _, rigName := range rigsToCheck {
		stale, checked, err := c.checkRig(ctx.TownRoot, rigName, cutoff)
		if err != nil {
			// Log but continue with other rigs
			continue
		}
		staleAttachments = append(staleAttachments, stale...)
		checkedCount += checked
	}

	// Also check town-level beads for pinned attachments
	townStale, townChecked, err := c.checkBeadsDir(ctx.TownRoot, filepath.Join(ctx.TownRoot, ".beads"), cutoff)
	if err == nil {
		staleAttachments = append(staleAttachments, townStale...)
		checkedCount += townChecked
	}

	if len(staleAttachments) > 0 {
		details := make([]string, 0, len(staleAttachments))
		for _, sa := range staleAttachments {
			location := sa.Rig
			if location == "" {
				location = "town"
			}
			assigneeInfo := ""
			if sa.Assignee != "" {
				assigneeInfo = fmt.Sprintf(" (assignee: %s)", sa.Assignee)
			}
			details = append(details, fmt.Sprintf("%s: %s → %s%s (stale for %s)",
				location, sa.PinnedTitle, sa.MoleculeTitle, assigneeInfo, formatDuration(sa.StaleDuration)))
		}

		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusWarning,
			Message: fmt.Sprintf("%d stale attachment(s) found (no activity for >%s)", len(staleAttachments), formatDuration(c.Threshold)),
			Details: details,
			FixHint: "Check if polecats are stuck or crashed. Use 'gt witness nudge <polecat>' or 'gt polecat kill <name>' if needed",
		}
	}

	if checkedCount == 0 {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: "No attachments to check",
		}
	}

	return &CheckResult{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: fmt.Sprintf("Checked %d attachment(s), none stale", checkedCount),
	}
}

// checkRig checks a single rig for stale attachments.
func (c *StaleAttachmentsCheck) checkRig(townRoot, rigName string, cutoff time.Time) ([]StaleAttachment, int, error) {
	// Check rig-level beads and polecats
	rigPath := filepath.Join(townRoot, rigName)

	// Each polecat has its own beads directory
	polecatsDir := filepath.Join(rigPath, "polecats")
	polecatDirs, err := filepath.Glob(filepath.Join(polecatsDir, "*", ".beads"))
	if err != nil {
		return nil, 0, err
	}

	var allStale []StaleAttachment
	var totalChecked int

	for _, beadsPath := range polecatDirs {
		// Extract polecat name from path
		polecatPath := filepath.Dir(beadsPath)
		polecatName := filepath.Base(polecatPath)

		stale, checked, err := c.checkBeadsDirWithContext(rigPath, beadsPath, cutoff, rigName, polecatName)
		if err != nil {
			continue
		}
		allStale = append(allStale, stale...)
		totalChecked += checked
	}

	// Also check rig-level beads (crew workers, etc.)
	crewDir := filepath.Join(rigPath, "crew")
	crewDirs, err := filepath.Glob(filepath.Join(crewDir, "*", ".beads"))
	if err == nil {
		for _, beadsPath := range crewDirs {
			workerPath := filepath.Dir(beadsPath)
			workerName := filepath.Base(workerPath)

			stale, checked, err := c.checkBeadsDirWithContext(rigPath, beadsPath, cutoff, rigName, "crew/"+workerName)
			if err != nil {
				continue
			}
			allStale = append(allStale, stale...)
			totalChecked += checked
		}
	}

	return allStale, totalChecked, nil
}

// checkBeadsDir checks a beads directory for stale attachments.
func (c *StaleAttachmentsCheck) checkBeadsDir(townRoot, beadsDir string, cutoff time.Time) ([]StaleAttachment, int, error) {
	return c.checkBeadsDirWithContext(townRoot, beadsDir, cutoff, "", "")
}

// checkBeadsDirWithContext checks a beads directory for stale attachments with rig context.
func (c *StaleAttachmentsCheck) checkBeadsDirWithContext(workDir, beadsDir string, cutoff time.Time, rigName, workerName string) ([]StaleAttachment, int, error) {
	// Create beads client for the directory containing .beads
	parentDir := filepath.Dir(beadsDir)
	bd := beads.New(parentDir)

	// List all pinned beads (attachments are stored on pinned beads)
	pinnedIssues, err := bd.List(beads.ListOptions{
		Status:   beads.StatusPinned,
		Priority: -1, // No filter
	})
	if err != nil {
		return nil, 0, err
	}

	var staleAttachments []StaleAttachment
	var checked int

	for _, pinned := range pinnedIssues {
		// Parse attachment fields
		attachment := beads.ParseAttachmentFields(pinned)
		if attachment == nil || attachment.AttachedMolecule == "" {
			continue // No attachment
		}

		checked++

		// Fetch the attached molecule to check its updated_at timestamp
		mol, err := bd.Show(attachment.AttachedMolecule)
		if err != nil {
			// Molecule might have been deleted or is inaccessible
			// This itself could be a problem worth reporting
			staleAttachments = append(staleAttachments, StaleAttachment{
				Rig:           rigName,
				PinnedBeadID:  pinned.ID,
				PinnedTitle:   pinned.Title,
				Assignee:      pinned.Assignee,
				MoleculeID:    attachment.AttachedMolecule,
				MoleculeTitle: "(molecule not found)",
				LastUpdated:   time.Time{},
				StaleDuration: time.Since(cutoff) + c.Threshold, // Report as stale
			})
			continue
		}

		// Parse the molecule's updated_at timestamp
		updatedAt, err := parseTimestamp(mol.UpdatedAt)
		if err != nil {
			continue // Skip if we can't parse the timestamp
		}

		// Check if the molecule is stale (hasn't been updated since cutoff)
		// Only check molecules that are still in progress
		if mol.Status == "in_progress" && updatedAt.Before(cutoff) {
			staleAttachments = append(staleAttachments, StaleAttachment{
				Rig:           rigName,
				PinnedBeadID:  pinned.ID,
				PinnedTitle:   pinned.Title,
				Assignee:      pinned.Assignee,
				MoleculeID:    mol.ID,
				MoleculeTitle: mol.Title,
				LastUpdated:   updatedAt,
				StaleDuration: time.Since(updatedAt),
			})
		}
	}

	return staleAttachments, checked, nil
}

// parseTimestamp parses an ISO 8601 timestamp string.
func parseTimestamp(ts string) (time.Time, error) {
	// Try RFC3339 first (most common)
	t, err := time.Parse(time.RFC3339, ts)
	if err == nil {
		return t, nil
	}

	// Try without timezone
	t, err = time.Parse("2006-01-02T15:04:05", ts)
	if err == nil {
		return t, nil
	}

	// Try date only
	t, err = time.Parse("2006-01-02", ts)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", ts)
}
