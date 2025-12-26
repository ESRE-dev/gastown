package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/style"
)

func runMoleculeList(cmd *cobra.Command, args []string) error {
	workDir, err := findLocalBeadsDir()
	if err != nil {
		return fmt.Errorf("not in a beads workspace: %w", err)
	}

	// Collect molecules from requested sources
	type moleculeEntry struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Source      string `json:"source"`
		StepCount   int    `json:"step_count,omitempty"`
		Status      string `json:"status,omitempty"`
		Description string `json:"description,omitempty"`
	}

	var entries []moleculeEntry

	// Load from catalog (unless --db only)
	if !moleculeDBOnly {
		catalog, err := loadMoleculeCatalog(workDir)
		if err != nil {
			return fmt.Errorf("loading catalog: %w", err)
		}

		for _, mol := range catalog.List() {
			steps, _ := beads.ParseMoleculeSteps(mol.Description)
			entries = append(entries, moleculeEntry{
				ID:          mol.ID,
				Title:       mol.Title,
				Source:      mol.Source,
				StepCount:   len(steps),
				Description: mol.Description,
			})
		}
	}

	// Load from database (unless --catalog only)
	if !moleculeCatalogOnly {
		b := beads.New(workDir)
		issues, err := b.List(beads.ListOptions{
			Type:     "molecule",
			Status:   "all",
			Priority: -1,
		})
		if err != nil {
			return fmt.Errorf("listing molecules: %w", err)
		}

		// Track catalog IDs to avoid duplicates
		catalogIDs := make(map[string]bool)
		for _, e := range entries {
			catalogIDs[e.ID] = true
		}

		for _, mol := range issues {
			// Skip if already in catalog (catalog takes precedence)
			if catalogIDs[mol.ID] {
				continue
			}

			steps, _ := beads.ParseMoleculeSteps(mol.Description)
			entries = append(entries, moleculeEntry{
				ID:          mol.ID,
				Title:       mol.Title,
				Source:      "database",
				StepCount:   len(steps),
				Status:      mol.Status,
				Description: mol.Description,
			})
		}
	}

	if moleculeJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	// Human-readable output
	fmt.Printf("%s Molecules (%d)\n\n", style.Bold.Render("🧬"), len(entries))

	if len(entries) == 0 {
		fmt.Printf("  %s\n", style.Dim.Render("(no molecules defined)"))
		return nil
	}

	// Create styled table
	table := style.NewTable(
		style.Column{Name: "ID", Width: 20},
		style.Column{Name: "TITLE", Width: 35},
		style.Column{Name: "STEPS", Width: 5, Align: style.AlignRight},
		style.Column{Name: "SOURCE", Width: 10},
	)

	for _, mol := range entries {
		// Format steps count
		stepStr := ""
		if mol.StepCount > 0 {
			stepStr = fmt.Sprintf("%d", mol.StepCount)
		}

		// Format title with status
		title := mol.Title
		if mol.Status == "closed" {
			title = style.Dim.Render(mol.Title + " [closed]")
		}

		// Format source
		source := style.Dim.Render(mol.Source)

		table.AddRow(mol.ID, title, stepStr, source)
	}

	fmt.Print(table.Render())

	return nil
}

func runMoleculeShow(cmd *cobra.Command, args []string) error {
	molID := args[0]

	workDir, err := findLocalBeadsDir()
	if err != nil {
		return fmt.Errorf("not in a beads workspace: %w", err)
	}

	// Try catalog first
	catalog, err := loadMoleculeCatalog(workDir)
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	var mol *beads.Issue
	var source string

	if catalogMol := catalog.Get(molID); catalogMol != nil {
		mol = catalogMol.ToIssue()
		source = catalogMol.Source
	} else {
		// Fall back to database
		b := beads.New(workDir)
		mol, err = b.Show(molID)
		if err != nil {
			return fmt.Errorf("getting molecule: %w", err)
		}
		source = "database"
	}

	if mol.Type != "molecule" {
		return fmt.Errorf("%s is not a molecule (type: %s)", molID, mol.Type)
	}

	// Parse steps
	steps, parseErr := beads.ParseMoleculeSteps(mol.Description)
	_ = source // silence unused warning; used in output formatting below

	// For JSON, include parsed steps
	if moleculeJSON {
		type moleculeOutput struct {
			*beads.Issue
			Source     string               `json:"source"`
			Steps      []beads.MoleculeStep `json:"steps,omitempty"`
			ParseError string               `json:"parse_error,omitempty"`
		}
		out := moleculeOutput{Issue: mol, Source: source, Steps: steps}
		if parseErr != nil {
			out.ParseError = parseErr.Error()
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	// Human-readable output
	fmt.Printf("\n%s: %s %s\n", style.Bold.Render(mol.ID), mol.Title, style.Dim.Render(fmt.Sprintf("[%s]", source)))
	fmt.Printf("Type: %s\n", mol.Type)

	if parseErr != nil {
		fmt.Printf("\n%s Parse error: %s\n", style.Bold.Render("⚠"), parseErr)
	}

	// Show steps
	fmt.Printf("\nSteps (%d):\n", len(steps))
	if len(steps) == 0 {
		fmt.Printf("  %s\n", style.Dim.Render("(no steps defined)"))
	} else {
		// Find which steps are ready (no dependencies)
		for _, step := range steps {
			needsStr := ""
			if len(step.Needs) == 0 {
				needsStr = style.Dim.Render("(ready first)")
			} else {
				needsStr = fmt.Sprintf("Needs: %s", strings.Join(step.Needs, ", "))
			}

			tierStr := ""
			if step.Tier != "" {
				tierStr = fmt.Sprintf(" [%s]", step.Tier)
			}

			fmt.Printf("  %-12s → %s%s\n", step.Ref, needsStr, tierStr)
		}
	}

	// Count instances (need beads client for this)
	b := beads.New(workDir)
	instances, _ := findMoleculeInstances(b, molID)
	fmt.Printf("\nInstances: %d\n", len(instances))

	return nil
}

func runMoleculeParse(cmd *cobra.Command, args []string) error {
	molID := args[0]

	workDir, err := findLocalBeadsDir()
	if err != nil {
		return fmt.Errorf("not in a beads workspace: %w", err)
	}

	b := beads.New(workDir)
	mol, err := b.Show(molID)
	if err != nil {
		return fmt.Errorf("getting molecule: %w", err)
	}

	// Validate the molecule
	validationErr := beads.ValidateMolecule(mol)

	// Parse steps regardless of validation
	steps, parseErr := beads.ParseMoleculeSteps(mol.Description)

	if moleculeJSON {
		type parseOutput struct {
			Valid           bool                 `json:"valid"`
			ValidationError string               `json:"validation_error,omitempty"`
			ParseError      string               `json:"parse_error,omitempty"`
			Steps           []beads.MoleculeStep `json:"steps"`
		}
		out := parseOutput{
			Valid: validationErr == nil,
			Steps: steps,
		}
		if validationErr != nil {
			out.ValidationError = validationErr.Error()
		}
		if parseErr != nil {
			out.ParseError = parseErr.Error()
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	// Human-readable output
	fmt.Printf("\n%s: %s\n\n", style.Bold.Render(mol.ID), mol.Title)

	if validationErr != nil {
		fmt.Printf("%s Validation failed: %s\n\n", style.Bold.Render("✗"), validationErr)
	} else {
		fmt.Printf("%s Valid molecule\n\n", style.Bold.Render("✓"))
	}

	if parseErr != nil {
		fmt.Printf("Parse error: %s\n\n", parseErr)
	}

	fmt.Printf("Parsed Steps (%d):\n", len(steps))
	for i, step := range steps {
		fmt.Printf("\n  [%d] %s\n", i+1, style.Bold.Render(step.Ref))
		if step.Title != step.Ref {
			fmt.Printf("      Title: %s\n", step.Title)
		}
		if len(step.Needs) > 0 {
			fmt.Printf("      Needs: %s\n", strings.Join(step.Needs, ", "))
		}
		if step.Tier != "" {
			fmt.Printf("      Tier: %s\n", step.Tier)
		}
		if step.Instructions != "" {
			// Show first line of instructions
			firstLine := strings.SplitN(step.Instructions, "\n", 2)[0]
			if len(firstLine) > 60 {
				firstLine = firstLine[:57] + "..."
			}
			fmt.Printf("      Instructions: %s\n", style.Dim.Render(firstLine))
		}
	}

	return nil
}

func runMoleculeInstances(cmd *cobra.Command, args []string) error {
	molID := args[0]

	workDir, err := findLocalBeadsDir()
	if err != nil {
		return fmt.Errorf("not in a beads workspace: %w", err)
	}

	b := beads.New(workDir)

	// Verify the molecule exists
	mol, err := b.Show(molID)
	if err != nil {
		return fmt.Errorf("getting molecule: %w", err)
	}

	if mol.Type != "molecule" {
		return fmt.Errorf("%s is not a molecule (type: %s)", molID, mol.Type)
	}

	// Find all instances
	instances, err := findMoleculeInstances(b, molID)
	if err != nil {
		return fmt.Errorf("finding instances: %w", err)
	}

	if moleculeJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(instances)
	}

	// Human-readable output
	fmt.Printf("\n%s Instances of %s (%d)\n\n",
		style.Bold.Render("📋"), molID, len(instances))

	if len(instances) == 0 {
		fmt.Printf("  %s\n", style.Dim.Render("(no instantiations found)"))
		return nil
	}

	fmt.Printf("%-16s %-12s %s\n",
		style.Bold.Render("Parent"),
		style.Bold.Render("Status"),
		style.Bold.Render("Created"))
	fmt.Println(strings.Repeat("-", 50))

	for _, inst := range instances {
		// Calculate progress from children
		progress := ""
		if len(inst.Children) > 0 {
			closed := 0
			for _, childID := range inst.Children {
				child, err := b.Show(childID)
				if err == nil && child.Status == "closed" {
					closed++
				}
			}
			progress = fmt.Sprintf(" (%d/%d complete)", closed, len(inst.Children))
		}

		statusStr := inst.Status
		if inst.Status == "closed" {
			statusStr = style.Dim.Render("done")
		} else if inst.Status == "in_progress" {
			statusStr = "active"
		}

		created := ""
		if inst.CreatedAt != "" {
			// Parse and format date
			created = inst.CreatedAt[:10] // Just the date portion
		}

		fmt.Printf("%-16s %-12s %s%s\n", inst.ID, statusStr, created, progress)
	}

	return nil
}

// findMoleculeInstances finds all parent issues that have steps instantiated from the given molecule.
func findMoleculeInstances(b *beads.Beads, molID string) ([]*beads.Issue, error) {
	// Get all issues and look for ones with children that have instantiated_from metadata
	// This is a brute-force approach - could be optimized with better queries

	// Strategy: search for issues whose descriptions contain "instantiated_from: <molID>"
	allIssues, err := b.List(beads.ListOptions{Status: "all", Priority: -1})
	if err != nil {
		return nil, err
	}

	// Find issues that reference this molecule
	parentIDs := make(map[string]bool)
	for _, issue := range allIssues {
		if strings.Contains(issue.Description, fmt.Sprintf("instantiated_from: %s", molID)) {
			// This is a step - find its parent
			if issue.Parent != "" {
				parentIDs[issue.Parent] = true
			}
		}
	}

	// Fetch the parent issues
	var parents []*beads.Issue
	for parentID := range parentIDs {
		parent, err := b.Show(parentID)
		if err == nil {
			parents = append(parents, parent)
		}
	}

	return parents, nil
}
