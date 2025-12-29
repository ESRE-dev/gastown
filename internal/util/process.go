package util

import "os"

// ProcessExists checks if a process with the given PID exists.
// It uses the Unix convention of sending signal 0 to test for process existence.
func ProcessExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(nil) == nil
}
