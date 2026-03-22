package hooks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/steveyegge/gastown/internal/config"
)

// ClaudeInstaller implements HookInstaller for Claude Code and similar JSON-settings
// agents (Gemini, Cursor, etc.) that use the settings.json merge flow.
type ClaudeInstaller struct {
	preset *config.AgentPresetInfo
}

// Install writes/updates hook files for the given role using the preset's
// hook provider, directory, and settings file configuration.
func (c *ClaudeInstaller) Install(workDir, settingsDir, role string) error {
	p := c.presetOrDefault()
	return InstallForRole(
		p.HooksProvider,
		settingsDir,
		workDir,
		role,
		p.HooksDir,
		p.HooksSettingsFile,
		p.HooksUseSettingsDir,
	)
}

// DiscoverTargets finds all managed hook locations in the workspace.
// For Claude, these are .claude/settings.json files. For other JSON-settings
// agents, the directory and file names come from the preset.
func (c *ClaudeInstaller) DiscoverTargets(townRoot string) ([]Target, error) {
	p := c.presetOrDefault()
	return discoverTargetsForProvider(townRoot, p.HooksDir, p.HooksSettingsFile)
}

// SyncTarget ensures the target's hook config matches expectations.
// Uses the ComputeExpected → LoadSettings → compare → merge → write flow.
func (c *ClaudeInstaller) SyncTarget(target Target, _ string) (SyncResult, error) {
	// Compute expected hooks for this target
	expected, err := ComputeExpected(target.Key)
	if err != nil {
		return 0, fmt.Errorf("computing expected config: %w", err)
	}

	// Load existing settings (returns zero-value if file doesn't exist)
	current, err := LoadSettings(target.Path)
	if err != nil {
		return 0, fmt.Errorf("loading current settings: %w", err)
	}

	// Check if the file exists
	_, statErr := os.Stat(target.Path)
	fileExists := statErr == nil

	// Compare hooks sections
	if fileExists && HooksEqual(expected, &current.Hooks) {
		return SyncUnchanged, nil
	}

	// Update hooks section, preserving all other fields (including unknown ones)
	current.Hooks = *expected

	// Ensure enabledPlugins map exists with beads disabled (Gas Town standard)
	if current.EnabledPlugins == nil {
		current.EnabledPlugins = make(map[string]bool)
	}
	current.EnabledPlugins["beads@beads-marketplace"] = false

	// Create hook directory if needed
	hookDir := filepath.Dir(target.Path)
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return 0, fmt.Errorf("creating hook directory: %w", err)
	}

	// Write settings using MarshalSettings to preserve unknown fields
	data, err := MarshalSettings(current)
	if err != nil {
		return 0, fmt.Errorf("marshaling settings: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(target.Path, data, 0644); err != nil {
		return 0, fmt.Errorf("writing settings: %w", err)
	}

	if fileExists {
		return SyncUpdated, nil
	}
	return SyncCreated, nil
}

// Format returns the hook format identifier.
func (c *ClaudeInstaller) Format() string {
	p := c.presetOrDefault()
	return p.HooksProvider
}

// presetOrDefault returns the preset, falling back to Claude defaults.
func (c *ClaudeInstaller) presetOrDefault() *config.AgentPresetInfo {
	if c.preset != nil {
		return c.preset
	}
	return config.GetAgentPresetByName("claude")
}

// discoverTargetsForProvider finds all managed hook locations in the workspace,
// using the given hooks directory and settings filename instead of hardcoded
// .claude/settings.json paths.
func discoverTargetsForProvider(townRoot, hooksDir, hooksFile string) ([]Target, error) {
	var targets []Target

	// Town-level targets (mayor/deacon)
	targets = append(targets, Target{
		Path: filepath.Join(townRoot, "mayor", hooksDir, hooksFile),
		Key:  "mayor",
		Role: "mayor",
	})
	targets = append(targets, Target{
		Path: filepath.Join(townRoot, "deacon", hooksDir, hooksFile),
		Key:  "deacon",
		Role: "deacon",
	})

	// Scan rigs
	entries, err := os.ReadDir(townRoot)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "mayor" || entry.Name() == "deacon" ||
			entry.Name() == ".beads" || entry.Name()[0] == '.' {
			continue
		}

		rigName := entry.Name()
		rigPath := filepath.Join(townRoot, rigName)

		if !isRig(rigPath) {
			continue
		}

		// Crew — shared settings
		crewDir := filepath.Join(rigPath, "crew")
		if info, err := os.Stat(crewDir); err == nil && info.IsDir() {
			targets = append(targets, Target{
				Path: filepath.Join(crewDir, hooksDir, hooksFile),
				Key:  rigName + "/crew",
				Rig:  rigName,
				Role: "crew",
			})
		}

		// Polecats — shared settings
		polecatsDir := filepath.Join(rigPath, "polecats")
		if info, err := os.Stat(polecatsDir); err == nil && info.IsDir() {
			targets = append(targets, Target{
				Path: filepath.Join(polecatsDir, hooksDir, hooksFile),
				Key:  rigName + "/polecats",
				Rig:  rigName,
				Role: "polecat",
			})
		}

		// Witness
		witnessDir := filepath.Join(rigPath, "witness")
		if info, err := os.Stat(witnessDir); err == nil && info.IsDir() {
			targets = append(targets, Target{
				Path: filepath.Join(witnessDir, hooksDir, hooksFile),
				Key:  rigName + "/witness",
				Rig:  rigName,
				Role: "witness",
			})
		}

		// Refinery
		refineryDir := filepath.Join(rigPath, "refinery")
		if info, err := os.Stat(refineryDir); err == nil && info.IsDir() {
			targets = append(targets, Target{
				Path: filepath.Join(refineryDir, hooksDir, hooksFile),
				Key:  rigName + "/refinery",
				Rig:  rigName,
				Role: "refinery",
			})
		}
	}

	return targets, nil
}
