package hooks

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckUncommittedChanges(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := HookContext{
		EventType: EventPreShutdown,
		RigPath:   tmpDir,
		AgentRole: "test",
		Ctx:       context.Background(),
	}

	// Test without .git directory
	result := checkUncommittedChanges(ctx)
	if result.Err != nil {
		t.Errorf("expected no error without .git, got %v", result.Err)
	}
	if result.Block {
		t.Error("should not block without .git directory")
	}

	// Test with .git directory
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}

	result = checkUncommittedChanges(ctx)
	if result.Err != nil {
		t.Errorf("expected no error with .git, got %v", result.Err)
	}
	// Current implementation always returns success - this is a placeholder
}

func TestCheckRuntimeState(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := HookContext{
		EventType: EventPreShutdown,
		RigPath:   tmpDir,
		AgentRole: "test",
		Ctx:       context.Background(),
	}

	// Test without .runtime directory
	result := checkRuntimeState(ctx)
	if result.Err != nil {
		t.Errorf("expected no error without .runtime, got %v", result.Err)
	}
	if result.Block {
		t.Error("should not block without .runtime directory")
	}

	// Test with .runtime directory but no lock files
	runtimeDir := filepath.Join(tmpDir, ".runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		t.Fatalf("failed to create .runtime directory: %v", err)
	}

	result = checkRuntimeState(ctx)
	if result.Err != nil {
		t.Errorf("expected no error with empty .runtime, got %v", result.Err)
	}
	if result.Block {
		t.Error("should not block with no lock files")
	}

	// Test with agent.lock file
	lockPath := filepath.Join(runtimeDir, "agent.lock")
	if err := os.WriteFile(lockPath, []byte("locked"), 0644); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	result = checkRuntimeState(ctx)
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
	if !result.Block {
		t.Error("expected to block when lock file exists")
	}

	// Clean up lock file
	if err := os.Remove(lockPath); err != nil {
		t.Fatalf("failed to remove lock file: %v", err)
	}

	// Test with witness.lock file
	lockPath = filepath.Join(runtimeDir, "witness.lock")
	if err := os.WriteFile(lockPath, []byte("locked"), 0644); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	result = checkRuntimeState(ctx)
	if !result.Block {
		t.Error("expected to block when witness.lock exists")
	}
}

func TestEnsureCleanShutdown(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := HookContext{
		EventType: EventPreShutdown,
		RigPath:   tmpDir,
		AgentRole: "test",
		Ctx:       context.Background(),
	}

	// Test without .runtime directory
	result := ensureCleanShutdown(ctx)
	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}
	if result.Block {
		t.Error("ensureCleanShutdown should never block")
	}

	// Test with .runtime/tmp directory and temporary files
	tmpFilesDir := filepath.Join(tmpDir, ".runtime", "tmp")
	if err := os.MkdirAll(tmpFilesDir, 0755); err != nil {
		t.Fatalf("failed to create tmp directory: %v", err)
	}

	// Create some temporary files
	for i := 0; i < 3; i++ {
		tmpFile := filepath.Join(tmpFilesDir, "temp_"+string(rune('0'+i))+".tmp")
		if err := os.WriteFile(tmpFile, []byte("temp"), 0644); err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
	}

	result = ensureCleanShutdown(ctx)
	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}
	if result.Block {
		t.Error("ensureCleanShutdown should never block")
	}

	// Verify files were cleaned
	entries, err := os.ReadDir(tmpFilesDir)
	if err != nil {
		t.Fatalf("failed to read tmp directory: %v", err)
	}

	fileCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			fileCount++
		}
	}

	if fileCount != 0 {
		t.Errorf("expected 0 files after cleanup, got %d", fileCount)
	}
}

func TestRegisterBuiltin(t *testing.T) {
	// Register a custom builtin
	called := false
	customHook := func(ctx HookContext) HookResult {
		called = true
		return Success("custom hook executed", 0)
	}

	RegisterBuiltin("test-custom-hook", customHook)

	// Verify it was registered
	names := GetBuiltinNames()
	found := false
	for _, name := range names {
		if name == "test-custom-hook" {
			found = true
			break
		}
	}

	if !found {
		t.Error("custom hook not found in builtin names")
	}

	// Verify it can be called
	fn, exists := builtinHooks["test-custom-hook"]
	if !exists {
		t.Fatal("custom hook not registered")
	}

	ctx := HookContext{
		EventType: EventPostSessionStart,
		RigPath:   t.TempDir(),
		AgentRole: "test",
		Ctx:       context.Background(),
	}

	result := fn(ctx)
	if !called {
		t.Error("custom hook was not called")
	}
	if result.Message != "custom hook executed" {
		t.Errorf("unexpected message: %q", result.Message)
	}
}

func TestGetBuiltinNames(t *testing.T) {
	names := GetBuiltinNames()

	expectedBuiltins := []string{
		"check-uncommitted-changes",
		"check-runtime-state",
		"ensure-clean-shutdown",
	}

	if len(names) < len(expectedBuiltins) {
		t.Errorf("expected at least %d builtins, got %d", len(expectedBuiltins), len(names))
	}

	// Verify expected builtins are present
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	for _, expected := range expectedBuiltins {
		if !nameMap[expected] {
			t.Errorf("expected builtin %q not found", expected)
		}
	}
}
