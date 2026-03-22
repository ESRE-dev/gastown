// Package hooks — hook_installer.go defines the HookInstaller interface
// for agent-specific hook lifecycle management.

package hooks

import "github.com/steveyegge/gastown/internal/config"

// SyncResult indicates the outcome of a SyncTarget call.
type SyncResult int

const (
	// SyncUnchanged means the target's hooks were already up-to-date.
	SyncUnchanged SyncResult = iota
	// SyncCreated means the target's hook file was newly created.
	SyncCreated
	// SyncUpdated means the target's hook file was updated.
	SyncUpdated
)

// HookInstaller abstracts hook lifecycle operations for different agent runtimes.
//
// Claude Code uses JSON settings merging (DiscoverTargets → ComputeExpected →
// MergeHooks → write settings.json). OpenCode uses self-contained JS plugin
// templates. Other runtimes (Gemini, Cursor, Copilot) use their own formats.
//
// The interface unifies discovery, installation, and synchronization so callers
// can work with any agent runtime without knowing its hook format.
type HookInstaller interface {
	// Install writes/updates hook files for the given role.
	// workDir is the agent's working directory (e.g., rig/witness/).
	// settingsDir is where hook settings live (may differ from workDir for
	// agents that support --settings or equivalent).
	// role is the GasTown role (mayor, witness, crew, refinery, polecat, deacon).
	Install(workDir, settingsDir, role string) error

	// DiscoverTargets finds all locations in a town where hooks should exist
	// for this agent type. Returns Target structs with paths to hook files.
	DiscoverTargets(townRoot string) ([]Target, error)

	// SyncTarget ensures a discovered target has correct, up-to-date hooks.
	// For Claude: computes expected config, merges overrides, writes settings.json.
	// For simpler agents: checks template freshness and overwrites if stale.
	SyncTarget(target Target, townRoot string) (SyncResult, error)

	// Format returns the hook format identifier (e.g., "claude", "opencode").
	Format() string
}

// InstallerForPreset returns the appropriate HookInstaller for the given agent preset.
// Falls back to ClaudeInstaller for unrecognized presets since Claude is the default.
func InstallerForPreset(preset config.AgentPreset) HookInstaller {
	info := config.GetAgentPresetByName(string(preset))
	if info == nil {
		return &ClaudeInstaller{}
	}
	switch info.HooksProvider {
	case "opencode":
		return &OpenCodeInstaller{preset: info}
	default:
		// Claude, Gemini, Cursor, Copilot, etc. all use the same
		// JSON-settings merge flow with different paths.
		return &ClaudeInstaller{preset: info}
	}
}
