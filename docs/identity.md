# Agent Identity and Attribution

> Canonical format for agent identity in Gas Town

## BD_ACTOR Format Convention

The `BD_ACTOR` environment variable identifies agents in slash-separated path format.
This is set automatically when agents are spawned and used for all attribution.

### Format by Role Type

| Role Type | Format | Example |
|-----------|--------|---------|
| **Mayor** | `mayor` | `mayor` |
| **Deacon** | `deacon` | `deacon` |
| **Witness** | `{rig}/witness` | `gastown/witness` |
| **Refinery** | `{rig}/refinery` | `gastown/refinery` |
| **Crew** | `{rig}/crew/{name}` | `gastown/crew/joe` |
| **Polecat** | `{rig}/polecats/{name}` | `gastown/polecats/toast` |

### Why Slashes?

The slash format mirrors filesystem paths and enables:
- Hierarchical parsing (extract rig, role, name)
- Consistent mail addressing (`gt mail send gastown/witness`)
- Path-like routing in beads operations
- Visual clarity about agent location

## Attribution Model

Gas Town uses three fields for complete provenance:

### Git Commits

```bash
GIT_AUTHOR_NAME="gastown/crew/joe"      # Who did the work (agent)
GIT_AUTHOR_EMAIL="steve@example.com"    # Who owns the work (overseer)
```

Result in git log:
```
abc123 Fix bug (gastown/crew/joe <steve@example.com>)
```

**Interpretation**:
- The agent `gastown/crew/joe` authored the change
- The work belongs to the workspace owner (`steve@example.com`)
- Both are preserved in git history forever

### Beads Records

```json
{
  "id": "gt-xyz",
  "created_by": "gastown/crew/joe",
  "updated_by": "gastown/witness"
}
```

The `created_by` field is populated from `BD_ACTOR` when creating beads.
The `updated_by` field tracks who last modified the record.

### Event Logging

All events include actor attribution:

```json
{
  "ts": "2025-01-15T10:30:00Z",
  "type": "sling",
  "actor": "gastown/crew/joe",
  "payload": { "bead": "gt-xyz", "target": "gastown/polecats/toast" }
}
```

## Environment Setup

The daemon sets these automatically when spawning agents:

```bash
# Set by daemon for polecat 'toast' in rig 'gastown'
export BD_ACTOR="gastown/polecats/toast"
export GIT_AUTHOR_NAME="gastown/polecats/toast"
export GT_ROLE="polecat"
export GT_RIG="gastown"
export GT_POLECAT="toast"
```

### Manual Override

For local testing or debugging:

```bash
export BD_ACTOR="gastown/crew/debug"
bd create --title="Test issue"  # Will show created_by: gastown/crew/debug
```

## Identity Parsing

The format supports programmatic parsing:

```go
// identityToBDActor converts daemon identity to BD_ACTOR format
// Town level: mayor, deacon
// Rig level: {rig}/witness, {rig}/refinery
// Workers: {rig}/crew/{name}, {rig}/polecats/{name}
```

| Input | Parsed Components |
|-------|-------------------|
| `mayor` | role=mayor |
| `deacon` | role=deacon |
| `gastown/witness` | rig=gastown, role=witness |
| `gastown/refinery` | rig=gastown, role=refinery |
| `gastown/crew/joe` | rig=gastown, role=crew, name=joe |
| `gastown/polecats/toast` | rig=gastown, role=polecat, name=toast |

## Audit Queries

Attribution enables powerful audit queries:

```bash
# All work by an agent
bd audit --actor=gastown/crew/joe

# All work in a rig
bd audit --actor=gastown/*

# All polecat work
bd audit --actor=*/polecats/*

# Git history by agent
git log --author="gastown/crew/joe"
```

## Design Principles

1. **Agents are not anonymous** - Every action is attributed
2. **Work is owned, not authored** - Agent creates, overseer owns
3. **Attribution is permanent** - Git commits preserve history
4. **Format is parseable** - Enables programmatic analysis
5. **Consistent across systems** - Same format in git, beads, events
