# Gas Town — OpenTelemetry Observability

Local telemetry stack for Gas Town. Collects metrics and logs from `gt`, Claude agents, and bd calls into VictoriaMetrics/VictoriaLogs, visualized via Grafana.

## Stack

| Service | Port | Role |
|---------|------|------|
| VictoriaMetrics | 8428 | OTLP metrics (counters, histograms) |
| VictoriaLogs | 9428 | OTLP logs (structured events) |
| Grafana | 9429 | Visualization |

## Start

```bash
# From the opentelemetry/ directory
docker compose up -d

# Or from the repo root
docker compose -f opentelemetry/docker-compose.yml up -d
```

Grafana: http://localhost:9429 — credentials `admin` / `admin`

## Configuration

### Setup script (recommended)

```bash
source opentelemetry/setup.sh
```

This script exports all variables needed for full telemetry: gt, bd, and Claude Code.

### Manual variables

For persistent activation, add to `~/.zshrc` or `~/.bashrc`:

```bash
# gt telemetry
export GT_OTEL_METRICS_URL=http://localhost:8428/opentelemetry/api/v1/push
export GT_OTEL_LOGS_URL=http://localhost:9428/insert/opentelemetry/v1/logs

# bd telemetry (same endpoints, bd's own variable names)
export BD_OTEL_METRICS_URL=http://localhost:8428/opentelemetry/api/v1/push
export BD_OTEL_LOGS_URL=http://localhost:9428/insert/opentelemetry/v1/logs
```

Once set, every `gt` and `bd` command automatically sends its metrics and logs.

### Verification

```bash
gt status    # triggers bd calls → metrics + logs visible
bd list      # direct bd call → metrics + logs visible
```

**VictoriaMetrics** (Grafana datasource or direct vmui):
```promql
gastown_bd_calls_total
bd_storage_operations_total
```
→ http://localhost:8428/vmui/#/?query=gastown_bd_calls_total

**VictoriaLogs** (live-tail):
→ http://localhost:9428/select/vmui/#/?query=*&view=liveTailing

Useful LogsQL queries:
```
*                                    # all logs
service_name:gastown                 # gt events only
"bd.call"                            # bd calls
"session.start" OR "session.stop"    # Claude session lifecycle
"polecat.spawn"                      # polecat starts
```

## Claude Code Telemetry

When `GT_OTEL_METRICS_URL` is set, Gas Town **automatically** configures Claude agent sessions to send their own OTLP metrics to VictoriaMetrics. No extra configuration required.

The following variables are injected into each Claude session at startup:

```
CLAUDE_CODE_ENABLE_TELEMETRY=1
OTEL_METRICS_EXPORTER=otlp
OTEL_METRIC_EXPORT_INTERVAL=1000
OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=http://localhost:8428/opentelemetry/api/v1/push
OTEL_EXPORTER_OTLP_METRICS_PROTOCOL=http/protobuf
BD_OTEL_METRICS_URL=http://localhost:8428/opentelemetry/api/v1/push
BD_OTEL_LOGS_URL=http://localhost:9428/insert/opentelemetry/v1/logs
```

This means `bd` calls made **from inside a Claude session** also emit metrics, with the same GT correlation labels.

### Grafana Correlation

Each agent sends metrics with GT labels that link them to their context:

| Label | Example | Description |
|-------|---------|-------------|
| `gt.role` | `mol/witness` | Compound role (rig/type) |
| `gt.rig` | `mol` | Rig name |
| `gt.actor` | `mol/witness` | Unique bd actor ID |
| `gt.agent` | `jana` | Polecat or crew member name |

Example Grafana query to filter metrics for a specific rig:

```promql
{gt_rig="mol"}
```

## Gas Town Metrics (`gastown_*`)

| Metric | Type | Description |
|--------|------|-------------|
| `gastown_bd_calls_total` | counter | bd CLI calls by subcommand and status |
| `gastown_bd_duration_ms` | histogram | bd call round-trip latency (P50/P95/P99) |
| `gastown_session_starts_total` | counter | Claude session starts |
| `gastown_session_stops_total` | counter | Claude session stops |
| `gastown_prompt_sends_total` | counter | Prompts sent to agents |
| `gastown_pane_reads_total` | counter | tmux pane reads |
| `gastown_prime_total` | counter | `gt prime` executions |
| `gastown_agent_state_changes_total` | counter | Agent state transitions |
| `gastown_polecat_spawns_total` | counter | Polecat starts |
| `gastown_polecat_removes_total` | counter | Polecat removals |
| `gastown_sling_dispatches_total` | counter | `gt sling` dispatches |
| `gastown_mail_operations_total` | counter | bd mail operations |
| `gastown_nudge_total` | counter | `gt nudge` calls |
| `gastown_done_total` | counter | `gt done` executions |
| `gastown_daemon_agent_restarts_total` | counter | Daemon-initiated agent restarts |
| `gastown_formula_instantiations_total` | counter | Formula instantiations |
| `gastown_convoy_creates_total` | counter | Auto-convoy creations |

## bd Metrics (`bd_*`)

| Metric | Type | Description |
|--------|------|-------------|
| `bd_storage_operations_total` | counter | Storage operations by type |
| `bd_storage_operation_duration_ms` | histogram | Storage operation duration |
| `bd_storage_errors_total` | counter | Storage errors |
| `bd_db_retry_count_total` | counter | SQL retries in server mode |
| `bd_db_lock_wait_ms` | histogram | Wait time for dolt-access.lock |
| `bd_issue_count` | gauge | Issue count by status (open/in_progress/…) |
| `bd_ai_input_tokens_total` | counter | Anthropic input tokens by model |
| `bd_ai_output_tokens_total` | counter | Anthropic output tokens by model |
| `bd_ai_request_duration_ms` | histogram | Anthropic API call latency |

## Gas Town Log Events in VictoriaLogs

Every gt operation emits **both** a metric (VictoriaMetrics) and a log event (VictoriaLogs). Events carry `service.name=gastown` and all `gt.*` context labels.

| Event (body) | Key attributes | Description |
|---|---|---|
| `bd.call` | `subcommand`, `args`, `duration_ms`, `status`, `stdout` (≤2KB), `stderr` (≤1KB) | bd call (gt → bd) |
| `session.start` | `session_id`, `role`, `status` | Claude session start |
| `session.stop` | `session_id`, `status` | Claude session stop |
| `prompt.send` | `session`, `keys_len`, `debounce_ms` | Prompt sent to an agent |
| `pane.read` | `session`, `lines_requested`, `content_len` | tmux pane read |
| `prime` | `role`, `hook_mode`, `status` | `gt prime` |
| `agent.state_change` | `agent_id`, `new_state`, `has_hook_bead` | Agent state transition |
| `polecat.spawn` | `name`, `status` | Polecat start |
| `polecat.remove` | `name`, `status` | Polecat removal |
| `sling` | `bead`, `target`, `status` | `gt sling` dispatch |
| `mail` | `operation`, `status` | bd mail operation |
| `nudge` | `target`, `status` | `gt nudge` |
| `done` | `exit_type`, `status` | `gt done` (COMPLETED/ESCALATED/DEFERRED) |
| `daemon.restart` | `agent_type` | Daemon-initiated agent restart |
| `formula.instantiate` | `formula_name`, `bead_id`, `status` | Formula instantiation |
| `convoy.create` | `bead_id`, `status` | Auto-convoy creation |

Live-tail all gt activity:
```
http://localhost:9428/select/vmui/#/?query=service_name%3Agastown&view=liveTailing
```

## Stop and Cleanup

```bash
# Stop containers
docker compose -f opentelemetry/docker-compose.yml down

# Also remove volumes (deletes all data)
docker compose -f opentelemetry/docker-compose.yml down -v
```
