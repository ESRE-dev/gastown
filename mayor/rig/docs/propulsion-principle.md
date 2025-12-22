# The Universal Gas Town Propulsion Principle

> How stateless agents do work: one rule to drive them all.

## The One Rule

```
IF your startup hook finds work → THEN you do the work.
```

That's it. Every agent in Gas Town follows this single principle. The startup hook
checks for attached molecules (work to do), and if found, the agent executes.

There is no scheduler telling agents what to do. No central orchestrator dispatching
tasks. No message queues with pending work. Just: **hook finds work → work happens.**

## Why This Works

### 1. Stateless Agents

Gas Town agents are **stateless** - they don't remember what they were doing. Every
session starts fresh. But work state isn't in agent memory; it's in **Beads**.

```
Agent Session 1:
  - Starts fresh
  - Reads Beads: "design step completed, implement step in_progress"
  - Continues implement step
  - Crashes mid-work

Agent Session 2:
  - Starts fresh (no memory of Session 1)
  - Reads Beads: "design step completed, implement step in_progress"
  - Continues implement step from wherever it was
  - Completes successfully
```

No work is lost. No coordination needed. The agent doesn't even know it crashed.

### 2. Molecule-Driven Execution

Agents don't "work on issues" - they **execute molecules**. A molecule is a
crystallized workflow: a DAG of steps with dependencies, quality gates, and
completion criteria.

```markdown
## Molecule: engineer-in-box

## Step: design
Think about architecture. Write a brief design summary.

## Step: implement
Write the code. Follow codebase conventions.
Needs: design

## Step: test
Write and run tests. Cover edge cases.
Needs: implement

## Step: submit
Submit for merge via refinery.
Needs: test
```

The molecule defines what work means. The agent just follows the DAG.

### 3. Beads as Control Plane

Gas Town intentionally blurs data plane and control plane. In traditional systems:
- **Data plane**: Stores information
- **Control plane**: Coordinates behavior

In Gas Town, **the control state IS data in Beads**:
- Molecule steps are beads issues
- Step status is issue status
- Dependencies are beads edges
- Agent state is assignment to issues

Agents read Beads to know what to do. There's no separate orchestrator.

## The Sling Lifecycle

Agents follow a **sling lifecycle**: spawn → attach → execute → burn.

```
                 ┌──────────────┐
                 │   SPAWN      │   Agent session created
                 └──────┬───────┘
                        │
                        ▼
                 ┌──────────────┐
                 │   ATTACH     │   Molecule bound to agent's pinned bead
                 └──────┬───────┘
                        │
                        ▼
          ┌─────────────────────────┐
          │        EXECUTE          │   Work through DAG steps
          │  (survives restarts)    │   Each step: claim → work → close
          └─────────────┬───────────┘
                        │
                        ▼
                 ┌──────────────┐
                 │    BURN      │   Molecule completes, detaches
                 └──────┬───────┘
                        │
            ┌───────────┴───────────┐
            ▼                       ▼
     ┌─────────────┐         ┌─────────────┐
     │   SQUASH    │         │   REPEAT    │
     │  (archive)  │         │  (patrol)   │
     └─────────────┘         └─────────────┘
```

**Key properties:**
- **ATTACH**: Work is bound to the agent via a pinned bead
- **EXECUTE**: Any restart resumes from last completed step
- **BURN**: Molecule is "consumed" - work is done
- **SQUASH**: Compress execution trace into permanent digest

The "sling" metaphor: agents are flung into work, execute their arc, and land.

## Agent Startup Protocol

Every agent follows the same startup protocol:

```bash
# 1. Load context
gt prime                    # Load role context, check mail

# 2. Check for attached molecule
bd list --status=in_progress --assignee=<self>

# 3. If attached: resume from current step
bd ready                    # Find next step to work on

# 4. If not attached: wait for work or spawn new molecule
# (Patrol agents bond a new patrol molecule)
# (Polecats wait for assignment)
```

### What `gt prime` Does

The `gt prime` command is the propulsion trigger:

1. **Load CLAUDE.md** - Role-specific context and instructions
2. **Check inbox** - Any mail waiting for this agent?
3. **Show handoff** - Display any pending handoff from previous session
4. **Signal ready** - Agent is primed and ready to work

After `gt prime`, the agent checks for attached work. If found, propulsion begins.

### The Pinned Bead

Each agent has a **pinned bead** - a personal handoff message that persists across
sessions. The pinned bead can have an **attached molecule**:

```json
{
  "id": "hq-polecat-nux-pinned",
  "type": "handoff",
  "title": "Polecat Nux Handoff",
  "attached_molecule": "gt-abc123.exec-001"
}
```

The rule is simple:

```
IF attached_molecule IS NOT NULL:
    YOU MUST EXECUTE IT
```

This is the propulsion contract. The attachment stays until the molecule burns.

## Propulsion Patterns

### Pattern: Polecat Work

Polecats are **ephemeral** - spawned for one molecule, deleted when done.

```
1. gt spawn --issue gt-xyz --molecule mol-engineer-in-box
2. Polecat session created
3. Molecule bonded, attached to polecat's pinned bead
4. Polecat wakes, runs gt prime
5. Startup hook finds attached molecule
6. Polecat executes molecule steps
7. Molecule burns, polecat requests shutdown
8. Witness kills polecat, removes worktree
```

**Propulsion trigger**: The spawned session has work attached. Hook fires, work happens.

### Pattern: Patrol Loop

Patrol agents (Deacon, Witness) run **continuous loops**:

```
1. Daemon pokes Deacon (heartbeat)
2. Deacon wakes, runs gt prime
3. Startup hook checks for attached molecule
4. If none: bond new mol-deacon-patrol
5. Execute patrol steps (inbox, health, gc, etc.)
6. Molecule burns
7. If context low: immediately bond new patrol, goto 5
8. If context high: exit (daemon will respawn)
```

**Propulsion trigger**: Daemon heartbeat + no attached molecule = bond new patrol.

### Pattern: Quiescent Wake

Some agents (Witness, Refinery) go quiescent when idle:

```
1. Witness finishes work, no polecats active
2. Witness burns molecule, goes quiescent (session killed)
3. Later: gt spawn in this rig
4. Daemon detects trigger, wakes Witness
5. Witness runs gt prime
6. Startup hook finds work (new spawn request)
7. Witness bonds patrol molecule, executes
```

**Propulsion trigger**: External event (spawn) → wake → hook finds work.

## Anti-Patterns

### Anti-Pattern: Relying on Memory

❌ **Wrong**: Agent remembers what it was doing
```
Agent: "I was working on the auth feature..."
       (Crash. Memory lost. Work unknown.)
```

✅ **Right**: Agent reads state from Beads
```
Agent: "Let me check my attached molecule..."
       bd show gt-xyz.exec-001
       "Step 3 of 5 is in_progress. Resuming."
```

### Anti-Pattern: Central Dispatch

❌ **Wrong**: Scheduler tells agents what to do
```
Scheduler: "Agent 1, do task A. Agent 2, do task B."
           (Scheduler crashes. All coordination lost.)
```

✅ **Right**: Agents find their own work
```
Agent: "What's attached to my pinned bead?"
       "Nothing. What's ready to claim?"
       bd ready
       "gt-xyz is ready. Claiming it."
```

### Anti-Pattern: Work Queues

❌ **Wrong**: Pull work from a message queue
```
Agent: (pulls from RabbitMQ)
       (MQ crashes. Messages lost. Work duplicated.)
```

✅ **Right**: Work state is in Beads
```
Agent: "What molecules are in_progress assigned to me?"
       "gt-xyz.exec-001. Resuming step 3."
       (Agent crashes. Restarts. Same query. Same answer.)
```

### Anti-Pattern: Idle Polling

❌ **Wrong**: Agent polls for work in a tight loop
```
while True:
    work = check_for_work()
    if work:
        do_work(work)
    sleep(1)  # Wasteful, burns context
```

✅ **Right**: Event-driven wake + hook
```
# Agent is quiescent (no session)
# External event triggers wake
# Startup hook finds attached work
# Agent executes
# Agent goes quiescent again
```

## Nondeterministic Idempotence

The propulsion principle enables **nondeterministic idempotence**:

- **Deterministic structure**: Molecule defines exactly what steps exist
- **Nondeterministic execution**: Any agent can execute any ready step
- **Idempotent progress**: Completed steps stay completed, re-entry is safe

```
Time 0: Worker A starts design step
Time 1: Worker A completes design
Time 2: Worker A starts implement step
Time 3: Worker A crashes
Time 4: Worker B wakes up
Time 5: Worker B queries ready work
Time 6: Worker B sees implement is ready (design done, implement pending)
Time 7: Worker B continues implement step
Time 8: Worker B completes implement
```

No coordination needed. No handoff protocol. Just: **hook finds work → work happens.**

## Summary

The Universal Gas Town Propulsion Principle:

1. **One Rule**: Hook finds work → work happens
2. **Stateless Agents**: State lives in Beads, not memory
3. **Molecule-Driven**: Agents execute DAGs, not instructions
4. **Sling Lifecycle**: Spawn → Attach → Execute → Burn
5. **Startup Protocol**: `gt prime` → check attachment → execute or wait
6. **Nondeterministic Idempotence**: Any agent can continue any molecule

The propulsion principle is what makes autonomous operation possible. Agents don't
need to coordinate, remember, or wait for instructions. They just check their hook
and go.

---

*"The best architecture is invisible. Agents don't coordinate - they just work."*
