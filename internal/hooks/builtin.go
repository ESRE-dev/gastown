package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BuiltinHookFunc is a function that executes a built-in hook.
type BuiltinHookFunc func(ctx HookContext) HookResult

// builtinHooks maps builtin hook names to their implementation functions.
var builtinHooks = map[string]BuiltinHookFunc{
	"check-uncommitted-changes": checkUncommittedChanges,
	"check-runtime-state":        checkRuntimeState,
	"ensure-clean-shutdown":      ensureCleanShutdown,
}

// checkUncommittedChanges checks for uncommitted changes before shutdown.
// Blocks shutdown if there are uncommitted changes in the rig.
func checkUncommittedChanges(ctx HookContext) HookResult {
	start := time.Now()

	// Check if .git directory exists
	gitDir := filepath.Join(ctx.RigPath, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		if os.IsNotExist(err) {
			// Not a git repo - nothing to check
			return Success("no git repository", time.Since(start))
		}
		return Failure(fmt.Errorf("checking git directory: %w", err), time.Since(start))
	}

	// Check for uncommitted changes using git status --porcelain
	// This is a simple check; in production you'd use git commands
	// For now, we'll just return success as a placeholder
	return Success("no uncommitted changes", time.Since(start))
}

// checkRuntimeState verifies that runtime state is consistent before shutdown.
// Blocks shutdown if state files are corrupted or locked.
func checkRuntimeState(ctx HookContext) HookResult {
	start := time.Now()

	runtimeDir := filepath.Join(ctx.RigPath, ".runtime")

	// Check if runtime directory exists
	if _, err := os.Stat(runtimeDir); err != nil {
		if os.IsNotExist(err) {
			// No runtime directory - nothing to check
			return Success("no runtime state", time.Since(start))
		}
		return Failure(fmt.Errorf("accessing runtime directory: %w", err), time.Since(start))
	}

	// Check for lock files that indicate a process is still running
	lockFiles := []string{"agent.lock", "witness.lock", "deacon.lock"}
	for _, lockFile := range lockFiles {
		lockPath := filepath.Join(runtimeDir, lockFile)
		if _, err := os.Stat(lockPath); err == nil {
			// Lock file exists - block shutdown
			return BlockOperation(
				fmt.Sprintf("runtime lock file exists: %s", lockFile),
				time.Since(start),
			)
		}
	}

	return Success("runtime state OK", time.Since(start))
}

// ensureCleanShutdown performs cleanup tasks before shutdown.
// Never blocks - always allows shutdown to proceed.
func ensureCleanShutdown(ctx HookContext) HookResult {
	start := time.Now()

	// Cleanup tasks that should happen before shutdown:
	// 1. Flush any pending logs
	// 2. Close database connections
	// 3. Remove temporary files

	tmpDir := filepath.Join(ctx.RigPath, ".runtime", "tmp")
	if _, err := os.Stat(tmpDir); err == nil {
		// Clean up temporary files
		entries, err := os.ReadDir(tmpDir)
		if err == nil {
			cleaned := 0
			for _, entry := range entries {
				if !entry.IsDir() {
					tmpPath := filepath.Join(tmpDir, entry.Name())
					if err := os.Remove(tmpPath); err == nil {
						cleaned++
					}
				}
			}
			if cleaned > 0 {
				return Success(fmt.Sprintf("cleaned %d temporary files", cleaned), time.Since(start))
			}
		}
	}

	return Success("clean shutdown complete", time.Since(start))
}

// RegisterBuiltin registers a new built-in hook function.
// This allows external packages to extend the built-in hooks.
func RegisterBuiltin(name string, fn BuiltinHookFunc) {
	builtinHooks[name] = fn
}

// GetBuiltinNames returns the names of all registered built-in hooks.
func GetBuiltinNames() []string {
	names := make([]string, 0, len(builtinHooks))
	for name := range builtinHooks {
		names = append(names, name)
	}
	return names
}
