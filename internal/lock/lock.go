// Package lock provides agent identity locking to prevent multiple agents
// from claiming the same worker identity.
//
// Lock files are stored at <worker>/.runtime/agent.lock and contain:
// - PID of the owning process
// - Timestamp when lock was acquired
// - Session ID (tmux session name)
//
// Stale locks (where the PID is dead) are automatically cleaned up.
package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Common errors
var (
	ErrLocked       = errors.New("worker is locked by another agent")
	ErrNotLocked    = errors.New("worker is not locked")
	ErrStaleLock    = errors.New("stale lock detected")
	ErrInvalidLock  = errors.New("invalid lock file")
)

// LockInfo contains information about who holds a lock.
type LockInfo struct {
	PID       int       `json:"pid"`
	AcquiredAt time.Time `json:"acquired_at"`
	SessionID string    `json:"session_id,omitempty"`
	Hostname  string    `json:"hostname,omitempty"`
}

// IsStale checks if the lock is stale (owning process is dead).
func (l *LockInfo) IsStale() bool {
	return !processExists(l.PID)
}

// Lock represents an agent identity lock for a worker directory.
type Lock struct {
	workerDir string
	lockPath  string
}

// New creates a Lock for the given worker directory.
func New(workerDir string) *Lock {
	return &Lock{
		workerDir: workerDir,
		lockPath:  filepath.Join(workerDir, ".runtime", "agent.lock"),
	}
}

// Acquire attempts to acquire the lock for this worker.
// Returns ErrLocked if another live process holds the lock.
// Automatically cleans up stale locks.
func (l *Lock) Acquire(sessionID string) error {
	// Check for existing lock
	info, err := l.Read()
	if err == nil {
		// Lock exists - check if stale
		if info.IsStale() {
			// Stale lock - remove it
			if err := l.Release(); err != nil {
				return fmt.Errorf("removing stale lock: %w", err)
			}
		} else {
			// Active lock - check if it's us
			if info.PID == os.Getpid() {
				// We already hold it - refresh
				return l.write(sessionID)
			}
			// Another process holds it
			return fmt.Errorf("%w: PID %d (session: %s, acquired: %s)",
				ErrLocked, info.PID, info.SessionID, info.AcquiredAt.Format(time.RFC3339))
		}
	}

	// No lock or stale lock removed - acquire it
	return l.write(sessionID)
}

// Release releases the lock if we hold it.
func (l *Lock) Release() error {
	if err := os.Remove(l.lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing lock file: %w", err)
	}
	return nil
}

// Read reads the current lock info without modifying it.
func (l *Lock) Read() (*LockInfo, error) {
	data, err := os.ReadFile(l.lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotLocked
		}
		return nil, fmt.Errorf("reading lock file: %w", err)
	}

	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLock, err)
	}

	return &info, nil
}

// Check checks if the worker is locked by another agent.
// Returns nil if unlocked or locked by us.
// Returns ErrLocked if locked by another live process.
// Automatically cleans up stale locks.
func (l *Lock) Check() error {
	info, err := l.Read()
	if err != nil {
		if errors.Is(err, ErrNotLocked) {
			return nil // Not locked
		}
		return err
	}

	// Check if stale
	if info.IsStale() {
		// Clean up stale lock
		_ = l.Release()
		return nil
	}

	// Check if it's us
	if info.PID == os.Getpid() {
		return nil
	}

	// Locked by another process
	return fmt.Errorf("%w: PID %d (session: %s)", ErrLocked, info.PID, info.SessionID)
}

// Status returns a human-readable status of the lock.
func (l *Lock) Status() string {
	info, err := l.Read()
	if err != nil {
		if errors.Is(err, ErrNotLocked) {
			return "unlocked"
		}
		return fmt.Sprintf("error: %v", err)
	}

	if info.IsStale() {
		return fmt.Sprintf("stale (dead PID %d)", info.PID)
	}

	if info.PID == os.Getpid() {
		return "locked (by us)"
	}

	return fmt.Sprintf("locked by PID %d (session: %s)", info.PID, info.SessionID)
}

// ForceRelease removes the lock regardless of who holds it.
// Use with caution - only for doctor --fix scenarios.
func (l *Lock) ForceRelease() error {
	return l.Release()
}

// write creates or updates the lock file.
func (l *Lock) write(sessionID string) error {
	// Ensure .runtime directory exists
	dir := filepath.Dir(l.lockPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}

	hostname, _ := os.Hostname()
	info := LockInfo{
		PID:        os.Getpid(),
		AcquiredAt: time.Now(),
		SessionID:  sessionID,
		Hostname:   hostname,
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling lock info: %w", err)
	}

	if err := os.WriteFile(l.lockPath, data, 0644); err != nil {
		return fmt.Errorf("writing lock file: %w", err)
	}

	return nil
}

// processExists checks if a process with the given PID exists and is alive.
func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}

	// On Unix, sending signal 0 checks if process exists without affecting it
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Try to send signal 0 - this will fail if process doesn't exist
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// FindAllLocks scans a directory tree for agent.lock files.
// Returns a map of worker directory -> LockInfo.
func FindAllLocks(root string) (map[string]*LockInfo, error) {
	locks := make(map[string]*LockInfo)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Base(path) == "agent.lock" && filepath.Base(filepath.Dir(path)) == ".runtime" {
			workerDir := filepath.Dir(filepath.Dir(path))
			lock := New(workerDir)
			lockInfo, err := lock.Read()
			if err == nil {
				locks[workerDir] = lockInfo
			}
		}

		return nil
	})

	return locks, err
}

// CleanStaleLocks removes all stale locks in a directory tree.
// Returns the number of stale locks cleaned.
func CleanStaleLocks(root string) (int, error) {
	locks, err := FindAllLocks(root)
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for workerDir, info := range locks {
		if info.IsStale() {
			lock := New(workerDir)
			if err := lock.Release(); err == nil {
				cleaned++
			}
		}
	}

	return cleaned, nil
}

// DetectCollisions finds workers with multiple agents claiming the same identity.
// This detects the case where multiple processes think they own the same worker
// by comparing tmux sessions with lock files.
// Returns a list of collision descriptions.
func DetectCollisions(root string, activeSessions []string) []string {
	var collisions []string

	locks, err := FindAllLocks(root)
	if err != nil {
		return collisions
	}

	// Build set of active sessions
	activeSet := make(map[string]bool)
	for _, s := range activeSessions {
		activeSet[s] = true
	}

	for workerDir, info := range locks {
		if info.IsStale() {
			collisions = append(collisions,
				fmt.Sprintf("stale lock in %s (dead PID %d, session: %s)",
					workerDir, info.PID, info.SessionID))
			continue
		}

		// Check if the session in the lock matches an active session
		if info.SessionID != "" && !activeSet[info.SessionID] {
			collisions = append(collisions,
				fmt.Sprintf("orphaned lock in %s (session %s not found, PID %d still alive)",
					workerDir, info.SessionID, info.PID))
		}
	}

	return collisions
}
