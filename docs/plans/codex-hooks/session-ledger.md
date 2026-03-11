# Session Ledger: codex-hooks

## Current Status

- Phase: Stage 5 implementation
- Branch: `integration/codex-hooks`
- Selected profiles: `general`, `go-development`
- Tracking: milestone mode
- Milestone status: `codex-hooks: implementation complete` in progress

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

## Open Risks / Blockers

- Need to verify the Codex hook JSON shape matches the runtime's current expectations while keeping the template intentionally minimal.
- `docs/plans/codex-hooks/session-context.md` remains untracked until a later cleanup step explicitly stages it.
