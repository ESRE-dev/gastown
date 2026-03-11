package doltserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanStalePortFiles(t *testing.T) {
	townRoot := t.TempDir()

	// Create a .beads dir with a stale dolt-server.port
	beadsDir := filepath.Join(townRoot, "somerig", ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}
	portFile := filepath.Join(beadsDir, "dolt-server.port")
	if err := os.WriteFile(portFile, []byte("13592"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create another .beads dir with correct port
	correctDir := filepath.Join(townRoot, "other", ".beads")
	if err := os.MkdirAll(correctDir, 0755); err != nil {
		t.Fatal(err)
	}
	correctFile := filepath.Join(correctDir, "dolt-server.port")
	if err := os.WriteFile(correctFile, []byte("3307"), 0644); err != nil {
		t.Fatal(err)
	}

	cleanStalePortFiles(townRoot, 3307)

	// Stale file should be updated
	data, err := os.ReadFile(portFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "3307" {
		t.Errorf("stale port file not fixed: got %q, want %q", string(data), "3307")
	}

	// Correct file should be unchanged
	data, err = os.ReadFile(correctFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "3307" {
		t.Errorf("correct port file changed: got %q, want %q", string(data), "3307")
	}
}

func TestCleanStalePortFiles_FixesMetadataJSON(t *testing.T) {
	townRoot := t.TempDir()

	// Create a .beads dir with a stale metadata.json
	beadsDir := filepath.Join(townRoot, "rig", ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	meta := map[string]interface{}{
		"dolt_mode":        "server",
		"dolt_database":    "testdb",
		"dolt_server_port": 13592,
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	metaFile := filepath.Join(beadsDir, "metadata.json")
	if err := os.WriteFile(metaFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	cleanStalePortFiles(townRoot, 3307)

	// Read back and verify port was fixed
	updated, err := os.ReadFile(metaFile)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(updated, &result); err != nil {
		t.Fatal(err)
	}
	port, ok := result["dolt_server_port"].(float64)
	if !ok || int(port) != 3307 {
		t.Errorf("metadata.json port not fixed: got %v, want 3307", result["dolt_server_port"])
	}
	// Other fields preserved
	if result["dolt_mode"] != "server" {
		t.Errorf("dolt_mode lost: got %v", result["dolt_mode"])
	}
	if result["dolt_database"] != "testdb" {
		t.Errorf("dolt_database lost: got %v", result["dolt_database"])
	}
}

func TestCleanStalePortFiles_SkipsCorrectMetadata(t *testing.T) {
	townRoot := t.TempDir()

	beadsDir := filepath.Join(townRoot, "rig", ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	meta := map[string]interface{}{
		"dolt_server_port": 3307,
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	metaFile := filepath.Join(beadsDir, "metadata.json")
	if err := os.WriteFile(metaFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Get original mtime
	origInfo, _ := os.Stat(metaFile)
	origMod := origInfo.ModTime()

	cleanStalePortFiles(townRoot, 3307)

	// File should not have been rewritten
	newInfo, _ := os.Stat(metaFile)
	if !newInfo.ModTime().Equal(origMod) {
		t.Error("metadata.json was rewritten even though port was correct")
	}
}

func TestCleanStalePortFiles_SkipsMetadataWithoutPort(t *testing.T) {
	townRoot := t.TempDir()

	beadsDir := filepath.Join(townRoot, "rig", ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	meta := map[string]interface{}{
		"dolt_mode":     "server",
		"dolt_database": "testdb",
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	metaFile := filepath.Join(beadsDir, "metadata.json")
	if err := os.WriteFile(metaFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	origInfo, _ := os.Stat(metaFile)
	origMod := origInfo.ModTime()

	cleanStalePortFiles(townRoot, 3307)

	// File should not have been rewritten (no dolt_server_port field)
	newInfo, _ := os.Stat(metaFile)
	if !newInfo.ModTime().Equal(origMod) {
		t.Error("metadata.json without dolt_server_port was unexpectedly modified")
	}
}
