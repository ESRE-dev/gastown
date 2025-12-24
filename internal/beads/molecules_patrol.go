// Package beads provides a wrapper for the bd (beads) CLI.
package beads

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

## Step: trigger-pending-spawns
Nudge newly spawned polecats that are ready for input.

When polecats are spawned, their Claude session takes 10-20 seconds to initialize.
The spawn command returns immediately without waiting. This step finds spawned
polecats that are now ready and sends them a trigger to start working.

` + "```" + `bash
# For each rig with polecats
for rig in gastown beads; do
    gt polecats $rig
    # For each working polecat, check if Claude is ready
    # Use tmux capture-pane to look for "> " prompt
done
` + "```" + `

For each ready polecat that hasn't been triggered yet:
1. Send "Begin." to trigger UserPromptSubmit hook
2. The hook injects mail, polecat sees its assignment
3. Mark polecat as triggered in state

Use WaitForClaudeReady from tmux package (polls for "> " prompt).
Timeout: 60 seconds per polecat. If not ready, try again next cycle.
Needs: inbox-check

## Step: health-scan
Check Witness and Refinery health for each rig.

**ZFC Principle**: You (Claude) make the judgment call about what is "stuck" or
"unresponsive" - there are no hardcoded thresholds in Go. Read the signals,
consider context, and decide.

For each rig, run:
` + "```" + `bash
gt witness status <rig>
gt refinery status <rig>
` + "```" + `

**Signals to assess:**

| Component | Healthy Signals | Concerning Signals |
|-----------|-----------------|-------------------|
| Witness | State: running, recent activity | State: not running, no heartbeat |
| Refinery | State: running, queue processing | Queue stuck, merge failures |

**Tracking unresponsive cycles:**

Maintain in your patrol state (persisted across cycles):
` + "```" + `
health_state:
  <rig>:
    witness:
      unresponsive_cycles: 0
      last_seen_healthy: <timestamp>
    refinery:
      unresponsive_cycles: 0
      last_seen_healthy: <timestamp>
` + "```" + `

**Decision matrix** (you decide the thresholds based on context):

| Cycles Unresponsive | Suggested Action |
|---------------------|------------------|
| 1-2 | Note it, check again next cycle |
| 3-4 | Attempt restart: gt witness restart <rig> |
| 5+ | Escalate to Mayor with context |

**Restart commands:**
` + "```" + `bash
gt witness restart <rig>
gt refinery restart <rig>
` + "```" + `

**Escalation:**
` + "```" + `bash
gt mail send mayor/ -s "Health: <rig> <component> unresponsive" \
  -m "Component has been unresponsive for N cycles. Restart attempts failed.
      Last healthy: <timestamp>
      Error signals: <details>"
` + "```" + `

Reset unresponsive_cycles to 0 when component responds normally.
Needs: trigger-pending-spawns

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

// WitnessPatrolMolecule returns the witness-patrol molecule definition.
// This is the per-rig worker monitor's patrol loop using the Christmas Ornament pattern.
func WitnessPatrolMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-witness-patrol",
		Title: "Witness Patrol",
		Description: `Per-rig worker monitor patrol loop using the Christmas Ornament pattern.

The Witness is the Pit Boss for your rig. You watch polecats, nudge them toward
completion, verify clean git state before kills, and escalate stuck workers.

**You do NOT do implementation work.** Your job is oversight, not coding.

This molecule uses dynamic bonding to spawn mol-polecat-arm for each worker,
enabling parallel inspection with a fanout gate for aggregation.

## The Christmas Ornament Shape

` + "```" + `
                     ★ mol-witness-patrol (trunk)
                    /|\
          ┌────────┘ │ └────────┐
       PREFLIGHT  DISCOVERY  CLEANUP
          │          │           │
      inbox-check  survey    aggregate (WaitsFor: all-children)
      check-refnry   │       save-state
      load-state     │       generate-summary
                     ↓       context-check
             ┌───────┼───────┐  burn-or-loop
             ●       ●       ●   mol-polecat-arm (dynamic)
            ace     nux    toast
` + "```" + `

---
# PREFLIGHT PHASE
---

## Step: inbox-check
Process witness mail: lifecycle requests, help requests.

` + "```" + `bash
gt mail inbox
` + "```" + `

Handle by message type:
- **LIFECYCLE/Shutdown**: Queue for pre-kill verification
- **Blocked/Help**: Assess if resolvable or escalate
- **HANDOFF**: Load predecessor state
- **Work complete**: Verify issue closed, proceed to pre-kill

Record any pending actions for later steps.
Mark messages as processed when complete.

## Step: check-refinery
Ensure the refinery is alive and processing merge requests.

**Redundant system**: This check runs in both gt spawn and Witness patrol
to ensure the merge queue processor stays operational.

` + "```" + `bash
# Check if refinery session is running
gt session status <rig>/refinery

# Check for merge requests in queue
bd list --type=merge-request --status=open
` + "```" + `

If merge requests are waiting AND refinery is not running:
` + "```" + `bash
gt session start <rig>/refinery
gt mail send <rig>/refinery -s "PATROL: Wake up" -m "Merge requests in queue. Please process."
` + "```" + `

If refinery is running but queue is non-empty for >30 min, send nudge.
This ensures polecats don't wait forever for their branches to merge.
Needs: inbox-check

## Step: load-state
Read handoff bead and get nudge counts.

Load persistent state from the witness handoff bead:
- Active workers and their status from last cycle
- Nudge counts per worker per issue
- Last nudge timestamps
- Pending escalations

` + "```" + `bash
bd show <handoff-bead-id>
` + "```" + `

If no handoff exists (fresh start), initialize empty state.
This state persists across wisp burns and session cycles.
Needs: check-refinery

---
# DISCOVERY PHASE (Dynamic Bonding)
---

## Step: survey-workers
List polecats and bond mol-polecat-arm for each one.

` + "```" + `bash
# Get list of polecats
gt polecat list <rig>
` + "```" + `

For each polecat discovered, dynamically bond an inspection arm:

` + "```" + `bash
# Bond mol-polecat-arm for each polecat
for polecat in $(gt polecat list <rig> --names); do
  bd mol bond mol-polecat-arm $PATROL_WISP_ID \
    --ref arm-$polecat \
    --var polecat_name=$polecat \
    --var rig=<rig>
done
` + "```" + `

This creates child wisps like:
- patrol-x7k.arm-ace (5 steps)
- patrol-x7k.arm-nux (5 steps)
- patrol-x7k.arm-toast (5 steps)

Each arm runs in PARALLEL. The aggregate step will wait for all to complete.

If no polecats are found, this step completes immediately with no children.
Needs: load-state

---
# CLEANUP PHASE (Gate + Fixed Steps)
---

## Step: aggregate
Collect outcomes from all polecat inspection arms.
WaitsFor: all-children

This is a **fanout gate** - it cannot proceed until ALL dynamically-bonded
polecat arms have completed their inspection cycles.

Once all arms complete, collect their outcomes:
- Actions taken per polecat (nudge, kill, escalate, none)
- Updated nudge counts
- Any errors or issues discovered

Build the consolidated state for save-state.
Needs: survey-workers

## Step: save-state
Update handoff bead with new states.

Persist state to the witness handoff bead:
- Updated worker statuses from all arms
- Current nudge counts per worker
- Nudge timestamps
- Actions taken this cycle
- Pending items for next cycle

` + "```" + `bash
bd update <handoff-bead-id> --description="<serialized state>"
` + "```" + `

This state survives wisp burns and session cycles.
Needs: aggregate

## Step: generate-summary
Summarize this patrol cycle for digest.

Include:
- Workers inspected (count, names)
- Nudges sent (count, to whom)
- Sessions killed (count, names)
- Escalations (count, issues)
- Issues found (brief descriptions)
- Actions pending for next cycle

This becomes the digest when the patrol wisp is squashed.
Needs: save-state

## Step: context-check
Check own context usage.

If context is HIGH (>80%):
- Ensure state is saved to handoff bead
- Prepare for burn/respawn

If context is LOW:
- Can continue patrolling
Needs: generate-summary

## Step: burn-or-loop
End of patrol cycle decision.

If context is LOW:
- Burn this wisp (no audit trail needed for patrol cycles)
- Sleep briefly to avoid tight loop (30-60 seconds)
- Return to inbox-check step

If context is HIGH:
- Burn wisp with summary digest
- Exit cleanly (daemon will respawn fresh Witness)

` + "```" + `bash
bd mol burn   # Destroy ephemeral wisp
` + "```" + `

The daemon ensures Witness is always running.
Needs: context-check`,
	}
}

// PolecatArmMolecule returns the polecat-arm molecule definition.
// This is dynamically bonded by mol-witness-patrol for each polecat being monitored.
func PolecatArmMolecule() BuiltinMolecule {
	return BuiltinMolecule{
		ID:    "mol-polecat-arm",
		Title: "Polecat Arm",
		Description: `Single polecat inspection and action cycle.

This molecule is bonded dynamically by mol-witness-patrol's survey-workers step.
Each polecat being monitored gets one arm that runs in parallel with other arms.

## Variables

| Variable | Required | Description |
|----------|----------|-------------|
| polecat_name | Yes | Name of the polecat to inspect |
| rig | Yes | Rig containing the polecat |

## Step: capture
Capture recent tmux output for {{polecat_name}}.

` + "```" + `bash
tmux capture-pane -t gt-{{rig}}-{{polecat_name}} -p | tail -50
` + "```" + `

Record:
- Last activity timestamp (when was last tool call?)
- Visible errors or stack traces
- Completion indicators ("Done", "Finished", etc.)

## Step: assess
Categorize polecat state based on captured output.

States:
- **working**: Recent tool calls, active processing
- **idle**: At prompt, no recent activity
- **error**: Showing errors or stack traces
- **requesting_shutdown**: Sent LIFECYCLE/Shutdown mail
- **done**: Showing completion indicators

Calculate: minutes since last activity.
Needs: capture

## Step: load-history
Read nudge history for {{polecat_name}} from patrol state.

` + "```" + `
nudge_count = state.nudges[{{polecat_name}}].count
last_nudge_time = state.nudges[{{polecat_name}}].timestamp
` + "```" + `

This data was loaded by the parent patrol's load-state step and passed
to the arm via the bonding context.
Needs: assess

## Step: decide
Apply the nudge matrix to determine action for {{polecat_name}}.

| State | Idle Time | Nudge Count | Action |
|-------|-----------|-------------|--------|
| working | any | any | none |
| idle | <10min | any | none |
| idle | 10-15min | 0 | nudge-1 (gentle) |
| idle | 15-20min | 1 | nudge-2 (direct) |
| idle | 20+min | 2 | nudge-3 (final) |
| idle | any | 3 | escalate |
| error | any | any | assess-severity |
| requesting_shutdown | any | any | pre-kill-verify |
| done | any | any | pre-kill-verify |

Nudge text:
1. "How's progress? Need any help?"
2. "Please wrap up soon. What's blocking you?"
3. "Final check. Will escalate in 5 min if no response."

Record decision and rationale.
Needs: load-history

## Step: execute
Take the decided action for {{polecat_name}}.

**nudge-N**:
` + "```" + `bash
tmux send-keys -t gt-{{rig}}-{{polecat_name}} "{{nudge_text}}" Enter
` + "```" + `

**pre-kill-verify**:
` + "```" + `bash
cd polecats/{{polecat_name}}
git status                    # Must be clean
git log origin/main..HEAD     # Check for unpushed
bd show <assigned-issue>      # Verify closed/deferred
` + "```" + `
If clean: kill session, remove worktree, delete branch
If dirty: record failure, retry next cycle

**escalate**:
` + "```" + `bash
gt mail send mayor/ -s "Escalation: {{polecat_name}} stuck" -m "..."
` + "```" + `

**none**: No action needed.

Record: action taken, result, updated nudge count.
Needs: decide

## Output

The arm completes with:
- action_taken: none | nudge-1 | nudge-2 | nudge-3 | killed | escalated
- result: success | failed | pending
- updated_state: New nudge count and timestamp for {{polecat_name}}

This data feeds back to the parent patrol's aggregate step.`,
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
git branch -D <polecat-branch>  # Local delete (branches never go to origin)
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
