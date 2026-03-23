// Package opencode provides HTTP API integration with OpenCode agents.
//
// GasTown agents running OpenCode expose an HTTP server for reliable message
// delivery, idle detection, and readiness checking. This package handles port
// assignment, discovery, and HTTP client operations that replace fragile tmux
// screen-scraping for OpenCode agents.
//
// Non-OpenCode agents (Claude Code, Gemini, etc.) continue using tmux directly.
package opencode

import (
	"fmt"
	"strconv"
	"strings"
)

// DefaultPort is the port OpenCode uses when no explicit port is configured.
// When --port is 0 (the default), OpenCode tries 4096 first, then falls back
// to a random OS-assigned port.
const DefaultPort = 4096

// BasePort is the starting port for GasTown-managed OpenCode instances.
// Each agent gets BasePort + slot index to avoid conflicts.
const BasePort = 4096

// PortEnvKey is the tmux session environment variable where GasTown stores
// the assigned OpenCode HTTP port. Other GasTown components read this to
// discover the port for nudge delivery, idle detection, and readiness checks.
const PortEnvKey = "GT_OPENCODE_PORT"

// SessionIDEnvKey is the environment variable where OpenCode stores its
// internal session ID. GasTown reads this for HTTP API calls that require
// the OpenCode session ID (e.g., POST /session/:id/prompt_async).
const SessionIDEnvKey = "OPENCODE_SESSION_ID"

// AssignPort returns a unique HTTP server port for an OpenCode agent based on
// its slot index. Slot 0 gets the base port (4096), slot 1 gets 4097, etc.
// This prevents port conflicts when multiple OpenCode agents run in the same rig.
//
// The slot index comes from SessionManager.polecatSlot(), which assigns a
// stable index based on the polecat's position among existing polecat directories.
func AssignPort(slot int) int {
	return BasePort + slot
}

// TmuxEnvGetter abstracts tmux environment access for testing.
type TmuxEnvGetter interface {
	GetEnvironment(session, key string) (string, error)
}

// DiscoverPort reads the OpenCode HTTP server port for a tmux session.
// Returns the port and true if found, or (DefaultPort, false) if the port
// cannot be determined.
//
// Discovery layers (in order):
//  1. tmux session environment variable GT_OPENCODE_PORT
//  2. Default port (4096) as fallback
func DiscoverPort(tmux TmuxEnvGetter, session string) (int, bool) {
	portStr, err := tmux.GetEnvironment(session, PortEnvKey)
	if err == nil && portStr != "" {
		portStr = strings.TrimSpace(portStr)
		if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
			return port, true
		}
	}
	return DefaultPort, false
}

// IsOpenCodeSession checks if a tmux session is running an OpenCode agent
// by reading the GT_AGENT environment variable.
func IsOpenCodeSession(tmux TmuxEnvGetter, session string) bool {
	agentName, err := tmux.GetEnvironment(session, "GT_AGENT")
	if err != nil {
		return false
	}
	return strings.TrimSpace(agentName) == "opencode"
}

// InjectPortFlag appends "--port <port>" to an OpenCode startup command.
// The flag is inserted before the --prompt flag if present, otherwise appended.
//
// Example:
//
//	InjectPortFlag("exec env ... opencode --prompt 'hello'", 4097)
//	→ "exec env ... opencode --port 4097 --prompt 'hello'"
func InjectPortFlag(command string, port int) string {
	portFlag := fmt.Sprintf("--port %d", port)

	// Try to insert before --prompt flag (opencode-specific ordering)
	if idx := strings.Index(command, " --prompt "); idx >= 0 {
		return command[:idx] + " " + portFlag + command[idx:]
	}

	// No --prompt flag found; append to end
	return command + " " + portFlag
}
