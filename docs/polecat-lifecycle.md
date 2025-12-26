# Polecat Lifecycle

> Polecats restart after each molecule step. This is intentional.

## Execution Model

| Phase | What Happens |
|-------|--------------|
| **Spawn** | Worktree created, session started, molecule slung to hook |
| **Step** | Polecat reads hook, executes ONE step, runs `gt mol step done` |
| **Restart** | Session respawns with fresh context, next step on hook |
| **Complete** | Last step done → POLECAT_DONE mail → cleanup wisp created |
| **Cleanup** | Witness verifies git clean, kills session, burns wisp |

```
spawn → step → restart → step → restart → ... → complete → cleanup
         └──────────────────────────────────────┘
              (fresh session each step)
```

## Why Restart Every Step?

| Reason | Explanation |
|--------|-------------|
| **Atomicity** | Each step completes fully or not at all |
| **No wandering** | Polecat can't half-finish and get distracted |
| **Context fresh** | No accumulation of stale context across steps |
| **Crash recovery** | Restart = re-read hook = continue from last completed step |

**Trade-off**: Session restart overhead. Worth it for reliability at current cognition levels.

## Step Packing (Author Responsibility)

Formula authors must size steps appropriately:

| Too Small | Too Large |
|-----------|-----------|
| Restart overhead dominates | Context exhaustion mid-step |
| Thrashing | Partial completion, unreliable |

**Rule of thumb**: A step should use 30-70% of available context. Batch related micro-tasks.

## The `gt mol step done` Command

Canonical way to complete a step:

```bash
gt mol step done <step-id>
```

1. Closes the step in beads
2. Finds next ready step (dependency-aware)
3. Updates hook to next step
4. Respawns pane with fresh session

**Never use `bd close` directly** - it skips the restart logic.

## Cleanup: The Finalizer Pattern

When polecat signals completion:

```
POLECAT_DONE mail → Witness creates cleanup wisp → Witness processes wisp → Burn
```

The wisp's existence IS the pending cleanup. No explicit queue.

| Cleanup Step | Verification |
|--------------|--------------|
| Git status | Must be clean |
| Unpushed commits | None allowed |
| Issue state | Closed or deferred |
| Productive work | Commits reference issue (ZFC - Witness judges) |

Failed cleanup? Leave wisp, retry next cycle.

---

## Evolution Path

Current design will evolve as model cognition improves:

| Phase | Refresh Trigger | Who Decides | Witness Load |
|-------|-----------------|-------------|--------------|
| **Now** | Step boundary | Formula (fixed) | High |
| **Spoon-feeding** | Context % + task size | Witness | Medium |
| **Self-managed** | Self-awareness | Polecat | Low |

### Now (Step-Based Restart)

- Restart every step, guaranteed
- Conservative, reliable
- `gt mol step done` handles everything

### Spoon-feeding (Future)

Requires: Claude Code exposes context usage

```
Polecat completes step
  → Witness checks: 65% context used
  → Next task estimate: 10% context
  → Decision: "send another" or "recycle"
```

Witness becomes supervisor, not babysitter.

### Self-Managed (Future)

Requires: Model cognition threshold + Gas Town patterns in training

```
Polecat completes step
  → Self-assesses: "I'm at 80%, should recycle"
  → Runs gt handoff, respawns
```

Polecats become autonomous. Witness becomes auditor.

---

## Key Commands

| Command | Effect |
|---------|--------|
| `gt mol step done <step>` | Complete step, restart for next |
| `gt mol status` | Show what's on hook |
| `gt mol progress <mol>` | Show molecule completion state |
| `gt done` | Signal POLECAT_DONE to Witness |
| `gt handoff` | Write notes, respawn (manual refresh) |
