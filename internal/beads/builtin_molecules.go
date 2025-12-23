// Package beads provides a wrapper for the bd (beads) CLI.
package beads

// BuiltinMolecule defines a built-in molecule template.
type BuiltinMolecule struct {
	ID          string // Well-known ID (e.g., "mol-engineer-in-box")
	Title       string
	Description string
}

// BuiltinMolecules returns all built-in molecule definitions.
// Molecules are defined in separate files by category:
//   - molecules_work.go: EngineerInBox, QuickFix, Research, PolecatWork, ReadyWork
//   - molecules_patrol.go: DeaconPatrol, WitnessPatrol, RefineryPatrol
//   - molecules_session.go: CrewSession, PolecatSession, Bootstrap, VersionBump, InstallGoBinary
func BuiltinMolecules() []BuiltinMolecule {
	return []BuiltinMolecule{
		// Work molecules
		EngineerInBoxMolecule(),
		QuickFixMolecule(),
		ResearchMolecule(),
		PolecatWorkMolecule(),
		ReadyWorkMolecule(),

		// Patrol molecules
		DeaconPatrolMolecule(),
		RefineryPatrolMolecule(),
		WitnessPatrolMolecule(),

		// Session and utility molecules
		CrewSessionMolecule(),
		PolecatSessionMolecule(),
		BootstrapGasTownMolecule(),
		VersionBumpMolecule(),
		InstallGoBinaryMolecule(),
	}
}

// SeedBuiltinMolecules creates all built-in molecules in the beads database.
// It skips molecules that already exist (by title match).
// Returns the number of molecules created.
func (b *Beads) SeedBuiltinMolecules() (int, error) {
	molecules := BuiltinMolecules()
	created := 0

	// Get existing molecules to avoid duplicates
	existing, err := b.List(ListOptions{Type: "molecule", Priority: -1})
	if err != nil {
		return 0, err
	}

	// Build map of existing molecule titles
	existingTitles := make(map[string]bool)
	for _, issue := range existing {
		existingTitles[issue.Title] = true
	}

	// Create each molecule if it doesn't exist
	for _, mol := range molecules {
		if existingTitles[mol.Title] {
			continue // Already exists
		}

		_, err := b.Create(CreateOptions{
			Title:       mol.Title,
			Type:        "molecule",
			Priority:    2, // Medium priority
			Description: mol.Description,
		})
		if err != nil {
			return created, err
		}
		created++
	}

	return created, nil
}
