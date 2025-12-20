// Package beads provides a wrapper for the bd (beads) CLI.
package beads

// BuiltinMolecule defines a built-in molecule template.
type BuiltinMolecule struct {
	ID          string // Well-known ID (e.g., "mol-engineer-in-box")
	Title       string
	Description string
}

// BuiltinMolecules returns all built-in molecule definitions.
func BuiltinMolecules() []BuiltinMolecule {
	return []BuiltinMolecule{
		EngineerInBoxMolecule(),
		QuickFixMolecule(),
		ResearchMolecule(),
		InstallGoBinaryMolecule(),
		BootstrapGasTownMolecule(),
	}
}

// EngineerInBoxMolecule returns the engineer-in-box molecule definition.
// This is a full workflow from design to merge.
func EngineerInBoxMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-engineer-in-box",
		Title: "Engineer in a Box",
		Description: `Full workflow from design to merge.

## Step: design
Think carefully about architecture. Consider:
- Existing patterns in the codebase
- Trade-offs between approaches
- Testability and maintainability

Write a brief design summary before proceeding.

## Step: implement
Write the code. Follow codebase conventions.
Needs: design

## Step: review
Self-review the changes. Look for:
- Bugs and edge cases
- Style issues
- Missing error handling
Needs: implement

## Step: test
Write and run tests. Cover happy path and edge cases.
Fix any failures before proceeding.
Needs: implement

## Step: submit
Submit for merge via refinery.
Needs: review, test`,
	}
}

// QuickFixMolecule returns the quick-fix molecule definition.
// This is a fast path for small changes.
func QuickFixMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-quick-fix",
		Title: "Quick Fix",
		Description: `Fast path for small changes.

## Step: implement
Make the fix. Keep it focused.

## Step: test
Run relevant tests. Fix any regressions.
Needs: implement

## Step: submit
Submit for merge.
Needs: test`,
	}
}

// ResearchMolecule returns the research molecule definition.
// This is an investigation workflow.
func ResearchMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-research",
		Title: "Research",
		Description: `Investigation workflow.

## Step: investigate
Explore the question. Search code, read docs,
understand context. Take notes.

## Step: document
Write up findings. Include:
- What you learned
- Recommendations
- Open questions
Needs: investigate`,
	}
}

// InstallGoBinaryMolecule returns the install-go-binary molecule definition.
// This is a single step to rebuild and install the gt binary after code changes.
func InstallGoBinaryMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-install-go-binary",
		Title: "Install Go Binary",
		Description: `Single step to rebuild and install the gt binary after code changes.

## Step: install
Build and install the gt binary locally.

Run from the rig directory:
` + "```" + `
go build -o gt ./cmd/gt
go install ./cmd/gt
` + "```" + `

Verify the installed binary is updated:
` + "```" + `
which gt
gt --version  # if version command exists
` + "```",
	}
}

// BootstrapGasTownMolecule returns the bootstrap molecule for new Gas Town installations.
// This walks a user through setting up Gas Town from scratch after brew install.
func BootstrapGasTownMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-bootstrap",
		Title: "Bootstrap Gas Town",
		Description: `Complete setup of a new Gas Town installation.

Run this after installing gt and bd via Homebrew. This molecule guides you through
creating a harness, setting up rigs, and configuring your environment.

## Step: locate-harness
Determine where to install the Gas Town harness.

Ask the user for their preferred location. Common choices:
- ~/gt (recommended - short, easy to type)
- ~/gastown
- ~/workspace/gt

Validate the path:
- Must not already exist (or be empty)
- Parent directory must be writable
- Avoid paths with spaces

Store the chosen path for subsequent steps.

## Step: create-harness
Create the harness directory structure.

` + "```" + `bash
mkdir -p {{harness_path}}
cd {{harness_path}}
gt install . --name {{harness_name}}
` + "```" + `

If the user wants to track the harness in git:
` + "```" + `bash
gt git-init --github={{github_repo}} --private
` + "```" + `

The harness now has:
- mayor/ directory
- .beads/ for town-level tracking
- CLAUDE.md for mayor context

Needs: locate-harness

## Step: setup-rigs
Configure which rigs to add to the harness.

Default rigs for Gas Town development:
- gastown (git@github.com:steveyegge/gastown.git)
- beads (git@github.com:steveyegge/beads.git)

For each rig, run:
` + "```" + `bash
gt rig add <name> <git-url> --prefix <prefix>
` + "```" + `

This creates the full rig structure:
- refinery/rig/ (canonical main clone)
- mayor/rig/ (mayor's working clone)
- crew/main/ (default human workspace)
- witness/ (polecat monitor)
- polecats/ (worker directory)

Needs: create-harness

## Step: build-gt
Build the gt binary from source.

` + "```" + `bash
cd {{harness_path}}/gastown/mayor/rig
go build -o gt ./cmd/gt
` + "```" + `

Verify the build succeeded:
` + "```" + `bash
./gt version
` + "```" + `

Needs: setup-rigs
Tier: haiku

## Step: install-paths
Install gt to a location in PATH.

Check if ~/bin or ~/.local/bin is in PATH:
` + "```" + `bash
echo $PATH | tr ':' '\n' | grep -E '(~/bin|~/.local/bin|/home/.*/bin)'
` + "```" + `

Copy the binary:
` + "```" + `bash
mkdir -p ~/bin
cp {{harness_path}}/gastown/mayor/rig/gt ~/bin/gt
` + "```" + `

If ~/bin is not in PATH, add to shell config:
` + "```" + `bash
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
# or ~/.bashrc for bash users
` + "```" + `

Verify:
` + "```" + `bash
which gt
gt version
` + "```" + `

Needs: build-gt
Tier: haiku

## Step: init-beads
Initialize beads databases in all clones.

For each rig's mayor clone:
` + "```" + `bash
cd {{harness_path}}/<rig>/mayor/rig
bd init --prefix <rig-prefix>
` + "```" + `

For the town-level beads:
` + "```" + `bash
cd {{harness_path}}
bd init --prefix gm
` + "```" + `

Configure sync-branch for multi-clone setups:
` + "```" + `bash
echo "sync-branch: beads-sync" >> .beads/config.yaml
` + "```" + `

Needs: setup-rigs
Tier: haiku

## Step: sync-beads
Sync beads from remotes and fix any issues.

For each initialized beads database:
` + "```" + `bash
bd sync
bd doctor --fix
` + "```" + `

This imports existing issues from JSONL and sets up git hooks.

Needs: init-beads
Tier: haiku

## Step: verify
Verify the installation is complete and working.

Run health checks:
` + "```" + `bash
gt status          # Should show rigs with crew/refinery/mayor
gt doctor          # Check for issues
bd list            # Should show issues from synced beads
` + "```" + `

Test spawning capability (dry run):
` + "```" + `bash
gt spawn --help
` + "```" + `

Print summary:
- Harness location
- Installed rigs
- gt version
- bd version

Needs: sync-beads, install-paths`,
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
