package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
)

func TestNewPatrolRolesHavePromptsCheck(t *testing.T) {
	check := NewPatrolRolesHavePromptsCheck()
	if check == nil {
		t.Fatal("NewPatrolRolesHavePromptsCheck() returned nil")
	}
	if check.Name() != "patrol-roles-have-prompts" {
		t.Errorf("Name() = %q, want %q", check.Name(), "patrol-roles-have-prompts")
	}
	if !check.CanFix() {
		t.Error("CanFix() should return true")
	}
}

func setupRigConfig(t *testing.T, tmpDir string, rigNames []string) {
	t.Helper()
	mayorDir := filepath.Join(tmpDir, "mayor")
	if err := os.MkdirAll(mayorDir, 0755); err != nil {
		t.Fatalf("mkdir mayor: %v", err)
	}

	rigsConfig := config.RigsConfig{Rigs: make(map[string]config.RigEntry)}
	for _, name := range rigNames {
		rigsConfig.Rigs[name] = config.RigEntry{}
	}

	data, err := json.Marshal(rigsConfig)
	if err != nil {
		t.Fatalf("marshal rigs.json: %v", err)
	}

	if err := os.WriteFile(filepath.Join(mayorDir, "rigs.json"), data, 0644); err != nil {
		t.Fatalf("write rigs.json: %v", err)
	}
}

func setupRigTemplatesDir(t *testing.T, tmpDir, rigName string) string {
	t.Helper()
	templatesDir := filepath.Join(tmpDir, rigName, "mayor", "rig", "internal", "templates", "roles")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	return templatesDir
}

func TestPatrolRolesHavePromptsCheck_NoRigs(t *testing.T) {
	tmpDir := t.TempDir()

	check := NewPatrolRolesHavePromptsCheck()
	ctx := &CheckContext{TownRoot: tmpDir}

	result := check.Run(ctx)

	if result.Status != StatusOK {
		t.Errorf("Status = %v, want OK (no rigs configured)", result.Status)
	}
}

func TestPatrolRolesHavePromptsCheck_NoTemplatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	setupRigConfig(t, tmpDir, []string{"myproject"})

	check := NewPatrolRolesHavePromptsCheck()
	ctx := &CheckContext{TownRoot: tmpDir}

	result := check.Run(ctx)

	if result.Status != StatusWarning {
		t.Errorf("Status = %v, want Warning", result.Status)
	}
	if len(check.missingByRig) != 1 {
		t.Errorf("missingByRig count = %d, want 1", len(check.missingByRig))
	}
	if len(check.missingByRig["myproject"]) != 3 {
		t.Errorf("missing templates for myproject = %d, want 3", len(check.missingByRig["myproject"]))
	}
}

func TestPatrolRolesHavePromptsCheck_SomeTemplatesMissing(t *testing.T) {
	tmpDir := t.TempDir()
	setupRigConfig(t, tmpDir, []string{"myproject"})
	templatesDir := setupRigTemplatesDir(t, tmpDir, "myproject")

	if err := os.WriteFile(filepath.Join(templatesDir, "deacon.md.tmpl"), []byte("test"), 0644); err != nil {
		t.Fatalf("write deacon.md.tmpl: %v", err)
	}

	check := NewPatrolRolesHavePromptsCheck()
	ctx := &CheckContext{TownRoot: tmpDir}

	result := check.Run(ctx)

	if result.Status != StatusWarning {
		t.Errorf("Status = %v, want Warning", result.Status)
	}
	if len(check.missingByRig["myproject"]) != 2 {
		t.Errorf("missing templates = %d, want 2 (witness, refinery)", len(check.missingByRig["myproject"]))
	}
}

func TestPatrolRolesHavePromptsCheck_AllTemplatesExist(t *testing.T) {
	tmpDir := t.TempDir()
	setupRigConfig(t, tmpDir, []string{"myproject"})
	templatesDir := setupRigTemplatesDir(t, tmpDir, "myproject")

	for _, tmpl := range requiredRolePrompts {
		if err := os.WriteFile(filepath.Join(templatesDir, tmpl), []byte("test content"), 0644); err != nil {
			t.Fatalf("write %s: %v", tmpl, err)
		}
	}

	check := NewPatrolRolesHavePromptsCheck()
	ctx := &CheckContext{TownRoot: tmpDir}

	result := check.Run(ctx)

	if result.Status != StatusOK {
		t.Errorf("Status = %v, want OK", result.Status)
	}
	if len(check.missingByRig) != 0 {
		t.Errorf("missingByRig count = %d, want 0", len(check.missingByRig))
	}
}

func TestPatrolRolesHavePromptsCheck_Fix(t *testing.T) {
	tmpDir := t.TempDir()
	setupRigConfig(t, tmpDir, []string{"myproject"})

	check := NewPatrolRolesHavePromptsCheck()
	ctx := &CheckContext{TownRoot: tmpDir}

	result := check.Run(ctx)
	if result.Status != StatusWarning {
		t.Fatalf("Initial Status = %v, want Warning", result.Status)
	}

	err := check.Fix(ctx)
	if err != nil {
		t.Fatalf("Fix() error = %v", err)
	}

	templatesDir := filepath.Join(tmpDir, "myproject", "mayor", "rig", "internal", "templates", "roles")
	for _, tmpl := range requiredRolePrompts {
		path := filepath.Join(templatesDir, tmpl)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("Fix() did not create %s: %v", tmpl, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("Fix() created empty file %s", tmpl)
		}
	}

	result = check.Run(ctx)
	if result.Status != StatusOK {
		t.Errorf("After Fix(), Status = %v, want OK", result.Status)
	}
}

func TestPatrolRolesHavePromptsCheck_FixPartial(t *testing.T) {
	tmpDir := t.TempDir()
	setupRigConfig(t, tmpDir, []string{"myproject"})
	templatesDir := setupRigTemplatesDir(t, tmpDir, "myproject")

	existingContent := []byte("existing custom content")
	if err := os.WriteFile(filepath.Join(templatesDir, "deacon.md.tmpl"), existingContent, 0644); err != nil {
		t.Fatalf("write deacon.md.tmpl: %v", err)
	}

	check := NewPatrolRolesHavePromptsCheck()
	ctx := &CheckContext{TownRoot: tmpDir}

	result := check.Run(ctx)
	if result.Status != StatusWarning {
		t.Fatalf("Initial Status = %v, want Warning", result.Status)
	}
	if len(check.missingByRig["myproject"]) != 2 {
		t.Fatalf("missing = %d, want 2", len(check.missingByRig["myproject"]))
	}

	err := check.Fix(ctx)
	if err != nil {
		t.Fatalf("Fix() error = %v", err)
	}

	deaconContent, err := os.ReadFile(filepath.Join(templatesDir, "deacon.md.tmpl"))
	if err != nil {
		t.Fatalf("read deacon.md.tmpl: %v", err)
	}
	if string(deaconContent) != string(existingContent) {
		t.Error("Fix() should not overwrite existing deacon.md.tmpl")
	}

	for _, tmpl := range []string{"witness.md.tmpl", "refinery.md.tmpl"} {
		path := filepath.Join(templatesDir, tmpl)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("Fix() did not create %s: %v", tmpl, err)
		}
	}
}

func TestPatrolRolesHavePromptsCheck_MultipleRigs(t *testing.T) {
	tmpDir := t.TempDir()
	setupRigConfig(t, tmpDir, []string{"project1", "project2"})

	templatesDir1 := setupRigTemplatesDir(t, tmpDir, "project1")
	for _, tmpl := range requiredRolePrompts {
		if err := os.WriteFile(filepath.Join(templatesDir1, tmpl), []byte("test"), 0644); err != nil {
			t.Fatalf("write %s: %v", tmpl, err)
		}
	}

	check := NewPatrolRolesHavePromptsCheck()
	ctx := &CheckContext{TownRoot: tmpDir}

	result := check.Run(ctx)

	if result.Status != StatusWarning {
		t.Errorf("Status = %v, want Warning (project2 missing)", result.Status)
	}
	if _, ok := check.missingByRig["project1"]; ok {
		t.Error("project1 should not be in missingByRig")
	}
	if len(check.missingByRig["project2"]) != 3 {
		t.Errorf("project2 missing = %d, want 3", len(check.missingByRig["project2"]))
	}
}

func TestPatrolRolesHavePromptsCheck_FixHint(t *testing.T) {
	tmpDir := t.TempDir()
	setupRigConfig(t, tmpDir, []string{"myproject"})

	check := NewPatrolRolesHavePromptsCheck()
	ctx := &CheckContext{TownRoot: tmpDir}

	result := check.Run(ctx)

	if result.FixHint == "" {
		t.Error("FixHint should not be empty for warning status")
	}
	if result.FixHint != "Run 'gt doctor --fix' to copy embedded templates to rig repos" {
		t.Errorf("FixHint = %q, unexpected value", result.FixHint)
	}
}
