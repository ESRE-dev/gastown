package hooks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/steveyegge/gastown/internal/config"
)

// OpenCodeInstaller implements HookInstaller for the OpenCode runtime.
// OpenCode uses a self-contained JS plugin (gastown.js) rather than JSON
// settings merging. Sync is template-freshness based: the installed file
// is compared against the embedded template and overwritten if stale.
type OpenCodeInstaller struct {
	preset *config.AgentPresetInfo
}

// Install writes/updates the OpenCode plugin file for the given role.
func (o *OpenCodeInstaller) Install(workDir, settingsDir, role string) error {
	p := o.presetOrDefault()
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

// DiscoverTargets finds all managed OpenCode plugin locations in the workspace.
// These are .opencode/plugins/gastown.js files in each role directory.
func (o *OpenCodeInstaller) DiscoverTargets(townRoot string) ([]Target, error) {
	p := o.presetOrDefault()
	return discoverTargetsForProvider(townRoot, p.HooksDir, p.HooksSettingsFile)
}

// SyncTarget ensures the target's plugin file matches the current template.
// Unlike Claude's JSON-merge flow, OpenCode sync is simple template freshness:
// resolve the expected template, compare bytes, and overwrite if different.
func (o *OpenCodeInstaller) SyncTarget(target Target, _ string) (SyncResult, error) {
	p := o.presetOrDefault()

	// Resolve the expected template content for this target's role.
	expected, err := resolveTemplate(p.HooksProvider, p.HooksSettingsFile, target.Role)
	if err != nil {
		return 0, fmt.Errorf("resolving template for %s/%s: %w", p.HooksProvider, target.Role, err)
	}

	// Apply {{GT_BIN}} substitution (same as Install does).
	if bytes.Contains(expected, []byte("{{GT_BIN}}")) {
		gtBin := resolveGTBinary()
		expected = bytes.ReplaceAll(expected, []byte("{{GT_BIN}}"), []byte(gtBin))
	}

	// Check if the file exists and read current content.
	current, readErr := os.ReadFile(target.Path)
	fileExists := readErr == nil

	// Compare contents — if identical, nothing to do.
	if fileExists && bytes.Equal(current, expected) {
		return SyncUnchanged, nil
	}

	// Create directory if needed.
	if err := os.MkdirAll(filepath.Dir(target.Path), 0755); err != nil {
		return 0, fmt.Errorf("creating plugin directory: %w", err)
	}

	// Write the template.
	if err := os.WriteFile(target.Path, expected, 0644); err != nil {
		return 0, fmt.Errorf("writing plugin file: %w", err)
	}

	if fileExists {
		return SyncUpdated, nil
	}
	return SyncCreated, nil
}

// Format returns the hook format identifier.
func (o *OpenCodeInstaller) Format() string {
	return "opencode"
}

// presetOrDefault returns the preset, falling back to OpenCode defaults.
func (o *OpenCodeInstaller) presetOrDefault() *config.AgentPresetInfo {
	if o.preset != nil {
		return o.preset
	}
	return config.GetAgentPresetByName("opencode")
}
