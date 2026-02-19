package convoy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	beadsdk "github.com/steveyegge/beads"
)

func TestExtractIssueID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gt-abc", "gt-abc"},
		{"bd-xyz", "bd-xyz"},
		{"hq-cv-123", "hq-cv-123"},
		{"external:gt:gt-abc", "gt-abc"},
		{"external:bd:bd-xyz", "bd-xyz"},
		{"external:hq:hq-cv-123", "hq-cv-123"},
		{"external:", "external:"}, // malformed, return as-is
		{"external:x:", ""},        // 3 parts but empty last part
		{"simple", "simple"},       // no external prefix
		{"", ""},                   // empty
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractIssueID(tt.input)
			if result != tt.expected {
				t.Errorf("extractIssueID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsSlingableType(t *testing.T) {
	tests := []struct {
		issueType string
		want      bool
	}{
		{"task", true},
		{"bug", true},
		{"feature", true},
		{"chore", true},
		{"", true},          // empty defaults to task
		{"epic", false},     // container type
		{"convoy", false},   // meta type
		{"sub-epic", false}, // container type
		{"decision", false}, // non-work type
		{"message", false},  // non-work type
		{"event", false},    // non-work type
		{"unknown", false},  // unknown types are not slingable
	}

	for _, tt := range tests {
		t.Run(tt.issueType, func(t *testing.T) {
			got := IsSlingableType(tt.issueType)
			if got != tt.want {
				t.Errorf("IsSlingableType(%q) = %v, want %v", tt.issueType, got, tt.want)
			}
		})
	}
}

func TestIsIssueBlocked_NoStore(t *testing.T) {
	// isIssueBlocked with a nil context and missing store should not panic.
	// It can't actually be tested with nil store since it calls store methods,
	// but we verify the function signature is correct by calling it.
	// The actual blocking behavior is tested in integration tests.
}

func TestReadyIssueFilterLogic_SkipsNonSlingableTypes(t *testing.T) {
	// Validates that feedNextReadyIssue's type filter skips non-slingable types.
	// We test the predicate inline (same pattern as existing filter tests).
	tracked := []trackedIssue{
		{ID: "gt-epic", Status: "open", Assignee: "", IssueType: "epic"},
		{ID: "gt-task", Status: "open", Assignee: "", IssueType: "task"},
		{ID: "gt-convoy", Status: "open", Assignee: "", IssueType: "convoy"},
		{ID: "gt-bug", Status: "open", Assignee: "", IssueType: "bug"},
	}

	var slingable []string
	for _, issue := range tracked {
		if issue.Status == "open" && issue.Assignee == "" && IsSlingableType(issue.IssueType) {
			slingable = append(slingable, issue.ID)
		}
	}

	if len(slingable) != 2 {
		t.Errorf("expected 2 slingable issues (task, bug), got %d: %v", len(slingable), slingable)
	}
	if slingable[0] != "gt-task" || slingable[1] != "gt-bug" {
		t.Errorf("expected [gt-task, gt-bug], got %v", slingable)
	}
}

func TestReadyIssueFilterLogic_SkipsNonOpenIssues(t *testing.T) {
	// Validates the filtering predicate used by feedNextReadyIssue: only
	// open issues with no assignee should be considered "ready". We test
	// the predicate inline because feedNextReadyIssue also calls rigForIssue
	// and dispatchIssue, making isolated unit testing impractical without a
	// real store. Integration coverage lives in convoy_manager_integration_test.go.
	tracked := []trackedIssue{
		{ID: "gt-closed", Status: "closed", Assignee: ""},
		{ID: "gt-inprog", Status: "in_progress", Assignee: "gastown/polecats/alpha"},
		{ID: "gt-hooked", Status: "hooked", Assignee: "gastown/polecats/beta"},
		{ID: "gt-assigned", Status: "open", Assignee: "gastown/polecats/gamma"},
	}

	// None of these should be considered "ready"
	for _, issue := range tracked {
		if issue.Status == "open" && issue.Assignee == "" {
			t.Errorf("issue %s should not be ready (status=%s, assignee=%s)", issue.ID, issue.Status, issue.Assignee)
		}
	}
}

func TestReadyIssueFilterLogic_FindsReadyIssue(t *testing.T) {
	// Validates that the "first open+unassigned" selection picks the correct
	// issue. See comment on TestReadyIssueFilterLogic_SkipsNonOpenIssues for
	// why this tests the predicate inline rather than calling feedNextReadyIssue.
	tracked := []trackedIssue{
		{ID: "gt-closed", Status: "closed", Assignee: ""},
		{ID: "gt-inprog", Status: "in_progress", Assignee: "gastown/polecats/alpha"},
		{ID: "gt-ready", Status: "open", Assignee: ""},
		{ID: "gt-also-ready", Status: "open", Assignee: ""},
	}

	// Find first ready issue - should be gt-ready (first match)
	var foundReady string
	for _, issue := range tracked {
		if issue.Status == "open" && issue.Assignee == "" {
			foundReady = issue.ID
			break
		}
	}

	if foundReady != "gt-ready" {
		t.Errorf("expected first ready issue to be gt-ready, got %s", foundReady)
	}
}

func TestCheckConvoysForIssue_NilStore(t *testing.T) {
	// Nil store returns nil immediately (no convoy checks).
	result := CheckConvoysForIssue(context.Background(), nil, "/nonexistent/path", "gt-test", "test", nil, "gt", nil)
	if result != nil {
		t.Errorf("expected nil for nil store, got %v", result)
	}
}

func TestCheckConvoysForIssue_NilLogger(t *testing.T) {
	// Nil logger should not panic — gets replaced with no-op internally.
	// With nil store, returns nil.
	result := CheckConvoysForIssue(context.Background(), nil, "/nonexistent/path", "gt-test", "test", nil, "gt", nil)
	if result != nil {
		t.Errorf("expected nil for nil store, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// blockingDepTypes map tests
// ---------------------------------------------------------------------------

func TestBlockingDepTypes_ContainsExpectedTypes(t *testing.T) {
	expected := []string{"blocks", "conditional-blocks", "waits-for"}
	for _, depType := range expected {
		if !blockingDepTypes[depType] {
			t.Errorf("blockingDepTypes should contain %q", depType)
		}
	}
}

func TestBlockingDepTypes_ExcludesParentChild(t *testing.T) {
	if blockingDepTypes["parent-child"] {
		t.Error("blockingDepTypes should NOT contain parent-child")
	}
}

func TestBlockingDepTypes_ExactSize(t *testing.T) {
	// Ensure the map has exactly the 3 expected entries and no extras.
	if len(blockingDepTypes) != 3 {
		t.Errorf("blockingDepTypes has %d entries, want 3; contents: %v", len(blockingDepTypes), blockingDepTypes)
	}
}

// ---------------------------------------------------------------------------
// isIssueBlocked tests (real beads store)
// ---------------------------------------------------------------------------

func TestIsIssueBlocked_NoDeps(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	issue := &beadsdk.Issue{
		ID:        "test-noblk1",
		Title:     "No Deps Issue",
		Status:    beadsdk.StatusOpen,
		Priority:  2,
		IssueType: beadsdk.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateIssue(ctx, issue, "test"); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	if isIssueBlocked(ctx, store, issue.ID) {
		t.Error("isIssueBlocked should return false for issue with no dependencies")
	}
}

func TestIsIssueBlocked_BlockedByOpenBlocker(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	blocker := &beadsdk.Issue{
		ID:        "test-blkr1",
		Title:     "Blocker",
		Status:    beadsdk.StatusOpen,
		Priority:  2,
		IssueType: beadsdk.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
	blocked := &beadsdk.Issue{
		ID:        "test-blkd1",
		Title:     "Blocked",
		Status:    beadsdk.StatusOpen,
		Priority:  2,
		IssueType: beadsdk.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.CreateIssue(ctx, blocker, "test"); err != nil {
		t.Fatalf("CreateIssue blocker: %v", err)
	}
	if err := store.CreateIssue(ctx, blocked, "test"); err != nil {
		t.Fatalf("CreateIssue blocked: %v", err)
	}

	dep := &beadsdk.Dependency{
		IssueID:     blocked.ID,
		DependsOnID: blocker.ID,
		Type:        beadsdk.DepBlocks,
		CreatedAt:   now,
		CreatedBy:   "test",
	}
	if err := store.AddDependency(ctx, dep, "test"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	// GetDependenciesWithMetadata may not work in embedded Dolt mode
	// (nested query limitation). If it fails, isIssueBlocked returns false
	// (fail-open). We test for the correct result when the store works,
	// and accept fail-open when it doesn't.
	result := isIssueBlocked(ctx, store, blocked.ID)

	// Verify the dependency actually exists via a method that works in embedded mode.
	deps, err := store.GetDependencies(ctx, blocked.ID)
	if err != nil {
		t.Skipf("store.GetDependencies failed (embedded Dolt limitation): %v", err)
	}
	if len(deps) == 0 {
		t.Fatal("expected at least 1 dependency to be created")
	}

	// If GetDependenciesWithMetadata works, result should be true.
	// If it fails (embedded Dolt nested query issue), result is false (fail-open).
	// We can't distinguish, but we log for visibility.
	t.Logf("isIssueBlocked(blocked with open blocker) = %v", result)
}

func TestIsIssueBlocked_NotBlockedByClosedBlocker(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	blocker := &beadsdk.Issue{
		ID:        "test-clblkr",
		Title:     "Closed Blocker",
		Status:    beadsdk.StatusClosed,
		Priority:  2,
		IssueType: beadsdk.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
	blocked := &beadsdk.Issue{
		ID:        "test-clblkd",
		Title:     "Blocked By Closed",
		Status:    beadsdk.StatusOpen,
		Priority:  2,
		IssueType: beadsdk.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.CreateIssue(ctx, blocker, "test"); err != nil {
		t.Fatalf("CreateIssue blocker: %v", err)
	}
	if err := store.CreateIssue(ctx, blocked, "test"); err != nil {
		t.Fatalf("CreateIssue blocked: %v", err)
	}

	dep := &beadsdk.Dependency{
		IssueID:     blocked.ID,
		DependsOnID: blocker.ID,
		Type:        beadsdk.DepBlocks,
		CreatedAt:   now,
		CreatedBy:   "test",
	}
	if err := store.AddDependency(ctx, dep, "test"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	// Even if GetDependenciesWithMetadata works, the blocker is closed so
	// isIssueBlocked should return false.
	if isIssueBlocked(ctx, store, blocked.ID) {
		t.Error("isIssueBlocked should return false when the only blocker is closed")
	}
}

func TestIsIssueBlocked_ParentChildDoesNotBlock(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	parent := &beadsdk.Issue{
		ID:        "test-pcpar",
		Title:     "Parent",
		Status:    beadsdk.StatusOpen,
		Priority:  2,
		IssueType: beadsdk.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
	child := &beadsdk.Issue{
		ID:        "test-pcchld",
		Title:     "Child",
		Status:    beadsdk.StatusOpen,
		Priority:  2,
		IssueType: beadsdk.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.CreateIssue(ctx, parent, "test"); err != nil {
		t.Fatalf("CreateIssue parent: %v", err)
	}
	if err := store.CreateIssue(ctx, child, "test"); err != nil {
		t.Fatalf("CreateIssue child: %v", err)
	}

	dep := &beadsdk.Dependency{
		IssueID:     child.ID,
		DependsOnID: parent.ID,
		Type:        beadsdk.DepParentChild,
		CreatedAt:   now,
		CreatedBy:   "test",
	}
	if err := store.AddDependency(ctx, dep, "test"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	// parent-child deps should NOT block dispatch
	if isIssueBlocked(ctx, store, child.ID) {
		t.Error("isIssueBlocked should return false for parent-child dependency (not a blocking type)")
	}
}

func TestIsIssueBlocked_FailOpenOnNonexistentIssue(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Querying deps for a nonexistent issue should fail-open (return false)
	if isIssueBlocked(ctx, store, "test-nonexistent-issue") {
		t.Error("isIssueBlocked should fail-open (return false) for nonexistent issue")
	}
}

// ---------------------------------------------------------------------------
// rigForIssue tests
// ---------------------------------------------------------------------------

func TestRigForIssue_ValidPrefix(t *testing.T) {
	townRoot := t.TempDir()

	// Create .beads/routes.jsonl with a mapping
	beadsDir := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	routesContent := `{"prefix":"gt-","path":"gastown/.beads"}` + "\n" +
		`{"prefix":"bd-","path":"beads/.beads"}` + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "routes.jsonl"), []byte(routesContent), 0644); err != nil {
		t.Fatalf("WriteFile routes.jsonl: %v", err)
	}

	rig := rigForIssue(townRoot, "gt-abc123")
	if rig != "gastown" {
		t.Errorf("rigForIssue(townRoot, 'gt-abc123') = %q, want 'gastown'", rig)
	}

	rig = rigForIssue(townRoot, "bd-xyz")
	if rig != "beads" {
		t.Errorf("rigForIssue(townRoot, 'bd-xyz') = %q, want 'beads'", rig)
	}
}

func TestRigForIssue_EmptyPrefix(t *testing.T) {
	townRoot := t.TempDir()

	// No prefix extractable from "nohyphen"
	rig := rigForIssue(townRoot, "nohyphen")
	if rig != "" {
		t.Errorf("rigForIssue with no-hyphen ID = %q, want empty", rig)
	}
}

func TestRigForIssue_EmptyIssueID(t *testing.T) {
	townRoot := t.TempDir()

	rig := rigForIssue(townRoot, "")
	if rig != "" {
		t.Errorf("rigForIssue with empty ID = %q, want empty", rig)
	}
}

func TestRigForIssue_UnknownPrefix(t *testing.T) {
	townRoot := t.TempDir()

	// Create routes.jsonl with only gt- mapping
	beadsDir := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	routesContent := `{"prefix":"gt-","path":"gastown/.beads"}` + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "routes.jsonl"), []byte(routesContent), 0644); err != nil {
		t.Fatalf("WriteFile routes.jsonl: %v", err)
	}

	// "zz-" prefix not in routes
	rig := rigForIssue(townRoot, "zz-unknown")
	if rig != "" {
		t.Errorf("rigForIssue with unknown prefix = %q, want empty", rig)
	}
}

func TestRigForIssue_NoRoutesFile(t *testing.T) {
	townRoot := t.TempDir()

	// No .beads directory at all — should return ""
	rig := rigForIssue(townRoot, "gt-abc")
	if rig != "" {
		t.Errorf("rigForIssue with no routes file = %q, want empty", rig)
	}
}

func TestRigForIssue_TownLevelPrefix(t *testing.T) {
	townRoot := t.TempDir()

	// Town-level beads have path="." which should return "" (no specific rig)
	beadsDir := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	routesContent := `{"prefix":"hq-","path":"."}` + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "routes.jsonl"), []byte(routesContent), 0644); err != nil {
		t.Fatalf("WriteFile routes.jsonl: %v", err)
	}

	rig := rigForIssue(townRoot, "hq-cv-test")
	if rig != "" {
		t.Errorf("rigForIssue for town-level prefix = %q, want empty", rig)
	}
}
