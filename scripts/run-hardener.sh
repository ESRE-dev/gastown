#!/usr/bin/env bash
# One-shot launcher for migration hardener agent.
# Intended to be run inside a tmux session.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

MISSION="$REPO_ROOT/.claude/agents/at-migration-mission.md"
SYSTEM_PROMPT="$(cat "$MISSION")

---

You are a solo migration hardening agent. Work through the mission above
systematically, phase by phase. You have full autonomy.

Working directory: $REPO_ROOT
Push target: origin/main (direct push, no PRs)
VM: migration-test-lab (access via: gcloud compute ssh migration-test-lab --zone=us-west1-b)

WORKFLOW:
1. Phase 1 (Audit) — read all migration code, catalog gaps
2. Phase 2 — work through the edge case matrix, writing Go tests
3. Phase 3 — fix bugs you discover
4. Phase 4 — VM integration tests with multiple configurations
5. Phase 5 — document and report

CRITICAL REMINDERS:
- Run 'go test ./... && golangci-lint run ./...' before every push
- After EVERY migration test: check for zombie artifacts (bd daemons, SQLite files, JSONL files)
- Commit frequently, push to main regularly
- Use TaskCreate to track your progress through the edge case matrix
- If context gets full, commit+push everything, then start a new session and continue

START NOW. Begin with Phase 1 — read the migration code."

exec claude \
    --permission-mode acceptEdits \
    --agent migration-hardener \
    --append-system-prompt "$SYSTEM_PROMPT"
