// Package telemetry — recorder.go
// Recording helper functions for all GT telemetry events.
// Each function emits both an OTel log event (→ VictoriaLogs) and increments
// a metric counter (→ VictoriaMetrics).
package telemetry

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
)

const (
	meterRecorderName = "github.com/steveyegge/gastown"
	loggerName        = "gastown"
)

// recorderInstruments holds all lazy-initialized OTel metric counters.
type recorderInstruments struct {
	bdTotal         metric.Int64Counter
	sessionTotal    metric.Int64Counter
	promptTotal     metric.Int64Counter
	paneReadTotal   metric.Int64Counter
	primeTotal      metric.Int64Counter
	agentStateTotal metric.Int64Counter
	polecatTotal    metric.Int64Counter
	slingTotal      metric.Int64Counter
	mailTotal       metric.Int64Counter
}

var (
	instOnce sync.Once
	inst     recorderInstruments
)

// initInstruments registers all recorder metric counters against the current
// global MeterProvider. Must be called after telemetry.Init so the real
// provider is set. Also called lazily on first use as a safety net.
func initInstruments() {
	instOnce.Do(func() {
		m := otel.GetMeterProvider().Meter(meterRecorderName)
		inst.bdTotal, _ = m.Int64Counter("gastown.bd.calls.total",
			metric.WithDescription("Total bd CLI command invocations"),
		)
		inst.sessionTotal, _ = m.Int64Counter("gastown.session.starts.total",
			metric.WithDescription("Total agent session starts"),
		)
		inst.promptTotal, _ = m.Int64Counter("gastown.prompt.sends.total",
			metric.WithDescription("Total tmux SendKeys prompt dispatches"),
		)
		inst.paneReadTotal, _ = m.Int64Counter("gastown.pane.reads.total",
			metric.WithDescription("Total tmux CapturePane calls"),
		)
		inst.primeTotal, _ = m.Int64Counter("gastown.prime.total",
			metric.WithDescription("Total gt prime invocations"),
		)
		inst.agentStateTotal, _ = m.Int64Counter("gastown.agent.state_changes.total",
			metric.WithDescription("Total agent state transitions"),
		)
		inst.polecatTotal, _ = m.Int64Counter("gastown.polecat.spawns.total",
			metric.WithDescription("Total polecat spawns"),
		)
		inst.slingTotal, _ = m.Int64Counter("gastown.sling.dispatches.total",
			metric.WithDescription("Total sling work dispatches"),
		)
		inst.mailTotal, _ = m.Int64Counter("gastown.mail.operations.total",
			metric.WithDescription("Total mail/bd SDK operations"),
		)
	})
}

// statusStr returns "ok" or "error" depending on whether err is nil.
func statusStr(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

// emit sends an OTel log event with the given body and key-value attributes.
func emit(ctx context.Context, body string, severity otellog.Severity, attrs ...otellog.KeyValue) {
	logger := global.GetLoggerProvider().Logger(loggerName)
	var r otellog.Record
	r.SetBody(otellog.StringValue(body))
	r.SetSeverity(severity)
	r.AddAttributes(attrs...)
	logger.Emit(ctx, r)
}

// errKV returns a log KeyValue with the error message, or empty string if nil.
func errKV(err error) otellog.KeyValue {
	if err != nil {
		return otellog.String("error", err.Error())
	}
	return otellog.String("error", "")
}

// severity returns SeverityInfo on success, SeverityError on failure.
func severity(err error) otellog.Severity {
	if err != nil {
		return otellog.SeverityError
	}
	return otellog.SeverityInfo
}

// RecordBDCall records a bd CLI invocation (metrics + log event).
// args is the full argument list; args[0] is used as the subcommand label.
func RecordBDCall(ctx context.Context, args []string, err error) {
	initInstruments()
	subcommand := ""
	if len(args) > 0 {
		subcommand = args[0]
	}
	status := statusStr(err)
	inst.bdTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("subcommand", subcommand),
		),
	)
	emit(ctx, "bd.call", severity(err),
		otellog.String("subcommand", subcommand),
		otellog.Int64("args_count", int64(len(args))),
		otellog.String("status", status),
		errKV(err),
	)
}

// RecordSessionStart records an agent session start (metrics + log event).
func RecordSessionStart(ctx context.Context, sessionID, role string, err error) {
	initInstruments()
	status := statusStr(err)
	inst.sessionTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("role", role),
		),
	)
	emit(ctx, "session.start", severity(err),
		otellog.String("session_id", sessionID),
		otellog.String("role", role),
		otellog.String("status", status),
		errKV(err),
	)
}

// RecordPromptSend records a tmux SendKeys prompt dispatch (metrics + log event).
func RecordPromptSend(ctx context.Context, session, keys string, debounceMs int, err error) {
	initInstruments()
	status := statusStr(err)
	inst.promptTotal.Add(ctx, 1,
		metric.WithAttributes(attribute.String("status", status)),
	)
	emit(ctx, "prompt.send", severity(err),
		otellog.String("session", session),
		otellog.Int64("keys_len", int64(len(keys))),
		otellog.Int64("debounce_ms", int64(debounceMs)),
		otellog.String("status", status),
		errKV(err),
	)
}

// RecordPaneRead records a tmux CapturePane call (metrics + log event).
func RecordPaneRead(ctx context.Context, session string, lines, contentLen int, err error) {
	initInstruments()
	status := statusStr(err)
	inst.paneReadTotal.Add(ctx, 1,
		metric.WithAttributes(attribute.String("status", status)),
	)
	emit(ctx, "pane.read", severity(err),
		otellog.String("session", session),
		otellog.Int64("lines_requested", int64(lines)),
		otellog.Int64("content_len", int64(contentLen)),
		otellog.String("status", status),
		errKV(err),
	)
}

// RecordPrime records a gt prime invocation (metrics + log event).
func RecordPrime(ctx context.Context, role string, hookMode bool, err error) {
	initInstruments()
	status := statusStr(err)
	inst.primeTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("role", role),
			attribute.Bool("hook_mode", hookMode),
		),
	)
	emit(ctx, "prime", severity(err),
		otellog.String("role", role),
		otellog.Bool("hook_mode", hookMode),
		otellog.String("status", status),
		errKV(err),
	)
}

// RecordAgentStateChange records an agent state transition (metrics + log event).
func RecordAgentStateChange(ctx context.Context, agentID, newState string, hookBead *string, err error) {
	initInstruments()
	status := statusStr(err)
	hasHookBead := hookBead != nil && *hookBead != ""
	inst.agentStateTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("new_state", newState),
		),
	)
	emit(ctx, "agent.state_change", severity(err),
		otellog.String("agent_id", agentID),
		otellog.String("new_state", newState),
		otellog.Bool("has_hook_bead", hasHookBead),
		otellog.String("status", status),
		errKV(err),
	)
}

// RecordPolecatSpawn records a polecat spawn attempt (metrics + log event).
func RecordPolecatSpawn(ctx context.Context, name string, err error) {
	initInstruments()
	status := statusStr(err)
	inst.polecatTotal.Add(ctx, 1,
		metric.WithAttributes(attribute.String("status", status)),
	)
	emit(ctx, "polecat.spawn", severity(err),
		otellog.String("name", name),
		otellog.String("status", status),
		errKV(err),
	)
}

// RecordSling records a sling work dispatch (metrics + log event).
func RecordSling(ctx context.Context, bead, target string, err error) {
	initInstruments()
	status := statusStr(err)
	inst.slingTotal.Add(ctx, 1,
		metric.WithAttributes(attribute.String("status", status)),
	)
	emit(ctx, "sling", severity(err),
		otellog.String("bead", bead),
		otellog.String("target", target),
		otellog.String("status", status),
		errKV(err),
	)
}

// RecordMail records a mail/bd SDK operation (metrics + log event).
func RecordMail(ctx context.Context, operation string, err error) {
	initInstruments()
	status := statusStr(err)
	inst.mailTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("operation", operation),
		),
	)
	emit(ctx, "mail", severity(err),
		otellog.String("operation", operation),
		otellog.String("status", status),
		errKV(err),
	)
}
