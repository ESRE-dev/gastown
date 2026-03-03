package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/steveyegge/gastown/internal/nudge"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/townlog"
)

// acpDebugLogger provides file-based debug logging for ACP when GT_ACP_DEBUG=1.
// It lazily opens the log file on first use and keeps it open for the session.
type acpDebugLogger struct {
	mu       sync.Mutex
	file     *os.File
	townRoot string
	enabled  bool
}

var debugLogger = &acpDebugLogger{
	enabled: os.Getenv("GT_ACP_DEBUG") != "",
}

// init opens the log file if debugging is enabled. It is called lazily
// when the first debug log is written, with the townRoot context.
func (l *acpDebugLogger) init(townRoot string) error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return nil
	}

	if townRoot == "" {
		return fmt.Errorf("townRoot is empty")
	}

	logDir := filepath.Join(townRoot, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("creating logs directory: %w", err)
	}

	logPath := filepath.Join(logDir, "acp.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening acp.log: %w", err)
	}

	l.file = f
	l.townRoot = townRoot
	return nil
}

// log writes a debug message to the ACP log file if debugging is enabled.
func (l *acpDebugLogger) log(format string, args ...any) {
	if !l.enabled {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.file, "%s %s\n", timestamp, msg)
}

// close closes the log file if it was opened.
func (l *acpDebugLogger) close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

// debugLog logs to acp.log when GT_ACP_DEBUG=1. It lazily initializes
// the log file on first call with the given townRoot.
func debugLog(townRoot, format string, args ...any) {
	if !debugLogger.enabled {
		return
	}
	// Lazy init - only creates file on first debug log
	if debugLogger.file == nil {
		if err := debugLogger.init(townRoot); err != nil {
			// Can't log anywhere useful if file init fails, silently return
			return
		}
	}
	debugLogger.log(format, args...)
}

// logEvent logs an important event to town.log (failures, errors, lifecycle).
func logEvent(townRoot, eventType, context string) {
	if townRoot == "" {
		return
	}
	logger := townlog.NewLogger(townRoot)
	_ = logger.Log(townlog.EventType(eventType), "mayor/acp", context)
	// Also log to acp.log if debug mode is enabled
	debugLog(townRoot, "[%s] %s", eventType, context)
}

const pollInterval = 30 * time.Second

type mailMessage struct {
	ID       string
	Subject  string
	From     string
	Read     bool
	Escalate bool
}

type Propeller struct {
	proxy       *Proxy
	townRoot    string
	session     string
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mailIDs     map[string]mailMessage
	mailMu      sync.RWMutex
	hookState   string
	hookMu      sync.RWMutex
	warnedNoSID bool
}

func NewPropeller(proxy *Proxy, townRoot, session string) *Propeller {
	return &Propeller{
		proxy:    proxy,
		townRoot: townRoot,
		session:  session,
		mailIDs:  make(map[string]mailMessage),
	}
}

func (p *Propeller) Start(ctx context.Context) {
	debugLog(p.townRoot, "[Propeller] Starting for session %q in town %q", p.session, p.townRoot)
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.wg.Add(1)
	go p.waitForSessionAndPoll()
}

// waitForSessionAndPoll waits for the ACP handshake to complete (sessionID available)
// before starting the poll loop. This ensures the proxy is ready to inject prompts.
// If no IDE connects within the timeout, polling continues with degraded functionality
// (notifications will be skipped since there's no session to inject into).
func (p *Propeller) waitForSessionAndPoll() {
	defer p.wg.Done()

	// Wait for sessionID with a timeout
	waitCtx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()

	if p.proxy != nil {
		if err := p.proxy.WaitForSessionID(waitCtx); err != nil {
			// Log to town.log - this is a significant event (degraded mode)
			logEvent(p.townRoot, "acp_degraded", "sessionID not available: no IDE connected, notifications disabled")
			debugLog(p.townRoot, "[Propeller] SessionID not available after 30s: %v", err)
			debugLog(p.townRoot, "[Propeller] Continuing with degraded mode - mail/hook detection will work but notifications will be skipped")
			debugLog(p.townRoot, "[Propeller] This is expected if no ACP client (IDE) is connected to the proxy")
		} else {
			debugLog(p.townRoot, "[Propeller] SessionID available, starting poll loop with full notification support")
		}
	}

	p.pollLoop()
}

func (p *Propeller) Stop() {
	debugLog(p.townRoot, "[Propeller] Stopping")
	// Close the debug log file if it was opened
	debugLogger.close()
	logEvent(p.townRoot, "acp_stop", "propeller stopped")
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}

func (p *Propeller) pollLoop() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	p.pollOnce()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.pollOnce()
		}
	}
}

func (p *Propeller) pollOnce() {
	debugLog(p.townRoot, "[Propeller] polling...")
	ctx := p.ctx

	p.detectMailChanges(ctx)
	p.detectHookChanges(ctx)
	p.detectNudges(ctx)
}

func (p *Propeller) detectMailChanges(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "gt", "mail", "inbox", "--identity", "mayor/", "--json")
	cmd.Env = append(cmd.Environ(), "GT_TOWN_ROOT="+p.townRoot)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			debugLog(p.townRoot, "[Propeller] detectMailChanges: command error: %v, stderr: %s", err, string(exitErr.Stderr))
		} else {
			debugLog(p.townRoot, "[Propeller] detectMailChanges: command error: %v", err)
		}
		return
	}

	var messages []struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
		From    string `json:"from"`
		Read    bool   `json:"read"`
	}

	if err := json.Unmarshal(output, &messages); err != nil {
		debugLog(p.townRoot, "[Propeller] detectMailChanges: JSON parse error: %v", err)
		return
	}

	p.mailMu.Lock()
	defer p.mailMu.Unlock()

	newMailIDs := make(map[string]mailMessage)
	unreadCount := 0
	escalationCount := 0
	newCount := 0

	for _, msg := range messages {
		newMailIDs[msg.ID] = mailMessage{
			ID:      msg.ID,
			Subject: msg.Subject,
			From:    msg.From,
			Read:    msg.Read,
		}

		isNew := false
		if _, exists := p.mailIDs[msg.ID]; !exists {
			isNew = true
			newCount++
		}

		if isNew || !msg.Read {
			unreadCount++
		}

		isEscalation := strings.Contains(strings.ToLower(msg.Subject), "escalation") ||
			strings.Contains(strings.ToLower(msg.Subject), "help") ||
			strings.Contains(strings.ToLower(msg.Subject), "urgent")
		if isEscalation && (isNew || !msg.Read) {
			escalationCount++
		}
	}

	debugLog(p.townRoot, "[Propeller] detectMailChanges: total=%d old=%d new=%d unread=%d", len(newMailIDs), len(p.mailIDs), newCount, unreadCount)
	if newCount > 0 && unreadCount > 0 {
		p.notifyMailEvent(unreadCount, escalationCount)
	}

	p.mailIDs = newMailIDs
}

func (p *Propeller) notifyMailEvent(count, escalationCount int) {
	if p.proxy == nil {
		style.PrintWarning("ACP Propeller cannot notify: proxy is nil")
		return
	}

	debugLog(p.townRoot, "[Propeller] notifyMailEvent: count=%d escalation=%d", count, escalationCount)

	text := "📬 You have new mail. Run 'gt mail inbox --identity mayor/' to read."
	if escalationCount > 0 {
		text = fmt.Sprintf("🚨 URGENT: You have %d new escalation/urgent messages in your inbox. Please run 'gt mail inbox --identity mayor/' and handle them immediately.", escalationCount)
	}

	meta := map[string]string{
		"gt/eventType": "mail",
		"gt/count":     strconv.Itoa(count),
	}
	if escalationCount > 0 {
		meta["gt/escalationCount"] = strconv.Itoa(escalationCount)
	}

	if err := p.notify(text, meta, escalationCount > 0); err != nil {
		style.PrintWarning("ACP Propeller failed to deliver mail notification: %v", err)
	}
}

func (p *Propeller) detectHookChanges(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "gt", "hook", "show", "mayor", "--json")
	cmd.Env = append(cmd.Environ(), "GT_TOWN_ROOT="+p.townRoot)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	var hookResp struct {
		HasWork  bool   `json:"hasWork"`
		Title    string `json:"title"`
		Molecule string `json:"molecule"`
	}

	if err := json.Unmarshal(output, &hookResp); err != nil {
		return
	}

	newState := "idle"
	if hookResp.HasWork {
		newState = "working"
	}

	p.hookMu.Lock()
	oldState := p.hookState
	if oldState != newState {
		p.hookState = newState
		p.hookMu.Unlock()
		p.notifyHookChange(oldState, newState, hookResp.Title)
	} else {
		p.hookMu.Unlock()
	}
}

func (p *Propeller) notifyHookChange(oldState, newState, title string) {
	if p.proxy == nil {
		style.PrintWarning("ACP Propeller cannot notify: proxy is nil")
		return
	}

	meta := map[string]string{
		"gt/eventType": "hook",
		"gt/oldState":  oldState,
		"gt/newState":  newState,
	}

	text := "⚓ Hook status changed"
	if newState == "working" && title != "" {
		text = fmt.Sprintf("⚓ NEW WORK HOOKED: %s\n\nPlease run 'gt hook' to see your assignment and begin work.", title)
		if err := p.notify(text, meta, false); err != nil {
			style.PrintWarning("ACP Propeller failed to deliver hook notification: %v", err)
		}
		return
	} else if newState == "idle" {
		text = "⚓ Mayor hook cleared"
	}

	p.notifyWithMeta(text, meta)
}

func (p *Propeller) notifyWithMeta(text string, meta map[string]string) {
	if p.proxy == nil || text == "" {
		return
	}

	sessionID := p.proxy.SessionID()
	if sessionID == "" {
		// Log once when sessionID is not available (e.g., no IDE connected to ACP proxy)
		if !p.warnedNoSID {
			p.warnedNoSID = true
			debugLog(p.townRoot, "[Propeller] notifyWithMeta: sessionID not available - ACP handshake may not have completed (no IDE connected?). Notifications will be skipped.")
		}
		return
	}

	debugLog(p.townRoot, "[Propeller] notifyWithMeta: sessionID=%q text=%q", sessionID, text)

	params := map[string]any{
		"update": map[string]any{
			"sessionUpdate": "agent_message_chunk",
			"content": map[string]any{
				"type": "text",
				"text": "\n\n" + text + "\n\n",
			},
			"_meta": meta,
		},
	}

	if err := p.proxy.InjectNotificationToUI("session/update", params); err != nil {
		style.PrintWarning("ACP Propeller failed to inject notification: %v", err)
	}
}

// notify sends a notification to both the UI (via session/update) and the Agent (via session/prompt).
// This couples the two operations, ensuring consistency with tmux session behavior where notifications
// are delivered as terminal input (UI sees update, Agent receives prompt).
// The SessionID and IsBusy checks ensure we don't interrupt an active agent turn, unless urgent is true.
func (p *Propeller) notify(text string, meta map[string]string, urgent bool) error {
	if p.proxy == nil || text == "" {
		return nil
	}

	// Always notify the UI
	p.notifyWithMeta(text, meta)

	// Notify the Agent only if session is ready.
	// We bypass the IsBusy check if urgent is true (e.g. nudges/escalations).
	if p.proxy.SessionID() != "" {
		if urgent || !p.proxy.IsBusy() {
			// Try a few times in case of transient turn-state changes
			var err error
			for i := 0; i < 3; i++ {
				err = p.proxy.InjectPrompt(text)
				if err == nil {
					return nil
				}
				debugLog(p.townRoot, "[Propeller] InjectPrompt attempt %d failed: %v", i+1, err)
				time.Sleep(100 * time.Millisecond)
			}
			// Log failure to town.log
			logEvent(p.townRoot, "acp_error", fmt.Sprintf("failed to inject prompt after retries: %v", err))
			style.PrintWarning("ACP Propeller failed to inject agent prompt after retries: %v", err)
			return err
		}
	}
	return nil
}

func (p *Propeller) detectNudges(ctx context.Context) {
	if p.townRoot == "" || p.session == "" {
		debugLog(p.townRoot, "[Propeller] detectNudges: early return - townRoot=%q session=%q", p.townRoot, p.session)
		return
	}

	nudges, err := nudge.Drain(p.townRoot, p.session)
	if err != nil {
		debugLog(p.townRoot, "[Propeller] detectNudges: Drain error: %v", err)
		return
	}
	if len(nudges) == 0 {
		return
	}

	debugLog(p.townRoot, "[Propeller] detectNudges: drained %d nudge(s)", len(nudges))

	var urgent, normal []nudge.QueuedNudge
	for _, n := range nudges {
		if n.Priority == nudge.PriorityUrgent {
			urgent = append(urgent, n)
		} else {
			normal = append(normal, n)
		}
	}

	var promptBuilder strings.Builder
	if len(urgent) > 0 {
		promptBuilder.WriteString(fmt.Sprintf("🚨 URGENT NUDGE RECEIVED (%d messages):\n", len(urgent)))
		for _, n := range urgent {
			msg := fmt.Sprintf("[%s] %s", n.Sender, n.Message)
			promptBuilder.WriteString(fmt.Sprintf("- %s\n", msg))
		}
		if len(normal) > 0 {
			promptBuilder.WriteString(fmt.Sprintf("\nPlus %d non-urgent nudge(s):\n", len(normal)))
			for _, n := range normal {
				msg := fmt.Sprintf("[%s] %s", n.Sender, n.Message)
				promptBuilder.WriteString(fmt.Sprintf("- %s\n", msg))
			}
		}
	} else {
		promptBuilder.WriteString(fmt.Sprintf("📨 NUDGE RECEIVED (%d messages):\n", len(normal)))
		for _, n := range normal {
			msg := fmt.Sprintf("[%s] %s", n.Sender, n.Message)
			promptBuilder.WriteString(fmt.Sprintf("- %s\n", msg))
		}
	}

	text := promptBuilder.String()

	meta := map[string]string{
		"gt/eventType": "nudge",
		"gt/count":     strconv.Itoa(len(nudges)),
		"gt/urgent":    strconv.Itoa(len(urgent)),
		"gt/drained":   "true",
		"gt/session":   p.session,
	}

	if err := p.notify(text, meta, len(urgent) > 0); err != nil {
		style.PrintWarning("ACP Propeller failed to deliver nudge: %v", err)
	}
}
