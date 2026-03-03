//go:build windows

package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/steveyegge/gastown/internal/style"
)

type handshakeState int

const (
	handshakeInit handshakeState = iota
	handshakeWaitingForInit
	handshakeWaitingForSessionNew
	handshakeComplete
)

const (
	startupPromptStateIdle      = ""
	startupPromptStatePending   = "pending"
	startupPromptStateInjecting = "injecting"
	startupPromptStateComplete  = "complete"
	startupPromptStateFailed    = "failed"
)

type Proxy struct {
	cmd                *exec.Cmd
	stdin              io.WriteCloser
	stdout             io.ReadCloser
	stderr             io.ReadCloser
	sessionID          string
	sessionMux         sync.RWMutex
	done               chan struct{}
	doneOnce           sync.Once
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	handshakeState     handshakeState
	handshakeMux       sync.Mutex
	promptMux          sync.Mutex
	activePromptID     string
	stdinMux           sync.Mutex
	stdoutMux          sync.Mutex
	uiEncoder          *json.Encoder
	startupPrompt      string
	startupPromptState string
	startupPromptMux   sync.RWMutex
	shutdownOnce       sync.Once
	isShuttingDown     atomic.Bool // Atomic flag to prevent writes during shutdown
	lastActivity       atomic.Int64
	pidFilePath        string
	townRoot           string
}

// SetTownRoot sets the town root for logging important events to town.log.
func (p *Proxy) SetTownRoot(townRoot string) {
	p.townRoot = townRoot
}

type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type SessionNewResult struct {
	SessionID string `json:"sessionId"`
}

func NewProxy() *Proxy {
	debugLog("", "[Proxy] Created new proxy, initial handshakeState=%d", handshakeInit)
	p := &Proxy{
		done:           make(chan struct{}),
		handshakeState: handshakeInit,
		uiEncoder:      json.NewEncoder(os.Stdout),
	}
	p.lastActivity.Store(time.Now().UnixNano())
	return p
}

func (p *Proxy) SetPIDFilePath(path string) {
	p.pidFilePath = path
}

func (p *Proxy) SetStartupPrompt(prompt string) {
	p.startupPromptMux.Lock()
	p.startupPrompt = prompt
	p.startupPromptState = startupPromptStatePending
	p.startupPromptMux.Unlock()
}

func (p *Proxy) getStartupPrompt() string {
	p.startupPromptMux.RLock()
	defer p.startupPromptMux.RUnlock()
	return p.startupPrompt
}

func (p *Proxy) setStartupPromptState(state string) {
	p.startupPromptMux.Lock()
	p.startupPromptState = state
	p.startupPromptMux.Unlock()
}

func (p *Proxy) getStartupPromptState() string {
	p.startupPromptMux.RLock()
	defer p.startupPromptMux.RUnlock()
	return p.startupPromptState
}


func (p *Proxy) Start(ctx context.Context, agentPath string, agentArgs []string, cwd string) error {
	childCtx, cancel := context.WithCancel(ctx)
	p.ctx = childCtx
	p.cancel = cancel

	p.cmd = exec.CommandContext(childCtx, agentPath, agentArgs...)
	p.cmd.Dir = cwd

	var err error
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("creating stdin pipe: %w", err)
	}

	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		cancel()
		p.stdinMux.Lock()
		if p.stdin != nil {
			p.stdin.Close()
			p.stdin = nil
		}
		p.stdinMux.Unlock()
		p.cmd.Wait()
		return fmt.Errorf("creating stdout pipe: %w", err)
	}

	p.stderr, err = p.cmd.StderrPipe()
	if err != nil {
		cancel()
		p.stdinMux.Lock()
		if p.stdin != nil {
			p.stdin.Close()
			p.stdin = nil
		}
		p.stdinMux.Unlock()
		p.stdout.Close()
		return fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := p.cmd.Start(); err != nil {
		cancel()
		p.stdinMux.Lock()
		if p.stdin != nil {
			p.stdin.Close()
			p.stdin = nil
		}
		p.stdinMux.Unlock()
		return fmt.Errorf("starting agent: %w", err)
	}

	p.wg.Add(1)
	go p.forwardAgentStderr()

	return nil
}

func (p *Proxy) isProcessAlive() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	// On Windows, checking if a process is alive is often done by checking if wait fails
	// or by trying to get a handle. os.Process.Signal(0) is partially supported.
	return p.cmd.Process.Signal(os.Signal(nil)) == nil
}

func (p *Proxy) writeToAgent(msg any) error {
	method := "unknown"
	var id any
	if m, ok := msg.(*JSONRPCMessage); ok {
		method = m.Method
		id = m.ID
	}

	if p.isShuttingDown.Load() {
		debugLog(p.townRoot, "[Proxy] writeToAgent: dropped write during shutdown (method=%s id=%v)", method, id)
		return fmt.Errorf("proxy is shutting down")
	}

	p.stdinMux.Lock()
	defer p.stdinMux.Unlock()

	if p.isShuttingDown.Load() {
		return fmt.Errorf("proxy is shutting down")
	}

	if p.stdin == nil {
		return fmt.Errorf("agent stdin is nil")
	}

	if !p.isProcessAlive() {
		debugLog(p.townRoot, "[Proxy] writeToAgent: failed (process dead) (method=%s id=%v)", method, id)
		return fmt.Errorf("agent process is not running")
	}

	isPrompt := false
	if m, ok := msg.(*JSONRPCMessage); ok && m.Method == "session/prompt" && m.ID != nil {
		isPrompt = true
		p.promptMux.Lock()
		if idStr, ok := m.ID.(string); ok {
			p.activePromptID = idStr
		} else {
			p.activePromptID = fmt.Sprintf("%v", m.ID)
		}
		debugLog(p.townRoot, "[Proxy] writeToAgent: marking busy (id=%s)", p.activePromptID)
		p.promptMux.Unlock()
	}

	p.lastActivity.Store(time.Now().UnixNano())
	debugLog(p.townRoot, "[Proxy] writeToAgent: encoding message (method=%s id=%v)", method, id)

	err := json.NewEncoder(p.stdin).Encode(msg)
	if err != nil {
		debugLog(p.townRoot, "[Proxy] writeToAgent: encode failed: %v", err)
		if isPrompt {
			p.promptMux.Lock()
			p.activePromptID = ""
			p.promptMux.Unlock()
		}
		return fmt.Errorf("writing to agent: %w", err)
	}

	return nil
}

func (p *Proxy) Forward() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	defer p.Shutdown()

	errChan := make(chan error, 1)
	p.wg.Add(3)
	go p.forwardToAgent()
	go p.forwardFromAgent()
	go p.runKeepAlive()

	if p.pidFilePath != "" {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.monitorPIDFile(p.ctx)
		}()
	}

	go func() {
		errChan <- p.cmd.Wait()
	}()

	var exitErr error
	select {
	case <-sigChan:
		debugLog(p.townRoot, "[Proxy] Forward: received signal")
	case <-p.done:
		debugLog(p.townRoot, "[Proxy] Forward: done channel signaled")
	case err := <-errChan:
		exitErr = err
		debugLog(p.townRoot, "[Proxy] Forward: agent process exited: %v", err)
	}

	p.Shutdown()

	doneChan := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		debugLog(p.townRoot, "[Proxy] Forward: all goroutines exited")
	case <-time.After(200 * time.Millisecond):
		debugLog(p.townRoot, "[Proxy] Forward: wait timeout, proceeding with exit")
	}

	if exitErr != nil {
		logEvent(p.townRoot, "acp_error", fmt.Sprintf("agent exited with error: %v", exitErr))
		debugLog(p.townRoot, "[Proxy] Agent exited with error: %v", exitErr)
		return exitErr
	}
	return nil
}

func (p *Proxy) forwardToAgent() {
	defer p.wg.Done()
	defer func() {
		debugLog(p.townRoot, "[Proxy] forwardToAgent: exiting, triggering Shutdown()")
		p.Shutdown()
	}()

	reader := bufio.NewReader(os.Stdin)
	receivedInput := false

	for {
		select {
		case <-p.done:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if !receivedInput && p.handshakeState == handshakeInit {
					logEvent(p.townRoot, "acp_error", "stdin closed before handshake - no ACP client connected")
					debugLog(p.townRoot, "[Proxy] stdin closed before handshake - no ACP client connected?")
				} else {
					logEvent(p.townRoot, "acp_shutdown", "stdin EOF - ACP client disconnected")
					debugLog(p.townRoot, "[Proxy] forwardToAgent: stdin EOF (client disconnected)")
				}
			} else {
				debugLog(p.townRoot, "[Proxy] forwardToAgent: stdin read error: %v", err)
				p.markDone()
			}
			return
		}

		receivedInput = true
		var msg JSONRPCMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		p.trackHandshakeRequest(&msg)

		if err := p.writeToAgent(&msg); err != nil {
			debugLog(p.townRoot, "[Proxy] forwardToAgent: writeToAgent failed: %v", err)
			p.markDone()
			return
		}
	}
}

func (p *Proxy) trackHandshakeRequest(msg *JSONRPCMessage) {
	if msg.Method == "" {
		return
	}

	p.handshakeMux.Lock()
	defer p.handshakeMux.Unlock()

	if msg.Method == "initialize" && p.handshakeState == handshakeInit {
		p.handshakeState = handshakeWaitingForInit
	}
}

func (p *Proxy) forwardFromAgent() {
	defer p.wg.Done()

	reader := bufio.NewReader(p.stdout)

	for {
		select {
		case <-p.done:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				debugLog(p.townRoot, "[Proxy] forwardFromAgent: agent stdout EOF (agent terminated)")
				logEvent(p.townRoot, "acp_shutdown", "agent stdout EOF - agent terminated gracefully")
				p.markDone()
			} else {
				logEvent(p.townRoot, "acp_error", fmt.Sprintf("agent stdout read error: %v", err))
				debugLog(p.townRoot, "[Proxy] forwardFromAgent: agent stdout read error: %v", err)
				p.markDone()
			}
			return
		}

		var msg JSONRPCMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		p.lastActivity.Store(time.Now().UnixNano())
		p.extractSessionID(&msg)
		shouldInjectPrompt := p.trackHandshakeResponse(&msg)
		p.trackPromptResponse(&msg)

		isInjectedResponse := false
		if idStr, ok := msg.ID.(string); ok && strings.HasPrefix(idStr, "gt-inject-") {
			isInjectedResponse = true
		}

		if isInjectedResponse && msg.Error != nil {
			debugLog(p.townRoot, "[Proxy] Injected prompt %v failed: %d %s", msg.ID, msg.Error.Code, msg.Error.Message)
		}

		if !isInjectedResponse {
			p.stdoutMux.Lock()
			err = p.uiEncoder.Encode(&msg)
			p.stdoutMux.Unlock()
		}
		if err != nil {
			p.markDone()
			return
		}

		if shouldInjectPrompt {
			if err := p.injectStartupPrompt(); err != nil {
				style.PrintWarning("failed to inject startup prompt: %v", err)
			}
		}
	}
}

func (p *Proxy) forwardAgentStderr() {
	defer p.wg.Done()
	reader := bufio.NewReader(p.stderr)

	for {
		select {
		case <-p.done:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				debugLog(p.townRoot, "[Agent stderr] read error: %v", err)
			}
			return
		}

		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			continue
		}

		fmt.Fprintln(os.Stderr, line)
		debugLog(p.townRoot, "[Agent] %s", line)
	}
}

func (p *Proxy) trackPromptResponse(msg *JSONRPCMessage) {
	if msg.ID == nil {
		return
	}

	p.promptMux.Lock()
	defer p.promptMux.Unlock()

	if p.activePromptID == "" {
		return
	}

	var idStr string
	if s, ok := msg.ID.(string); ok {
		idStr = s
	} else {
		idStr = fmt.Sprintf("%v", msg.ID)
	}

	if idStr == p.activePromptID {
		p.activePromptID = ""
		if idStr == "gastown-startup-prompt" {
			p.setStartupPromptState(startupPromptStateComplete)
		}
	}
}

func (p *Proxy) trackHandshakeResponse(msg *JSONRPCMessage) bool {
	if msg.ID == nil || msg.Result == nil {
		return false
	}

	p.handshakeMux.Lock()
	defer p.handshakeMux.Unlock()

	if p.handshakeState == handshakeWaitingForInit {
		p.handshakeState = handshakeWaitingForSessionNew
		return false
	}

	if p.handshakeState == handshakeWaitingForSessionNew && p.sessionID != "" {
		p.handshakeState = handshakeComplete
		return p.getStartupPrompt() != ""
	}

	return false
}

func (p *Proxy) injectStartupPrompt() error {
	prompt := p.getStartupPrompt()
	if prompt == "" {
		p.setStartupPromptState(startupPromptStateIdle)
		return nil
	}

	p.setStartupPromptState(startupPromptStateInjecting)

	p.sessionMux.RLock()
	sessionID := p.sessionID
	p.sessionMux.RUnlock()

	params := map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]string{
			{"type": "text", "text": prompt},
		},
	}
	paramsBytes, _ := json.Marshal(params)

	req := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      "gastown-startup-prompt",
		Method:  "session/prompt",
		Params:  paramsBytes,
	}

	if err := p.writeToAgent(&req); err != nil {
		p.setStartupPromptState(startupPromptStateFailed)
		return fmt.Errorf("sending startup prompt: %w", err)
	}

	// We no longer block here. The response will be handled by forwardFromAgent
	// and trackPromptResponse will update the startupPromptState to complete
	// when the response is received.
	return nil
}

func (p *Proxy) SessionID() string {
	p.sessionMux.RLock()
	defer p.sessionMux.RUnlock()
	return p.sessionID
}

func (p *Proxy) WaitForSessionID(ctx context.Context) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		p.sessionMux.RLock()
		sid := p.sessionID
		p.sessionMux.RUnlock()

		if sid != "" {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.done:
			return fmt.Errorf("proxy shutting down")
		case <-ticker.C:
		}
	}
}

func (p *Proxy) WaitForReady(ctx context.Context) error {
	if err := p.WaitForSessionID(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if p.isShuttingDown.Load() {
			return fmt.Errorf("proxy is shutting down")
		}

		p.promptMux.Lock()
		busy := p.activePromptID != ""
		p.promptMux.Unlock()

		state := p.getStartupPromptState()
		if !busy && (state == startupPromptStateIdle || state == startupPromptStateComplete || state == startupPromptStateFailed) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.done:
			return fmt.Errorf("proxy shutting down")
		case <-ticker.C:
		}
	}
}

func (p *Proxy) IsBusy() bool {
	p.promptMux.Lock()
	defer p.promptMux.Unlock()
	return p.activePromptID != ""
}

func (p *Proxy) extractSessionID(msg *JSONRPCMessage) {
	if msg.ID != nil && msg.Result != nil {
		var result SessionNewResult
		if err := json.Unmarshal(msg.Result, &result); err == nil && result.SessionID != "" {
			p.sessionMux.Lock()
			p.sessionID = result.SessionID
			p.sessionMux.Unlock()
		}
	}
}

func (p *Proxy) InjectNotificationToUI(method string, params any) error {
	if p.isShuttingDown.Load() {
		return fmt.Errorf("proxy is shutting down")
	}

	p.sessionMux.RLock()
	sessionID := p.sessionID
	p.sessionMux.RUnlock()

	if method == "session/update" && sessionID == "" {
		return fmt.Errorf("cannot inject session/update: empty sessionID")
	}

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
	}

	if sessionID != "" || params != nil {
		paramMap := make(map[string]any)
		if sessionID != "" {
			paramMap["sessionId"] = sessionID
		}
		if params != nil {
			if v, ok := params.(map[string]any); ok {
				for k, val := range v {
					paramMap[k] = val
				}
			} else {
				paramMap["params"] = params
			}
		}
		rawParams, _ := json.Marshal(paramMap)
		msg.Params = rawParams
	}

	debugLog(p.townRoot, "[Proxy] Injecting notification to UI: method=%s sessionId=%s", method, sessionID)
	p.stdoutMux.Lock()
	err := p.uiEncoder.Encode(&msg)
	p.stdoutMux.Unlock()
	return err
}

func (p *Proxy) InjectPrompt(prompt string) error {
	if p.isShuttingDown.Load() {
		return fmt.Errorf("proxy is shutting down")
	}

	p.sessionMux.RLock()
	sessionID := p.sessionID
	p.sessionMux.RUnlock()

	if sessionID == "" {
		return fmt.Errorf("cannot inject prompt: empty sessionID")
	}

	// Check if agent is busy to prevent race conditions.
	// If startup prompt is still in-flight, wait briefly for readiness.
	if p.IsBusy() {
		state := p.getStartupPromptState()
		if state == startupPromptStatePending || state == startupPromptStateInjecting {
			waitCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := p.WaitForReady(waitCtx); err != nil {
				debugLog(p.townRoot, "[Proxy] InjectPrompt: agent still busy after waiting for startup readiness: %v", err)
				return fmt.Errorf("agent is busy processing another prompt")
			}
		} else {
			debugLog(p.townRoot, "[Proxy] InjectPrompt: agent is busy, skipping injection to prevent race condition")
			return fmt.Errorf("agent is busy processing another prompt")
		}
	}

	params := map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]string{
			{"type": "text", "text": prompt},
		},
	}
	paramsBytes, _ := json.Marshal(params)

	req := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      fmt.Sprintf("gt-inject-prompt-%d", time.Now().UnixNano()),
		Method:  "session/prompt",
		Params:  paramsBytes,
	}

	logEvent(p.townRoot, "acp_prompt", fmt.Sprintf("injecting prompt: %s", truncateStr(prompt, 100)))
	debugLog(p.townRoot, "[Proxy] Injecting prompt to agent: sessionId=%s text=%q", sessionID, truncateStr(prompt, 50))
	return p.writeToAgent(&req)
}



func (p *Proxy) SendCancelNotification() error {
	p.sessionMux.RLock()
	sessionID := p.sessionID
	p.sessionMux.RUnlock()

	if sessionID == "" {
		return nil
	}

	debugLog(p.townRoot, "[Proxy] Sending session/cancel notification for session %s", sessionID)
	params := map[string]any{"sessionId": sessionID}
	paramsBytes, _ := json.Marshal(params)

	notification := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "session/cancel",
		Params:  paramsBytes,
	}

	return p.writeToAgent(&notification)
}

func (p *Proxy) monitorPIDFile(ctx context.Context) {
	if p.pidFilePath == "" {
		return
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.done:
			return
		case <-ticker.C:
			if _, err := os.Stat(p.pidFilePath); os.IsNotExist(err) {
				logEvent(p.townRoot, "acp_shutdown", "PID file removed, initiating graceful shutdown")
				debugLog(p.townRoot, "[Proxy] PID file removed, initiating graceful shutdown")
				_ = p.SendCancelNotification()
				p.Shutdown()
				return
			}
		}
	}
}

func (p *Proxy) Shutdown() {
	p.shutdownOnce.Do(func() {
		debugLog(p.townRoot, "[Proxy] Shutdown: initiating graceful shutdown")
		p.isShuttingDown.Store(true)
		p.markDone()

		if p.cancel != nil {
			p.cancel()
		}

		p.stdinMux.Lock()
		if p.stdin != nil {
			p.stdin.Close()
			p.stdin = nil
		}
		p.stdinMux.Unlock()

		if p.cmd != nil && p.cmd.Process != nil {
			p.cmd.Process.Kill()
		}
	})
}

// runKeepAlive periodically sends a no-op message to the agent to prevent
// inactivity timeouts. Heartbeats are only sent when the connection is idle.
func (p *Proxy) runKeepAlive() {
	defer p.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			if p.isShuttingDown.Load() {
				return
			}

			// Don't send heartbeat if we're currently in a turn
			if p.IsBusy() {
				continue
			}

			last := p.lastActivity.Load()
			// If idle for more than 45 seconds, send a heartbeat
			if time.Since(time.Unix(0, last)) > 45*time.Second {
				p.sessionMux.RLock()
				sid := p.sessionID
				p.sessionMux.RUnlock()

				if sid == "" {
					continue
				}

				// We use session/prompt with an empty prompt array.
				// This resets the agent's idle timer without performing any work.
				id := fmt.Sprintf("gt-inject-keepalive-%d", time.Now().UnixNano())
				params := map[string]any{
					"sessionId": sid,
					"prompt":    []any{},
				}
				paramsBytes, _ := json.Marshal(params)

				msg := &JSONRPCMessage{
					JSONRPC: "2.0",
					Method:  "session/prompt",
					ID:      id,
					Params:  paramsBytes,
				}

				debugLog(p.townRoot, "[Proxy] Sending KeepAlive heartbeat to agent")
				if err := p.writeToAgent(msg); err != nil {
					debugLog(p.townRoot, "[Proxy] KeepAlive heartbeat failed: %v", err)
				}
			}
		}
	}
}

func (p *Proxy) markDone() {
	p.doneOnce.Do(func() {
		close(p.done)
	})
}

func (p *Proxy) agentDone() <-chan error {
	ch := make(chan error, 1)
	go func() {
		err := p.cmd.Wait()
		ch <- err
	}()
	return ch
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
