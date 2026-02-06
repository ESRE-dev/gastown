// Package plan provides markdown-to-epic conversion for GasTown.
package plan

// PlanDocument represents a parsed markdown plan document.
type PlanDocument struct {
	Title    string         // Document title from H1
	Sections []PlanSection  // Top-level sections (headers)
}

// PlanSection represents a section of the plan (typically from headers).
type PlanSection struct {
	Title    string      // Section title (e.g., "Phase 1: Setup")
	Level    int         // Header level (1-6)
	Items    []PlanItem  // Direct children of this section
	Children []PlanSection // Nested subsections
}

// PlanItem represents a task item in the plan.
type PlanItem struct {
	Text        string      // Item text content
	Checked     bool        // For checkbox items (- [ ] or - [x])
	IsCheckbox  bool        // Whether this is a checkbox item
	IsNumbered  bool        // Whether this is from a numbered list
	Number      int         // Sequential number if from numbered list
	Children    []PlanItem  // Nested items (indented)
	Level       int         // Indentation level
}

// EpicPlan represents the structured epic to be created in beads.
type EpicPlan struct {
	Title       string         // Epic title
	Description string         // Epic description
	Children    []EpicChild    // Child issues to create
	Priority    int            // Epic priority (0-4)
}

// EpicChild represents a child issue to be created under the epic.
type EpicChild struct {
	Title       string       // Issue title
	Description string       // Issue description
	Type        string       // Issue type: "task", "bug", "feature"
	Priority    int          // Issue priority (0-4)
	Children    []EpicChild  // Nested children
	DependsOn   []int        // Indices of siblings this depends on
	Sequential  bool         // Whether children should be sequential
}
