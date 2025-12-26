# Witness Patrol Design

> The Witness is the Pit Boss. Oversight, not implementation.

## Core Responsibilities

| Duty | Action |
|------|--------|
| Handle POLECAT_DONE | Create cleanup wisp, process later |
| Handle HELP requests | Assess, help or escalate to Mayor |
| Ensure refinery alive | Restart if needed |
| Survey workers | Detect stuck polecats, nudge or escalate |
| Process cleanups | Verify git clean, kill session, burn wisp |

## Patrol Shape (Linear)

```
inbox-check → process-cleanups → check-refinery → survey-workers → context-check → loop
```

No dynamic arms. No fanout gates. Simple loop like Deacon.

## Key Design Principles

| Principle | Meaning |
|-----------|---------|
| **Discovery over tracking** | Observe reality each cycle, don't maintain state |
| **Events over state** | POLECAT_DONE triggers wisps, not queue updates |
| **Cleanup wisps as finalizers** | Pending cleanup = wisp exists |
| **Task tool for parallelism** | Subagents inspect polecats, not molecule arms |
| **Fresh judgment each cycle** | No persistent nudge counters |

## Cleanup: The Finalizer Pattern

```
POLECAT_DONE arrives
       ↓
Create wisp: bd create --wisp --title "cleanup:<polecat>" --labels cleanup
       ↓
(wisp exists = cleanup pending)
       ↓
Witness process-cleanups step:
  - Verify: git status clean, no unpushed, issue closed
  - Execute: gt session kill, worktree removed
  - Burn wisp
       ↓
Failed? Leave wisp, retry next cycle
```

## Assessing Stuck Polecats

With step-based restarts, polecats are either:
- **Working a step**: Active tool calls, progress
- **Starting a step**: Just respawned, reading hook
- **Stuck on a step**: No progress, same step for multiple cycles

| Observation | Action |
|-------------|--------|
| Active tool calls | None |
| Just started step (<5 min) | None |
| Idle 5-15 min, same step | Gentle nudge |
| Idle 15+ min, same step | Direct nudge |
| Idle 30+ min despite nudges | Escalate to Mayor |
| Errors visible | Assess, help or escalate |
| Says "done" but no POLECAT_DONE | Nudge to signal completion |

**No persistent nudge counts**. Each cycle: observe reality, make fresh judgment.

"How long stuck on same step" is discoverable from beads timestamps.

## Parallelism via Task Tool

Inspect multiple polecats concurrently using subagents:

```markdown
## survey-workers step

For each polecat, launch Task tool subagent:
- Capture tmux output
- Assess state (working/idle/error/done)
- Check beads for step progress
- Decide and execute action

Task tool handles parallelism. One subagent per polecat.
```

## Formula

See `.beads/formulas/mol-witness-patrol.formula.toml`

## Related

- [polecat-lifecycle.md](polecat-lifecycle.md) - Step-based execution model
- [molecular-chemistry.md](molecular-chemistry.md) - MEOW stack
