# Hook, Sling, Handoff: Work Assignment Commands

> **Status**: Implemented
> **Updated**: 2024-12-24

## The Propulsion Principle (GUPP)

The Gastown Universal Propulsion Principle is simple:

> **If you find something on your hook, YOU RUN IT.**

No decisions. No "should I?" Just ignition. Agents execute what's on their
hook rather than deciding what to do.

```
Hook has work → Work happens.
```

That's the whole engine. Everything else is plumbing.

## The Command Menagerie

Three commands for putting work on hooks, each with distinct semantics:

| Command | Action | Context | Use Case |
|---------|--------|---------|----------|
| `gt hook <bead>` | Attach only | Preserved | Assign work for later |
| `gt sling <bead>` | Attach + run | Preserved | Kick off work immediately |
| `gt handoff <bead>` | Attach + restart | Fresh | Restart with new context |

### The Relationships

```
gt hook = pin (assign)
gt sling = hook + run now
gt handoff = hook + restart (fresh context via GUPP)
```

A hypothetical `gt run` lurks in the liminal UX space - it would be "start
working on the hooked item now" without the attach step. Currently implicit
in startup via GUPP.

## gt hook: Durability Primitive

```bash
gt hook <bead-id> [flags]
```

**What it does**: Attaches work to your hook. Nothing more.

**When to use**:
- You want to assign work but chat with the agent first
- Setting up work before triggering execution
- Preparing for a handoff without immediate restart

**Example**:
```bash
gt hook gt-abc                    # Attach issue
gt hook gt-abc -s "Context here"  # With subject for later handoff mail
```

The hook provides **durability** - the agent can restart, compact, or hand off,
but until the hook is changed or closed, that agent owns the work.

## gt sling: Hook and Run Now

```bash
gt sling <bead-id> [target] [flags]
```

**What it does**:
1. Attaches bead to the hook (durability)
2. Injects a prompt to start working NOW

**When to use**:
- You've been chatting with an agent and want to kick off a workflow
- You want to assign work to another agent that has useful context
- You (Overseer) want to start work then attend to another window
- Starting work without losing current conversation context

**Examples**:
```bash
gt sling gt-abc                       # Hook and start on it now
gt sling gt-abc -s "Fix the bug"      # With context subject
gt sling gt-abc crew                  # Sling to crew worker
gt sling gt-abc gastown/crew/max      # Sling to specific agent
```

**Key distinction from handoff**: Sling preserves context. The agent doesn't
restart - they receive an injected prompt and begin working with their current
conversation history intact.

## gt handoff: Hook and Restart

```bash
gt handoff [bead-or-role] [flags]
```

**What it does**:
1. If bead provided: attaches to hook first
2. Restarts the session (respawns pane)
3. New session wakes, finds hook, runs via GUPP

**When to use**:
- Context has become too large or stale
- You want a fresh session but with work continuity
- Handing off your own session before context limits
- Triggering restart on another agent's session

**Examples**:
```bash
gt handoff                          # Just restart (uses existing hook)
gt handoff gt-abc                   # Hook bead, then restart
gt handoff gt-abc -s "Fix it"       # Hook with context, then restart
gt handoff -s "Context" -m "Notes"  # Restart with handoff mail
gt handoff crew                     # Hand off crew session
gt handoff mayor                    # Hand off mayor session
```

**Interaction with roles**: The optional argument is polymorphic:
- If it looks like a bead ID (prefix `gt-`, `hq-`, `bd-`): hooks it
- Otherwise: treats it as a role to hand off

## Agent Lifecycle with Hooks

```
┌────────────────────────────────────────────────────────────────┐
│                    Agent Hook Lifecycle                         │
├────────────────────────────────────────────────────────────────┤
│                                                                  │
│  STARTUP (GUPP)                                                  │
│  ┌─────────────────┐                                            │
│  │ gt mol status   │ → hook has work? → RUN IT                  │
│  └─────────────────┘                                            │
│          │                                                       │
│          ▼                                                       │
│  ┌─────────────────┐                                            │
│  │   Work on it    │ ← agent executes molecule/bead             │
│  └─────────────────┘                                            │
│          │                                                       │
│          ▼                                                       │
│  ┌─────────────────┐                                            │
│  │  Complete/Exit  │ → close hook, check for next               │
│  └─────────────────┘                                            │
│                                                                  │
│  REASSIGNMENT (sling)                                           │
│  ┌─────────────────┐                                            │
│  │ gt sling <id>   │ → hook updates, prompt injected            │
│  └─────────────────┘                                            │
│          │                                                       │
│          ▼                                                       │
│  ┌─────────────────┐                                            │
│  │  Starts working │ ← preserves context                        │
│  └─────────────────┘                                            │
│                                                                  │
│  CONTEXT REFRESH (handoff)                                      │
│  ┌─────────────────┐                                            │
│  │gt handoff <id>  │ → hook updates, session restarts           │
│  └─────────────────┘                                            │
│          │                                                       │
│          ▼                                                       │
│  ┌─────────────────┐                                            │
│  │  Fresh context  │ ← GUPP kicks in on startup                 │
│  └─────────────────┘                                            │
│                                                                  │
└────────────────────────────────────────────────────────────────┘
```

## Command Quick Reference

```bash
# Just assign, don't start
gt hook gt-xyz

# Assign and start now (keep context)
gt sling gt-xyz
gt sling gt-xyz crew              # To another agent

# Assign and restart (fresh context)
gt handoff gt-xyz
gt handoff                        # Just restart, use existing hook

# Check what's on hook
gt mol status
```

## See Also

- `docs/propulsion-principle.md` - GUPP design
- `docs/wisp-architecture.md` - Ephemeral wisps for patrol loops
- `internal/wisp/` - Hook storage implementation
