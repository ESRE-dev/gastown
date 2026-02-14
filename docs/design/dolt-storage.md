# Dolt Storage Architecture

> **Status**: Canonical reference — consolidates all prior Dolt design docs
> **Date**: 2026-02-11 (updated)
> **Context**: Dolt as the unified data layer for Beads and Gas Town
> **Consolidates**: DOLT-STORAGE-DESIGN.md, THREE-PLANES.md, dolt-integration-analysis-v{1,2}.md,
> dolt-license-analysis.md (all deleted; available in git history under ~/hop/docs/)
> **Key decisions**: SQLite retired. JSONL retired (fully replaced by git remotes). Dolt is the
> only backend. Server mode is **required** (embedded mode fully removed — no fallback).
> Git remotes (Dolt v1.81.8) replace JSONL for backup and federation.
> **Migration history**: See `~/gt/mayor/DOLT-HEALTH-P0.md` for the 2-week war-room
> document that drove the migration from embedded to server mode (Jan-Feb 2026).

---

## Part 1: Architecture Decisions

### What's Settled

| Decision | Details |
|----------|---------|
| **Dolt is the only backend** | SQLite retired. No dual-backend. |
| **JSONL fully retired** | Replaced by git remotes for backup. No JSONL export, import, or sync. |
| **Dolt Server is required** | One server per town, serving all rig databases. No embedded fallback. |
| **Embedded mode removed** | File-level locking causes hangs under concurrent load. Removed entirely — not kept as fallback. |
| **Git remotes for backup & federation** | Dolt v1.81.8 supports Git repos as native Dolt remotes. Data in `refs/dolt/data`, orthogonal to source code. |
| **Single binary** | Pure-Go Dolt (`bd`). No CGO needed for local ops. |
| **Licensing** | Dolt is Apache 2.0, compatible with Beads/Gas Town MIT. Standard attribution. |

### Server Mode Architecture

```
┌─────────────────────────────────┐
│  Dolt SQL Server (per town)     │
│  - Port 3307                    │
│  - Serves all rig databases     │
│  - Multi-client concurrency     │
│  - Managed by gt daemon         │
│  - Auto-start, auto-restart     │
└─────────────────────────────────┘
           │
           ├── hq/       (town-level beads, hq-* prefix)
           ├── gastown/  (gt-* prefix)
           ├── beads/    (bd-* prefix)
           └── ...       (other rigs)
```

All `bd` commands connect via MySQL protocol. There is no embedded fallback.
If the server is not running, `bd` fails fast with a clear error message
pointing the user to `gt dolt start`.

### Why Embedded Mode Was Removed

Embedded Dolt uses file-level locking (noms LOCK). In multi-agent environments,
this causes severe problems:

- `gt status` spawns 40+ `bd` processes to check all rigs
- Each process contends for the same lock file
- Processes hang indefinitely waiting for locks
- A semaphore hack (MaxConcurrentBd=3) serializes access but kills parallelism
- Even read-only operations acquire exclusive locks in the embedded driver

Embedded was initially kept as a fallback, but this created complexity for no
benefit: if the server is down, the data lives on the server's data directory
(`~/.dolt-data/`), so embedded can't access it anyway. Removing embedded entirely
enables significant code simplification (see Part 11).

### Server Topology Options

| Topology | Use Case |
|----------|----------|
| **One server per town** | Default. Single server at `~/gt/.dolt-data/` serves hq + all rigs. Simple operations. |
| **One server per rig** | Isolation between rigs. Useful if rigs have vastly different load patterns or need independent lifecycle. |

Gas Town currently uses one server per town. Per-rig servers are available if
isolation requirements emerge.

---

## Part 2: Three Data Planes

Beads serves three distinct data planes with different requirements. Collapsing
them into one transport (JSONL-in-git) is why scaling hurt.

### Plane 1: Operational

The "live game state" — work in progress, status changes, assignments, patrol
results, molecule transitions, heartbeats.

| Property | Value |
|----------|-------|
| Mutation rate | High (seconds) |
| Mutability | Fully mutable |
| Visibility | Local (town/rig) |
| Durability | Days to weeks |
| Federation | Not federated |
| Transport | **Dolt SQL Server** |
| Backup | **Git remote** (periodic `dolt push`) |

Forensics via `dolt_history_*` tables and `AS OF` queries replaces git-based
JSONL forensics. No git, no JSONL for this plane.

### Plane 2: Ledger

Completed work — the permanent record. Closed beads, validated deliverables.
Accumulates into CVs and skill vectors for HOP.

| Property | Value |
|----------|-------|
| Mutation rate | Low (task completion boundaries) |
| Mutability | Append-only |
| Visibility | Federated (cross-town) |
| Durability | Permanent |
| Transport | **Git remote** (`dolt push` to shared/public repos) |

The compelling variant: **closed-beads-only push**. A dedicated Dolt branch
containing only completed beads could push to a public-facing git remote.
Open/in-progress beads stay on `main` branch, pushed only to the private
operational remote. Operational churn stays private; meaningful completed
units go to the permanent public record.

### Plane 3: Design

Work imagined but not yet claimed — epics, RFCs, specs, plan beads. The "global
idea scratchpad" that needs maximum visibility and cross-town discoverability.

| Property | Value |
|----------|-------|
| Mutation rate | Conversational (minutes to hours) |
| Visibility | Global (maximally visible) |
| Durability | Until crystallized into operational work |
| Transport | **Git remote** to shared "Commons" repo (future) |

### The Lifecycle of Work

```
DESIGN PLANE                  OPERATIONAL PLANE              LEDGER PLANE
(global, collaborative)       (local, real-time)             (permanent, federated)

1. Epic created ──────────>
2. Discussed, refined
3. Subtask claimed ───────> 4. Work begins
                             5. Status changes (high freq)
                             6. Agent works, iterates
                             7. Work completes ────────────> 8. dolt push to ledger remote
                                                              9. Skills derived
                                                             10. CV accumulates
```

---

## Part 3: Git Remotes — The Federation Transport

> **Status**: Available. Dolt v1.81.8 shipped Git remote support on 2026-02-11.
> Delivered by Dustin Brown (Dolt team). Replaces the speculative "dolt-in-git"
> design that assumed binary files committed into git branches.

### What Dolt Shipped

Dolt v1.81.8 supports standard Git repositories as native Dolt remotes. This is
fundamentally different from the original "dolt-in-git" plan — and better for us.

**How it works:**
- `dolt remote add origin git@github.com:org/repo.git` — standard git URL
- Uses your existing SSH credentials (git CLI must be on PATH)
- Dolt stores data in a special `refs/dolt/data` git ref
- This ref is orthogonal to source code branches — no interference
- Not fetched/pulled/cloned by normal git operations
- Standard `dolt push/pull/fetch/clone` manipulate this ref directly
- Can coexist in the same repo as source code

**What changed from the original plan:**

| Aspect | Original "dolt-in-git" | Git Remotes (shipped) |
|--------|----------------------|----------------------|
| Storage | Binary files on git branches | Special `refs/dolt/data` ref |
| Git clone | Gets Dolt data (queryable) | Does NOT get Dolt data |
| Dolt clone | N/A | Gets Dolt data (queryable) |
| Merge | Custom git merge driver | Native Dolt merge |
| File splitting | Needed (50MB limit) | Not needed (ref, not files) |
| Impact on git | Affects clone size, history | Zero impact |
| Setup | `.gitattributes` config | `dolt remote add` |

### What This Eliminates

| Eliminated | Why |
|-----------|-----|
| JSONL entirely | `dolt push` IS the backup — native binary to git remote |
| `bd sync --flush-only` | Replaced by `dolt push` |
| `bd daemon` for JSONL sync | No serialization layer needed |
| JSONL merge conflicts | Cell-level Dolt merge natively |
| Two sources of truth | Dolt DB is the only source; remote is a replica |
| 10% agent token tax | No JSONL sync overhead |
| File splitting concerns | Data in ref, not regular files; no 50MB limit |
| Custom merge driver | Dolt handles its own merging |

### How Push/Pull Works

```
LOCAL (Gas Town)                         REMOTE (GitHub)
┌──────────────────────────┐            ┌──────────────────────────┐
│  Dolt SQL Server         │            │  Git Repository          │
│  ~/.dolt-data/gastown/   │            │  github.com/org/repo     │
│                          │  dolt push │                          │
│  main branch ────────────┼───────────>│  refs/dolt/data          │
│  (all beads data)        │            │  (Dolt binary data)      │
│                          │  dolt pull │                          │
│  main branch <───────────┼────────────│  refs/dolt/data          │
└──────────────────────────┘            ├──────────────────────────┤
                                        │  Regular git branches:   │
                                        │  main, feature/*, etc.   │
                                        │  (source code — untouched│
                                        │   by Dolt operations)    │
                                        └──────────────────────────┘
```

### Requirements and Constraints

| Requirement | Status |
|-------------|--------|
| Git CLI on PATH | Yes (all Gas Town hosts have git) |
| SSH credentials (non-interactive) | Yes (SSH keys, no password prompts) |
| Remote repo exists with ≥1 branch | Must create repos / repos already exist |
| Dolt v1.81.8+ | Must upgrade Dolt binary |

**Known limitation**: Interactive credential prompts (stdin-based username/password)
don't work because Dolt remote operations manipulate stdin/stdout/stderr. The Dolt
team has filed a tracking issue. This doesn't affect us — we use SSH keys.

### Verifying Remote Data

To confirm a git remote has Dolt data:

```bash
git ls-remote origin refs/dolt/data
# Output: <hash>    refs/dolt/data
```

To remove Dolt data from a remote (destructive):

```bash
git push origin :refs/dolt/data
```

### Recovery from Remote

```bash
# Full recovery: clone Dolt database from git remote
dolt clone git@github.com:org/repo.git recovered-db
cd recovered-db

# Or: fetch into existing database
cd ~/.dolt-data/gastown
dolt remote add backup git@github.com:org/repo.git
dolt pull backup main
```

---

## Part 4: JSONL — Fully Retired

> **Status**: JSONL is fully retired. Git remotes replace JSONL for all use cases.
> As of 2026-02-11, there is no JSONL in the data path.

JSONL previously served as an interim backup mechanism while waiting for git
remote support. Now that Dolt git remotes have shipped:

- **Backup**: `dolt push` to git remote (replaces `bd sync --flush-only`)
- **Recovery**: `dolt clone` from git remote (replaces JSONL import)
- **Federation**: `dolt pull` from shared git remote (replaces JSONL exchange)

**What this means for code:**
- Remove `bd sync --flush-only` command (or repurpose as `dolt push` alias)
- Remove JSONL export code paths
- Remove JSONL import code paths
- Remove `bd daemon` JSONL sync logic
- Session close protocol: `dolt push` replaces `bd sync --flush-only`

**What this means for operations:**
- No more `.jsonl` files in git repos
- No more `issues.jsonl` / `mail.jsonl` files to manage
- No more JSONL merge conflicts on `git pull`
- Simpler mental model: Dolt is the only format, everywhere

---

## Part 5: What Dolt Unlocks

### Already Valuable for Beads

| Feature | What It Enables |
|---------|-----------------|
| Cell-level merge | Two agents update different fields → clean merge |
| `dolt_history_*` | Full row-level history, queryable via SQL |
| `AS OF` queries | "What did this look like yesterday?" |
| Branch isolation | Each polecat on own branch during work |
| `dolt_diff` | "What changed between these points?" → activity feeds |

### Unlocks for Gas Town (Now Active)

| Feature | What It Enables |
|---------|-----------------|
| **SQL server mode** | Multi-writer concurrency — the solution to embedded mode's lock contention |
| Conflict-as-data | `dolt_conflicts` table, programmatic resolution |
| Schema versioning | Migrations travel with data |
| VCS stored procedures | `DOLT_COMMIT`, `DOLT_MERGE` as SQL |
| **Git remotes** | Push/pull to GitHub for backup and federation |

### Unlocks for HOP (impossible with SQLite)

| Feature | What It Enables |
|---------|-----------------|
| Cross-time skill queries | "What Go work in Q4?" via `dolt_history` join |
| **Federated validation** | `dolt pull` remote ledger, query entity chains locally |
| Ledger compaction with proof | `dolt_history` proves faithful compaction |
| **Native git remotes** | Push/pull database state via standard git repos — federation transport |
| **Cross-town work sharing** | Two towns push/pull from shared git remote |
| **Ledger publication** | Push closed-beads-only branch to public repo |

---

## Part 6: Gas Town Current State (2026-02-11)

### What's Working

- Dolt SQL Server as the **only** access method — embedded mode fully removed
- Centralized data directory at `~/gt/.dolt-data/` with per-rig subdirectories
- `gt daemon` auto-starts, monitors, and auto-restarts the Dolt server
- Server commands: `gt dolt start`, `gt dolt stop`, `gt dolt status`, `gt dolt logs`
- Branch-per-polecat write concurrency: 50 concurrent writers tested clean (see Part 12)
- Daemon health checks every 30s with exponential backoff on crash restart
- Migration tool (`bd migrate dolt`) tested on lab VM, bugs fixed
- All 4+ databases live and serving (hq, beads, gastown, wyvern, plus test rigs)
- **Git remote support available** (Dolt v1.81.8) — needs integration into Gas Town

**Note on standalone Beads**: The `bd` CLI for standalone use (outside Gas Town) still
retains embedded Dolt as an option. Embedded removal applies to Gas Town only — standalone
users may not have a Dolt server running. This distinction is in the `bd` codebase, not `gt`.

### Server Management

```bash
# Daemon manages server lifecycle automatically (preferred)
gt daemon start     # Daemon auto-starts Dolt server

# Manual management (for debugging or one-off use)
gt dolt start       # Start the Dolt SQL server (port 3307)
gt dolt stop        # Stop the server
gt dolt status      # Check server status, list databases
gt dolt logs        # View server logs
gt dolt sql         # Open SQL shell
gt dolt init-rig X  # Initialize a new rig database
gt dolt list        # List all rig databases
gt dolt migrate     # Migrate from old .beads/dolt/ layout
```

### Architecture

```
~/gt/                           ← Town root
├── .dolt-data/                 ← Centralized Dolt data directory
│   ├── hq/                     ← Town beads (hq-* prefix)
│   │   └── .dolt/              ← Dolt internals (+ remote config)
│   ├── gastown/                ← Gastown rig (gt-* prefix)
│   ├── beads/                  ← Beads rig (bd-* prefix)
│   └── wyvern/                 ← Wyvern rig (wy-* prefix)
├── daemon/
│   ├── dolt.pid                ← Server PID file (daemon-managed)
│   ├── dolt-server.log         ← Server log
│   └── dolt-state.json         ← Server state
├── mayor/
│   └── daemon.json             ← Daemon config (dolt_server + dolt_remotes)
└── [rigs]/                     ← Rig directories (code, not data)
```

The Dolt server runs with `--data-dir ~/.dolt-data`, making each subdirectory
a separate database accessible via `USE <rigname>` in SQL. The daemon ensures
the server is running on every heartbeat (3-minute interval) and auto-restarts
on crash.

---

## Part 7: Configuration

### Server Configuration

The Dolt server is configured via `gt dolt` commands. Key settings:

| Setting | Default | Description |
|---------|---------|-------------|
| Port | 3307 | MySQL protocol port (avoids conflict with MySQL on 3306) |
| User | root | Default Dolt user (no password for localhost) |
| Data dir | `~/.dolt-data/` | Contains all rig databases |
| Log file | `~/gt/daemon/dolt.log` | Server log output |
| PID file | `~/gt/daemon/dolt.pid` | Process ID for management |

### Connection String

```
root@tcp(127.0.0.1:3307)/        # Server root
root@tcp(127.0.0.1:3307)/gastown # Specific rig database
```

### Sync Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| `dolt-native` | Pure Dolt server, git remotes for backup | Gas Town (current default) |
| `git-portable` | Dolt + git remotes | Standalone Beads with remote backup |

### Conflict Resolution

Default: `newest` (most recent `updated_at` wins, like Google Docs).

Per-field strategies available:
- **Arrays** (labels, waiters): `union` merge
- **Counters** (compaction_level): `max`
- **Human judgment** (estimated_minutes): `manual`

---

## Part 8: Technical Details

### Dolt Commit Strategy

Default: auto-commit on every write (safe, auditable). Agents can batch:

```go
store.SetAutoCommit(false)
defer store.SetAutoCommit(true)
store.UpdateIssue(ctx, issue1)
store.UpdateIssue(ctx, issue2)
store.Commit(ctx, "Batch update: processed 2 issues")
```

This is ZFC-compliant: Go provides a safe default, agents can override.

### Multi-Table Schema

```sql
CREATE TABLE issues (
    id VARCHAR(64) PRIMARY KEY,
    type VARCHAR(32),
    title TEXT,
    description TEXT,
    status VARCHAR(32),
    priority INT,
    owner VARCHAR(255),
    assignee VARCHAR(255),
    labels JSON,
    parent VARCHAR(64),
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    closed_at TIMESTAMP
);

CREATE TABLE mail (
    id VARCHAR(64) PRIMARY KEY,
    thread_id VARCHAR(64),
    from_addr VARCHAR(255),
    to_addrs JSON,
    subject TEXT,
    body TEXT,
    sent_at TIMESTAMP,
    read_at TIMESTAMP
);

CREATE TABLE channels (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255),
    type VARCHAR(32),
    config JSON,
    created_at TIMESTAMP
);
```

### Bootstrap Flow

**Gas Town (existing install — migration from embedded):**
1. Run `gt dolt migrate` to move town-level `.beads/dolt/` to `~/.dolt-data/hq/`
2. Manually move rig-level databases: `mv <rig>/mayor/rig/.beads/dolt/beads ~/.dolt-data/<rigname>`
3. Update all `metadata.json` files: `dolt_mode: "server"`, `dolt_database: "<rigname>"`
4. Enable `dolt_server` in `mayor/daemon.json`, restart daemon

**Fresh Gas Town install:**
1. `gt dolt init-rig hq` — initialize town-level database
2. `gt dolt init-rig <rigname>` — initialize per-rig databases
3. Enable `dolt_server` in `mayor/daemon.json`
4. `gt daemon start` — daemon auto-starts the Dolt server

**Fresh Beads install (standalone):**
1. `dolt sql-server --port 3307 --data-dir <path>` — start server
2. `bd` connects via MySQL protocol, creates database and schema automatically

### Error Recovery

| Failure | Recovery |
|---------|----------|
| Disk crash / data loss | `dolt clone <git-remote>` to rebuild from remote |
| Dolt corruption | `dolt clone <git-remote>` — remote is the backup |
| Merge conflict | Auto-resolve (newest wins) or `dolt_conflicts` table |
| Server crash | Daemon auto-restarts; data durable on disk |
| Remote unreachable | Local operations continue; push retried on next heartbeat |

---

## Part 9: Dolt Team Clarifications

### From Tim Sehn (CEO) and Dustin Brown (engineer), January 2026

#### Concurrency

> **Dustin**: Concurrency with the driver is supported, multiple goroutines can
> write to the same embedded Dolt.
>
> **Tim**: Concurrency is handled by standard SQL transaction semantics.

#### Scale

> **Tim**: Little scale impact from high commit rates. Don't compact before >1M
> commits. Run `dolt_gc()` when the journal file (`vvvvvvvvvvv...` in `.dolt/`)
> exceeds ~50MB.

#### Branches

> **Tim**: Branches are just pointers to commits, like Git. Millions of branches
> without issue.

#### Merge Performance

> **Tim**: We merge the Prolly Trees — much smarter/faster than sequential replay.
> See: https://www.dolthub.com/blog/2025-07-16-announcing-fast-merge/

#### Replication

> **Tim**: All async, push/pull Git model not binlog. Can set up "push on write"
> or manual pushes. Works on dolt commits, not transaction commits.

#### Hosting

> **Tim**: Hosted Dolt (like AWS RDS) starts at $50/month. DoltHub Pro (like
> GitHub) is free for first 1GB, $50/month + $1/GB after.
> See: https://www.dolthub.com/blog/2024-08-02-dolt-deployments/

### From Dustin Brown, February 2026 — Git Remote Support

> **Dustin**: Git remotes are now supported in Dolt v1.81.8, and they work like
> any other Dolt remote, the Dolt remote interface has not changed.
>
> To use Git remotes, there is a hard dependency on the Git CLI binary and being
> on your PATH, as Dolt calls this under-the-hood. For this reason, if your Git CLI
> is already credentialed to your Git remote (like GitHub), everything will just work.
>
> One exception: if your Git credentials require any inline argument for auth, like
> a username and/or password that it collects from STDIN, the Dolt remote operations
> do not currently work. [Tracking issue filed.]
>
> The other requirement for using Git remotes is your Git repository must exist on
> the remote and must contain at least one branch.
>
> You do not necessarily need separate Git repos for your Dolt remote data and your
> source code, they can live safely together in the same Git remote repo without
> affecting each other.
>
> When Dolt pushes to the Git remote, it creates a custom 'ref' that contains only
> the Dolt remote data, and nothing else. This ref and Dolt data is orthogonal to
> the source files checked into the Git repo on the other branches.
>
> This special ref is not fetched, pulled or cloned when the Git repo is fetched,
> pulled, or cloned. Instead, Dolt uses Git internal commands to manipulate and
> maintain this ref directly.

**Key operational details from Dustin:**

```bash
# Adding a git remote to a Dolt database
dolt remote add origin git@github.com:org/repo.git     # SSH
dolt remote add origin https://github.com/org/repo.git # HTTPS

# Internal representation
dolt remote -v
# origin git+ssh://git@github.com/org/repo.git

# Works via SQL stored procedures too
# CALL DOLT_REMOTE('add', 'origin', 'https://github.com/org/repo.git')

# Then standard push/pull/fetch/clone
dolt push origin main
dolt pull origin main
dolt clone https://github.com/org/repo.git
```

---

## Part 10: Roadmap

### Completed

- **Dolt Server mode**: Required for all access. Commands: `gt dolt start/stop/status`
- **Centralized data directory**: `~/.dolt-data/` with per-rig subdirectories
- **Migration tooling**: `gt dolt migrate` + `bd migrate dolt` (tested on lab VM, bugs fixed)
- **Daemon integration**: Dolt server auto-starts/stops/restarts via `gt daemon`
- **All databases migrated**: hq, beads, gastown, wyvern (plus test rigs)
- **Embedded mode removed from Gas Town**: No embedded fallback in `gt`. Server-only.
- **Branch-per-polecat for write concurrency**: Each polecat gets a Dolt branch at sling
  time. 50 concurrent writers tested clean. See Part 12.
- **Doctor checks**: `gt doctor` validates Dolt server health, stale redirects, routes,
  and beads integrity (44 check types total)

### Immediate

1. **Git remote integration**: Dolt v1.81.8 shipped git remote support. Integration work:
   - Upgrade Dolt binary to v1.81.8+
   - Configure remotes for each rig database (see Part 13)
   - Add `dolt push` to daemon heartbeat loop
   - Add `gt dolt remote` subcommands for management
   - Add `gt dolt push` / `gt dolt pull` for manual operations
   - Update session close protocol: `dolt push` replaces `bd sync --flush-only`
2. **JSONL removal**: Remove all JSONL code paths (`bd sync`, export, import).
   Git remotes fully replace JSONL for backup and recovery.
3. **Gas Town pristine state**: Clean up old `.beads/dolt/` directories, stale
   SQLite, misrouted beads, stale JSONL files.
4. **Dolt optimistic lock fix (upstream)**: The Dolt team is working on a fix for
   the `optimistic lock failed on database Root update` error. Branch-per-polecat
   eliminates this as a blocking issue for Gas Town, but the upstream fix will
   benefit standalone `bd` users and simplify the architecture further.

### Next

- Closed-beads-only ledger publication (separate Dolt branch → public git remote)
- Remove embedded mode from `bd` CLI (Gas Town done; standalone beads next)
- Ship `bd` release (server-only binary option → ~20MB vs ~120MB)
- Cross-town federation via shared git remotes
- Per-rig server option for isolation (if demand emerges)

### Future

- Design Plane / The Commons architecture (with Brendan Hopper)
- Cross-town delegation via design plane
- Charsheet / boot block integration via Dolt remotes to DoltHub

---

## Decision Log

| Decision | Rationale | Date |
|----------|-----------|------|
| Dolt only, retire SQLite | One backend, better conflicts | 2026-01-15 |
| JSONL retired as source of truth | Dolt is truth; JSONL is interim backup | 2026-01-15 |
| ~~Embedded Dolt default~~ | ~~No server process, just works~~ | ~~2026-01-30~~ |
| **Server mode is default** | Embedded file locking causes hangs under multi-agent concurrency | 2026-02-05 |
| **Embedded mode removed entirely** | No fallback — data lives on server, embedded can't access it. Enables major code simplification. | 2026-02-05 |
| **Daemon manages Dolt server** | Auto-start on heartbeat, auto-restart on crash, graceful shutdown | 2026-02-05 |
| **One server per town** | Centralized `.dolt-data/` serves all rigs; simple ops, single process | 2026-02-05 |
| Single binary (pure-Go) | No CGO needed for local ops | 2026-01-30 |
| ~~Dolt-in-git replaces JSONL~~ | ~~Native binary in git, cell-level merge~~ | ~~2026-01-30~~ |
| **Git remotes replace JSONL** | Dolt v1.81.8 ships git remote support. `refs/dolt/data` is orthogonal to source code. Simpler than the original dolt-in-git plan (no merge driver, no file splitting). Eliminates JSONL entirely. | 2026-02-11 |
| Three data planes | Different data needs different transport | 2026-01-29 |
| Closed-beads-only ledger | Operational churn stays local | 2026-01-30 |
| Newest-wins conflict default | Matches Google Docs mental model | 2026-01-15 |
| Auto-commit per write | Safe default, agents can batch | 2026-01-15 |
| ~~dolt_diff() for export~~ | ~~No dirty_issues table; Dolt IS the tracker~~ | ~~2026-01-16~~ |
| ~~Per-worktree export state~~ | ~~Prevent polecats exporting each other's work~~ | ~~2026-01-16~~ |
| Apache 2.0 compatible with MIT | Standard attribution, no architectural impact | 2026-01-13 |
| **Branch-per-polecat** | Per-worker Dolt branches eliminate optimistic lock contention at 50+ concurrent writers. Tested 2026-02-08. | 2026-02-08 |
| **Dolt team fixing optimistic lock** | Upstream fix in progress for `Error 1105: optimistic lock failed`. Branch-per-polecat is the Gas Town workaround; upstream fix benefits standalone bd users. | 2026-02-09 |
| **JSONL fully retired** | Git remotes provide backup and recovery. No JSONL export, import, sync, or daemon. Code paths can be removed. | 2026-02-11 |
| **Per-rig git remotes** | Each rig database pushes to its own GitHub repo. Rig source code and Dolt data coexist in same repo via `refs/dolt/data`. | 2026-02-11 |

---

## Part 12: Branch-Per-Polecat (Write Concurrency Fix)

> Added 2026-02-08 by Mayor. **Implemented and deployed.**
> Stress test evidence in `~/gt/mayor/dolt-branch-test.go`.

### The Problem

Dolt's optimistic locking causes `Error 1105: optimistic lock failed on database Root
update` when multiple agents commit to the same branch concurrently. At 20 concurrent
writers on `main`, 50% fail. The Phase 0 band-aid (10 retries with exponential backoff)
helps but doesn't solve the architectural ceiling.

The Dolt team is aware of this issue and is working on an upstream fix to the optimistic
lock mechanism. However, branch-per-polecat is the correct architectural solution
regardless — it eliminates contention by design rather than by retry.

### The Fix

Each polecat gets its own Dolt branch. Branches are independent Root pointers — no
contention between branches. Merges are sequential (refinery or gt done).

```
gt sling <bead> <rig>
  └─ CALL DOLT_BRANCH('polecat-furiosa-1707350000')
     └─ Polecat env: BD_BRANCH=polecat-furiosa-1707350000
        └─ bd connects, runs: CALL DOLT_CHECKOUT('polecat-furiosa-1707350000')
           └─ All bd creates/updates/closes write to polecat branch
              └─ Zero contention with other polecats

gt done
  └─ CALL DOLT_CHECKOUT('main')
     └─ CALL DOLT_MERGE('polecat-furiosa-1707350000')
        └─ CALL DOLT_BRANCH('-D', 'polecat-furiosa-1707350000')
```

### Stress Test Results

| Concurrency | Single Branch (main) | Per-Worker Branches | Sequential Merge |
|-------------|---------------------|--------------------|-----------------|
| 10 | 10/10 (100%) | 10/10 (100%) | 10/10 (100%) |
| 20 | 10/20 (50%) | **20/20 (100%)** | 20/20 (100%) |
| 50 | 25/50 (50%) | **50/50 (100%)** | 50/50 (100%) |

Each worker performed 5 insert+commit cycles. All workers launched simultaneously
via barrier. 50 workers = 250 total Dolt commits, all successful, in 2 seconds.
Sequential merge of all 50 branches completed in 312ms.

### Why This Works

Tim Sehn (Dolt CEO): "Branches are just pointers to commits, like Git. Millions of
branches without issue." And: "We merge the Prolly Trees — much smarter/faster than
sequential replay."

Each branch has its own Root. DOLT_COMMIT on branch A doesn't touch branch B's Root.
The optimistic lock only fires when two writers try to update the SAME Root. With
per-polecat branches, this never happens.

### Implementation (Gas Town side) — Complete

1. `gt sling` (`internal/polecat/session_manager.go`): After worktree creation, creates
   Dolt branch via SQL: `CALL DOLT_BRANCH('polecat-<name>-<timestamp>')`
2. Sets `BD_BRANCH` env var in the polecat's tmux session environment
3. `gt done` (`internal/cmd/done.go`): Reads `BD_BRANCH`, merges branch to main via
   `CALL DOLT_MERGE()`, deletes branch, unsets env var
4. `gt polecat nuke`: deletes branch as part of cleanup (idempotent)
5. Branch lifecycle managed in `internal/doltserver/doltserver.go`:
   `CreatePolecatBranch`, `MergePolecatBranch`, `DeletePolecatBranch`

### Implementation (Beads side) — Complete

1. On connection open, checks `BD_BRANCH` env var
2. If set, runs `CALL DOLT_CHECKOUT('<branch>')` on the connection
3. All subsequent operations happen on that branch transparently
4. No other bd code changes needed — SQL operations are branch-agnostic

### Merge Conflicts

Conflicts should be rare: each polecat works on different issues (different rows).
If conflicts occur (e.g., two polecats update the same parent epic's child count):
- Dolt's `dolt_conflicts` table captures them
- `newest-wins` resolution applies (our default)
- Worst case: retry the merge after resolving

### Relationship to AT War Rigs

Dolt branches and AT War Rigs are orthogonal solutions to different problems:
- **Branches**: Solve write contention at the storage layer (launch-track)
- **AT War Rigs**: Solve coordination overhead at the session layer (post-launch)

Both could coexist. With branches, AT War Rigs become less urgent — the Dolt
contention ceiling is removed regardless of how sessions are managed.

### Interaction with Git Remotes

Only `main` branch gets pushed to git remotes. Polecat branches are ephemeral —
they're created on sling, merged to main on completion, and deleted. The push
cycle runs after polecat branches have merged, so the remote always gets a
consistent main-branch view.

```
Polecat branches (local only):
  polecat-furiosa-170... ──merge──> main ──dolt push──> git remote
  polecat-nux-170...     ──merge──> main ──dolt push──> git remote
```

---

## Part 11: Code Simplification (Embedded Removal)

> **Gas Town status**: Complete — `gt` has no embedded Dolt code paths.
> **Standalone Beads status**: Pending — `bd` CLI still retains embedded for users without a server.

Removing embedded mode entirely enables significant cleanup across the `bd` codebase.
This is not incremental — it's a wholesale removal of a code path that no longer executes.

### What Gets Removed

| Component | File | What |
|-----------|------|------|
| **Embedded Dolt driver** | `go.mod` | `go-dolt` dependency — largest single dep in the binary |
| **Advisory lock layer** | `access_lock.go` | Entire file: shared/exclusive flock, `AcquireAccessLock()`, `dolt-access.lock` |
| **Embedded connection** | `store.go` | `openEmbeddedConnection()`, `withEmbeddedDolt()`, embedded backoff/retry |
| **UOW1/UOW2 init path** | `store.go` | Embedded-only `CREATE DATABASE` + schema init via embedded driver |
| **Server fallback** | `factory_dolt.go` | `isServerConnectionError()` fallback to embedded (lines 39-55) |
| **JSONL bootstrap** | `factory_dolt.go` | `bootstrapEmbeddedDolt()`, `hasDoltSubdir()` |
| **JSONL export** | various | `bd sync --flush-only`, JSONL serialization, export state tracking |
| **Read-only distinction** | `main.go` | `isReadOnlyCommand()` map — server handles concurrency natively |
| **Semaphore hacks** | gt hooks, `main.go` | `MaxConcurrentBd=3` (G1/G5) — no contention with server |
| **Lock timeout config** | `main.go` | 5s/15s read/write timeouts — no advisory locks |
| **`BD_SKIP_ACCESS_LOCK`** | `store.go` | Debug env var for bypassing flock |
| **Embedded build tags** | Various | `//go:build cgo` guards |

### Impact

| Metric | Before | After (estimated) |
|--------|--------|-------------------|
| Binary size | ~120MB (embedded Dolt engine) | ~20MB (MySQL client only) |
| Build time | ~90s (CGO, Dolt compilation) | ~15s (pure Go, no CGO) |
| `store.go` complexity | Two code paths (embedded + server) | One code path (server only) |
| Lock-related code | ~300 lines across 4 files | 0 |
| External deps | go-dolt + go-sql-driver/mysql | go-sql-driver/mysql only |

### What Stays

- `openServerConnection()` in `store.go` — the MySQL connection path
- `initSchemaOnDB()` — schema creation (runs via MySQL now)
- `dolt.Config` struct — simplified (remove `Path`, `OpenTimeout`, embedded fields)
- `metadata.json` config — `dolt_mode` field becomes vestigial (always server)

### Migration Path

1. Remove embedded code paths from `store.go` and `factory_dolt.go`
2. Remove `access_lock.go` entirely
3. Remove `go-dolt` from `go.mod`
4. Remove CGO build tags
5. Simplify `main.go` — remove `isReadOnlyCommand()`, lock timeout logic
6. Remove semaphore infrastructure from gt hooks
7. Update `metadata.json` handling — `dolt_mode: "server"` becomes the only valid value
8. Clean up old `.beads/dolt/` directories and `dolt-access.lock` files
9. Remove JSONL export/sync code (replaced by git remotes)

---

## Part 13: Git Remote Architecture

> Added 2026-02-11 by Mayor. Design for integrating Dolt git remotes into Gas Town.

### Topology: Per-Rig Remotes

Each rig's Dolt database gets a git remote pointing to that rig's GitHub repo.
Source code lives on regular branches; Dolt data lives in `refs/dolt/data`.

```
~/.dolt-data/
├── hq/           → git@github.com:<owner>/gt-hq.git          (or dedicated town repo)
├── gastown/      → git@github.com:anthropics/gas-town.git     (same repo as source code)
├── beads/        → git@github.com:anthropics/beads.git        (same repo as source code)
├── wyvern/       → git@github.com:steveyegge/wyvern.git       (same repo as source code)
└── sky/          → git@github.com:<owner>/sky.git
```

**Why per-rig remotes (not one shared remote)?**
- Each rig is a separate project with its own GitHub repo
- Dolt data coexists in the same repo — no extra repos to manage
- Access control follows existing repo permissions
- Federation: other towns `dolt clone` from the repo they're interested in
- Clean separation: if you share a rig, you share its beads data

**HQ beads (town-level):**
Town beads (`hq-*` prefix) don't map to a single project repo. Options:

| Option | Pros | Cons |
|--------|------|------|
| Dedicated `gt-hq` repo | Clean separation, private | Extra repo to manage |
| In the `gas-town` repo | No extra repo | Conflates town beads with gastown beads |
| Town root as git repo | Town config + beads together | ~/gt may not be a git repo |

Recommended: **Dedicated `gt-hq` repo** (private). Town beads are organizational
data (mayor mail, convoys, agent identity) — they belong in their own repo with
their own access control.

### Remote Configuration

**Initial setup (per Dolt database):**

```bash
# For each rig database:
cd ~/.dolt-data/gastown
dolt remote add origin git@github.com:anthropics/gas-town.git
dolt push origin main

# For HQ:
cd ~/.dolt-data/hq
dolt remote add origin git@github.com:steveyegge/gt-hq.git
dolt push origin main
```

**Via SQL (from Dolt SQL server):**

```sql
USE gastown;
CALL DOLT_REMOTE('add', 'origin', 'git@github.com:anthropics/gas-town.git');
CALL DOLT_PUSH('origin', 'main');
```

**Verification:**

```bash
# From any clone of the source repo:
git ls-remote origin refs/dolt/data
# Should show: <hash>    refs/dolt/data
```

### Daemon Integration

The daemon's heartbeat loop adds `dolt push` for configured remotes:

```
Daemon Heartbeat (every 3 minutes):
  1. Check Dolt server health → restart if crashed
  2. Check Deacon health → poke if needed
  3. Push Dolt remotes → for each configured rig:
     a. Connect to Dolt server
     b. USE <rig_database>
     c. CALL DOLT_PUSH('origin', 'main')
     d. Log success/failure (don't crash on remote errors)
```

**Configuration in `mayor/daemon.json`:**

```json
{
  "dolt_server": { "enabled": true, "port": 3307 },
  "dolt_remotes": {
    "enabled": true,
    "push_on_heartbeat": true,
    "databases": {
      "hq": "git@github.com:steveyegge/gt-hq.git",
      "gastown": "git@github.com:anthropics/gas-town.git",
      "beads": "git@github.com:anthropics/beads.git"
    }
  }
}
```

**Error handling:**
- Remote push failure is non-fatal (logged, retried next heartbeat)
- Network unreachable → local operations continue uninterrupted
- Auth failure → log error, alert via mail to mayor
- This is "graceful degradation" — the operational plane never stops for remote issues

### CLI Commands

```bash
# Remote management
gt dolt remote add <rig> <git-url>    # Configure remote for a rig database
gt dolt remote list                    # Show configured remotes
gt dolt remote remove <rig>            # Remove remote configuration

# Manual push/pull
gt dolt push [rig]                     # Push to remote (all rigs if none specified)
gt dolt pull [rig]                     # Pull from remote

# Recovery
gt dolt clone <git-url> <rig>          # Clone database from remote
```

### Push/Pull Semantics

**Push** (`dolt push origin main`):
- Pushes the current state of `main` branch to `refs/dolt/data` on the remote
- Only `main` gets pushed — polecat branches are local and ephemeral
- Polecat branches merge to main before the push cycle runs
- Idempotent: pushing the same state is a no-op

**Pull** (`dolt pull origin main`):
- Fetches `refs/dolt/data` from the remote and merges into local `main`
- Dolt handles merge conflicts (cell-level merge, newest-wins default)
- Use case: recovery, or multi-town federation

**Clone** (`dolt clone <git-url>`):
- Creates a fresh Dolt database from the remote's `refs/dolt/data`
- Use case: disaster recovery, new town bootstrapping, federation

### Federation via Git Remotes

Git remotes are the transport layer for HOP federation:

```
Town A (Steve)                           Town B (Alice)
┌──────────────────┐                    ┌──────────────────┐
│ Dolt Server      │                    │ Dolt Server      │
│ gastown/         │                    │ gastown/         │
│                  │  dolt push         │                  │
│  main ───────────┼──────> GitHub ────>│ (dolt pull)      │
│                  │                    │                  │
└──────────────────┘                    └──────────────────┘
                         │
                         ▼
                  ┌──────────────────┐
                  │ GitHub repo      │
                  │ refs/dolt/data   │
                  │ (shared remote)  │
                  └──────────────────┘
```

**Phase 1 (now)**: Each town pushes to its own repo for backup.
**Phase 2**: Towns can add each other's repos as Dolt remotes to pull data.
**Phase 3**: Selective sharing — push only closed/public beads to shared repos.

### Interaction with Data Planes

| Plane | Remote Type | Access | Push Trigger |
|-------|-------------|--------|-------------|
| **Operational** | Private git remote | Town owner only | Daemon heartbeat (periodic) |
| **Ledger** | Shared/public git remote | Federated (cross-town) | On bead close (future) |
| **Design** | Commons git remote | Global (maximally visible) | On create/update (future) |

**Phase 1 implementation pushes all data to the operational remote.** Ledger/Design
plane separation comes later, either via:
- Separate Dolt branches pushed to different remotes
- Separate Dolt databases (ledger DB with only closed beads)
- Query-time filtering (push everything, consumers filter by status)

### Session Close Protocol (Updated)

The session close checklist changes from JSONL to git remote:

```
Before:
[ ] bd sync --flush-only    (export beads to JSONL)

After:
[ ] dolt push               (push beads to git remote)
```

For agents, this can be automated in `gt done`:
1. Merge polecat Dolt branch to main
2. Push main to git remote
3. Clean up worktree and session

### Security Considerations

- **SSH keys**: Non-interactive auth via SSH keys (required — interactive prompts don't work)
- **Private repos**: Operational plane data stays in private repos
- **Public repos**: Only ledger plane (closed beads) goes to public repos (future)
- **No data leakage**: `refs/dolt/data` is not fetched by `git clone` or `git pull` —
  only `dolt clone` / `dolt pull` access it. Regular repo cloners don't see beads data.
- **Access control**: Follows GitHub repo permissions. Read access to repo → can `dolt pull`.
  Write access → can `dolt push`.

### HOP Alignment

This architecture maps directly to HOP concepts:

| HOP Concept | Git Remote Implementation |
|-------------|--------------------------|
| Entity chain (CV) | Dolt DB pushed to entity's git remote |
| Organization chain | Town HQ database pushed to town repo |
| Project chain | Rig database pushed to project repo |
| Federation | `dolt remote add` + `dolt pull` from peer repos |
| Ledger publication | Push closed-beads-only branch to public remote |
| Cross-chain refs | `hop://` URIs resolve to git remote URLs |
| Boot blocks | Charsheet data loaded via `dolt clone` from DoltHub/GitHub |

The git remote transport satisfies FEDERATION.md's requirement for "chain format
flexibility" — the federation protocol only cares about work block format at chain
boundaries, and Dolt natively handles the internal representation.
