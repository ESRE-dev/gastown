package commands

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBuildCommand_Claude(t *testing.T) {
	cmd := FindByName("handoff")
	if cmd == nil {
		t.Fatal("handoff command not found")
	}
	content, err := BuildCommand(*cmd, "claude")
	if err != nil {
		t.Fatalf("BuildCommand failed: %v", err)
	}

	// Check frontmatter
	if !strings.Contains(content, "description: Hand off to fresh session") {
		t.Error("missing description")
	}
	if !strings.Contains(content, "allowed-tools: Bash(gt handoff:*)") {
		t.Error("missing allowed-tools for Claude")
	}
	if !strings.Contains(content, "argument-hint: [message]") {
		t.Error("missing argument-hint for Claude")
	}

	// Check body
	if !strings.Contains(content, "$ARGUMENTS") {
		t.Error("missing $ARGUMENTS in body")
	}
}

func TestBuildCommand_OpenCode(t *testing.T) {
	cmd := FindByName("handoff")
	if cmd == nil {
		t.Fatal("handoff command not found")
	}
	content, err := BuildCommand(*cmd, "opencode")
	if err != nil {
		t.Fatalf("BuildCommand failed: %v", err)
	}

	// OpenCode skill frontmatter: name + description, no Claude-specific fields.
	if !strings.Contains(content, "name: handoff") {
		t.Error("missing name field in OpenCode skill frontmatter")
	}
	if !strings.Contains(content, "description: Hand off to fresh session") {
		t.Error("missing description")
	}
	if strings.Contains(content, "allowed-tools") {
		t.Error("OpenCode should not have allowed-tools")
	}
	if strings.Contains(content, "argument-hint") {
		t.Error("OpenCode should not have argument-hint")
	}

	// Check body
	if !strings.Contains(content, "$ARGUMENTS") {
		t.Error("missing $ARGUMENTS in body")
	}
}

func TestBuildCommand_Copilot(t *testing.T) {
	cmd := FindByName("handoff")
	if cmd == nil {
		t.Fatal("handoff command not found")
	}
	content, err := BuildCommand(*cmd, "copilot")
	if err != nil {
		t.Fatalf("BuildCommand failed: %v", err)
	}

	// Check frontmatter - only description, no Claude-specific fields
	if !strings.Contains(content, "description: Hand off to fresh session") {
		t.Error("missing description")
	}
	if strings.Contains(content, "allowed-tools") {
		t.Error("Copilot should not have allowed-tools")
	}
	if strings.Contains(content, "argument-hint") {
		t.Error("Copilot should not have argument-hint")
	}

	// Check body
	if !strings.Contains(content, "$ARGUMENTS") {
		t.Error("missing $ARGUMENTS in body")
	}
}

func TestBuildCommand_Review_Claude(t *testing.T) {
	cmd := FindByName("review")
	if cmd == nil {
		t.Fatal("review command not found")
	}
	content, err := BuildCommand(*cmd, "claude")
	if err != nil {
		t.Fatalf("BuildCommand failed: %v", err)
	}

	// Check frontmatter
	if !strings.Contains(content, "description: Review code changes with structured grading") {
		t.Error("missing description")
	}
	if !strings.Contains(content, "allowed-tools:") {
		t.Error("missing allowed-tools for Claude")
	}
	if !strings.Contains(content, "argument-hint:") {
		t.Error("missing argument-hint for Claude")
	}

	// Check body
	if !strings.Contains(content, "$ARGUMENTS") {
		t.Error("missing $ARGUMENTS in body")
	}
	if !strings.Contains(content, "CRITICAL") {
		t.Error("missing CRITICAL severity in body")
	}
	if !strings.Contains(content, "Grade") {
		t.Error("missing Grade in body")
	}
}

func TestNames(t *testing.T) {
	names := Names()
	if len(names) < 2 {
		t.Errorf("expected at least 2 commands, got %d", len(names))
	}
	if !slices.Contains(names, "handoff") {
		t.Error("missing handoff command")
	}
	if !slices.Contains(names, "review") {
		t.Error("missing review command")
	}
}

func TestProvisionFor_Claude(t *testing.T) {
	tmp := t.TempDir()

	if err := ProvisionFor(tmp, "claude"); err != nil {
		t.Fatalf("ProvisionFor claude failed: %v", err)
	}

	// Claude commands go to .claude/commands/<name>.md
	handoffPath := filepath.Join(tmp, ".claude", "commands", "handoff.md")
	data, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("handoff.md not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "allowed-tools: Bash(gt handoff:*)") {
		t.Error("missing Claude-specific allowed-tools in handoff")
	}

	reviewPath := filepath.Join(tmp, ".claude", "commands", "review.md")
	if _, err := os.Stat(reviewPath); err != nil {
		t.Fatalf("review.md not created: %v", err)
	}
}

func TestProvisionFor_OpenCode(t *testing.T) {
	tmp := t.TempDir()

	if err := ProvisionFor(tmp, "opencode"); err != nil {
		t.Fatalf("ProvisionFor opencode failed: %v", err)
	}

	// OpenCode commands go to .opencode/skills/<name>/SKILL.md
	handoffPath := filepath.Join(tmp, ".opencode", "skills", "handoff", "SKILL.md")
	data, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("handoff SKILL.md not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: handoff") {
		t.Error("missing name field in OpenCode skill frontmatter")
	}
	if !strings.Contains(content, "description: Hand off to fresh session") {
		t.Error("missing description in OpenCode skill frontmatter")
	}
	if strings.Contains(content, "allowed-tools") {
		t.Error("OpenCode should not have allowed-tools")
	}

	reviewPath := filepath.Join(tmp, ".opencode", "skills", "review", "SKILL.md")
	if _, err := os.Stat(reviewPath); err != nil {
		t.Fatalf("review SKILL.md not created: %v", err)
	}
}

func TestProvisionFor_DoesNotOverwrite(t *testing.T) {
	tmp := t.TempDir()

	// First provision
	if err := ProvisionFor(tmp, "opencode"); err != nil {
		t.Fatalf("first ProvisionFor failed: %v", err)
	}

	handoffPath := filepath.Join(tmp, ".opencode", "skills", "handoff", "SKILL.md")

	// Overwrite with sentinel
	sentinel := "CUSTOM CONTENT"
	if err := os.WriteFile(handoffPath, []byte(sentinel), 0644); err != nil {
		t.Fatal(err)
	}

	// Re-provision should not overwrite
	if err := ProvisionFor(tmp, "opencode"); err != nil {
		t.Fatalf("second ProvisionFor failed: %v", err)
	}

	data, _ := os.ReadFile(handoffPath)
	if string(data) != sentinel {
		t.Error("ProvisionFor overwrote existing file")
	}
}

func TestMissingFor_Claude(t *testing.T) {
	tmp := t.TempDir()

	missing := MissingFor(tmp, "claude")
	if len(missing) != len(Commands) {
		t.Errorf("expected %d missing, got %d", len(Commands), len(missing))
	}

	// Provision, then check again
	if err := ProvisionFor(tmp, "claude"); err != nil {
		t.Fatal(err)
	}
	missing = MissingFor(tmp, "claude")
	if len(missing) != 0 {
		t.Errorf("expected 0 missing after provision, got %d: %v", len(missing), missing)
	}
}

func TestMissingFor_OpenCode(t *testing.T) {
	tmp := t.TempDir()

	missing := MissingFor(tmp, "opencode")
	if len(missing) != len(Commands) {
		t.Errorf("expected %d missing, got %d", len(Commands), len(missing))
	}

	// Provision, then check again
	if err := ProvisionFor(tmp, "opencode"); err != nil {
		t.Fatal(err)
	}
	missing = MissingFor(tmp, "opencode")
	if len(missing) != 0 {
		t.Errorf("expected 0 missing after provision, got %d: %v", len(missing), missing)
	}
}

func TestIsKnownAgent_ValidAgents(t *testing.T) {
	if !IsKnownAgent("claude") {
		t.Error("claude should be a known agent")
	}
	if !IsKnownAgent("opencode") {
		t.Error("opencode should be a known agent")
	}
	if IsKnownAgent("unknown-agent") {
		t.Error("unknown-agent should not be a known agent")
	}
}
