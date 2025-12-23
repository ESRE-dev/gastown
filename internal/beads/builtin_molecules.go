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
		PolecatWorkMolecule(),
		VersionBumpMolecule(),
		DeaconPatrolMolecule(),
		RefineryPatrolMolecule(),
		CrewSessionMolecule(),
		PolecatSessionMolecule(),
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

// PolecatWorkMolecule returns the polecat-work molecule definition.
// This is the full polecat lifecycle from assignment to decommission.
// It's an operational molecule that enables crash recovery and context survival.
func PolecatWorkMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-polecat-work",
		Title: "Polecat Work",
		Description: `Full polecat lifecycle from assignment to decommission.

This molecule is your contract. Follow it to one of its defined exits.
The Witness doesn't care which exit you take, only that you exit properly.

**State Machine**: A polecat that crashes can restart, read its molecule state,
and continue from the last completed step. No work is lost.

**Non-Linear Exits**: If blocked at any step, skip to exit-decision directly.

## Step: load-context
Run gt prime and bd prime. Verify issue assignment.
Check inbox for any relevant messages.

Read the assigned issue and understand the requirements.
Identify any blockers or missing information.

**If blocked here**: Missing requirements? Unclear scope? Jump to exit-decision
with exit_type=escalate.

## Step: implement
Implement the solution. Follow codebase conventions.
File discovered work as new issues with bd create.

Make regular commits with clear messages.
Keep changes focused on the assigned issue.

**Dynamic modifications allowed**:
- Add extra review or test steps if needed
- File discovered blockers as issues
- Request session refresh if context is filling up

**If blocked here**: Dependency missing? Work too large? Jump to exit-decision.
Needs: load-context

## Step: self-review
Review your own changes. Look for:
- Bugs and edge cases
- Style issues
- Missing error handling
- Security concerns

Fix any issues found before proceeding.
Needs: implement

## Step: verify-tests
Run existing tests. Add new tests for new functionality.
Ensure adequate coverage.

` + "```" + `bash
go test ./...
` + "```" + `

Fix any test failures before proceeding.
Needs: implement

## Step: rebase-main
Rebase against main to incorporate any changes.
Resolve conflicts if needed.

` + "```" + `bash
git fetch origin main
git rebase origin/main
` + "```" + `

If there are conflicts, resolve them carefully and
continue the rebase. If conflicts are unresolvable, jump to exit-decision
with exit_type=escalate.
Needs: self-review, verify-tests

## Step: submit-merge
Submit to merge queue via beads.

**IMPORTANT**: Do NOT use gh pr create or GitHub PRs.
The Refinery processes merges via beads merge-request issues.

1. Push your branch to origin
2. Create a beads merge-request: bd create --type=merge-request --title="Merge: <summary>"
3. Signal ready: gt done

` + "```" + `bash
git push origin HEAD
bd create --type=merge-request --title="Merge: <issue-summary>"
gt done  # Signal work ready for merge queue
` + "```" + `

If there are CI failures, fix them before proceeding.
Needs: rebase-main

## Step: exit-decision
**CONVERGENCE POINT**: All exits pass through here.

Determine your exit type and take appropriate action:

### Exit Type: COMPLETED (normal)
Work finished successfully. Submit-merge done.
` + "```" + `bash
# Document completion
bd update <step-id> --status=closed
` + "```" + `

### Exit Type: BLOCKED
External dependency prevents progress.
` + "```" + `bash
# 1. File the blocker
bd create --type=task --title="Blocker: <description>" --priority=1

# 2. Link dependency
bd dep add <your-issue> <blocker-id>

# 3. Defer your issue
bd update <your-issue> --status=deferred

# 4. Notify witness
gt mail send <rig>/witness -s "Blocked: <issue-id>" -m "Blocked by <blocker-id>. Deferring."
` + "```" + `

### Exit Type: REFACTOR
Work is too large for one polecat session.
` + "```" + `bash
# Option A: Self-refactor
# 1. Break into sub-issues
bd create --type=task --title="Sub: part 1" --parent=<your-issue>
bd create --type=task --title="Sub: part 2" --parent=<your-issue>

# 2. Close what you completed, defer the rest
bd close <completed-sub-issues>
bd update <your-issue> --status=deferred

# Option B: Request refactor
gt mail send mayor/ -s "Refactor needed: <issue-id>" -m "
Issue too large. Completed X, remaining Y needs breakdown.
Recommend splitting into: ...
"
bd update <your-issue> --status=deferred
` + "```" + `

### Exit Type: ESCALATE
Need human judgment or authority.
` + "```" + `bash
# 1. Document what you know
bd comment <your-issue> "Escalating because: <reason>. Context: <details>"

# 2. Mail human
gt mail send --human -s "Escalation: <issue-id>" -m "
Need human decision on: <specific question>
Context: <what you've tried>
Options I see: <A, B, C>
"

# 3. Defer the issue
bd update <your-issue> --status=deferred
` + "```" + `

**Record your exit**: Update this step with your exit type and actions taken.
Needs: load-context

## Step: request-shutdown
Wait for termination.

All exit paths converge here. Your work is either:
- Merged (COMPLETED)
- Deferred with proper handoff (BLOCKED/REFACTOR/ESCALATE)

The polecat is now ready to be cleaned up.
Do not exit directly - wait for Witness to kill the session.
Needs: exit-decision`,
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

// DeaconPatrolMolecule returns the deacon-patrol molecule definition.
// This is the Mayor's daemon loop for handling callbacks, health checks, and cleanup.
func DeaconPatrolMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-deacon-patrol",
		Title: "Deacon Patrol",
		Description: `Mayor's daemon patrol loop.

The Deacon is the Mayor's background process that runs continuously,
handling callbacks, monitoring rig health, and performing cleanup.
Each patrol cycle runs these steps in sequence, then loops or exits.

## Step: inbox-check
Handle callbacks from agents.

Check the Mayor's inbox for messages from:
- Witnesses reporting polecat status
- Refineries reporting merge results
- Polecats requesting help or escalation
- External triggers (webhooks, timers)

Process each message:
` + "```" + `bash
gt mail inbox
# For each message:
gt mail read <id>
# Handle based on message type
` + "```" + `

Callbacks may spawn new polecats, update issue state, or trigger other actions.

## Step: health-scan
Ping Witnesses and Refineries.

For each rig, verify:
- Witness is responsive
- Refinery is processing queue
- No stalled operations

` + "```" + `bash
gt status --health
# Check each rig
for rig in $(gt rigs); do
    gt rig status $rig
done
` + "```" + `

Report any issues found. Restart unresponsive components if needed.
Needs: inbox-check

## Step: plugin-run
Execute registered plugins.

Scan ~/gt/plugins/ for plugin directories. Each plugin has a plugin.md with
YAML frontmatter defining its gate (when to run) and instructions (what to do).

See docs/deacon-plugins.md for full documentation.

Gate types:
- cooldown: Time since last run (e.g., 24h)
- cron: Schedule-based (e.g., "0 9 * * *")
- condition: Metric threshold (e.g., wisp count > 50)
- event: Trigger-based (e.g., startup, heartbeat)

For each plugin:
1. Read plugin.md frontmatter to check gate
2. Compare against state.json (last run, etc.)
3. If gate is open, execute the plugin

Plugins marked parallel: true can run concurrently using Task tool subagents.
Sequential plugins run one at a time in directory order.

Skip this step if ~/gt/plugins/ does not exist or is empty.
Needs: health-scan

## Step: orphan-check
Find abandoned work.

Scan for orphaned state:
- Issues marked in_progress with no active polecat
- Polecats that stopped responding mid-work
- Merge queue entries with no polecat owner
- Wisp sessions that outlived their spawner

` + "```" + `bash
bd list --status=in_progress
gt polecats --all --orphan
` + "```" + `

For each orphan:
- Check if polecat session still exists
- If not, mark issue for reassignment or retry
- File incident beads if data loss occurred
Needs: health-scan

## Step: session-gc
Clean dead sessions.

Garbage collect terminated sessions:
- Remove stale polecat directories
- Clean up wisp session artifacts
- Prune old logs and temp files
- Archive completed molecule state

` + "```" + `bash
gt gc --sessions
gt gc --wisps --age=1h
` + "```" + `

Preserve audit trail. Only clean sessions confirmed dead.
Needs: orphan-check

## Step: context-check
Check own context limit.

The Deacon runs in a Claude session with finite context.
Check if approaching the limit:

` + "```" + `bash
gt context --usage
` + "```" + `

If context is high (>80%), prepare for handoff:
- Summarize current state
- Note any pending work
- Write handoff to molecule state

This enables the Deacon to burn and respawn cleanly.
Needs: session-gc

## Step: loop-or-exit
Burn and let daemon respawn, or exit if context high.

Decision point at end of patrol cycle:

If context is LOW:
- Sleep briefly (avoid tight loop)
- Return to inbox-check step

If context is HIGH:
- Write state to persistent storage
- Exit cleanly
- Let the daemon orchestrator respawn a fresh Deacon

The daemon ensures Deacon is always running:
` + "```" + `bash
# Daemon respawns on exit
gt daemon status
` + "```" + `

This enables infinite patrol duration via context-aware respawning.
Needs: context-check`,
	}
}

// RefineryPatrolMolecule returns the refinery-patrol molecule definition.
// This is the merge queue processor's patrol loop with verification gates.
func RefineryPatrolMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-refinery-patrol",
		Title: "Refinery Patrol",
		Description: `Merge queue processor patrol loop.

The Refinery is the Engineer in the engine room. You process polecat branches,
merging them to main one at a time with sequential rebasing.

**The Scotty Test**: Before proceeding past any failure, ask yourself:
"Would Scotty walk past a warp core leak because it existed before his shift?"

## Step: inbox-check
Check mail for MR submissions, escalations, messages.

` + "```" + `bash
gt mail inbox
# Process any urgent items
` + "```" + `

Handle shutdown requests, escalations, and status queries.

## Step: queue-scan
Fetch remote and identify polecat branches waiting.

` + "```" + `bash
git fetch origin
git branch -r | grep polecat
gt refinery queue <rig>
` + "```" + `

If queue empty, skip to context-check step.
Track branch list for this cycle.
Needs: inbox-check

## Step: process-branch
Pick next branch. Rebase on current main.

` + "```" + `bash
git checkout -b temp origin/<polecat-branch>
git rebase origin/main
` + "```" + `

If rebase conflicts and unresolvable:
- git rebase --abort
- Notify polecat to fix and resubmit
- Skip to loop-check for next branch

Needs: queue-scan

## Step: run-tests
Run the test suite.

` + "```" + `bash
go test ./...
` + "```" + `

Track results: pass count, fail count, specific failures.
Needs: process-branch

## Step: handle-failures
**VERIFICATION GATE**: This step enforces the Beads Promise.

If tests PASSED: This step auto-completes. Proceed to merge.

If tests FAILED:
1. Diagnose: Is this a branch regression or pre-existing on main?
2. If branch caused it:
   - Abort merge
   - Notify polecat: "Tests failing. Please fix and resubmit."
   - Skip to loop-check
3. If pre-existing on main:
   - Option A: Fix it yourself (you're the Engineer!)
   - Option B: File a bead: bd create --type=bug --priority=1 --title="..."

**GATE REQUIREMENT**: You CANNOT proceed to merge-push without:
- Tests passing, OR
- Fix committed, OR
- Bead filed for the failure

This is non-negotiable. Never disavow. Never "note and proceed."
Needs: run-tests

## Step: merge-push
Merge to main and push immediately.

` + "```" + `bash
git checkout main
git merge --ff-only temp
git push origin main
git branch -d temp
git push origin --delete <polecat-branch>
` + "```" + `

Main has moved. Any remaining branches need rebasing on new baseline.
Needs: handle-failures

## Step: loop-check
More branches to process?

If yes: Return to process-branch with next branch.
If no: Continue to generate-summary.

Track: branches processed, branches skipped (with reasons).
Needs: merge-push

## Step: generate-summary
Summarize this patrol cycle.

Include:
- Branches processed (count, names)
- Test results (pass/fail)
- Issues filed (if any)
- Branches skipped (with reasons)
- Any escalations sent

This becomes the digest when the patrol is squashed.
Needs: loop-check

## Step: context-check
Check own context usage.

If context is HIGH (>80%):
- Write handoff summary
- Prepare for burn/respawn

If context is LOW:
- Can continue processing
Needs: generate-summary

## Step: burn-or-loop
End of patrol cycle decision.

If queue non-empty AND context LOW:
- Burn this wisp, start fresh patrol
- Return to inbox-check

If queue empty OR context HIGH:
- Burn wisp with summary digest
- Exit (daemon will respawn if needed)
Needs: context-check`,
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

3. Burn this session wisp (ephemeral, no audit needed):
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
