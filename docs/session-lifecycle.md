# Session Lifecycle: Context Cycling in Gas Town

> **Status**: Foundational Architecture
> **See also**: [beads-data-plane.md](beads-data-plane.md), [propulsion-principle.md](propulsion-principle.md)

## Overview

Gas Town agents have persistent identities, but their sessions are ephemeral. This is the
"cattle not pets" model from Kubernetes: the agent (Mayor, Witness, polecat Toast) is
the persistent identity; the Claude Code session running that agent is disposable cattle
that can be killed and respawned at any time.

Work persists in beads. Sessions come and go. This document explains the unified model
for session lifecycle across all roles.

## The Single-Bond Principle

> A bead should be achievable in a single typical Claude Code session.

In molecular chemistry, a single bond connects exactly two atoms. In Gas Town, a single
session should complete exactly one bead. This is the atomic unit of efficient work.

**Why this matters:**
- Clean execution: start session → do work → cycle
- No mid-work state to track outside beads
- Handoffs are clean boundaries, not messy interruptions
- The system stays efficient as it scales

**The ceiling rises over time.** What fits in "one session" grows with model capability.
Opus 4.5 can handle larger beads than earlier models. But the principle remains: if a
bead won't fit, decompose it.

### Violating the Principle

You can violate the Single-Bond Principle. Gas Town is flexible. But violations trigger
the **Happy Path** or **Sad Path**:

**Happy Path** (controlled):
1. Worker recognizes mid-work: "This won't fit in one session"
2. Choose:
   - (a) Partial implementation → handoff with context notes → successor continues
   - (b) Self-decompose: convert bead to epic, create sub-tasks, continue with first task
3. If scope grew substantially → notify Witness/Mayor/Human for rescheduling

**Sad Path** (uncontrolled):
- Worker underestimates → context fills unexpectedly → compaction triggers
- Compaction is the least desirable handoff: sudden, no context notes, state may be unclear
- Successor must reconstruct context from beads and git state

The Happy Path preserves continuity. The Sad Path forces recovery.

## The Context Budget Model

**N is not "sessions" or "rounds" - it's a proxy for context budget.**

Every action consumes context:

| Action Type | Context Cost | Examples |
|-------------|--------------|----------|
| Boring | ~1-5% | Health ping, empty inbox check, clean rebase |
| Interesting | 20-50%+ | Conflict resolution, debugging, implementation |

The heuristics (N=20 for Deacon, N=1 for Polecat) estimate "when will we hit ~80% context?"
They're tuning parameters, not laws of physics.

### Why "Interesting" Triggers Immediate Handoff

An "interesting" event consumes context like a full bead:
- Reading conflict diffs
- Debugging test failures
- Making implementation decisions
- Writing substantial code

After an interesting event, you've used your context budget. Fresh session handles
the next event better than a session at 80% capacity.

## Unified Session Cycling Model

All workers use the same model. Only the heuristic differs:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         SESSION LIFECYCLE                               │
│                                                                         │
│   ┌──────────┐    ┌──────────────┐    ┌─────────────┐    ┌──────────┐  │
│   │  Start   │───▶│ Execute Step │───▶│ Check Budget│───▶│ Handoff  │  │
│   │ Session  │    │              │    │             │    │ or Done  │  │
│   └──────────┘    └──────┬───────┘    └──────┬──────┘    └──────────┘  │
│                          │                   │                         │
│                          │    ┌──────────────┘                         │
│                          │    │                                        │
│                          ▼    ▼                                        │
│                    Budget OK? ──Yes──▶ Loop to next step               │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Per-Role Heuristics

| Role | N | Unit | "Interesting" Trigger |
|------|---|------|----------------------|
| **Polecat** | 1 | assignment | Every assignment is interesting |
| **Crew** | ∞ | human decides | Context full or human requests |
| **Deacon** | 20 | patrol rounds | Lifecycle request, remediation, escalation |
| **Witness** | 15 | polecat interactions | (interactions are the cost unit) |
| **Refinery** | 20 | simple MRs | Complex rebase, conflict resolution |

**Polecat (N=1)**: Every assignment fills context with implementation details. There's
no "boring" polecat work - it's all interesting. Forced to cycle when work is complete
and merged.

**Crew (N=∞)**: Human-managed. Cycle when it feels right, or when context fills naturally.

**Patrol Workers (N=15-20)**: Routine checks are cheap. Batch many before cycling. But
any "interesting" event resets to immediate handoff.

## Mid-Step Handoff

Any worker can handoff mid-step. The molecule tracks state:

```
Step 3 of 7, context filling
         │
         ▼
    gt handoff -s "Context notes" -m "Details..."
         │
         ▼
    Session dies, successor spawns
         │
         ▼
    gt mol status → sees step 3 open → continues
```

This is normal and supported. The Single-Bond Principle is aspirational, not mandatory.
Mid-step handoff is the Happy Path when a bead turns out larger than estimated.

## Handoff Mechanics

How handoff works depends on the role:

### Polecats: Outer Ring Handoff

Polecats can't self-respawn - they need cleanup (worktree removal). They go through
their Witness:

```
Polecat                          Witness
   │                                │
   │ gt done (or gt handoff)        │
   │                                │
   ▼                                │
Send LIFECYCLE mail ──────────────▶ │
   │                                │
   │ (wait for termination)         ▼
   │                          Patrol inbox-check step
   │                                │
   │                                ▼
   │                          Process LIFECYCLE request
   │                                │
   ◀─────────────────────────────── Kill session, cleanup worktree
```

### Non-Polecats: Self-Respawn

Crew, Mayor, Witness, Refinery, Deacon are persistent. They respawn themselves:

```
Agent
   │
   │ gt handoff -s "Context" -m "Notes"
   │
   ▼
Send handoff mail to self (optional)
   │
   ▼
tmux respawn-pane -k (kills self, starts fresh claude)
   │
   ▼
New session runs gt prime, finds hook, continues
```

No outer ring needed - they just restart in place.

## State Persistence

What survives a handoff:

| Persists | Where | Example |
|----------|-------|---------|
| Pinned molecule | Beads (rig) | What you're working on |
| Handoff mail | Beads (town) | Context notes for successor |
| Git commits | Git | Code changes |
| Issue state | Beads | Open/closed, assignee, etc. |

What doesn't survive:

| Lost | Why |
|------|-----|
| Claude context | Session dies |
| In-memory state | Process dies |
| Uncommitted changes | Not in git |
| Unflushed beads | Not synced |

**Key insight**: The molecule is the source of truth for work. Handoff mail is
supplementary context. You could lose all handoff mail and still know what to
work on from your hook.

## The Oversized Bead Protocol

When you realize mid-work that a bead won't fit in one session:

### Option A: Partial Implementation + Handoff

1. Commit what you have (even if incomplete)
2. Update bead description with progress notes
3. `gt handoff -s "Partial impl" -m "Completed X, remaining: Y, Z"`
4. Successor continues from your checkpoint

Best when: Work is linear, successor can pick up where you left off.

### Option B: Self-Decomposition

1. Convert current bead to epic (if not already)
2. Create sub-task beads for remaining work
3. Close or update original with "decomposed into X, Y, Z"
4. Continue with first sub-task

Best when: Work has natural breakpoints, parallelization possible.

### When to Escalate

Notify Witness/Mayor/Human when:
- Scope grew >2x from original estimate
- Decomposition affects other scheduled work
- Blockers require external input
- You're unsure which option to choose

```bash
# Escalate to Witness (polecat)
gt mail send <rig>/witness -s "Scope change: <bead-id>" -m "..."

# Escalate to Mayor (cross-rig)
gt mail send mayor/ -s "Scope change: <bead-id>" -m "..."

# Escalate to Human
gt mail send --human -s "Need input: <bead-id>" -m "..."
```

## Summary

1. **Single-Bond Principle**: One bead ≈ one session (aspirational)
2. **Context Budget**: N-heuristics proxy for "when will context fill?"
3. **Unified Model**: All roles cycle the same way, different heuristics
4. **Mid-Step Handoff**: Normal and supported via persistent molecules
5. **Happy Path**: Recognize early, decompose or partial-handoff
6. **Sad Path**: Compaction - recover from beads + git state

The system is designed for ephemeral sessions with persistent state. Embrace cycling.
Fresh context handles problems better than stale context at capacity.

---

## Related Documents

- [beads-data-plane.md](beads-data-plane.md) - How state persists in beads
- [propulsion-principle.md](propulsion-principle.md) - The "RUN IT" protocol
- [pinned-beads-design.md](pinned-beads-design.md) - Hook mechanics
- [wisp-architecture.md](wisp-architecture.md) - Ephemeral patrol molecules
