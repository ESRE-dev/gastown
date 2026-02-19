# Convoy System: Upstream Main

How the convoy system works on `upstream/main` before the parent PR
(`feat/convoy-manager-rewrite`) or Phase 1 changes.

---

## Architecture overview

```
gt sling (auto-creates convoy)
    │
    ▼
CONVOY (hq-cv-*)
    │ tracks: issue1, issue2, ...
    │ status: open / closed
    │ metadata: owner, notify, merge (stored as description text)
    │
    ├── Observed by (redundant):
    │   │
    │   ├── ConvoyWatcher (daemon)     bd activity --follow --town
    │   │   On close event:            gt convoy check <id>
    │   │
    │   ├── Witness (handlers.go)      6 call sites
    │   │   On merge/zombie:           CheckConvoysForIssue
    │   │                              + feedNextReadyIssue
    │   │
    │   └── Refinery (engineer.go)     1 call site (BROKEN)
    │       On MR close:               CheckConvoysForIssue
    │                                  passes rig path, not town root
    │
    └── Stranded scan (Deacon patrol cycle)
        gt convoy stranded             finds convoys with ready work
                                       suggests: gt sling mol-convoy-feed
```

---

## 1. Convoy creation

### `gt convoy create <name> [issues...]`

**File:** `internal/cmd/convoy.go:386-531`

1. If first arg looks like an issue ID, treats all args as issue IDs and
   auto-generates the convoy name from the first issue's title.
2. Registers `convoy` as a custom bead type via `beads.EnsureCustomTypes`.
3. Generates ID: `hq-cv-<5-random-chars>`.
4. Creates the convoy bead:
   `bd create --type=convoy --id=<id> --title=<name> --description=<desc> --json`
5. For each tracked issue:
   `bd dep add <convoyID> <issueID> --type=tracks`

Metadata (owner, notify, merge strategy) is stored as plain text lines in the
bead description field:
```
Owner: mayor/
Notify: mayor/
Merge: direct
Molecule: mol-xxx
```

Parsed back out via string splitting when needed.

**Flags:** `--molecule`, `--owner`, `--notify` (default `mayor/`), `--owned`,
`--merge` (`direct`/`mr`/`local`)

### `gt sling` auto-convoy

**File:** `internal/cmd/sling_convoy.go:205-268`

When slinging an issue without `--no-convoy`:

1. `isTrackedByConvoy(beadID)` checks if already tracked:
   - Primary: `bd dep list <beadID> --direction=up --type=tracks --json`,
     filters for `issue_type == "convoy"` and `status == "open"`.
   - Fallback 1: lists all open convoys, checks description for
     `"tracking <beadID>"` pattern (matches auto-created convoys).
   - Fallback 2: for each open convoy, `convoyTracksBead` checks
     `bd dep list <convoyID> --direction=down --type=tracks` for the
     bead ID (matches manually-created convoys where description differs).
2. If not tracked, `createAutoConvoy` creates one:
   - Title: `"Work: <issue-title>"`
   - Description: `"Auto-created convoy tracking <beadID>"`
   - Adds `tracks` dep.
3. On tracking failure, closes the orphan convoy immediately.

Batch slinging (`sling_batch.go:123-136`) creates a separate auto-convoy
per bead.

---

## 2. Convoy statuses

**File:** `internal/cmd/convoy.go:86-130`

Two valid statuses:
- `open`
- `closed`

All transitions are valid (open→closed, closed→open, same→same).
`ensureKnownConvoyStatus` rejects anything else.

---

## 3. ConvoyWatcher (daemon)

**File:** `internal/daemon/convoy_watcher.go` (238 lines)

### How it detects events

1. Spawns `bd activity --follow --town --json` as a long-running subprocess.
2. Reads NDJSON lines from stdout via `bufio.Scanner`.
3. On each line, checks `isCloseEvent()`: `Type == "status" && NewStatus == "closed"`.
4. For close events:
   - `bd dep list <issueID> --direction=up -t tracks --json` to find tracking
     convoys.
   - `gt convoy check <convoyID>` for each tracking convoy.

### Limitations

- **Single-store visibility.** `--town` flag means it only watches the town
  (hq) beads store. Close events for rig-level beads (`gt-*`, `gt-*`) in
  their own rig stores are invisible. The watcher can only detect convoy
  progression when close events happen to appear in hq activity.
- **Fragile subprocess.** Any interruption (Dolt restart, OOM, broken pipe)
  kills the stream. Retries after 5s, but events during the gap are silently
  dropped. No high-water mark or replay.
- **No continuation feed.** Unlike the observer pattern, the watcher only
  calls `gt convoy check` — it does NOT call `feedNextReadyIssue`. It checks
  completion but doesn't dispatch next work.

---

## 4. Observer pattern

**File:** `internal/convoy/observer.go` (309 lines)

### `CheckConvoysForIssue(townRoot, issueID, observer, logger)`

The shared function called by witness, refinery, and (indirectly) daemon.

```
1. getTrackingConvoys(townRoot, issueID)
   → bd dep list <issueID> --direction=up -t tracks --json
   → returns convoy IDs

2. For each convoy:
   a. isConvoyClosed(townRoot, convoyID)
      → bd show <convoyID> --json
      → skip if closed

   b. runConvoyCheck(townRoot, convoyID)
      → gt convoy check <convoyID>

   c. If STILL open after check:
      feedNextReadyIssue(townRoot, convoyID, observer, logger)
```

### `feedNextReadyIssue` — the event-driven dispatch

**File:** `internal/convoy/observer.go:147-176`

```
1. getConvoyTrackedIssues(townRoot, convoyID)
   → bd dep list <convoyID> --direction=down --type=tracks --json
   → unwrap external:prefix:id format
   → bd show <ids...> --json    (cross-rig status refresh)

2. For each tracked issue:
   → status == "open" && assignee == "" ?
   → YES: resolve rig via beads.ExtractPrefix + GetRigNameForPrefix
          → gt sling <issueID> <rig> --no-boot
          → return (one at a time)
   → NO: continue
```

**What it checks:** Open + no assignee. That's it.

**What it does NOT check:**
- Blocking dependencies (`blocks`, `conditional-blocks`, `waits-for`)
- Bead type (will sling epics, convoys, anything)
- Rig parked status
- Rig capacity (`max_polecats`)
- Pinned/deferred/ephemeral
- Tmux session liveness (only the stranded scan checks this)

### Subprocess count per cycle

A single `feedNextReadyIssue` call spawns up to:
1. `bd dep list` (tracked issues)
2. `bd show` (batch status refresh)
3. `gt sling` (dispatch)

A full `CheckConvoysForIssue` cycle can spawn up to 7 processes per convoy:
1. `bd dep list` (find tracking convoys — shared across all convoys)
2. `bd show` (first `isConvoyClosed` check — skip if already closed)
3. `gt convoy check` (completion check, which itself spawns more)
4. `bd show` (second `isConvoyClosed` check — gates feed dispatch)
5. `bd dep list` (tracked issues for feed)
6. `bd show` (batch status refresh for feed)
7. `gt sling` (dispatch)

---

## 5. Witness integration

**File:** `internal/witness/handlers.go`

Six call sites, all calling `convoy.CheckConvoysForIssue(townRoot, beadID, observer, nil)`:

| # | Handler | Line | Trigger | Observer ID |
|---|---------|------|---------|-------------|
| 1 | `HandleMerged` | 311 | Work confirmed on main | `"witness"` |
| 2 | `DetectZombiePolecats` | 1063 | Stuck-in-done zombie, bead closed | `"witness-zombie"` |
| 3 | `DetectZombiePolecats` | 1090 | Dead agent in session, bead closed | `"witness-zombie"` |
| 4 | `DetectZombiePolecats` | 1112 | Bead closed but polecat still running | `"witness-zombie"` |
| 5 | `DetectZombiePolecats` | 1143 | Done-intent set but session died, bead closed | `"witness-zombie"` |
| 6 | `DetectZombiePolecats` | 1255 | Standard zombie fallback, bead closed | `"witness-zombie"` |

All pass `nil` logger (silent). The witness is a **redundant observer** — it
catches cases the daemon misses (cross-rig closes invisible to the hq-only
`bd activity --follow --town` stream).

Five of the six calls are in zombie detection paths. The one in `HandleMerged`
fires when the refinery sends a MERGED signal confirming work landed on main.

---

## 6. Refinery integration

**File:** `internal/refinery/engineer.go:716`

One call site in `HandleMRInfoSuccess`, after closing the source issue:

```go
convoy.CheckConvoysForIssue(e.rig.Path, mr.SourceIssue, "refinery", logger)
```

### The bug

First argument is `e.rig.Path` (rig directory, e.g., `/home/user/town/gastown`),
not the town root (`/home/user/town`). `CheckConvoysForIssue` passes this as
`townRoot` to `getTrackingConvoys`, which runs `bd dep list` with
`cmd.Dir = townRoot`. When `bd dep list` runs from the rig directory, it
queries the rig's beads store, not the hq store where convoy tracking relations
live. The lookup silently fails every time.

**Impact:** The refinery's convoy observation never works. Convoy progression
depends entirely on the witness and daemon observers.

---

## 7. Stranded scan

**File:** `internal/cmd/convoy.go`

The stranded scan is NOT run by the daemon on a timer. It is invoked by
the Deacon agent during its `mol-deacon-patrol` formula execution (the
`feed-stranded-convoys` step calls `gt convoy stranded --json`). The
Deacon patrol cadence depends on the Deacon's work loop, not a fixed
interval. The daemon's `ConvoyWatcher` only handles close-event
completion checks — it never runs the stranded scan.

Note: the parent PR introduces `ConvoyManager` in the daemon with a
dedicated stranded scan timer (30s).

### `findStrandedConvoys(townBeads)` (line 1222)

1. `bd list --type=convoy --status=open --json` — all open convoys.
2. For each convoy:
   - `getTrackedIssues(townBeads, convoyID)` — tracked issues with fresh
     cross-rig status (via `bd dep list` + `bd show`).
   - 0 tracked issues → stranded (empty convoy, needs cleanup).
   - For each tracked issue: `isReadyIssue(t)` filter.
   - Any ready issues → stranded convoy.

### `isReadyIssue(t)` (line 1289)

```
1. status == closed/tombstone?     → not ready
2. t.Blocked == true?              → not ready (from bd show blocked_by)
3. status == open, no assignee?    → READY
4. non-open, no assignee?          → READY (edge case: detached molecule)
5. has assignee?
   → tmux has-session -t <session>
   → session dead?                 → READY (orphaned work)
   → session alive?                → not ready (worker active)
```

### What stranded scan does NOT do

- Does not filter by bead type. Epics, convoys, gates all pass through.
- Does not check rig parked status.
- Does not check rig capacity.
- Does not dispatch work. It only reports stranded convoys and suggests
  commands.

### `feedFirstReady`

**Does not exist on upstream main.** The stranded scan on main is
reporting-only. It suggests `gt sling mol-convoy-feed deacon/dogs` but does
not dispatch automatically.

The parent PR introduces `feedFirstReady` in the daemon's `ConvoyManager`
to auto-dispatch from stranded scan results.

---

## 8. Convoy completion

### `gt convoy check [convoy-id]`

**File:** `internal/cmd/convoy.go:624-656`

With a specific ID (`checkSingleConvoy`, line 659):

1. `bd show <convoyID> --json` — get convoy details.
2. Validate type is convoy, not already closed.
3. `getTrackedIssues(townBeads, convoyID)` — all tracked issues.
4. 0 tracked issues = complete (definitionally).
5. Check if all tracked issues are `closed` or `tombstone`.
6. If all closed: `bd close <convoyID> -r "<reason>"`.
7. `notifyConvoyCompletion` — sends mail to owner/notify addresses.

Without args (`checkAndCloseCompletedConvoys`, line 1335):

1. `bd list --type=convoy --status=open --json` — all open convoys.
2. For each: same check as above.

### `notifyConvoyCompletion` (line 1411)

1. Parses `Owner:` and `Notify:` from convoy description text.
2. For each address: `gt mail send <addr> -s "Convoy landed: <title>"`.
3. If `convoy.notify_on_complete` town setting enabled:
   `gt nudge mayor -m "<msg>"`.

---

## 9. Convoy feed

**There is no `gt convoy feed` command on upstream main.**

The `gt convoy stranded` output suggests using a formula:
```
gt sling mol-convoy-feed deacon/dogs --var convoy=<id>
```

Reactive feeding happens inside `CheckConvoysForIssue` via
`feedNextReadyIssue` in `observer.go`. This is called automatically by the
witness and refinery (not the daemon — the daemon's ConvoyWatcher only runs
`gt convoy check`, it does NOT call `feedNextReadyIssue`).

---

## 10. All convoy subcommands

| Command | Args | What it does |
|---------|------|-------------|
| `gt convoy create <name> [issues...]` | Min 1 | Create convoy tracking issues |
| `gt convoy add <convoy-id> <issues...>` | Min 2 | Add issues to convoy (reopens if closed) |
| `gt convoy status [convoy-id]` | Max 1 | Show progress, tracked issues, workers |
| `gt convoy list` | 0 | List convoys (open by default) |
| `gt convoy check [convoy-id]` | Max 1 | Check/auto-close completed convoys |
| `gt convoy stranded` | 0 | Find stranded convoys needing attention |
| `gt convoy close <convoy-id>` | 1 | Close convoy (verifies done or --force) |
| `gt convoy land <convoy-id>` | 1 | Land owned convoy (cleanup + close) |
| `gt convoy` (no subcommand) | 0 | With `--interactive`: launch TUI |

---

## 11. Known bugs and limitations on upstream main

### Bugs

1. **Refinery wrong path** (`engineer.go:716`). Passes `e.rig.Path` instead of
   town root. `bd dep list` runs from rig directory, silently fails to find
   hq-level convoy tracking. Refinery convoy observation never works.

2. **Daemon doesn't feed.** `ConvoyWatcher` calls `gt convoy check` but does
   NOT call `feedNextReadyIssue`. Only the witness and refinery observers
   (via `CheckConvoysForIssue`) trigger reactive feeding. Since the refinery
   call is broken (#1), only the witness actually feeds.

### Architectural limitations

3. **Single-store visibility.** Daemon watches `bd activity --follow --town`
   (hq only). Cross-rig close events are invisible unless they produce hq
   activity. The witness observers compensate, but this means convoy
   progression depends on the witness being alive and processing events.

4. **Fragile event tailing.** Any interruption to the `bd activity` subprocess
   drops events with no replay. 5s retry, but gap events are lost.

5. **No readiness checks in `feedNextReadyIssue`.** The only filter is
   `status == "open" && assignee == ""`. No blocks check, no type filter, no
   rig parked check, no capacity check. Will sling epics, blocked tasks,
   and tasks on parked/overloaded rigs.

6. **Stranded scan is reporting-only.** Does not auto-dispatch. Only
   suggests commands. Cadence depends on Deacon patrol loop timing.

7. **Subprocess overhead.** Every operation shells out to `bd` or `gt`. A
   single convoy check cycle can spawn 7+ processes per convoy (including
   two `isConvoyClosed` calls). No cancellation support (bare
   `exec.Command` without process groups).

8. **Owner/Notify as description text.** Convoy metadata stored as plain text
   lines in the bead description, parsed via string splitting. Fragile if
   descriptions contain similar patterns.

9. **Auto-convoy per sling.** `gt sling` creates a separate convoy per issue.
   Batch slinging creates N convoys for N issues. No convoy grouping beyond
   manual `gt convoy create`.

### What upstream main cannot do

- Filter non-slingable bead types (epics get slung)
- Check blocking dependencies before dispatch
- Enforce rig capacity
- Detect events from rig stores (only hq)
- Auto-dispatch from stranded scan results
- Retry after first dispatch failure
- Support staged convoy workflows

---

## 12. `gt sling` and convoy interaction

### Single-bead sling

**File:** `internal/cmd/sling.go:441-471`

`gt sling <beadID> <rig>` auto-creates a convoy unless `--no-convoy`:

1. `isTrackedByConvoy(beadID)` — checks `bd dep list --direction=up --type=tracks`
   for an open convoy, with fallback description scan and dep check.
2. If not tracked: `createAutoConvoy(beadID, title, owned, merge)` →
   - Title: `"Work: <issue-title>"`
   - Description: `"Auto-created convoy tracking <beadID>"`
   - `bd create --type=convoy --id=hq-cv-<random> ...`
   - `bd dep add <convoyID> <beadID> --type=tracks`
3. If already tracked: prints "Already tracked by convoy X", skips.

### Batch sling

**File:** `internal/cmd/sling_batch.go` (`runBatchSling`, lines 21-265)

`gt sling <bead1> <bead2> <bead3> <rig>` (multiple beads, last arg = rig):

- **Each bead gets its OWN convoy.** 3 beads = 3 convoys.
- Each bead gets its own polecat spawned independently.
- 2-second delay between spawns for Dolt lock contention.
- `--no-convoy` suppresses auto-convoy for all beads.

There is no "group sling" that creates one convoy for multiple beads.
To track multiple beads in one convoy: `gt convoy create "name" <issues>`
first, then sling individually.

### Flags

- `--no-convoy` — skip auto-convoy creation. Fully implemented. Used by
  `internal/deacon/redispatch.go:344` for redispatch (avoids duplicates).
- `--owned` — marks convoy with `gt:owned` label. Affects `gt done` merge
  path: owned+direct convoys push directly to main, skip refinery.
- `--merge` — sets merge strategy on convoy: `direct` (push to main),
  `mr` (default, create MR for refinery), `local` (stay on feature branch).

### Does sling refuse epics?

**No.** There is no bead type check in `sling.go`. Any bead ID is accepted.
An epic gets wrapped in `mol-polecat-work` and handed to a polecat — which
is semantically meaningless.

---

## 13. `gt done` and convoy interaction

**File:** `internal/cmd/done.go`

`gt done` reads convoy info to decide the merge path:

1. `getConvoyInfoForIssue(issueID)` — gets ConvoyInfo (ID, owned, merge
   strategy) from the convoy tracking this issue.
2. **`local` strategy** — skips push and MR. Work stays on feature branch.
3. **`direct` strategy** — pushes directly to default branch, skips MR,
   closes issue directly.
4. **`mr` strategy** (default) — push branch, create MR bead for refinery.
5. **`owned+direct`** — `convoyInfo.IsOwnedDirect()` returns true: skips
   merge queue entirely, closes issue with "Completed via owned+direct convoy."

The POLECAT_DONE notification to the witness includes `ConvoyID`,
`ConvoyOwned`, and `MergeStrategy` fields — but the witness template has
zero convoy instructions.

---

## 14. Formulas that reference convoys

### `mol-convoy-feed.formula.toml`

Full convoy feeding workflow for Dogs. Steps:
1. `load-convoy` — `gt convoy status {{convoy}} --json`
2. `check-capacity` — check rig capacity before dispatching
3. `dispatch-work` — `gt sling` for each ready issue
4. `report-results` — notify Deacon
5. `return-to-kennel`

Triggered by Deacon patrol when stranded convoys are detected.

### `mol-convoy-cleanup.formula.toml`

Convoy archival workflow for Dogs. Steps:
1. `load-convoy` — get convoy details
2. `generate-summary` — compile completion report
3. `archive-convoy` — `bd close <convoyID>`
4. `notify-overseer` — mail Mayor with summary
5. `return-to-kennel`

Triggered by Deacon patrol when completed convoys need cleanup.

### `mol-deacon-patrol.formula.toml`

Two dedicated convoy steps in the Deacon's patrol cycle:
- `check-convoy-completion` — runs `gt convoy list` + `gt convoy check`
  to auto-close completed convoys.
- `feed-stranded-convoys` — runs `gt convoy stranded --json`, dispatches
  dogs for feedable convoys, auto-closes empty convoys.

### `mol-dep-propagate.formula.toml`

No direct convoy mention, but the notification to witnesses says "Check
bd ready for available work" — which could surface convoy-tracked issues
via a parallel path.

### Formula `type = "convoy"` (different concept)

`code-review.formula.toml` and `design.formula.toml` use `type = "convoy"`
to mean **parallel multi-leg formula execution** — not the tracking convoy.
This overloaded term is undocumented.

---

## 15. What agents are told about convoys

### Per-role knowledge

| Role | Convoy in template | Convoy in formula | Actual convoy interaction |
|------|-------------------|-------------------|---------------------------|
| Mayor | `convoy list`, `status`, `create` | N/A | N/A |
| Crew | `convoy create` + `sling` pattern | N/A | N/A |
| Polecat | Nothing | Nothing | `gt done` reads convoy merge strategy silently |
| Witness | Nothing | Nothing | Go code calls `CheckConvoysForIssue` at 6 sites |
| Refinery | Nothing | Nothing | Go code calls `CheckConvoysForIssue` (broken) |
| Deacon | Nothing in template | `check-convoy-completion`, `feed-stranded-convoys` | Primary convoy lifecycle manager |
| Dog | "Handle stranded convoys" | `mol-convoy-feed`, `mol-convoy-cleanup` | Executes convoy feeding/cleanup |

### Key discrepancies

**1. Auto-convoy is invisible to all agents.** `gt sling` auto-creates a
convoy for every single-bead sling. No agent is told this happens. Agents
create convoys without knowing it.

**2. Batch sling creates N convoys, not 1.** `gt sling a b c rig` creates
3 individual convoys. The glossary describes convoys as grouping related
work, but batch sling defeats this — each issue gets its own "Work: X"
convoy. To group: must use `gt convoy create` first.

**3. Convoy merge strategy silently affects polecats.** `gt done` reads the
convoy's merge strategy (direct/mr/local) and changes behavior — potentially
pushing directly to main. Polecats have zero visibility into why.

**4. Witness is a convoy observer but doesn't know it.** Go code runs
`CheckConvoysForIssue` after merges and during zombie cleanup. The witness
template says nothing about convoys.

**5. Mayor prompt is incomplete.** Missing `convoy check`, `convoy stranded`,
`convoy close`, `convoy land`. Also missing `--owned`, `--merge`, and the
auto-convoy-on-sling behavior.

**6. "Convoy" means two things.** Formula `type = "convoy"` (parallel
multi-leg execution) vs tracking convoy (`hq-cv-*` beads). Undocumented
distinction.

**7. Convoy feeding relies on witness, but witness doesn't know.** On
upstream main, the daemon's ConvoyWatcher only checks completion — it does
NOT call `feedNextReadyIssue`. The witness Go code is the primary source of
event-driven convoy feeding (via `CheckConvoysForIssue` → `feedNextReadyIssue`).
But the witness agent has no idea it does this.

**8. Dogs are the best-informed agents.** The dog role template and
`mol-convoy-feed`/`mol-convoy-cleanup` formulas give dogs clear, accurate
convoy instructions. All other roles have minimal or zero convoy knowledge.

---

## 16. How tasks tracked by a convoy actually get slung (end to end)

Given the analysis above, here is the actual sequence on upstream main for
a convoy tracking 3 tasks:

### Path A: Manual sling (most common)

1. User runs `gt convoy create "My work" gt-task-1 gt-task-2 gt-task-3`.
2. User runs `gt sling gt-task-1 gastown`.
   - Sling checks `isTrackedByConvoy` — already tracked.
   - Skips auto-convoy. Spawns polecat for task-1.
3. Polecat works on task-1, runs `gt done`.
   - `gt done` reads convoy merge strategy from the convoy.
   - Pushes branch, creates MR (or direct merge, depending on strategy).
4. Witness receives POLECAT_DONE notification.
   - Witness Go code calls `convoy.CheckConvoysForIssue(townRoot, task1ID, "witness", nil)`.
   - `feedNextReadyIssue` finds task-2 (open, no assignee).
   - `gt sling gt-task-2 gastown --no-boot`.
5. Polecat works on task-2, runs `gt done`. Repeat.
6. After task-3 closes, `gt convoy check` finds all tracked issues closed.
   - `bd close <convoyID>`.
   - `notifyConvoyCompletion` mails owner.

**Key insight:** After the FIRST manual sling, subsequent tasks are fed
automatically via the witness observer's `feedNextReadyIssue`. The user
does NOT need to manually sling each task. But only ONE task is dispatched
per close event (serial execution).

### Path B: Deacon patrol (fallback)

If the witness misses a close event (e.g., cross-rig issue not visible
in hq activity, or witness was down):

1. Deacon patrol runs every ~5 minutes.
2. `feed-stranded-convoys` step runs `gt convoy stranded --json`.
3. Finds the convoy with ready issues.
4. Dispatches a dog with `mol-convoy-feed` formula.
5. Dog runs `gt sling` for ready issues.

This is the **safety net** — slower (5-minute poll) but catches what the
event-driven path misses.

### Path C: Daemon ConvoyWatcher (completion only)

The daemon watches `bd activity --follow --town` for close events. But it
only calls `gt convoy check` — no `feedNextReadyIssue`. So the daemon
detects convoy completion but does NOT dispatch next work. That relies on
the witness (Path A) or Deacon (Path B).

### What does NOT happen

- No task is dispatched automatically at convoy creation time.
  `gt convoy create` only creates the tracking bead and adds `tracks` deps.
  The user must manually sling the first task.
- Batch sling (`gt sling a b c rig`) slings all tasks at once. It does NOT
  create a single group convoy — each task gets its own convoy. There is no
  sequential dispatch.
- The stranded scan does NOT auto-dispatch. It reports and suggests commands.
  The Deacon patrol's `feed-stranded-convoys` step handles actual dispatch
  via dogs.
