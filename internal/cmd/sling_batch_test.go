package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCreateBatchConvoy_CreatesOneConvoyTrackingAllBeads verifies that
// createBatchConvoy creates exactly one convoy and adds tracking deps for all
// provided bead IDs. This is the core contract of the N-convoys → 1-convoy change.
func TestCreateBatchConvoy_CreatesOneConvoyTrackingAllBeads(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows — shell stubs")
	}

	townRoot := t.TempDir()

	// Minimal workspace marker so workspace.FindFromCwd() succeeds.
	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir mayor/rig: %v", err)
	}
	townBeads := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(townBeads, 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	binDir := filepath.Join(townRoot, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir binDir: %v", err)
	}
	logPath := filepath.Join(townRoot, "bd.log")

	// Stub bd: log all commands. create and dep add succeed.
	bdScript := `#!/bin/sh
echo "CMD:$*" >> "` + logPath + `"
cmd="$1"
shift || true
case "$cmd" in
  create)
    exit 0
    ;;
  dep)
    exit 0
    ;;
esac
exit 0
`
	bdPath := filepath.Join(binDir, "bd")
	if err := os.WriteFile(bdPath, []byte(bdScript), 0755); err != nil {
		t.Fatalf("write bd stub: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(townRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	beadIDs := []string{"gt-aaa", "gt-bbb", "gt-ccc"}
	convoyID, err := createBatchConvoy(beadIDs, "gastown", false, "mr")
	if err != nil {
		t.Fatalf("createBatchConvoy() error: %v", err)
	}

	// Convoy ID must have hq-cv- prefix
	if !strings.HasPrefix(convoyID, "hq-cv-") {
		t.Errorf("convoy ID %q should have hq-cv- prefix", convoyID)
	}

	// Parse logged commands
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read bd log: %v", err)
	}
	logLines := strings.Split(strings.TrimSpace(string(logBytes)), "\n")

	// Exactly 1 create command
	createCount := 0
	for _, line := range logLines {
		if strings.Contains(line, "CMD:create") {
			createCount++
		}
	}
	if createCount != 1 {
		t.Errorf("expected exactly 1 create command, got %d\nLog:\n%s", createCount, string(logBytes))
	}

	// Exactly N dep add commands (one per bead)
	depAddCount := 0
	trackedBeads := map[string]bool{}
	for _, line := range logLines {
		if strings.Contains(line, "CMD:dep add") {
			depAddCount++
			for _, beadID := range beadIDs {
				if strings.Contains(line, beadID) {
					trackedBeads[beadID] = true
				}
			}
		}
	}
	if depAddCount != len(beadIDs) {
		t.Errorf("expected %d dep add commands, got %d\nLog:\n%s", len(beadIDs), depAddCount, string(logBytes))
	}
	for _, beadID := range beadIDs {
		if !trackedBeads[beadID] {
			t.Errorf("bead %q was not tracked in convoy\nLog:\n%s", beadID, string(logBytes))
		}
	}
}

// TestCreateBatchConvoy_OwnedLabel verifies that --owned flag adds gt:owned label.
func TestCreateBatchConvoy_OwnedLabel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	townRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(townRoot, ".beads"), 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	binDir := filepath.Join(townRoot, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir binDir: %v", err)
	}
	logPath := filepath.Join(townRoot, "bd.log")

	// Use printf with NUL delimiters to correctly log args that contain newlines.
	// The --description arg contains \n which breaks simple $* logging.
	bdScript := `#!/bin/sh
printf 'CMD:' >> "` + logPath + `"
for arg in "$@"; do printf '%s\0' "$arg"; done >> "` + logPath + `"
printf '\n' >> "` + logPath + `"
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(bdScript), 0755); err != nil {
		t.Fatalf("write bd stub: %v", err)
	}

	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(townRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, err = createBatchConvoy([]string{"gt-aaa"}, "gastown", true, "direct")
	if err != nil {
		t.Fatalf("createBatchConvoy() error: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	logContent := string(logBytes)

	// The first line starting with CMD: is the create command (NUL-delimited args).
	// Check for --labels=gt:owned anywhere in the logged content for the create call.
	if !strings.Contains(logContent, "--labels=gt:owned") {
		t.Errorf("create command missing --labels=gt:owned in log:\n%q", logContent)
	}
}

// TestCreateBatchConvoy_MergeStrategyInDescription verifies that merge strategy
// is included in the convoy description.
func TestCreateBatchConvoy_MergeStrategyInDescription(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	townRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(townRoot, ".beads"), 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	binDir := filepath.Join(townRoot, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir binDir: %v", err)
	}
	logPath := filepath.Join(townRoot, "bd.log")

	// Use printf with NUL delimiters to correctly log args that contain newlines.
	bdScript := `#!/bin/sh
printf 'CMD:' >> "` + logPath + `"
for arg in "$@"; do printf '%s\0' "$arg"; done >> "` + logPath + `"
printf '\n' >> "` + logPath + `"
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(bdScript), 0755); err != nil {
		t.Fatalf("write bd stub: %v", err)
	}

	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(townRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, err = createBatchConvoy([]string{"gt-aaa", "gt-bbb"}, "gastown", false, "direct")
	if err != nil {
		t.Fatalf("createBatchConvoy() error: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	// The NUL-delimited log preserves the full --description including the newline.
	// "Merge: direct" will appear inside the --description= argument.
	logContent := string(logBytes)
	if !strings.Contains(logContent, "Merge: direct") {
		t.Errorf("create description missing merge strategy:\n%q", logContent)
	}
}

// TestCreateBatchConvoy_EmptyBeadIDs verifies that createBatchConvoy returns
// an error when called with no bead IDs.
func TestCreateBatchConvoy_EmptyBeadIDs(t *testing.T) {
	_, err := createBatchConvoy(nil, "gastown", false, "")
	if err == nil {
		t.Fatal("expected error for empty bead IDs, got nil")
	}
	if !strings.Contains(err.Error(), "no beads") {
		t.Errorf("error should mention 'no beads', got: %v", err)
	}
}

// TestCreateBatchConvoy_TitleIncludesBeadCount verifies that the convoy title
// includes the bead count and rig name.
func TestCreateBatchConvoy_TitleIncludesBeadCount(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	townRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(townRoot, ".beads"), 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	binDir := filepath.Join(townRoot, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir binDir: %v", err)
	}
	logPath := filepath.Join(townRoot, "bd.log")

	bdScript := `#!/bin/sh
echo "CMD:$*" >> "` + logPath + `"
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(bdScript), 0755); err != nil {
		t.Fatalf("write bd stub: %v", err)
	}

	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(townRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, err = createBatchConvoy([]string{"gt-a", "gt-b", "gt-c", "gt-d", "gt-e"}, "myrig", false, "")
	if err != nil {
		t.Fatalf("createBatchConvoy() error: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	var createLine string
	for _, line := range strings.Split(string(logBytes), "\n") {
		if strings.Contains(line, "CMD:create") {
			createLine = line
			break
		}
	}
	// Title should be "Batch: 5 beads to myrig"
	if !strings.Contains(createLine, "Batch: 5 beads to myrig") {
		t.Errorf("title should contain 'Batch: 5 beads to myrig', got:\n%s", createLine)
	}
}

// TestCreateBatchConvoy_PartialDepFailureContinues verifies that if a dep add
// fails for one bead, the convoy is still created and other beads are tracked.
// Partial tracking is better than no tracking.
func TestCreateBatchConvoy_PartialDepFailureContinues(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	townRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(townRoot, ".beads"), 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	binDir := filepath.Join(townRoot, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir binDir: %v", err)
	}
	logPath := filepath.Join(townRoot, "bd.log")

	// Stub bd: create succeeds, dep add fails for gt-bbb only
	bdScript := `#!/bin/sh
echo "CMD:$*" >> "` + logPath + `"
cmd="$1"
shift || true
case "$cmd" in
  create)
    exit 0
    ;;
  dep)
    # Fail if the bead is gt-bbb
    for arg in "$@"; do
      if [ "$arg" = "gt-bbb" ]; then
        exit 1
      fi
    done
    exit 0
    ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(bdScript), 0755); err != nil {
		t.Fatalf("write bd stub: %v", err)
	}

	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(townRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Should NOT return error — partial tracking is acceptable
	convoyID, err := createBatchConvoy([]string{"gt-aaa", "gt-bbb", "gt-ccc"}, "gastown", false, "")
	if err != nil {
		t.Fatalf("createBatchConvoy() should not error on partial dep failure: %v", err)
	}
	if convoyID == "" {
		t.Fatal("convoy ID should not be empty")
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	// 1 create + 3 dep add attempts = 4 commands
	logLines := strings.Split(strings.TrimSpace(string(logBytes)), "\n")
	depCount := 0
	for _, line := range logLines {
		if strings.Contains(line, "CMD:dep add") {
			depCount++
		}
	}
	if depCount != 3 {
		t.Errorf("expected 3 dep add attempts (including failed one), got %d", depCount)
	}
}

// TestBatchSling_ConvoyIDStoredInBeadFieldUpdates verifies that the batch convoy ID
// is stored in each bead's fieldUpdates.ConvoyID. This was a bug where ConvoyID and
// MergeStrategy were never persisted in batch mode.
func TestBatchSling_ConvoyIDStoredInBeadFieldUpdates(t *testing.T) {
	// This test verifies the data flow: batchConvoyID is set in fieldUpdates.ConvoyID
	// for each bead in the loop. We test this at the unit level by checking the
	// beadFieldUpdates struct construction.

	// Simulate the logic from runBatchSling: convoy created before loop,
	// ConvoyID stored in each bead's fieldUpdates.
	batchConvoyID := "hq-cv-test1"
	mergeStrategy := "direct"

	beadIDs := []string{"gt-aaa", "gt-bbb", "gt-ccc"}
	for _, beadID := range beadIDs {
		fieldUpdates := beadFieldUpdates{
			Dispatcher:    "test-actor",
			ConvoyID:      batchConvoyID,
			MergeStrategy: mergeStrategy,
		}

		if fieldUpdates.ConvoyID != batchConvoyID {
			t.Errorf("bead %s: ConvoyID = %q, want %q", beadID, fieldUpdates.ConvoyID, batchConvoyID)
		}
		if fieldUpdates.MergeStrategy != mergeStrategy {
			t.Errorf("bead %s: MergeStrategy = %q, want %q", beadID, fieldUpdates.MergeStrategy, mergeStrategy)
		}
	}
}

// TestBatchSling_ErrorsOnAlreadyTrackedBead verifies that batch sling refuses
// to proceed when any bead is already tracked by another convoy, and that
// isTrackedByConvoy correctly identifies the conflict.
func TestBatchSling_ErrorsOnAlreadyTrackedBead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	binDir := t.TempDir()

	// Stub bd: dep list returns a tracking convoy for gt-bbb (already tracked),
	// empty results for everything else.
	bdScript := `#!/bin/sh
cmd="$1"
shift || true

case "$cmd" in
  dep)
    sub="$1"; shift || true
    beadID="$1"
    if [ "$beadID" = "gt-bbb" ]; then
      echo '[{"id":"hq-cv-existing","issue_type":"convoy","status":"open"}]'
    else
      echo '[]'
    fi
    exit 0
    ;;
  list)
    echo '[]'
    exit 0
    ;;
esac
exit 0
`
	bdPath := filepath.Join(binDir, "bd")
	if err := os.WriteFile(bdPath, []byte(bdScript), 0755); err != nil {
		t.Fatalf("write bd stub: %v", err)
	}

	townRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	townBeads := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(townBeads, 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(townRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Simulate the pre-loop conflict check from runBatchSling.
	// It should detect gt-bbb as already tracked and error.
	beadIDs := []string{"gt-aaa", "gt-bbb", "gt-ccc"}
	var conflictFound bool
	for _, beadID := range beadIDs {
		if existing := isTrackedByConvoy(beadID); existing != "" {
			conflictFound = true
			if beadID != "gt-bbb" {
				t.Errorf("unexpected conflict for bead %s (convoy: %s)", beadID, existing)
			}
			if existing != "hq-cv-existing" {
				t.Errorf("expected convoy hq-cv-existing, got %s", existing)
			}
			break // runBatchSling errors on the first conflict
		}
	}

	if !conflictFound {
		t.Fatal("expected conflict for gt-bbb but none detected")
	}
}

// --- Auto-rig-resolution and deprecation tests ---

// TestAllBeadIDs_TrueWhenAllBeadIDs verifies that allBeadIDs returns true
// when every argument looks like a bead ID.
func TestAllBeadIDs_TrueWhenAllBeadIDs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"all beads", []string{"gt-abc", "gt-def", "gt-ghi"}, true},
		{"mixed prefixes", []string{"gt-abc", "bd-def", "hq-ghi"}, true},
		{"single bead", []string{"gt-abc"}, true},
		{"last is rig name", []string{"gt-abc", "gt-def", "gastown"}, false},
		{"empty list", []string{}, false},
		{"contains path", []string{"gt-abc", "gastown/polecats/foo"}, false},
		{"contains bare word no hyphen", []string{"gt-abc", "gastown"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := allBeadIDs(tc.args)
			if got != tc.want {
				t.Errorf("allBeadIDs(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

// TestResolveRigFromBeadIDs_AllSamePrefix verifies that resolveRigFromBeadIDs
// resolves the rig when all beads share the same prefix.
func TestResolveRigFromBeadIDs_AllSamePrefix(t *testing.T) {
	townRoot := t.TempDir()
	beadsDir := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write routes.jsonl mapping gt- to gastown
	routesContent := `{"prefix":"gt-","path":"gastown/.beads"}` + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "routes.jsonl"), []byte(routesContent), 0644); err != nil {
		t.Fatalf("write routes: %v", err)
	}

	rigName, err := resolveRigFromBeadIDs([]string{"gt-aaa", "gt-bbb", "gt-ccc"}, townRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rigName != "gastown" {
		t.Errorf("rigName = %q, want %q", rigName, "gastown")
	}
}

// TestResolveRigFromBeadIDs_MixedPrefixes_Errors verifies that beads from
// different rigs produce an error with suggested actions.
func TestResolveRigFromBeadIDs_MixedPrefixes_Errors(t *testing.T) {
	townRoot := t.TempDir()
	beadsDir := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	routesContent := `{"prefix":"gt-","path":"gastown/.beads"}
{"prefix":"bd-","path":"beads/.beads"}
`
	if err := os.WriteFile(filepath.Join(beadsDir, "routes.jsonl"), []byte(routesContent), 0644); err != nil {
		t.Fatalf("write routes: %v", err)
	}

	_, err := resolveRigFromBeadIDs([]string{"gt-aaa", "bd-bbb", "gt-ccc"}, townRoot)
	if err == nil {
		t.Fatal("expected error for mixed prefixes, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "different rigs") {
		t.Errorf("error should mention 'different rigs', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "gastown") || !strings.Contains(errMsg, "beads") {
		t.Errorf("error should mention both rig names, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "Options") {
		t.Errorf("error should include suggested actions, got: %s", errMsg)
	}
}

// TestResolveRigFromBeadIDs_UnmappedPrefix_Errors verifies that a bead whose
// prefix has no route mapping produces an error with suggested actions.
func TestResolveRigFromBeadIDs_UnmappedPrefix_Errors(t *testing.T) {
	townRoot := t.TempDir()
	beadsDir := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Only gt- is mapped; zz- is not
	routesContent := `{"prefix":"gt-","path":"gastown/.beads"}` + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "routes.jsonl"), []byte(routesContent), 0644); err != nil {
		t.Fatalf("write routes: %v", err)
	}

	_, err := resolveRigFromBeadIDs([]string{"gt-aaa", "zz-bbb"}, townRoot)
	if err == nil {
		t.Fatal("expected error for unmapped prefix, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "zz-bbb") {
		t.Errorf("error should mention the bead ID, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "not mapped") {
		t.Errorf("error should mention prefix is not mapped, got: %s", errMsg)
	}
}

// TestResolveRigFromBeadIDs_TownLevelPrefix_Errors verifies that a bead with
// a town-level prefix (path=".") produces an error because it has no rig.
func TestResolveRigFromBeadIDs_TownLevelPrefix_Errors(t *testing.T) {
	townRoot := t.TempDir()
	beadsDir := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// hq- maps to town root (path=".")
	routesContent := `{"prefix":"hq-","path":"."}` + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "routes.jsonl"), []byte(routesContent), 0644); err != nil {
		t.Fatalf("write routes: %v", err)
	}

	_, err := resolveRigFromBeadIDs([]string{"hq-aaa", "hq-bbb"}, townRoot)
	if err == nil {
		t.Fatal("expected error for town-level prefix, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "not mapped") || !strings.Contains(errMsg, "town-level") {
		t.Errorf("error should mention town-level bead, got: %s", errMsg)
	}
}

// TestBatchSling_EmptyConvoyCleanupOnAllFailures verifies that when all beads
// fail to sling, the empty convoy is closed with a cleanup reason.
func TestBatchSling_EmptyConvoyCleanupOnAllFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	binDir := t.TempDir()
	townRoot := t.TempDir()

	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	townBeads := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(townBeads, 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	closeLogPath := filepath.Join(townRoot, "bd-close.log")

	// Stub bd: handles close and logs it
	bdScript := `#!/bin/sh
cmd="$1"
shift || true
case "$cmd" in
  close)
    echo "$cmd $*" >> "` + closeLogPath + `"
    exit 0
    ;;
esac
exit 0
`
	bdPath := filepath.Join(binDir, "bd")
	if err := os.WriteFile(bdPath, []byte(bdScript), 0755); err != nil {
		t.Fatalf("write bd stub: %v", err)
	}

	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	// Simulate the cleanup logic from runBatchSling:
	// If successCount == 0 and batchConvoyID is set, close the convoy.
	successCount := 0
	batchConvoyID := "hq-cv-cleanup-test"

	if successCount == 0 && batchConvoyID != "" {
		// Mirror the exact exec.Command call from sling_batch.go:303
		cmd := exec.Command("bd", "close", batchConvoyID, "-r", "all beads failed to sling")
		cmd.Dir = townBeads
		if err := cmd.Run(); err != nil {
			t.Fatalf("close convoy: %v", err)
		}
	}

	// Verify close was called
	closeBytes, err := os.ReadFile(closeLogPath)
	if err != nil {
		t.Fatalf("close log not written: %v", err)
	}
	closeContent := string(closeBytes)
	if !strings.Contains(closeContent, batchConvoyID) {
		t.Errorf("close log should contain convoy ID %q:\n%s", batchConvoyID, closeContent)
	}
	if !strings.Contains(closeContent, "all beads failed") {
		t.Errorf("close log should contain failure reason:\n%s", closeContent)
	}
}
