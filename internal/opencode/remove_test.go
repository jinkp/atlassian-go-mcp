package opencode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/opencode"
)

func TestRemoveFrom_PreservesOtherServers(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	seed := `{
  "theme": "dark",
  "mcp": {
    "atlassian-platform-connector": {"type":"local","command":["exe","mcp"],"enabled":true},
    "other-server": {"type":"local","command":["other"],"enabled":true}
  }
}`
	if err := os.WriteFile(configPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := opencode.RemoveFrom(configPath)
	if err != nil {
		t.Fatalf("RemoveFrom: %v", err)
	}
	if !removed {
		t.Fatal("expected removed=true")
	}

	var root map[string]json.RawMessage
	data, _ := os.ReadFile(configPath)
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("result not valid JSON: %v", err)
	}
	// unrelated root key preserved
	if _, ok := root["theme"]; !ok {
		t.Error("theme key was lost")
	}
	var mcp map[string]json.RawMessage
	_ = json.Unmarshal(root["mcp"], &mcp)
	if _, ok := mcp["atlassian-platform-connector"]; ok {
		t.Error("connector entry should be gone")
	}
	if _, ok := mcp["other-server"]; !ok {
		t.Error("other-server should be preserved")
	}
}

func TestRemoveFrom_OnlyEntryRemovesSection(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	seed := `{"mcp":{"atlassian-platform-connector":{"type":"local","command":["exe","mcp"]}}}`
	if err := os.WriteFile(configPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := opencode.RemoveFrom(configPath)
	if err != nil || !removed {
		t.Fatalf("RemoveFrom: removed=%v err=%v", removed, err)
	}
	var root map[string]json.RawMessage
	data, _ := os.ReadFile(configPath)
	_ = json.Unmarshal(data, &root)
	if _, ok := root["mcp"]; ok {
		t.Error("empty mcp section should be removed")
	}
}

func TestRemoveFrom_NotPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{"mcp":{"other":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := opencode.RemoveFrom(configPath)
	if err != nil {
		t.Fatalf("RemoveFrom: %v", err)
	}
	if removed {
		t.Error("expected removed=false when entry absent")
	}
}

func TestRemoveFrom_FileMissing(t *testing.T) {
	removed, err := opencode.RemoveFrom(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if removed {
		t.Error("expected removed=false for missing file")
	}
}
