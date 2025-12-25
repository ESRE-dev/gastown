# The Propulsion Principle

> **Status**: Design document (experimental)
> **See also**: [sling-design.md](sling-design.md) for implementation details
> **See also**: [molecular-chemistry.md](molecular-chemistry.md) for the full Rig/Cook/Run lifecycle

## The Core Idea

We're trying a simple rule for agent behavior:

> **If you find something on your hook, YOU RUN IT.**

No decisions. No "should I?" No discretionary pondering. Just ignition.

```
Hook has work → Work happens.
```

That's the whole engine. Everything else is plumbing.

## Why It Works

The Propulsion Principle works because of three interlocking design decisions:

### 1. Agents Are Stateless

Agents have no memory between sessions. When a session starts, the agent has
no inherent knowledge of what it was doing before, what the current project
state is, or what decisions led to its existence.

This sounds like a limitation, but it's actually the foundation of resilience.
Stateless agents can:
- Be restarted at any time without data loss
- Survive context compaction (the agent re-reads its state)
- Hand off to new sessions seamlessly
- Recover from crashes without corruption

### 2. Work Is Molecule-Driven

All work in Gas Town follows the **Rig → Cook → Run** lifecycle:
- **Rig**: Compose workflow formulas (YAML source files)
- **Cook**: Transform formulas into executable protos (expand macros, apply aspects)
- **Run**: Agents execute the cooked workflow

A molecule (proto, mol, or wisp) defines:
- What steps need to happen
- What order they happen in (via dependencies)
- What each step should accomplish

The agent doesn't decide what to do. The molecule tells it. The agent's job is
execution, not planning. See [molecular-chemistry.md](molecular-chemistry.md)
for the full lifecycle.

### 3. Hooks Deliver Work

Work arrives on an agent's **hook** - a pinned molecule or assigned issue that
represents "your current work." When an agent wakes up:

1. Check the hook
2. Found something? **Execute it.**
3. Nothing? Check mail for new assignments.
4. Repeat.

The hook eliminates decision-making about what to work on. If it's on your hook,
it's your work. Run it.

## The Sling Lifecycle

The **sling** operation puts work on an agent's hook. This is the **Run** phase
of the Rig → Cook → Run lifecycle (formulas have already been cooked into protos):

```
┌─────────────────────────────────────────────────────────┐
│                    gt sling lifecycle                    │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  1. POUR (if proto)      2. ASSIGN           3. PIN     │
│     proto → mol/wisp        mol → agent         → hook  │
│                                                          │
│  ┌─────────┐            ┌─────────┐        ┌─────────┐  │
│  │  Proto  │ ────────►  │Mol/Wisp │ ─────► │  Hook   │  │
│  │(cooked) │   pour     │(instance)│ assign │(pinned) │  │
│  └─────────┘            └─────────┘        └─────────┘  │
│                                                  │       │
│                                            agent wakes   │
│                                                  │       │
│                                                  ▼       │
│                                            ┌─────────┐  │
│                                            │ IGNITION│  │
│                                            └─────────┘  │
└─────────────────────────────────────────────────────────┘
```

**Pour** instantiates a proto into a running mol (persistent) or wisp (ephemeral).
This is a phase transition from the Cook output to the Run phase.

**Key insight**: The agent never decides *whether* to run. The molecule tells
it *what* to do. It executes until complete, then checks the hook again.

## Agent Startup Protocol

Every agent follows the same startup protocol:

```bash
# 1. Check your hook
gt mol status                  # What's on my hook?

# 2. Found something?
# Output tells you exactly what to do.
# Follow the molecule phases.

# 3. Nothing on hook?
gt mail inbox                  # Check for new assignments

# 4. Repeat
```

**Old way** (too much thinking):
```bash
gt mail inbox
if has_molecule; then
    gt molecule instantiate ...
    # figure out what to do...
fi
```

**New way** (propulsion):
```bash
gt mol status              # What's on my hook?
# Just follow the molecule phases
```

The difference is profound: the old way requires the agent to understand its
situation and make decisions. The new way requires only execution.

## The Steam Engine Metaphor

Gas Town uses steam engine vocabulary throughout. The full lifecycle is
**Rig → Cook → Run**:

| Metaphor | Gas Town | Lifecycle Phase | Description |
|----------|----------|-----------------|-------------|
| **Recipe** | Formulas | Rig (source) | YAML files that compose workflows |
| **Fuel** | Proto molecules | Cook (artifact) | Cooked templates ready to instantiate |
| **Steam** | Wisps/Mols | Run (execution) | Active execution traces |
| **Distillate** | Digests | (post-Run) | Condensed permanent records |
| **Burn** | `bd mol burn` | (post-Run) | Discard without record |
| **Squash** | `bd mol squash` | (post-Run) | Compress into digest |

Claude is fire. Claude Code is a Steam engine. Gas Town is a Steam Train, with
Beads as the tracks. Wisps are steam vapors that dissipate after work is done.

The Propulsion Principle is the physics that makes the engine go:
**Hook has work → Work happens.**

## Examples

### Good: Following the Principle

```markdown
## Polecat Startup

1. Run `gt prime` to load context
2. Check `gt mol status` for pinned work
3. Found molecule? Execute each step in order.
4. Complete? Run `gt done` to submit to merge queue.
5. Request shutdown. You're ephemeral.
```

### Good: Witness Patrol

```markdown
## Witness Cycle

1. Bond a wisp molecule for this patrol cycle
2. Execute patrol steps (check polecats, check refinery)
3. Squash the wisp when done (creates digest)
4. Sleep until next cycle
5. Repeat forever
```

### Anti-Pattern: Decision Paralysis

```markdown
## DON'T DO THIS

1. Wake up
2. Think about what I should do...
3. Look at various issues and prioritize them
4. Decide which one seems most important
5. Start working on it
```

This violates propulsion. If there's nothing on your hook, you check mail or
wait. You don't go looking for work to decide to do. Work is *slung* at you.

### Anti-Pattern: Ignoring the Hook

```markdown
## DON'T DO THIS

1. Wake up
2. See molecule on my hook
3. But I notice a more interesting issue over there...
4. Work on that instead
```

If it's on your hook, you run it. Period. The hook is not a suggestion.

### Anti-Pattern: Partial Execution

```markdown
## DON'T DO THIS

1. Wake up
2. See molecule with 5 steps
3. Complete step 1
4. Get bored, request shutdown
5. Leave steps 2-5 incomplete
```

Molecules are executed to completion. If you can't finish, you squash or burn
explicitly. You don't just abandon mid-flight.

## Why Not Just Use TODO Lists?

LLM agents have short memories. A TODO list in the prompt will be forgotten
during context compaction. A molecule in beads survives indefinitely because
it's stored outside the agent's context.

**Molecules are external TODO lists that persist across sessions.**

This is the secret to autonomous operation: the agent's instructions survive
the agent's death. New agent, same molecule, same progress point.

## Relationship to Nondeterministic Idempotence

The Propulsion Principle enables **nondeterministic idempotence** - the property
that any workflow will eventually complete correctly, regardless of which agent
runs which step, and regardless of crashes or restarts.

| Property | How Propulsion Enables It |
|----------|---------------------------|
| **Deterministic structure** | Molecules define exact steps |
| **Nondeterministic execution** | Any agent can run any ready step |
| **Idempotent progress** | Completed steps stay completed |
| **Crash recovery** | Agent dies, molecule persists |
| **Session survival** | Restart = re-read hook = continue |

## Implementation Status

The Propulsion Principle is implemented via three commands:

| Command | Action | Context | Use Case |
|---------|--------|---------|----------|
| `gt hook <bead>` | Attach only | Preserved | Assign work for later |
| `gt sling <bead>` | Attach + run | Preserved | Kick off work immediately |
| `gt handoff <bead>` | Attach + restart | Fresh | Restart with new context |

The algebra:
```
gt sling = gt hook + gt nudge "start working"
gt handoff = gt hook + restart (GUPP kicks in)
```

See [sling-design.md](sling-design.md) for detailed command reference.

## Summary

1. **One rule**: If you find something on your hook, you run it.
2. **Stateless agents**: No memory between sessions - molecules provide continuity.
3. **Molecule-driven**: Work is defined by molecules, not agent decisions.
4. **Hook delivery**: Work arrives via sling, sits on hook until complete.
5. **Just execute**: No thinking about whether. Only doing.

The Propulsion Principle is what makes Gas Town work as an autonomous,
distributed, crash-resilient execution engine. It's the physics of the steam
train.

```
Hook has work → Work happens.
```
