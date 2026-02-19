# Test Plan: Daemon Convoy Management

> Test engineering analysis for the ConvoyManager (event-driven + stranded scan)

**Architecture (SDK)**: ConvoyManager uses beads SDK `GetAllEventsSince()` for event polling instead of the removed `bd activity` CLI. Observer uses `GetDependentsWithMetadata`, `GetDependenciesWithMetadata`, `GetIssue`, `GetIssuesByIDs` instead of bd CLI.

---

## Critical Invariants

| Invariant | Category | Blast Radius | Currently Tested? |
|-----------|----------|--------------|-------------------|
| Issue close triggers `CheckConvoysForIssue` for tracking convoys | Data | High | Yes (`TestEventPoll_DetectsCloseEvents`) |
| Non-close events produce zero side effects | Safety | Low | Partial (`TestEventPoll_SkipsNonCloseEvents` has no subprocess negative assertion) |
| High-water mark advances monotonically (no duplicate processing) | Data | High | Implicit (pollEvents updates lastEventID) |
| Convoy check is idempotent (already-closed no-op) | Data | Low | Yes (convoy_empty_test) |
| Stranded convoys with ready work get fed via `gt sling` | Liveness | High | Yes (`TestScanStranded_FeedsReadyIssues`) |
| Empty stranded convoys get auto-closed via `gt convoy check` | Data | Medium | Yes (`TestScanStranded_ClosesEmptyConvoys`) |
| Scan continues after individual dispatch failure | Liveness | Medium | Yes (`TestScanStranded_DispatchFailure`) |
| Poll failure logs and retries on next interval | Liveness | Medium | Partial (`GetAllEventsSince` retry path not explicitly asserted) |
| Context cancellation stops both goroutines cleanly | Liveness | High | Yes (lifecycle + stop-timeout tests) |
| One issue fed per convoy per scan call (no batch overflow) | Safety | Medium | Implicit (single-issue tested, multi-issue not) |
| feedFirstReady skips issues with unknown prefix/rig | Safety | Medium | No |
| Scan interval defaults to 2min when 0 or negative | Data | Low | Yes (`TestConvoyManager_ScanInterval_Configurable`) |
| `Stop()` is idempotent (double-call safe) | Safety | Low | Yes (`TestConvoyManager_DoubleStop_Idempotent`) |

---

## Failure Modes

### Event Poll (`runEventPoll` / `pollEvents`)

| Failure Mode | Likelihood | Detection | Recovery | Current Coverage |
|--------------|------------|-----------|----------|------------------|
| GetAllEventsSince returns error | Low | Logged | Retry next poll interval | Implicit |
| Beads store nil (no Dolt) | Medium | runEventPoll returns early | Stranded scan still runs | Yes |
| Close event with empty issue_id | Low | Skipped in pollEvents | N/A | No |
| CheckConvoysForIssue panics | Low | Process crash | Daemon restart | No |
| Context cancelled during poll | Low | ctx.Done() | Clean exit | Implicit via lifecycle tests |

### Stranded Scan (`runStrandedScan` / `scan`)

| Failure Mode | Likelihood | Detection | Recovery | Current Coverage |
|--------------|------------|-----------|----------|------------------|
| gt convoy stranded returns error | Low | Logged, skip cycle | Retry next interval | No |
| gt convoy stranded returns invalid JSON | Low | Logged, skip cycle | Retry next interval | No |
| gt sling dispatch fails | Medium | Logged | Continue to next convoy | Yes |
| gt convoy check fails for empty convoy | Low | Logged | Continue to next convoy | No |
| beads.ExtractPrefix returns "" | Low | Logged, skip issue | Continue | No |
| beads.GetRigNameForPrefix returns "" | Low | Logged, skip issue | Continue | No |
| Context cancelled mid-iteration over stranded list | Low | ctx.Done() select | Clean exit | No |
| Ticker fires during shutdown | Low | ctx.Done() check | Clean exit | No |
| Interval = 0 or negative | Low | Config validation | Use default | Yes |

### Lifecycle

| Failure Mode | Likelihood | Detection | Recovery | Current Coverage |
|--------------|------------|-----------|----------|------------------|
| Stop() called before Start() | Low | wg.Wait() returns immediately | Safe (0 goroutines) | No |
| Double Stop() | Low | cancel() idempotent, wg.Wait() no-ops | Safe | No |
| Start() called twice | Low | wg.Add(2) again, 4 goroutines | Bug — no guard | No |
| Daemon starts with missing town root | Low | Config load fails | Daemon fails to start | No |
| Manager blocks shutdown | Low | Shutdown timeout | Force kill | Yes (`TestDaemon_StopsManagerAndScanner`) |

---

## Existing Test Coverage

All tests in `convoy_manager_test.go`, `convoy/store_test.go`, and `daemon_test.go`:

| Test | Type | What It Proves |
|------|------|----------------|
| `TestEventPoll_DetectsCloseEvents` | Integration | pollEvents detects EventClosed, logs and calls CheckConvoysForIssue |
| `TestEventPoll_SkipsNonCloseEvents` | Integration | Create-only events produce no close detection |
| `TestConvoyManager_DoubleStop_Idempotent` | Unit | Stop() called twice does not deadlock |
| `TestGetTrackingConvoys_FiltersByTracksType` | Integration | Only "tracks" deps returned, blocks filtered out |
| `TestIsConvoyClosed_ReturnsCorrectStatus` | Integration | Open vs closed status from SDK |
| `TestManagerLifecycle_StartStop` | Smoke | Start+Stop completes without deadlock |
| `TestScanStranded_FeedsReadyIssues` | Integration | scan() → findStranded → feedFirstReady → gt sling logged |
| `TestScanStranded_ClosesEmptyConvoys` | Integration | scan() → findStranded → closeEmptyConvoy → gt convoy check logged |
| `TestScanStranded_NoStrandedConvoys` | Smoke | Empty stranded list doesn't crash (no side-effect assertion) |
| `TestScanStranded_DispatchFailure` | Integration | First sling fails → error logged, scan continues to second convoy |
| `TestConvoyManager_ScanInterval_Configurable` | Unit | 0 → default (2min), custom value preserved |
| `TestStrandedConvoyInfo_JSONParsing` | Unit | JSON struct round-trip for stranded convoy |
| `TestDaemon_StartsManagerAndScanner` | Integration | Daemon-style start+stop with mock bd/gt |
| `TestDaemon_StopsManagerAndScanner` | Integration | Stop completes within 5s (no hang) |

### Test Helpers

- **setupTestStore** (convoy/store_test.go, daemon/convoy_manager_test.go): Opens real beads Dolt database in temp dir. Skips if CGO/Dolt unavailable. Requires `SetConfig(issue_prefix)` before CreateIssue.

---

## Recommended Test Strategy

### Mock Strategy

Uses patterns from existing daemon tests:

- Temp dir for town root with `.beads/` and optional `routes.jsonl`
- Mock `bd` and `gt` binaries as shell scripts in temp bin dir
- Prepend mock bin dir to PATH via `t.Setenv`
- Mock scripts log invocations to files for assertion
- Skip on Windows (`runtime.GOOS == "windows"`)

### Priority 1 — Fill gaps on high-blast-radius invariants

| Test | Type | Gap Addressed |
|------|------|---------------|
| `TestProcessLine_NonCloseEvent_VerifyNoSideEffects` | Unit | Non-close events should invoke zero gt/bd subcommands (assert log file absent) |
| `TestFeedFirstReady_MultipleReadyIssues_DispatchesOnlyFirst` | Unit | 3 ready issues → sling log contains only first issue ID |
| `TestFeedFirstReady_UnknownPrefix_Skips` | Unit | Issue prefix not in routes.jsonl → sling never called, error logged |
| `TestFeedFirstReady_UnknownRig_Skips` | Unit | Prefix resolves but rig lookup fails → sling never called |
| `TestFeedFirstReady_EmptyReadyIssues_NoOp` | Unit | ReadyIssues=[] despite ReadyCount>0 |

### Priority 2 — Error-path coverage

| Test | Type | Gap Addressed |
|------|------|---------------|
| `TestFindStranded_GtFailure_ReturnsError` | Unit | gt convoy stranded exits non-zero |
| `TestFindStranded_InvalidJSON_ReturnsError` | Unit | gt returns non-JSON stdout |
| `TestScan_FindStrandedError_LogsAndContinues` | Unit | scan() doesn't panic on findStranded error |
| `TestProcessLine_EmptyIssueID` | Unit | Close event with empty issue_id |

### Priority 3 — Lifecycle edge cases

| Test | Type | Gap Addressed |
|------|------|---------------|
| `TestScan_ContextCancelled_MidIteration` | Unit | Large stranded list + cancel mid-loop |
| `TestScanStranded_MixedReadyAndEmpty` | Unit | Heterogeneous stranded list routed correctly |

---

## Harness Scorecard

| Dimension | Score (1-5) | Key Gap |
|-----------|-------------|---------|
| Fixtures & Setup | 3 | `mockBdGtForManagerTest` covers processLine path; scan tests duplicate setup inline |
| Isolation | 4 | Temp dirs + `t.Setenv(PATH)` is solid; Windows correctly skipped; no shared state |
| Observability | 3 | Shell-script log files work but are indirect; some tests lack assertions (prove no-panic only) |
| Speed | 4 | All convoy-manager tests run quickly; no long-running interval waits in current suite |
| Determinism | 4 | No real timing dependencies; ticker tests use long intervals to avoid races |

---

## Tooling Recommendations

### Side-Effect Logger for Negative Tests

**Problem**: Tests like `NonCloseEvent_NoAction` and `NoStrandedConvoys` can't assert "nothing happened" because mock scripts don't track invocations consistently.

**Proposal**: Have mock scripts always write to a call log file. Tests that expect no side effects assert the log file doesn't exist or is empty.

**Compound Value**: Converts every assertion-free test into a real negative test. Trivial to adopt.

**Exists Today?**: Partially — `mockBdGtForManagerTest` writes `bd-calls.log` and `gt-check.log`, but the scan-path mocks don't.

**Priority**: P1

### Shared Mock Builder for Scan Tests

**Problem**: Each stranded scan test copies ~30 lines of shell script construction. Adding a new subcommand to gt requires updating every test.

**Proposal**: Extract a `mockGtForScanTest(t, opts)` helper that takes a config struct (stranded JSON response, sling exit code, log paths). Returns `townRoot` and log paths.

**Compound Value**: Every new scan test becomes 5 lines of setup instead of 30. Adding new gt subcommands is one change.

**Exists Today?**: No. `mockBdGtForManagerTest` covers only the processLine/CheckConvoysForIssue path.

**Priority**: P2

### Test Clock Injection

**Problem**: ConvoyManager uses `time.Ticker` with 2-minute default. Testing "runs at interval" requires waiting 2 minutes or injecting a clock.

**Proposal**: Add `clock` field to ConvoyManager (interface with `NewTicker(d)`) defaulting to real time. Tests inject fake clock with immediate tick.

**Compound Value**: All periodic daemon components benefit.

**Exists Today?**: No. Tests use long intervals (10min) to prevent ticker firing during test.

**Priority**: P3

---

## Next Actions

1. Convert assertion-free tests to negative tests (add call-log files, assert absent/empty)
2. Add `TestFeedFirstReady_MultipleReadyIssues_DispatchesOnlyFirst`
3. Add `TestFeedFirstReady_UnknownPrefix_Skips` and `_UnknownRig_Skips`
4. Add `TestFindStranded_GtFailure_ReturnsError` and `_InvalidJSON`
5. Extract `mockGtForScanTest` helper to deduplicate scan test setup
6. Add `TestScanStranded_MixedReadyAndEmpty`
