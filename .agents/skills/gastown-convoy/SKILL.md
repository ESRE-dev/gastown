---
name: gastown-convoy
description: The definitive guide for working with gastown's convoy system — batch work tracking, event-driven feeding, and dispatch safety guards. Use when writing convoy code, debugging convoy behavior, adding convoy features, testing convoy changes, or answering questions about how convoys work. Triggers on convoy, convoy manager, convoy feeding, dispatch, stranded convoy, feedFirstReady, feedNextReadyIssue, IsSlingableType, isIssueBlocked, CheckConvoysForIssue, gt convoy, gt sling.
---

# Gastown Convoy System

The convoy system tracks batches of work across rigs. A convoy is a bead that `tracks` other beads via dependencies. The daemon monitors close events and feeds the next ready issue when one completes.

## Architecture

```
gt sling <bead> <rig>           gt convoy create <name> <beads...>
    │ (auto-creates convoy)         │ (explicit convoy)
    ▼                               ▼
CONVOY (hq-cv-*)
    │ tracks: issue1, issue2, ...
    │ status: open / closed
    │
    ├── Event-driven path (daemon ConvoyManager)
    │   pollStore → close event → CheckConvoysForIssue
    │   → runConvoyCheck → feedNextReadyIssue
    │
    └── Stranded scan path (daemon ConvoyManager)
        findStranded → feedFirstReady (or closeEmptyConvoy)
```

Two feed paths, same safety guards:
- **Event-driven** (`operations.go`): Reacts to close events within 5s. Calls `feedNextReadyIssue` which checks `IsSlingableType` + `isIssueBlocked` before dispatch.
- **Stranded scan** (`convoy_manager.go`): Runs every 30s. `feedFirstReady` iterates all ready issues. The ready list is pre-filtered by `IsSlingableType` in `findStrandedConvoys` (cmd/convoy.go).

## Key source files

| File | What it does |
|------|-------------|
| `internal/convoy/operations.go` | Core feeding logic: `CheckConvoysForIssue`, `feedNextReadyIssue`, `IsSlingableType`, `isIssueBlocked`, `getConvoyTrackedIssues` |
| `internal/daemon/convoy_manager.go` | `ConvoyManager` goroutines: `runEventPoll`, `runStrandedScan`, `feedFirstReady`, `findStranded` |
| `internal/cmd/convoy.go` | All `gt convoy` subcommands: create, add, check, status, list, stranded, close, land |
| `internal/cmd/sling.go` | `gt sling` — auto-creates convoy per bead (batch sling = N convoys, not 1) |
| `internal/cmd/sling_batch.go` | Batch sling loop — one auto-convoy per bead at lines 124-136 |
| `internal/cmd/sling_convoy.go` | Auto-convoy creation logic |
| `internal/daemon/daemon.go` | Daemon startup — creates `ConvoyManager` at line ~237 |

## Safety guards (the three rules)

These prevent the event-driven feeder from dispatching work it shouldn't:

### 1. Type filtering (`IsSlingableType`)

Only leaf work items dispatch. Defined in `operations.go`:

```go
var slingableTypes = map[string]bool{
    "task": true, "bug": true, "feature": true, "chore": true,
    "": true, // empty defaults to task
}
```

Epics, sub-epics, convoys, decisions — all skip. Applied in both `feedNextReadyIssue` (event path) and `findStrandedConvoys` (stranded path).

### 2. Blocks dep checking (`isIssueBlocked`)

Issues with unclosed `blocks`, `conditional-blocks`, or `waits-for` dependencies skip. `parent-child` is **not** blocking — a child task dispatches even if its parent epic is open. This is consistent with `bd ready` and molecule step behavior.

Fail-open on store errors (assumes not blocked) to avoid stalling convoys on transient Dolt issues.

### 3. Dispatch failure iteration

Both feed paths iterate past failures instead of giving up:
- `feedNextReadyIssue`: `continue` on dispatch failure, try next ready issue
- `feedFirstReady`: `for range ReadyIssues` with `continue` on skip/failure, `return` on first success

## CLI commands

```bash
# Create a convoy tracking specific beads
gt convoy create "Auth overhaul" gt-task1 gt-task2 gt-task3

# Add beads to existing convoy
gt convoy add hq-cv-abc gt-task4

# Check completion (auto-closes if all tracked issues done)
gt convoy check hq-cv-abc
gt convoy check              # check all open convoys

# View status
gt convoy status hq-cv-abc   # single convoy detail
gt convoy list               # all convoys
gt convoy list --all         # include closed

# Find stranded convoys (ready work, no workers)
gt convoy stranded
gt convoy stranded --json    # machine-readable

# Close/land
gt convoy close hq-cv-abc --reason "done"
gt convoy land hq-cv-abc     # cleanup worktrees + close

# Interactive TUI
gt convoy
```

## Batch sling behavior

`gt sling <bead1> <bead2> <bead3> <rig>` creates **3 separate auto-convoys**, one per bead. There is no grouping. Each convoy tracks exactly 1 issue. To get grouped tracking, create a convoy first, then sling:

```bash
gt convoy create "My batch" gt-task1 gt-task2 gt-task3
gt sling gt-task1 myrig
# daemon feeds gt-task2 when gt-task1 closes (if not blocked)
```

## Testing convoy changes

### Unit tests

```bash
# Core feeding logic (type filter, blocks check, iteration)
go test ./internal/convoy/... -v -count=1

# ConvoyManager (event poll, stranded scan, feedFirstReady)
go test ./internal/daemon/... -v -count=1 -run TestConvoy
go test ./internal/daemon/... -v -count=1 -run TestFeedFirstReady
go test ./internal/daemon/... -v -count=1 -run TestScanStranded
go test ./internal/daemon/... -v -count=1 -run TestEventPoll
go test ./internal/daemon/... -v -count=1 -run TestPollAllStores

# Convoy commands (stranded scan CLI path)
go test ./internal/cmd/... -v -count=1 -run TestConvoy
```

### Test patterns

Tests use mock `gt` shell scripts that log calls to temp files. The pattern from `convoy_manager_test.go`:

```go
paths := mockGtForScanTest(t, scanTestOpts{
    strandedJSON: `[{"id":"hq-cv1","ready_count":1,"ready_issues":["gt-issue1"]}]`,
    routes:       `{"prefix":"gt-","path":"gt/.beads"}` + "\n",
})
m := NewConvoyManager(paths.townRoot, logger, "gt", 10*time.Minute, nil, nil, nil)
m.scan()
// Assert by reading paths.slingLogPath, paths.checkLogPath
```

For event-driven path tests, use real beads stores via `setupTestStore(t)`:

```go
store, cleanup := setupTestStore(t)
defer cleanup()
// CreateIssue, CloseIssue, then pollAllStores
m := NewConvoyManager(townRoot, logger, "gt", 10*time.Minute,
    map[string]beadsdk.Storage{"hq": store}, nil, nil)
m.seeded = true
m.pollAllStores()
```

### Key test invariants

- `feedFirstReady` dispatches exactly 1 issue per call (first success wins)
- `feedFirstReady` iterates past failures (sling exit 1 → try next)
- Parked rigs are skipped in both event poll and feedFirstReady
- hq store is never skipped even if `isRigParked` returns true for everything
- High-water marks prevent event reprocessing across poll cycles
- First poll cycle is warm-up only (seeds marks, no processing)
- `IsSlingableType("epic") == false`, `IsSlingableType("task") == true`, `IsSlingableType("") == true`
- `isIssueBlocked` is fail-open (store error → not blocked)
- `parent-child` deps are NOT blocking

### Integration tests

```bash
go test ./internal/daemon/... -v -count=1 -run Integration
```

These use real beads stores and test the full event→convoy→feed pipeline.

### Full suite

```bash
go test ./internal/convoy/... ./internal/daemon/... ./internal/cmd/... -count=1
```

## Design reference

For deeper context on the pre-rewrite system (architecture, all call sites, bugs, limitations), read **[references/upstream-convoy-system.md](references/upstream-convoy-system.md)**. Useful when investigating legacy behavior or understanding why things are the way they are.

## Common pitfalls

- **`parent-child` is never blocking.** This is a deliberate design choice, not a bug. Consistent with `bd ready`, beads SDK, and molecule step behavior.
- **Batch sling doesn't group.** `gt sling a b c rig` = 3 convoys, not 1. To get grouped tracking, create a convoy first.
- **The stranded scan has its own blocked check.** `isReadyIssue` in cmd/convoy.go reads `t.Blocked` from issue details. `isIssueBlocked` in operations.go covers the event-driven path. Don't consolidate them without understanding both paths.
- **Empty IssueType is slingable.** Beads default to type "task" when IssueType is unset. Treating empty as non-slingable would break all legacy beads.
- **`isIssueBlocked` is fail-open.** Store errors assume not blocked. A transient Dolt error should not permanently stall a convoy — the next feed cycle retries with fresh state.
