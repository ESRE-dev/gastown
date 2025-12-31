# Convoys

Convoys are the primary unit for tracking batched work across rigs.

## Concept

A **convoy** is a persistent tracking unit that monitors related issues across
multiple rigs. When you kick off work - even a single issue - a convoy tracks it
so you can see when it lands and what was included.

```
                    Convoy (hq-abc)
                         │
            ┌────────────┼────────────┐
            │            │            │
            ▼            ▼            ▼
       ┌─────────┐  ┌─────────┐  ┌─────────┐
       │ gt-xyz  │  │ gt-def  │  │ bd-abc  │
       │ gastown │  │ gastown │  │  beads  │
       └────┬────┘  └────┬────┘  └────┬────┘
            │            │            │
            ▼            ▼            ▼
       ┌─────────┐  ┌─────────┐  ┌─────────┐
       │  nux    │  │ furiosa │  │  amber  │
       │(polecat)│  │(polecat)│  │(polecat)│
       └─────────┘  └─────────┘  └─────────┘
                         │
                    "the swarm"
                    (ephemeral)
```

## Convoy vs Swarm

| Concept | Persistent? | ID | Description |
|---------|-------------|-----|-------------|
| **Convoy** | Yes | hq-* | Tracking unit. What you create, track, get notified about. |
| **Swarm** | No | None | Ephemeral. "The workers currently on this convoy's issues." |

When you "kick off a swarm", you're really:
1. Creating a convoy (the tracking unit)
2. Assigning polecats to the tracked issues
3. The "swarm" is just those polecats while they're working

When issues close, the convoy lands and notifies you. The swarm dissolves.

## Convoy Lifecycle

```
OPEN ──(all issues close)──► LANDED/CLOSED
  ↑                              │
  └──(add more issues)───────────┘
       (auto-reopens)
```

| State | Description |
|-------|-------------|
| `open` | Active tracking, work in progress |
| `closed` | All tracked issues closed, notification sent |

Adding issues to a closed convoy reopens it automatically.

## Commands

### Create a Convoy

```bash
# Track multiple issues across rigs
gt convoy create "Deploy v2.0" gt-abc bd-xyz --notify gastown/joe

# Track a single issue (still creates convoy for dashboard visibility)
gt convoy create "Fix auth bug" gt-auth-fix

# With default notification (from config)
gt convoy create "Feature X" gt-a gt-b gt-c
```

### Add Issues

```bash
# Add to existing convoy
gt convoy add hq-abc gt-new-issue

# Adding to closed convoy reopens it
gt convoy add hq-abc gt-followup-fix
# → Convoy hq-abc reopened
```

### Check Status

```bash
# Show issues and active workers (the swarm)
gt convoy status hq-abc

# All active convoys (the dashboard)
gt convoy status
```

Example output:
```
Convoy: hq-abc (Deploy v2.0)
════════════════════════════

Progress: 3/5 complete

Issues
  ✓ gt-xyz: Update API              closed
  ✓ bd-abc: Fix validation          closed
  → bd-ghi: Update docs             in_progress  @beads/amber
  ○ gt-jkl: Deploy to prod          blocked by bd-ghi

Workers (the swarm)
  beads/amber     bd-ghi     running 12m
```

### List Convoys (Dashboard)

```bash
# Active convoys - the primary attention view
gt convoy list --active

# All convoys
gt convoy list

# JSON output
gt convoy list --json
```

## Notifications

When a convoy lands (all tracked issues closed), subscribers are notified:

```bash
# Explicit subscriber
gt convoy create "Feature X" gt-abc --notify gastown/joe

# Multiple subscribers
gt convoy create "Feature X" gt-abc --notify mayor/ --notify --human
```

Notification content:
```
📦 Convoy Landed: Deploy v2.0 (hq-abc)

Issues (3):
  ✓ gt-xyz: Update API endpoint
  ✓ gt-def: Add validation
  ✓ bd-abc: Update docs

Workers: gastown/nux, gastown/furiosa, beads/amber
Duration: 2h 15m
```

## Auto-Convoy on Sling

When you sling a single issue without an existing convoy:

```bash
gt sling bd-xyz beads/amber
```

This auto-creates a convoy so all work appears in the dashboard:
1. Creates convoy: "Work: bd-xyz"
2. Tracks the issue
3. Assigns the polecat

Even "swarm of one" gets convoy visibility.

## Cross-Rig Tracking

Convoys live in town-level beads (hq-* prefix) and can track issues from any rig:

```bash
# Track issues from multiple rigs
gt convoy create "Full-stack feature" \
  gt-frontend-abc \
  gt-backend-def \
  bd-docs-xyz
```

The `tracks` relation is:
- **Non-blocking**: doesn't affect issue workflow
- **Additive**: can add issues anytime
- **Cross-rig**: convoy in hq-*, issues in gt-*, bd-*, etc.

## Convoy vs Rig Status

| View | Scope | Shows |
|------|-------|-------|
| `gt convoy status [id]` | Cross-rig | Issues tracked by convoy + workers |
| `gt rig status <rig>` | Single rig | All workers in rig + their convoy membership |

Use convoys for "what's the status of this batch of work?"
Use rig status for "what's everyone in this rig working on?"

## See Also

- [Propulsion Principle](propulsion-principle.md) - Worker execution model
- [Mail Protocol](mail-protocol.md) - Notification delivery
