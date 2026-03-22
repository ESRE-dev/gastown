package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
)

// TestInstallerForPreset verifies the factory returns the correct installer type.
func TestInstallerForPreset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		preset     config.AgentPreset
		wantType   string // "*ClaudeInstaller" or "*OpenCodeInstaller"
		wantFormat string
	}{
		{
			name:       "claude returns ClaudeInstaller",
			preset:     config.AgentClaude,
			wantType:   "*hooks.ClaudeInstaller",
			wantFormat: "claude",
		},
		{
			name:       "opencode returns OpenCodeInstaller",
			preset:     config.AgentOpenCode,
			wantType:   "*hooks.OpenCodeInstaller",
			wantFormat: "opencode",
		},
		{
			name:       "unknown preset falls back to ClaudeInstaller",
			preset:     config.AgentPreset("unknown-preset"),
			wantType:   "*hooks.ClaudeInstaller",
			wantFormat: "claude",
		},
		{
			name:       "gemini returns ClaudeInstaller with gemini format",
			preset:     config.AgentGemini,
			wantType:   "*hooks.ClaudeInstaller",
			wantFormat: "gemini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			installer := InstallerForPreset(tt.preset)
			if installer == nil {
				t.Fatalf("InstallerForPreset(%q) returned nil", tt.preset)
			}

			// Check concrete type via Format() and type assertion
			switch tt.wantType {
			case "*hooks.ClaudeInstaller":
				if _, ok := installer.(*ClaudeInstaller); !ok {
					t.Errorf("InstallerForPreset(%q) = %T, want *ClaudeInstaller", tt.preset, installer)
				}
			case "*hooks.OpenCodeInstaller":
				if _, ok := installer.(*OpenCodeInstaller); !ok {
					t.Errorf("InstallerForPreset(%q) = %T, want *OpenCodeInstaller", tt.preset, installer)
				}
			}

			if got := installer.Format(); got != tt.wantFormat {
				t.Errorf("InstallerForPreset(%q).Format() = %q, want %q", tt.preset, got, tt.wantFormat)
			}
		})
	}
}

// TestDiscoverTargetsForProvider verifies the shared discover helper finds all
// expected targets in a temp town structure and excludes non-rig directories.
func TestDiscoverTargetsForProvider(t *testing.T) {
	t.Parallel()

	townRoot := t.TempDir()

	// Town-level directories
	mustMkdir(t, filepath.Join(townRoot, "mayor"))
	mustMkdir(t, filepath.Join(townRoot, "deacon"))

	// A rig with a .gt/ marker and all role subdirs
	rigPath := filepath.Join(townRoot, "myrig")
	mustMkdir(t, filepath.Join(rigPath, ".gt"))
	mustMkdir(t, filepath.Join(rigPath, "crew"))
	mustMkdir(t, filepath.Join(rigPath, "witness"))
	mustMkdir(t, filepath.Join(rigPath, "polecats"))
	mustMkdir(t, filepath.Join(rigPath, "refinery"))

	// A plain directory with no rig subdirs and no .gt/ marker — should be skipped.
	notARig := filepath.Join(townRoot, "notarig")
	mustMkdir(t, notARig)
	// notarig has no crew/, witness/, polecats/, or refinery/ subdirs,
	// so isRig() returns false and it must be excluded from targets.

	targets, err := discoverTargetsForProvider(townRoot, ".opencode/plugins", "gastown.js")
	if err != nil {
		t.Fatalf("discoverTargetsForProvider failed: %v", err)
	}

	// Build a map for easy lookup
	byKey := make(map[string]Target, len(targets))
	for _, tgt := range targets {
		byKey[tgt.Key] = tgt
	}

	// Verify expected targets are present
	expectedKeys := []string{
		"mayor",
		"deacon",
		"myrig/crew",
		"myrig/witness",
		"myrig/polecats",
		"myrig/refinery",
	}
	for _, key := range expectedKeys {
		tgt, ok := byKey[key]
		if !ok {
			t.Errorf("expected target %q not found; got keys: %v", key, targetKeys(targets))
			continue
		}
		// Verify path uses .opencode/plugins/gastown.js
		if !strings.HasSuffix(tgt.Path, filepath.Join(".opencode", "plugins", "gastown.js")) {
			t.Errorf("target %q path = %q, expected suffix %q", key, tgt.Path,
				filepath.Join(".opencode", "plugins", "gastown.js"))
		}
	}

	// Verify notarig is excluded
	for _, tgt := range targets {
		if strings.Contains(tgt.Path, "notarig") {
			t.Errorf("notarig (no .gt/) should be excluded, but found target: %+v", tgt)
		}
	}
}

// TestOpenCodeInstallerSyncTarget verifies the three sync states: created, unchanged, updated.
func TestOpenCodeInstallerSyncTarget(t *testing.T) {
	t.Parallel()

	// Build a representative installer using the real opencode preset.
	installer := InstallerForPreset(config.AgentOpenCode).(*OpenCodeInstaller)
	p := installer.presetOrDefault()

	// Resolve the expected template content for the "crew" role so we can compare.
	expected, err := resolveTemplate(p.HooksProvider, p.HooksSettingsFile, "crew")
	if err != nil {
		t.Fatalf("resolveTemplate failed: %v", err)
	}

	tests := []struct {
		name       string
		setup      func(t *testing.T, targetPath string) // pre-test filesystem state
		wantResult SyncResult
	}{
		{
			name: "creates new file when target does not exist",
			setup: func(t *testing.T, targetPath string) {
				// Nothing — file should not exist.
			},
			wantResult: SyncCreated,
		},
		{
			name: "unchanged when current content matches template",
			setup: func(t *testing.T, targetPath string) {
				mustWriteFile(t, targetPath, expected)
			},
			wantResult: SyncUnchanged,
		},
		{
			name: "updates stale file with different content",
			setup: func(t *testing.T, targetPath string) {
				mustWriteFile(t, targetPath, []byte("stale content"))
			},
			wantResult: SyncUpdated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			targetPath := filepath.Join(dir, ".opencode", "plugins", "gastown.js")

			// Optionally pre-create the file directory and file
			if tt.wantResult != SyncCreated {
				mustMkdir(t, filepath.Dir(targetPath))
			}
			tt.setup(t, targetPath)

			target := Target{
				Path: targetPath,
				Key:  "myrig/crew",
				Rig:  "myrig",
				Role: "crew",
			}

			got, err := installer.SyncTarget(target, dir)
			if err != nil {
				t.Fatalf("SyncTarget failed: %v", err)
			}
			if got != tt.wantResult {
				t.Errorf("SyncTarget() = %v, want %v", got, tt.wantResult)
			}

			// After create or update, the file should exist and be non-empty.
			if got == SyncCreated || got == SyncUpdated {
				data, err := os.ReadFile(targetPath)
				if err != nil {
					t.Fatalf("file not created/updated at %s: %v", targetPath, err)
				}
				if len(data) == 0 {
					t.Error("written file is empty")
				}
			}
		})
	}
}

// TestOpenCodeInstallerFormat verifies Format() returns the "opencode" identifier.
func TestOpenCodeInstallerFormat(t *testing.T) {
	t.Parallel()

	installer := &OpenCodeInstaller{}
	if got := installer.Format(); got != "opencode" {
		t.Errorf("OpenCodeInstaller.Format() = %q, want %q", got, "opencode")
	}
}

// --- helpers ---

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func targetKeys(targets []Target) []string {
	keys := make([]string, len(targets))
	for i, tgt := range targets {
		keys[i] = tgt.Key
	}
	return keys
}
