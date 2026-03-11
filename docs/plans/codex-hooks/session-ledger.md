# Session Ledger: codex-hooks

## Current Status

- Phase: delivery complete
- Branch: `integration/codex-hooks`
- Selected profiles: `general`, `go-development`
- Tracking: milestone mode
- Milestone status: implementation, verification, and ship milestones closed
- Review monitoring: hooked review workers exist, but parent verification found no live sessions and no completed review reports
- Final verification: targeted config/hooks/runtime checks passed; external sidecar review degraded with zero completed report artifacts

## Implementation Checklist

- Add a new built-in `codex-hooks` preset alongside the existing `codex` preset.
- Add minimal Codex hook templates for interactive and autonomous roles under `internal/hooks/templates/codex/`.
- Extend installer and preset tests for the new preset/provider behavior.
- Update provider integration docs for the opt-in experimental path and feature flag requirement.
- Verify with targeted Go tests and focused docs inspection.

## Milestone Self-Checks

### Slice 1
- **Goal:** Add the new preset, Codex hook templates, and targeted tests for their wiring.
- **Spec coverage:** Design -> Agent preset, Hook installer integration, Runtime behavior, Test coverage.
- **Proof model:** Red-green evidence. Start by adding targeted tests that fail without the new preset/template wiring, then implement until those tests pass.
- **Status:** complete
- **What changed:** Added the `codex-hooks` built-in preset in `internal/config/agents.go`, added minimal interactive/autonomous Codex hook templates under `internal/hooks/templates/codex/`, and extended preset/installer tests for the new preset name and role-aware template selection.
- **Evidence:** Initial red run failed on missing `AgentCodexHooks`; narrowed green runs passed for `./internal/config` targeted codex-hooks tests and `./internal/hooks -run TestInstallForRole_CodexRoleAware`.
- **Remaining risk:** The template schema is intentionally minimal and still depends on the current Codex hook file format matching the bead/spec assumptions.
- **Risks to watch:** Codex prompt delivery uses Gastown's positional prompt path; hook template schema must match the current Codex hook file shape.

### Slice 2
- **Goal:** Update user-facing docs for the opt-in `codex-hooks` path and verify the written guidance matches the implementation.
- **Spec coverage:** Design -> Documentation, Scope, Risks.
- **Proof model:** Alternate proof model: doc/code consistency check. Evidence will be exact docs text updates plus targeted inspection against the implemented preset/template behavior.
- **Status:** complete
- **What changed:** Updated `docs/agent-provider-integration.md` to document the experimental `codex-hooks` preset, the `[features].codex_hooks = true` prerequisite, the limited `SessionStart`/`Stop` scope, and the fact that default `codex` still follows the fallback/wrapper path.
- **Evidence:** `rg -n "codex-hooks|codex_hooks|gt-codex|Codex Hooks" docs/agent-provider-integration.md` plus targeted green checks after formatting: `go test ./internal/config -run 'TestBuiltinPresets|TestGetAgentPresetByName|TestRuntimeConfigFromPreset|TestIsKnownPreset|TestSupportsSessionResume|TestGetSessionIDEnvVar|TestGetProcessNames|TestListAgentPresetsMatchesConstants|TestAgentCommandGeneration|TestCodexHooksAgentPreset'`, `go test ./internal/hooks -run 'TestInstallForRole_CodexRoleAware'`, and `go test ./internal/runtime`.
- **Remaining risk:** Full-package `internal/config` still has the unrelated `TestAgentEnv_Dog` failure from earlier; feature-specific checks are green.

### Slice 3
- **Goal:** Verify the review-handoff state on the branch and record what is actually true from the parent session.
- **Spec coverage:** Review readiness, verification, and end-of-session review bookkeeping.
- **Proof model:** Alternate proof model: direct inspection of branch state, hook state, and the review artifact directory.
- **Status:** complete
- **What changed:** Verified that a checkpoint/review handoff exists on the branch, then corrected the ledger to distinguish hooked review work from completed review output.
- **Evidence:** `gt hook show gastown/furiosa`, `gt hook show gastown/rictus`, and `gt hook show gastown/dementus` show hooked review work; `gt peek` returned `session not found` for all three; `/Users/chall/gt/gastown/.runtime/reviews/codex-hooks/20260311-062955` contains only shared inputs with no review markdown outputs.
- **Remaining risk:** External review coverage is degraded; no sidecar review report exists yet, so final review synthesis cannot rely on reviewer findings.

## Commands Run + Outcomes

- `bd update gas-wisp-jemmm --status=in_progress` -> implementation stage claimed
- `bd update gas-7i2.2 --status=in_progress` -> implementation milestone marked in progress
- codebase inspection commands across `internal/config`, `internal/hooks`, `internal/runtime`, and docs -> implementation seams identified
- `go test ./internal/config ./internal/hooks ./internal/runtime` -> unrelated pre-existing failure in `internal/config` (`TestAgentEnv_Dog`), hooks/runtime packages passed
- `go test ./internal/config -run 'TestBuiltinPresets|TestGetAgentPresetByName|TestRuntimeConfigFromPreset|TestIsKnownPreset|TestSupportsSessionResume|TestGetSessionIDEnvVar|TestGetProcessNames|TestListAgentPresetsMatchesConstants|TestAgentCommandGeneration|TestCodexHooksAgentPreset'` -> passed
- `go test ./internal/hooks -run 'TestInstallForRole_CodexRoleAware'` -> passed
- `rg -n "codex-hooks|codex_hooks|gt-codex|Codex Hooks" docs/agent-provider-integration.md` -> doc text present in the expected sections
- `gofmt -w internal/config/agents.go internal/config/agents_test.go internal/hooks/installer_test.go` -> formatted
- `go test ./internal/runtime` -> passed
- `git commit -m "checkpoint: prepare codex-hooks for external review"` -> created checkpoint commit `3bf15d97`
- `git push` -> pushed checkpoint to `origin/integration/codex-hooks`
- `gt sling mol-review-implementation gastown --agent codex ...` -> spawned `furiosa` for Codex general review
- `gt sling mol-review-implementation gastown --agent claude ...` -> spawned `rictus` for Claude general review
- `gt sling mol-review-implementation gastown --agent codex --var review_profile=\"go-development\" ...` -> spawned `dementus` for specialist review
- `gt hook show gastown/furiosa` / `gt hook show gastown/rictus` / `gt hook show gastown/dementus` -> review wisps attached
- `find /Users/chall/gt/gastown/.runtime/reviews/codex-hooks -maxdepth 3 -type f | sort` -> review directory contains only shared inputs (`spec.md`, `session-context.md`, `session-ledger.md`)
- `gt peek gastown/polecats/furiosa` / `gt peek gastown/polecats/rictus` / `gt peek gastown/polecats/dementus` -> all returned `session not found`
- `go test ./internal/config -run 'TestBuiltinPresets|TestGetAgentPresetByName|TestRuntimeConfigFromPreset|TestIsKnownPreset|TestSupportsSessionResume|TestGetSessionIDEnvVar|TestGetProcessNames|TestListAgentPresetsMatchesConstants|TestAgentCommandGeneration|TestCodexHooksAgentPreset' && go test ./internal/hooks -run 'TestInstallForRole_CodexRoleAware' && go test ./internal/runtime && rg -n "codex-hooks|codex_hooks|gt-codex|Codex Hooks" docs/agent-provider-integration.md` -> all passed / matched during final verification

## Files Changed

- `docs/plans/codex-hooks/spec.md`
- `docs/plans/codex-hooks/session-context.md`
- `docs/plans/codex-hooks/session-ledger.md`
- `internal/config/agents.go`
- `internal/config/agents_test.go`
- `internal/hooks/installer_test.go`
- `docs/agent-provider-integration.md`
- `internal/hooks/templates/codex/hooks-interactive.json`
- `internal/hooks/templates/codex/hooks-autonomous.json`

## Review Checkpoint

- Review checkpoint commit: `3bf15d97`
- Review directory: `/Users/chall/gt/gastown/.runtime/reviews/codex-hooks/20260311-062955`
- Branch under review: `origin/integration/codex-hooks`

### Review Workers

- `gastown/furiosa` -> Codex general review hook present; no live session found during parent verification
- `gastown/rictus` -> Claude general review hook present; no live session found during parent verification
- `gastown/dementus` -> Codex `go-development` review hook present; no live session found during parent verification

## Open Risks / Blockers

- Need to verify the Codex hook JSON shape matches the runtime's current expectations while keeping the template intentionally minimal.
- Final review coverage is degraded: the review hooks exist, but no reviewer session/report was available when verified from the parent session.

## Review Monitoring Notes

- Parent verification timestamp: 2026-03-11
- `gastown/furiosa`: `gt hook show` reports hooked work; `gt peek` returned `session not found`.
- `gastown/rictus`: `gt hook show` reports hooked work; `gt peek` returned `session not found`.
- `gastown/dementus`: `gt hook show` reports hooked work; `gt peek` returned `session not found`.
- Review directory currently contains only `spec.md`, `session-context.md`, and `session-ledger.md`; no review markdown outputs were present.

## Review Synthesis

- Required review reports expected:
  - `/Users/chall/gt/gastown/.runtime/reviews/codex-hooks/20260311-062955/codex-review.md`
  - `/Users/chall/gt/gastown/.runtime/reviews/codex-hooks/20260311-062955/claude-review.md`
  - `/Users/chall/gt/gastown/.runtime/reviews/codex-hooks/20260311-062955/go-development-review.md`
- Actual review reports produced: none
- Terminal reviewer states:
  - `gastown/furiosa`: hook present, no live session found during parent verification
  - `gastown/rictus`: hook present, no live session found during parent verification
  - `gastown/dementus`: hook present, no live session found during parent verification
- Synthesis outcome: external review coverage is degraded to zero report artifacts. No reviewer findings can be deduplicated or compared, so Stage 8 must rely on the local proof model evidence already captured in this ledger and explicitly report the failed sidecar review runs.
