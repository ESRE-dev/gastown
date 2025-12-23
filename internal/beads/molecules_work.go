// Package beads provides a wrapper for the bd (beads) CLI.
package beads

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

// ReadyWorkMolecule returns the ready-work molecule definition.
// This is an autonomous backlog processing patrol for crew workers.
// It's a vapor-phase molecule (wisp) that scans backlogs, selects work,
// and processes items until context is low.
func ReadyWorkMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-ready-work",
		Title: "Ready Work",
		Description: `Autonomous backlog processing patrol for crew workers.

**Phase**: vapor (wisp) - ephemeral patrol cycles
**Squash**: after each work item or context threshold

This molecule enables crew workers to autonomously process backlog items
using an ROI heuristic. It scans multiple backlogs, selects the highest-value
achievable item, executes it, and loops until context is low.

## Variables

| Variable | Default | Description |
|----------|---------|-------------|
| backlog_priority | (see scan order) | Override backlog scan order |
| context_threshold | 20 | Percentage at which to handoff |
| max_items | unlimited | Maximum items to process per session |

## Step: orient
Load context and check for interrupts.

` + "```" + `bash
gt prime                    # Load Gas Town context
bd sync --from-main         # Fresh beads state
` + "```" + `

Check for:
- Mail with overseer instructions: ` + "`gt mail inbox`" + `
- Predecessor handoff: Look for 🤝 HANDOFF messages
- Current context state

If overseer mail directs specific work, attach that instead of autonomous scan.
If handoff exists, resume from handoff state.

## Step: scan-backlogs
Survey all backlogs in priority order.

Scan order (highest to lowest priority):
1. ` + "`gh pr list --state open`" + ` - PRs need review/merge
2. ` + "`gh issue list --state open --label untriaged`" + ` - Untriaged issues
3. ` + "`bd ready`" + ` - Beads issues ready to work
4. ` + "`gh issue list --state open --label triaged`" + ` - Triaged GitHub issues

For each backlog, capture:
- Total count of items
- Top 3-5 candidates with brief descriptions
- Estimated size category (small/medium/large)

` + "```" + `bash
# Example scan
echo "=== PRs ===" && gh pr list --state open --limit 10
echo "=== Untriaged ===" && gh issue list --state open --label untriaged --limit 10
echo "=== Beads Ready ===" && bd ready
echo "=== Triaged ===" && gh issue list --state open --label triaged --limit 10
` + "```" + `

If all backlogs empty: exit patrol (nothing to do).
Needs: orient

## Step: select-work
Apply ROI heuristic to select best work item.

**ROI Formula**: Impact / Effort, constrained by remaining context

Evaluation criteria:
1. **Estimate size** - Tokens needed (small=1k, medium=5k, large=20k+)
2. **Check context capacity** - Can this item fit in remaining context?
3. **Weight by impact**:
   - PRs: High (blocking others) → weight 3x
   - Untriaged: Medium (needs triage) → weight 2x
   - Beads ready: Medium (concrete work) → weight 2x
   - Triaged GH: Lower (already processed) → weight 1x
4. **Adjust by priority** - P0/P1 issues get 2x multiplier

Selection algorithm:
1. Filter to items that fit in remaining context
2. Score each: (impact_weight × priority_multiplier) / estimated_effort
3. Select highest scoring item
4. If tie: prefer PRs > untriaged > beads > triaged

If no achievable items (all too large): goto handoff step.
Record selection rationale for audit.
Needs: scan-backlogs

## Step: execute-work
Work the selected item based on its type.

**For PRs (gh pr)**:
- Review the changes
- If good: approve and merge
- If issues: request changes with specific feedback
- Close or comment as appropriate

**For untriaged issues (gh issue, no label)**:
- Read and understand the issue
- Add appropriate labels (bug, feature, enhancement, etc.)
- Set priority if determinable
- Convert to beads if actionable: ` + "`bd create --title=\"...\" --type=...`" + `
- Close if duplicate/invalid/wontfix

**For beads ready (bd)**:
- Claim: ` + "`bd update <id> --status=in_progress`" + `
- Implement the fix/feature
- Test changes
- Commit and push
- Close: ` + "`bd close <id>`" + `

**For triaged GitHub issues**:
- Implement the fix/feature
- Create PR or push directly
- Link to issue: ` + "`Fixes #<num>`" + `
- Close issue when merged

Commit regularly. Push changes. Update issue state.
Needs: select-work

## Step: check-context
Assess context state after completing work item.

` + "```" + `bash
# Estimate context usage (if tool available)
gt context --usage
` + "```" + `

Decision matrix:
| Context Used | Action |
|--------------|--------|
| < 60% | Loop to scan-backlogs (continue working) |
| 60-80% | One more small item, then handoff |
| > 80% | Goto handoff immediately |

Also check:
- Items processed this session vs max_items limit
- Time elapsed (soft limit for long sessions)
- Any new high-priority mail that should interrupt

If continuing: return to scan-backlogs step.
Needs: execute-work

## Step: handoff
Prepare for session transition.

1. **Summarize work completed**:
   - Items processed (count and types)
   - PRs reviewed/merged
   - Issues triaged
   - Beads closed
   - Any issues filed

2. **Note in-progress items**:
   - If interrupted mid-work, record state
   - File continuation issue if needed

3. **Send handoff mail**:
` + "```" + `bash
gt mail send <self-addr> -s "🤝 HANDOFF: Ready-work patrol" -m "
## Completed This Session
- <item counts and summaries>

## Backlog State
- PRs remaining: <count>
- Untriaged: <count>
- Beads ready: <count>
- Triaged: <count>

## Notes
<any context for successor>
"
` + "```" + `

4. **Squash wisp to digest**:
` + "```" + `bash
bd mol squash <wisp-id> --summary="Processed N items: X PRs, Y issues, Z beads"
` + "```" + `

5. **Exit for fresh session** - Successor picks up from handoff.
Needs: check-context`,
	}
}
