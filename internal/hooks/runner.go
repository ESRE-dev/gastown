package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// HookRunner loads hook configurations and executes hooks for events.
type HookRunner struct {
	rigPath string
	config  *GasTownHooksConfig
}

// NewHookRunner creates a new HookRunner for the given rig path.
// It loads the hooks configuration from .gastown/hooks.json if it exists.
func NewHookRunner(rigPath string) (*HookRunner, error) {
	runner := &HookRunner{
		rigPath: rigPath,
		config:  &GasTownHooksConfig{Hooks: make(map[EventType][]HookConfig)},
	}

	if err := runner.loadConfig(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("loading hooks config: %w", err)
		}
		// Config file doesn't exist - use empty config
	}

	return runner, nil
}

// loadConfig loads the hooks configuration from .gastown/hooks.json.
func (r *HookRunner) loadConfig() error {
	configPath := filepath.Join(r.rigPath, ".gastown", "hooks.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, r.config)
}

// Fire executes all hooks registered for the given event type.
// Returns a slice of HookResults, one for each hook executed.
// For pre-* events, if any hook returns Block=true, later hooks are skipped.
func (r *HookRunner) Fire(ctx HookContext) []HookResult {
	hooks, exists := r.config.Hooks[ctx.EventType]
	if !exists || len(hooks) == 0 {
		return nil
	}

	results := make([]HookResult, 0, len(hooks))
	isPre := isPreEvent(ctx.EventType)

	for _, hook := range hooks {
		result := r.executeHook(hook, ctx)
		results = append(results, result)

		// For pre-* events, stop if a hook blocks the operation
		if isPre && result.Block {
			break
		}
	}

	return results
}

// executeHook executes a single hook and returns the result.
func (r *HookRunner) executeHook(hook HookConfig, ctx HookContext) HookResult {
	start := time.Now()

	// Set up timeout if specified
	execCtx := ctx.Ctx
	if hook.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx.Ctx, time.Duration(hook.Timeout)*time.Second)
		defer cancel()
	}

	switch hook.Type {
	case HookTypeCommand:
		return r.executeCommand(hook, ctx, execCtx, start)
	case HookTypeBuiltin:
		return r.executeBuiltin(hook, ctx, execCtx, start)
	default:
		return Failure(fmt.Errorf("unknown hook type: %s", hook.Type), time.Since(start))
	}
}

// executeCommand executes a shell command hook.
func (r *HookRunner) executeCommand(hook HookConfig, ctx HookContext, execCtx context.Context, start time.Time) HookResult {
	if hook.Cmd == "" {
		return Failure(fmt.Errorf("command hook missing cmd field"), time.Since(start))
	}

	cmd := exec.CommandContext(execCtx, "sh", "-c", hook.Cmd)
	cmd.Dir = r.rigPath

	// Set environment variables for the hook
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GASTOWN_EVENT=%s", ctx.EventType),
		fmt.Sprintf("GASTOWN_RIG_PATH=%s", ctx.RigPath),
		fmt.Sprintf("GASTOWN_AGENT_ROLE=%s", ctx.AgentRole),
	)

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		return Failure(fmt.Errorf("command failed: %w: %s", err, string(output)), duration)
	}

	return Success(string(output), duration)
}

// executeBuiltin executes a built-in hook function.
func (r *HookRunner) executeBuiltin(hook HookConfig, ctx HookContext, execCtx context.Context, start time.Time) HookResult {
	if hook.Builtin == "" {
		return Failure(fmt.Errorf("builtin hook missing builtin field"), time.Since(start))
	}

	fn, exists := builtinHooks[hook.Builtin]
	if !exists {
		return Failure(fmt.Errorf("unknown builtin hook: %s", hook.Builtin), time.Since(start))
	}

	// Update context in case timeout was added
	ctx.Ctx = execCtx

	return fn(ctx)
}

// isPreEvent returns true if the event type is a pre-* event.
func isPreEvent(eventType EventType) bool {
	switch eventType {
	case EventPreSessionStart, EventPreShutdown:
		return true
	default:
		return false
	}
}

// HasHooks returns true if there are hooks registered for the given event type.
func (r *HookRunner) HasHooks(eventType EventType) bool {
	hooks, exists := r.config.Hooks[eventType]
	return exists && len(hooks) > 0
}

// GetHooks returns the hooks registered for the given event type.
func (r *HookRunner) GetHooks(eventType EventType) []HookConfig {
	return r.config.Hooks[eventType]
}
