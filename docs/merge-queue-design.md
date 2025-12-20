# Merge Queue Design

The merge queue is the coordination mechanism for landing completed work. It's implemented entirely in Beads - merge requests are just another issue type with dependencies.

**Key insight**: Git is already a ledger. Beads is already federated. The merge queue is just a query pattern over beads issues.

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         MERGE QUEUE                              │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │  MR #1   │→ │  MR #2   │→ │  MR #3   │→ │  MR #4   │        │
│  │ (ready)  │  │(blocked) │  │ (ready)  │  │(blocked) │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
│       ↓                           ↓                             │
│  ┌─────────────────────────────────────────────────────┐       │
│  │              ENGINEER (processes queue)              │       │
│  │  1. bd ready --type=merge-request                    │       │
│  │  2. Process in priority order                        │       │
│  │  3. Merge, test, close or reject                     │       │
│  └─────────────────────────────────────────────────────┘       │
│                           ↓                                     │
│                     ┌──────────┐                                │
│                     │   main   │                                │
│                     └──────────┘                                │
└─────────────────────────────────────────────────────────────────┘
```

## Merge Request Schema

A merge request is a beads issue with `type: merge-request`:

```yaml
id: gt-mr-abc123
type: merge-request
status: open                    # open, in_progress, closed
priority: P1                    # Inherited from source issue
title: "Merge: Fix login timeout (gt-xyz)"

# MR-specific fields (in description or structured)
branch: polecat/Nux/gt-xyz      # Source branch
target: main                    # Target branch (or integration/epic-id)
source_issue: gt-xyz            # The work being merged
worker: Nux                     # Who did the work
rig: gastown                    # Which rig

# Set on completion
merge_commit: abc123def         # SHA of merge commit (on success)
close_reason: merged            # merged, rejected, conflict, superseded

# Standard beads fields
created: 2025-12-17T10:00:00Z
updated: 2025-12-17T10:30:00Z
assignee: engineer              # The Engineer processing it
depends_on: [gt-mr-earlier]     # Ordering dependencies
```

### ID Convention

Merge request IDs follow the pattern: `<prefix>-mr-<hash>`

Example: `gt-mr-abc123` for a gastown merge request.

This distinguishes them from regular issues while keeping them in the same namespace.

### Creating Merge Requests

Workers submit to the queue via:

```bash
# Worker signals work is ready
gt mq submit                    # Auto-detects branch, issue, worker

# Explicit submission
gt mq submit --branch polecat/Nux/gt-xyz --issue gt-xyz

# Under the hood, this creates:
bd create --type=merge-request \
  --title="Merge: Fix login timeout (gt-xyz)" \
  --priority=P1 \
  --body="branch: polecat/Nux/gt-xyz
target: main
source_issue: gt-xyz
worker: Nux
rig: gastown"
```

## Queue Ordering

The queue is ordered by:

1. **Dependencies first** - If MR-B depends on MR-A, A merges first
2. **Priority** - P0 before P1 before P2
3. **Age** - Older requests before newer (FIFO within priority)

### Dependency-Based Ordering

When workers complete related work, dependencies ensure correct order:

```
gt-epic (epic)
├── gt-epic.1 (task) → gt-mr-001 (merge request)
├── gt-epic.2 (task) → gt-mr-002 (depends on gt-mr-001)
└── gt-epic.3 (task) → gt-mr-003 (depends on gt-mr-002)
```

The Engineer queries `bd ready --type=merge-request` to find MRs with no unmerged dependencies.

### Integration Branches

For batch work on an epic, use an integration branch:

```
                    main
                      │
                      ├──────────────────────────┐
                      │                          │
              integration/gt-epic          (other work)
                      │
           ┌──────────┼──────────┐
           │          │          │
        MR #1      MR #2      MR #3
```

Integration branch workflow:
1. Create integration branch from main: `integration/gt-epic`
2. MRs target the integration branch, not main
3. After all epic work merges, integration branch merges to main
4. Single PR for epic = easier review, atomic landing

```bash
# Create integration branch for epic
gt mq integration create gt-epic

# MRs auto-target integration branch
gt mq submit --epic gt-epic    # Targets integration/gt-epic

# Land entire epic to main
gt mq integration land gt-epic
```

## Engineer Processing Loop

The Engineer (formerly Refinery) processes the merge queue continuously:

```
┌─────────────────────────────────────────────────────────────┐
│                    ENGINEER LOOP                             │
│                                                              │
│  while true:                                                 │
│    1. ready_mrs = bd ready --type=merge-request             │
│                                                              │
│    2. if ready_mrs.empty():                                 │
│         sleep(poll_interval)                                │
│         continue                                            │
│                                                              │
│    3. mr = ready_mrs.first()  # Highest priority, oldest    │
│                                                              │
│    4. bd update mr.id --status=in_progress                  │
│                                                              │
│    5. result = process_merge(mr)                            │
│                                                              │
│    6. if result.success:                                    │
│         bd close mr.id --reason="merged: {sha}"             │
│       else:                                                  │
│         handle_failure(mr, result)                          │
│                                                              │
│    7. update_source_issue(mr)                               │
└─────────────────────────────────────────────────────────────┘
```

### Process Merge Steps

```python
def process_merge(mr):
    # 1. Fetch the branch
    git fetch origin {mr.branch}

    # 2. Check for conflicts with target
    conflicts = git_check_conflicts(mr.branch, mr.target)
    if conflicts:
        return Failure(reason="conflict", files=conflicts)

    # 3. Merge to local target
    git checkout {mr.target}
    git merge {mr.branch} --no-ff -m "Merge {mr.branch}: {mr.title}"

    # 4. Run tests (configurable)
    if config.run_tests:
        result = run_tests()
        if result.failed:
            git reset --hard HEAD~1  # Undo merge
            return Failure(reason="tests_failed", output=result.output)

    # 5. Push to origin
    git push origin {mr.target}

    # 6. Clean up source branch (optional)
    if config.delete_merged_branches:
        git push origin --delete {mr.branch}

    return Success(merge_commit=git_rev_parse("HEAD"))
```

### Handling Failures

| Failure | Action |
|---------|--------|
| **Conflict** | Assign back to worker, add `needs-rebase` label |
| **Tests fail** | Assign back to worker, add `needs-fix` label |
| **Build fail** | Assign back to worker, add `needs-fix` label |
| **Flaky test** | Retry once, then assign back |
| **Infra issue** | Retry with backoff, escalate if persistent |

```bash
# On conflict, Engineer does:
bd update gt-mr-xxx --assignee=Nux --labels=needs-rebase
gt send gastown/Nux -s "Rebase needed: gt-mr-xxx" \
  -m "Your branch conflicts with main. Please rebase and resubmit."
```

### Conflict Resolution Strategies

1. **Assign back to worker** (default) - Worker rebases and resubmits
2. **Auto-rebase** (configurable) - Engineer attempts `git rebase` automatically
3. **Semantic merge** (future) - AI-assisted conflict resolution

```yaml
# rig config.json
{
  "merge_queue": {
    "on_conflict": "assign_back",  # or "auto_rebase" or "semantic"
    "run_tests": true,
    "delete_merged_branches": true,
    "retry_flaky_tests": 1
  }
}
```

## CLI Commands

### gt mq (merge queue)

```bash
# Submit work to queue
gt mq submit                     # Auto-detect from current branch
gt mq submit --issue gt-xyz      # Explicit issue
gt mq submit --epic gt-epic      # Target integration branch

# View queue
gt mq list                       # Show all pending MRs
gt mq list --ready               # Show only ready-to-merge
gt mq list --mine                # Show MRs for my work
gt mq status gt-mr-xxx           # Detailed MR status

# Integration branches
gt mq integration create gt-epic # Create integration/gt-epic
gt mq integration land gt-epic   # Merge integration to main
gt mq integration status gt-epic # Show integration branch status

# Admin/debug
gt mq retry gt-mr-xxx            # Retry a failed MR
gt mq reject gt-mr-xxx --reason "..." # Reject an MR
gt mq reorder gt-mr-xxx --after gt-mr-yyy # Manual reorder
```

### Command Details

#### gt mq submit

```bash
gt mq submit [--branch BRANCH] [--issue ISSUE] [--epic EPIC]

# Auto-detection logic:
# 1. Branch: current git branch
# 2. Issue: parse from branch name (polecat/Nux/gt-xyz → gt-xyz)
# 3. Epic: if issue has parent epic, offer integration branch

# Creates merge-request bead and prints MR ID
```

#### gt mq list

```bash
gt mq list [--ready] [--status STATUS] [--worker WORKER]

# Output:
# ID          STATUS       PRIORITY  BRANCH                    WORKER  AGE
# gt-mr-001   ready        P0        polecat/Nux/gt-xyz        Nux     5m
# gt-mr-002   in_progress  P1        polecat/Toast/gt-abc      Toast   12m
# gt-mr-003   blocked      P1        polecat/Capable/gt-def    Capable 8m
#             (waiting on gt-mr-001)
```

## Beads Query Patterns

The merge queue is just queries over beads:

```bash
# Ready to merge (no blockers, not in progress)
bd ready --type=merge-request

# All open MRs
bd list --type=merge-request --status=open

# MRs for a specific epic (via labels or source_issue parent)
bd list --type=merge-request --label=epic:gt-xyz

# Recently merged
bd list --type=merge-request --status=closed --since=1d

# MRs by worker
bd list --type=merge-request --assignee=Nux
```

## State Machine

```
                    ┌─────────────┐
                    │   CREATED   │
                    │   (open)    │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
            ┌───────│    READY    │───────┐
            │       │   (open)    │       │
            │       └──────┬──────┘       │
            │              │              │
     blocked by      ┌─────▼─────┐     rejected
     dependency      │ PROCESSING│    (manual)
            │        │(in_progress)     │
            │        └─────┬─────┘      │
            │              │            │
            │     ┌────────┴────────┐   │
            │     │                 │   │
            │  success           failure
            │     │                 │   │
            │     ▼                 ▼   │
            │ ┌───────┐      ┌─────────┐│
            │ │MERGED │      │ FAILED  ││
            │ │(closed)      │ (open)  ││
            │ └───────┘      └────┬────┘│
            │                     │     │
            │              resubmit     │
            │                 │         │
            └─────────────────┴─────────┘
```

## Audit and Observability

The merge queue creates a complete audit trail for all integrated work:

| MQ Event | Record Created |
|----------|----------------|
| Merge request submitted | Work completion claim with author |
| Tests pass | Quality verification record |
| Refinery approves | Validation with reviewer attribution |
| Merge commit | Immutable integration record |
| Rejection | Feedback record with reason |

Every merge creates an immutable record:
- Who did the work (author attribution)
- Who validated it (Refinery attestation)
- When it landed (timestamp)
- What changed (commit diff)

This enables full work attribution and quality tracking across the swarm.

## Configuration

### Rig-Level Config

```json
// <rig>/config.json
{
  "merge_queue": {
    "enabled": true,
    "target_branch": "main",
    "integration_branches": true,
    "on_conflict": "assign_back",
    "run_tests": true,
    "test_command": "go test ./...",
    "delete_merged_branches": true,
    "retry_flaky_tests": 1,
    "poll_interval": "30s",
    "max_concurrent": 1
  }
}
```

### Per-Epic Overrides

```bash
# Create epic with custom merge config
bd create --type=epic --title="Risky refactor" \
  --body="merge_config:
  run_tests: true
  test_command: 'go test -race ./...'
  on_conflict: assign_back"
```

## Direct Landing (Bypass Queue)

For single-polecat work or emergencies, Mayor can bypass the queue:

```bash
gt land --direct <rig>/<polecat>

# This:
# 1. Verifies polecat session is terminated
# 2. Checks git state is clean
# 3. Merges directly to main (no MR created)
# 4. Closes the source issue
# 5. Cleans up the branch
```

Direct landing skips the queue but still records the work in beads.

## Failure Recovery

### Engineer Crash

If Engineer crashes mid-merge:
1. MR stays `in_progress`
2. On restart, Engineer queries `bd list --type=merge-request --status=in_progress`
3. For each, check git state and either complete or reset

### Partial Merge

If merge succeeds but push fails:
1. Engineer retries push with exponential backoff
2. If persistent failure, roll back local merge
3. Mark MR as failed with reason

### Conflicting Merges

If two MRs conflict:
1. First one wins (lands first)
2. Second gets conflict status
3. Worker rebases and resubmits
4. Dependencies prevent this for related work

## Observability

### Metrics

- `mq_pending_count` - MRs waiting
- `mq_processing_time` - Time from submit to merge
- `mq_success_rate` - Merges vs rejections
- `mq_conflict_rate` - How often conflicts occur
- `mq_test_failure_rate` - Test failures

### Logs

```
[MQ] Processing gt-mr-abc123 (priority=P0, age=5m)
[MQ] Fetching branch polecat/Nux/gt-xyz
[MQ] No conflicts detected
[MQ] Merging to main
[MQ] Running tests: go test ./...
[MQ] Tests passed (32s)
[MQ] Pushing to origin
[MQ] Merged: abc123def
[MQ] Closed gt-mr-abc123 (reason=merged)
[MQ] Closed source issue gt-xyz
```

### Dashboard

```
┌─────────────────────────────────────────────────────────────┐
│                    MERGE QUEUE STATUS                        │
├─────────────────────────────────────────────────────────────┤
│ Pending: 3    In Progress: 1    Merged (24h): 12    Failed: 2│
├─────────────────────────────────────────────────────────────┤
│ QUEUE:                                                       │
│   ► gt-mr-004  P0  polecat/Nux/gt-xyz        Processing...  │
│     gt-mr-005  P1  polecat/Toast/gt-abc      Ready          │
│     gt-mr-006  P1  polecat/Capable/gt-def    Blocked (005)  │
│     gt-mr-007  P2  polecat/Nux/gt-ghi        Ready          │
├─────────────────────────────────────────────────────────────┤
│ RECENT:                                                      │
│   ✓ gt-mr-003  Merged 5m ago   (12s processing)             │
│   ✓ gt-mr-002  Merged 18m ago  (45s processing)             │
│   ✗ gt-mr-001  Failed 22m ago  (conflict with main)         │
└─────────────────────────────────────────────────────────────┘
```

## Implementation Phases

### Phase 1: Schema & CLI (gt-kp2, gt-svi)
- Define merge-request type in beads (or convention)
- Implement `gt mq submit`, `gt mq list`, `gt mq status`
- Manual Engineer processing (no automation yet)

### Phase 2: Engineer Loop (gt-3x1)
- Implement processing loop
- Conflict detection and handling
- Test execution
- Success/failure handling

### Phase 3: Integration Branches
- `gt mq integration create/land/status`
- Auto-targeting based on epic
- Batch landing

### Phase 4: Advanced Features
- Auto-rebase on conflict
- Semantic merge (AI-assisted)
- Parallel test execution
- Cross-rig merge coordination

## Open Questions

1. **Beads schema extension**: Should merge-request be a first-class beads type, or just a convention (type field in description)?

2. **High-traffic rigs**: For very active rigs, should MRs go to a separate beads repo to reduce sync contention?

3. **Cross-rig merges**: If work spans multiple rigs, how do we coordinate? Federation design needed.

4. **Rollback**: If a merge causes problems, how do we track and revert? Need `gt mq revert` command.
