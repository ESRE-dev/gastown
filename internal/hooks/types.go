package hooks

import (
	"context"
	"time"
)

// EventType represents a lifecycle event in Gas Town that can trigger hooks.
type EventType string

// Event type constants for Gas Town lifecycle events.
const (
	EventPreSessionStart  EventType = "pre-session-start"
	EventPostSessionStart EventType = "post-session-start"
	EventPreShutdown      EventType = "pre-shutdown"
	EventPostShutdown     EventType = "post-shutdown"
	EventOnPaneOutput     EventType = "on-pane-output"
	EventSessionIdle      EventType = "session-idle"
	EventMailReceived     EventType = "mail-received"
	EventWorkAssigned     EventType = "work-assigned"
)

// AllEventTypes returns all supported event types.
var AllEventTypes = []EventType{
	EventPreSessionStart,
	EventPostSessionStart,
	EventPreShutdown,
	EventPostShutdown,
	EventOnPaneOutput,
	EventSessionIdle,
	EventMailReceived,
	EventWorkAssigned,
}

// HookType represents the type of hook to execute.
type HookType string

const (
	// HookTypeCommand executes a shell command.
	HookTypeCommand HookType = "command"

	// HookTypeBuiltin executes a built-in Go function.
	HookTypeBuiltin HookType = "builtin"
)

// HookConfig represents a single hook configuration.
type HookConfig struct {
	Type    HookType `json:"type"`              // Type of hook: "command" or "builtin"
	Cmd     string   `json:"cmd,omitempty"`     // Shell command to execute (for command hooks)
	Builtin string   `json:"builtin,omitempty"` // Built-in function name (for builtin hooks)
	Timeout int      `json:"timeout,omitempty"` // Timeout in seconds (0 = no timeout)
}

// HookResult represents the result of executing a hook.
type HookResult struct {
	Block   bool          // Whether to block the operation (for pre-* hooks)
	Message string        // Message to display/log
	Err     error         // Error if the hook failed
	Duration time.Duration // How long the hook took to execute
}

// HookContext provides context to hook execution.
type HookContext struct {
	EventType EventType              // The event that triggered the hook
	RigPath   string                 // Path to the rig directory
	AgentRole string                 // Role of the agent (witness, refinery, deacon, etc.)
	Metadata  map[string]interface{} // Event-specific metadata
	Ctx       context.Context        // Context for cancellation/timeout
}

// GasTownHooksConfig represents the .gastown/hooks.json configuration.
type GasTownHooksConfig struct {
	Hooks map[EventType][]HookConfig `json:"hooks"`
}

// Success creates a successful HookResult.
func Success(message string, duration time.Duration) HookResult {
	return HookResult{
		Block:    false,
		Message:  message,
		Duration: duration,
	}
}

// Failure creates a failed HookResult.
func Failure(err error, duration time.Duration) HookResult {
	return HookResult{
		Block:   false,
		Err:     err,
		Message: err.Error(),
		Duration: duration,
	}
}

// BlockOperation creates a HookResult that blocks the operation.
func BlockOperation(message string, duration time.Duration) HookResult {
	return HookResult{
		Block:    true,
		Message:  message,
		Duration: duration,
	}
}
