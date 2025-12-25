// Package beads provides a wrapper for the bd (beads) CLI.
package beads

// BuiltinMolecule defines a built-in molecule template.
// Deprecated: Molecules are now defined as formula files in .beads/formulas/
// and cooked into proto beads via `bd cook`. This type remains for backward
// compatibility but is no longer used.
type BuiltinMolecule struct {
	ID          string // Well-known ID (e.g., "mol-engineer-in-box")
	Title       string
	Description string
}

// BuiltinMolecules returns all built-in molecule definitions.
// Deprecated: Molecules are now defined as formula files (.beads/formulas/*.formula.json)
// and cooked into proto beads via `bd cook`. This function returns an empty slice.
// Use `bd cook` to create proto beads from formulas instead.
func BuiltinMolecules() []BuiltinMolecule {
	return []BuiltinMolecule{}
}

// SeedBuiltinMolecules is deprecated and does nothing.
// Molecules are now created by cooking formula files with `bd cook`.
// This function remains for backward compatibility with existing installations.
func (b *Beads) SeedBuiltinMolecules() (int, error) {
	// No-op: formulas are cooked on-demand, not seeded at install time
	return 0, nil
}
