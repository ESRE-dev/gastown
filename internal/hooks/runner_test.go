package hooks

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewHookRunner(t *testing.T) {
	tmpDir := t.TempDir()

	runner, err := NewHookRunner(tmpDir)
	if err != nil {
		t.Fatalf("NewHookRunner failed: %v", err)
	}

	if runner.rigPath != tmpDir {
		t.Errorf("expected rigPath %q, got %q", tmpDir, runner.rigPath)
	}

	if runner.config == nil {
		t.Fatal("config should not be nil")
	}
}

func TestNewHookRunnerWithConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gastown directory
	gastownDir := filepath.Join(tmpDir, ".gastown")
	if err := os.MkdirAll(gastownDir, 0755); err != nil {
		t.Fatalf("failed to create .gastown directory: %v", err)
	}

	// Write a hooks config
	configPath := filepath.Join(gastownDir, "hooks.json")
	configData := []byte(`{
		"hooks": {
			"pre-shutdown": [
				{
					"type": "command",
					"cmd": "echo 'shutting down'",
					"timeout": 10
				}
			]
		}
	}`)

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	runner, err := NewHookRunner(tmpDir)
	if err != nil {
		t.Fatalf("NewHookRunner failed: %v", err)
	}

	// Verify config was loaded
	if !runner.HasHooks(EventPreShutdown) {
		t.Error("expected pre-shutdown hooks to be loaded")
	}

	hooks := runner.GetHooks(EventPreShutdown)
	if len(hooks) != 1 {
		t.Errorf("expected 1 hook, got %d", len(hooks))
	}

	if hooks[0].Type != HookTypeCommand {
		t.Errorf("expected type %q, got %q", HookTypeCommand, hooks[0].Type)
	}
}

func TestFireCommandHook(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gastown directory
	gastownDir := filepath.Join(tmpDir, ".gastown")
	if err := os.MkdirAll(gastownDir, 0755); err != nil {
		t.Fatalf("failed to create .gastown directory: %v", err)
	}

	// Write a hooks config with a simple echo command
	configPath := filepath.Join(gastownDir, "hooks.json")
	configData := []byte(`{
		"hooks": {
			"post-session-start": [
				{
					"type": "command",
					"cmd": "echo 'session started'"
				}
			]
		}
	}`)

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	runner, err := NewHookRunner(tmpDir)
	if err != nil {
		t.Fatalf("NewHookRunner failed: %v", err)
	}

	ctx := HookContext{
		EventType: EventPostSessionStart,
		RigPath:   tmpDir,
		AgentRole: "test",
		Ctx:       context.Background(),
	}

	results := runner.Fire(ctx)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}
	if result.Block {
		t.Error("post-session-start should not block")
	}
}

func TestFireBuiltinHook(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gastown directory
	gastownDir := filepath.Join(tmpDir, ".gastown")
	if err := os.MkdirAll(gastownDir, 0755); err != nil {
		t.Fatalf("failed to create .gastown directory: %v", err)
	}

	// Write a hooks config with a builtin hook
	configPath := filepath.Join(gastownDir, "hooks.json")
	configData := []byte(`{
		"hooks": {
			"pre-shutdown": [
				{
					"type": "builtin",
					"builtin": "ensure-clean-shutdown"
				}
			]
		}
	}`)

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	runner, err := NewHookRunner(tmpDir)
	if err != nil {
		t.Fatalf("NewHookRunner failed: %v", err)
	}

	ctx := HookContext{
		EventType: EventPreShutdown,
		RigPath:   tmpDir,
		AgentRole: "test",
		Ctx:       context.Background(),
	}

	results := runner.Fire(ctx)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}
}

func TestFireBlockingHook(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gastown and .runtime directories
	gastownDir := filepath.Join(tmpDir, ".gastown")
	runtimeDir := filepath.Join(tmpDir, ".runtime")
	if err := os.MkdirAll(gastownDir, 0755); err != nil {
		t.Fatalf("failed to create .gastown directory: %v", err)
	}
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		t.Fatalf("failed to create .runtime directory: %v", err)
	}

	// Create a lock file that should block shutdown
	lockPath := filepath.Join(runtimeDir, "agent.lock")
	if err := os.WriteFile(lockPath, []byte("locked"), 0644); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	// Write a hooks config with the runtime state check
	configPath := filepath.Join(gastownDir, "hooks.json")
	configData := []byte(`{
		"hooks": {
			"pre-shutdown": [
				{
					"type": "builtin",
					"builtin": "check-runtime-state"
				}
			]
		}
	}`)

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	runner, err := NewHookRunner(tmpDir)
	if err != nil {
		t.Fatalf("NewHookRunner failed: %v", err)
	}

	ctx := HookContext{
		EventType: EventPreShutdown,
		RigPath:   tmpDir,
		AgentRole: "test",
		Ctx:       context.Background(),
	}

	results := runner.Fire(ctx)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Block {
		t.Error("expected hook to block shutdown due to lock file")
	}
}

func TestFireMultipleHooksStopsOnBlock(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gastown directory
	gastownDir := filepath.Join(tmpDir, ".gastown")
	if err := os.MkdirAll(gastownDir, 0755); err != nil {
		t.Fatalf("failed to create .gastown directory: %v", err)
	}

	// Register a custom builtin that blocks
	blockingHook := func(ctx HookContext) HookResult {
		return BlockOperation("test block", 0)
	}
	RegisterBuiltin("test-blocking-hook", blockingHook)

	// Write a hooks config with multiple hooks
	configPath := filepath.Join(gastownDir, "hooks.json")
	configData := []byte(`{
		"hooks": {
			"pre-shutdown": [
				{
					"type": "builtin",
					"builtin": "test-blocking-hook"
				},
				{
					"type": "builtin",
					"builtin": "ensure-clean-shutdown"
				}
			]
		}
	}`)

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	runner, err := NewHookRunner(tmpDir)
	if err != nil {
		t.Fatalf("NewHookRunner failed: %v", err)
	}

	ctx := HookContext{
		EventType: EventPreShutdown,
		RigPath:   tmpDir,
		AgentRole: "test",
		Ctx:       context.Background(),
	}

	results := runner.Fire(ctx)

	// Should only get 1 result because the first hook blocks
	if len(results) != 1 {
		t.Errorf("expected 1 result (second hook should not run), got %d", len(results))
	}

	if !results[0].Block {
		t.Error("expected first hook to block")
	}
}

func TestFireWithTimeout(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gastown directory
	gastownDir := filepath.Join(tmpDir, ".gastown")
	if err := os.MkdirAll(gastownDir, 0755); err != nil {
		t.Fatalf("failed to create .gastown directory: %v", err)
	}

	// Write a hooks config with a timeout
	configPath := filepath.Join(gastownDir, "hooks.json")
	configData := []byte(`{
		"hooks": {
			"post-session-start": [
				{
					"type": "command",
					"cmd": "sleep 5",
					"timeout": 1
				}
			]
		}
	}`)

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	runner, err := NewHookRunner(tmpDir)
	if err != nil {
		t.Fatalf("NewHookRunner failed: %v", err)
	}

	ctx := HookContext{
		EventType: EventPostSessionStart,
		RigPath:   tmpDir,
		AgentRole: "test",
		Ctx:       context.Background(),
	}

	start := time.Now()
	results := runner.Fire(ctx)
	elapsed := time.Since(start)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Should fail due to timeout
	if results[0].Err == nil {
		t.Error("expected timeout error")
	}

	// Should not take the full 5 seconds
	if elapsed > 3*time.Second {
		t.Errorf("timeout did not trigger in time: took %v", elapsed)
	}
}

func TestIsPreEvent(t *testing.T) {
	tests := []struct {
		event    EventType
		expected bool
	}{
		{EventPreSessionStart, true},
		{EventPreShutdown, true},
		{EventPostSessionStart, false},
		{EventPostShutdown, false},
		{EventOnPaneOutput, false},
		{EventSessionIdle, false},
	}

	for _, tt := range tests {
		result := isPreEvent(tt.event)
		if result != tt.expected {
			t.Errorf("isPreEvent(%q) = %v, want %v", tt.event, result, tt.expected)
		}
	}
}
