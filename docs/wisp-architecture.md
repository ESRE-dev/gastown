# Wisp Architecture: Simplified

> Status: Updated December 2024 - Simplified from separate directory to flag-based

## Overview

**Wisps** are ephemeral issues - transient workflow state that should not be synced
to the shared repository. They're used for operational messages (like lifecycle mail)
and patrol cycle traces that would otherwise accumulate unbounded.

## Core Principle

**Wisps are regular issues with `Wisp: true` flag.**

The old architecture used a separate `.beads-wisp/` directory. This was over-engineered.
The simplified approach:

| Old | New |
|-----|-----|
| `.beads-wisp/issues.jsonl` | `.beads/issues.jsonl` with `Wisp: true` |
| Separate directory, git init, gitignore | Single database, filtered on sync |
| Complex dual-inbox routing | Simple flag check |

## How It Works

### Creating Wisps

```bash
# Create an ephemeral issue
bd create --title "Patrol cycle" --wisp

# Send ephemeral mail (automatically sets Wisp=true)
gt mail send --wisp -s "Lifecycle: spawn" -m "..."
```

### Sync Filtering

When `bd sync` exports to JSONL for git:
- Issues with `Wisp: true` are **excluded**
- Only permanent issues are synced to remote
- No separate directory needed

### Querying

```bash
# List all issues (including wisps)
bd list

# List only wisps
bd list --wisp

# List only permanent issues
bd list --no-wisp
```

## Use Cases

### Ephemeral Mail (Lifecycle Messages)

Spawn notifications, session handoffs, and other operational messages that:
- Don't need to be synced to remote
- Would accumulate unbounded
- Have no long-term audit value

```bash
gt mail send gastown/polecats/nux --wisp \
  -s "LIFECYCLE: spawn" \
  -m "Work on issue gt-abc"
```

### Patrol Cycle Traces

Deacon, Witness, and Refinery run continuous loops. Each cycle would create
accumulating history. Wisps let them track cycle state without permanent records.

### Hook Files

Agent hook files (`hook-<agent>.json`) are stored in `.beads/` but are local-only
runtime state, not synced. These track what work is assigned to each agent for
restart-and-resume.

## Wisp Garbage Collection

Wisps can accumulate over time. The `wisp-gc` doctor check cleans up stale wisps:

```bash
# Check for stale wisps
gt doctor -v

# Auto-cleanup stale wisps (>1h old by default)
gt doctor --fix

# Manual cleanup via bd
bd wisp gc              # Preview what would be cleaned
bd wisp gc --force      # Actually delete stale wisps
bd wisp gc --age 2h     # Custom age threshold
```

The Deacon includes wisp-gc in its patrol cycle via `gt doctor --fix`.

### What Gets Cleaned

- Wisps older than the threshold (default 1 hour)
- Wisps not associated with active sessions
- Orphaned lifecycle mail

### What's Preserved

- Wisps with active sessions still running
- Wisps younger than the threshold
- Non-wisp issues (permanent records)

## Decision Matrix

| Question | Answer | Use |
|----------|--------|-----|
| Should this sync to remote? | No | Wisp |
| Is this operational/lifecycle? | Yes | Wisp |
| Would this accumulate unbounded? | Yes | Wisp |
| Does this need audit trail? | Yes | Regular issue |
| Might others need to see this? | Yes | Regular issue |

## Migration from .beads-wisp/

The old `.beads-wisp/` directories can be deleted:

```bash
# Remove legacy wisp directories
rm -rf ~/gt/.beads-wisp/
rm -rf ~/gt/gastown/.beads-wisp/
find ~/gt -type d -name '.beads-wisp' -exec rm -rf {} +
```

No migration needed - these contained transient data with no long-term value.

## Related

- [molecules.md](molecules.md) - Molecule system (wisps can be molecule instances)
- [architecture.md](architecture.md) - Overall Gas Town architecture
