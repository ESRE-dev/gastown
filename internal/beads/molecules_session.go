// Package beads provides a wrapper for the bd (beads) CLI.
package beads

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
creating an HQ, setting up rigs, and configuring your environment.

## Step: locate-hq
Determine where to install the Gas Town HQ.

Ask the user for their preferred location. Common choices:
- ~/gt (recommended - short, easy to type)
- ~/gastown
- ~/workspace/gt

Validate the path:
- Must not already exist (or be empty)
- Parent directory must be writable
- Avoid paths with spaces

Store the chosen path for subsequent steps.

## Step: create-hq
Create the HQ directory structure.

` + "```" + `bash
mkdir -p {{hq_path}}
cd {{hq_path}}
gt install . --name {{hq_name}}
` + "```" + `

If the user wants to track the HQ in git:
` + "```" + `bash
gt git-init --github={{github_repo}} --private
` + "```" + `

The HQ now has:
- mayor/ directory
- .beads/ for town-level tracking
- CLAUDE.md for mayor context

Needs: locate-hq

## Step: setup-rigs
Configure which rigs to add to the HQ.

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

Needs: create-hq

## Step: build-gt
Build the gt binary from source.

` + "```" + `bash
cd {{hq_path}}/gastown/mayor/rig
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
cp {{hq_path}}/gastown/mayor/rig/gt ~/bin/gt
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
cd {{hq_path}}/<rig>/mayor/rig
bd init --prefix <rig-prefix>
` + "```" + `

For the town-level beads:
` + "```" + `bash
cd {{hq_path}}
bd init --prefix hq
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
- HQ location
- Installed rigs
- gt version
- bd version

Needs: sync-beads, install-paths`,
	}
}

// VersionBumpMolecule returns the version-bump molecule definition.
// This is the release checklist for Gas Town versions.
func VersionBumpMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-version-bump",
		Title: "Version Bump",
		Description: `Release checklist for Gas Town version {{version}}.

This molecule ensures all release steps are completed properly.
Replace {{version}} with the target version (e.g., 0.1.0).

## Step: update-version
Update version string in internal/cmd/version.go.

Change the Version variable to the new version:
` + "```" + `go
var (
    Version   = "{{version}}"
    BuildTime = "unknown"
    GitCommit = "unknown"
)
` + "```" + `

## Step: rebuild-binary
Rebuild the gt binary with version info.

` + "```" + `bash
go build -ldflags="-X github.com/steveyegge/gastown/internal/cmd.Version={{version}} \
  -X github.com/steveyegge/gastown/internal/cmd.GitCommit=$(git rev-parse --short HEAD) \
  -X github.com/steveyegge/gastown/internal/cmd.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o gt ./cmd/gt
` + "```" + `

Verify the version:
` + "```" + `bash
./gt version
` + "```" + `

Needs: update-version

## Step: run-tests
Run the full test suite.

` + "```" + `bash
go test ./...
` + "```" + `

Fix any failures before proceeding.
Needs: rebuild-binary

## Step: update-changelog
Update CHANGELOG.md with release notes.

Add a new section at the top:
` + "```" + `markdown
## [{{version}}] - YYYY-MM-DD

### Added
- Feature descriptions

### Changed
- Change descriptions

### Fixed
- Bug fix descriptions
` + "```" + `

Needs: run-tests

## Step: commit-release
Commit the release changes.

` + "```" + `bash
git add -A
git commit -m "release: v{{version}}"
` + "```" + `

Needs: update-changelog

## Step: tag-release
Create and push the release tag.

` + "```" + `bash
git tag -a v{{version}} -m "Release v{{version}}"
git push origin main
git push origin v{{version}}
` + "```" + `

Needs: commit-release

## Step: verify-release
Verify the release is complete.

- Check that the tag exists on GitHub
- Verify CI/CD (if configured) completed successfully
- Test installation from the new tag:
` + "```" + `bash
go install github.com/steveyegge/gastown/cmd/gt@v{{version}}
gt version
` + "```" + `

Needs: tag-release

## Step: update-installations
Update local installations and restart daemons.

` + "```" + `bash
# Rebuild and install
go install ./cmd/gt

# Restart any running daemons
pkill -f "gt daemon" || true
gt daemon start
` + "```" + `

Needs: verify-release`,
	}
}

// CrewSessionMolecule returns the crew-session molecule definition.
// This is a light harness for crew workers that enables autonomous overnight work.
// Key insight: if there's an attached mol, continue working without awaiting input.
func CrewSessionMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-crew-session",
		Title: "Crew Session",
		Description: `Light session harness for crew workers.

This molecule enables autonomous work on long-lived molecules. The key insight:
**If there's an attached mol, continue working without awaiting input.**

This transforms crew workers from interactive assistants to autonomous workers
that can churn through long molecules overnight.

## Step: orient
Load context and identify self.

` + "```" + `bash
gt prime                    # Load Gas Town context
` + "```" + `

Identify yourself:
- Read crew.md for role context
- Note your rig and crew member name
- Understand the session wisp model

## Step: handoff-read
Check inbox for predecessor handoff.

` + "```" + `bash
gt mail inbox
` + "```" + `

Look for 🤝 HANDOFF messages from your previous session.
If found:
- Read the handoff carefully
- Load predecessor's context and state
- Note where they left off

If no handoff found, this is a fresh start.
Needs: orient

## Step: check-attachment
Look for pinned work to continue.

` + "```" + `bash
bd list --pinned --assignee=$(gt whoami) --status=in_progress
gt mol status
` + "```" + `

**DECISION POINT:**

If attachment found:
- This is autonomous continuation mode
- Proceed directly to execute step
- NO human input needed

If no attachment found:
- This is interactive mode
- Await user instruction before proceeding
- Mark this step complete when user provides direction
Needs: handoff-read

## Step: execute
Work the attached molecule.

Find next ready step in the attached mol:
` + "```" + `bash
bd ready --parent=<work-mol-root>
bd update <step> --status=in_progress
` + "```" + `

Work until one of:
- All steps in mol completed
- Context approaching limit (>80%)
- Natural stopping point reached
- Blocked by external dependency

Track progress in the mol itself (close completed steps).
File discovered work as new issues.
Needs: check-attachment

## Step: cleanup
End session with proper handoff.

1. Sync all state:
` + "```" + `bash
git add -A && git commit -m "WIP: <summary>" || true
git push origin HEAD
bd sync
` + "```" + `

2. Write handoff to successor (yourself):
` + "```" + `bash
gt mail send <self-addr> -s "🤝 HANDOFF: <brief context>" -m "
## Progress
- Completed: <what was done>
- Next: <what to do next>

## State
- Current step: <step-id>
- Blockers: <any blockers>

## Notes
<any context successor needs>
"
` + "```" + `

3. Session ends. Successor will pick up from handoff.
Needs: execute`,
	}
}

// PolecatSessionMolecule returns the polecat-session molecule definition.
// This is a one-shot session wisp that wraps polecat work.
// Unlike patrol wisps (which loop), this wisp terminates with the session.
func PolecatSessionMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-polecat-session",
		Title: "Polecat Session",
		Description: `One-shot session wisp for polecat workers.

This molecule wraps the polecat's work assignment. It handles:
1. Onboarding - read polecat.md, load context
2. Execution - run the attached work molecule
3. Cleanup - sync, burn, request shutdown

Unlike patrol wisps (which loop), this wisp terminates when work is done.
The attached work molecule is permanent and auditable.

## Step: orient
Read polecat.md protocol and initialize context.

` + "```" + `bash
gt prime               # Load Gas Town context
bd sync --from-main    # Fresh beads state
gt mail inbox          # Check for work assignment
` + "```" + `

Understand:
- Your identity (rig/polecat-name)
- The beads system
- Exit strategies (COMPLETED, BLOCKED, REFACTOR, ESCALATE)
- Handoff protocols

## Step: handoff-read
Check for predecessor session handoff.

If this polecat was respawned after a crash or context cycle:
- Check mail for 🤝 HANDOFF from previous session
- Load state from the attached work mol
- Resume from last completed step

` + "```" + `bash
gt mail inbox | grep HANDOFF
bd show <work-mol-id>  # Check step completion state
` + "```" + `
Needs: orient

## Step: find-work
Locate attached work molecule.

` + "```" + `bash
gt mol status          # Shows what's on your hook
` + "```" + `

The work mol should already be attached (done by spawn).
If not attached, check mail for work assignment.

Verify you have:
- A work mol ID
- Understanding of the work scope
- No blockers to starting
Needs: handoff-read

## Step: execute
Run the attached work molecule to completion.

For each ready step in the work mol:
` + "```" + `bash
bd ready --parent=<work-mol-root>
bd update <step> --status=in_progress
# ... do the work ...
bd close <step>
` + "```" + `

Continue until reaching the exit-decision step in the work mol.
All exit types (COMPLETED, BLOCKED, REFACTOR, ESCALATE) proceed to cleanup.

**Dynamic modifications allowed**:
- Add review or test steps if needed
- File discovered blockers as issues
- Request session refresh if context filling
Needs: find-work

## Step: cleanup
Finalize session and request termination.

1. Sync all state:
` + "```" + `bash
bd sync
git push origin HEAD
` + "```" + `

2. Update work mol based on exit type:
   - COMPLETED: ` + "`bd close <work-mol-root>`" + `
   - BLOCKED/REFACTOR/ESCALATE: ` + "`bd update <work-mol-root> --status=deferred`" + `

3. Burn this session wisp (no audit needed):
` + "```" + `bash
bd mol burn
` + "```" + `

4. Request shutdown from Witness:
` + "```" + `bash
gt mail send <rig>/witness -s "SHUTDOWN: <polecat-name>" -m "Session complete. Exit: <type>"
` + "```" + `

5. Wait for Witness to terminate session. Do not exit directly.
Needs: execute`,
	}
}
