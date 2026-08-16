package cursor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/cursor"
)

func TestRemoveFrom_PreservesOtherServers(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")
	seed := `{
  "mcpServers": {
    "atlassian-mcp": {"command":"exe","args":["mcp"]},
    "other-server": {"command":"other"}
  }
}`
	if err := os.WriteFile(configPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := cursor.RemoveFrom(configPath)
	if err != nil || !removed {
		t.Fatalf("RemoveFrom: removed=%v err=%v", removed, err)
	}

	var root map[string]json.RawMessage
	data, _ := os.ReadFile(configPath)
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("result not valid JSON: %v", err)
	}
	var mcp map[string]json.RawMessage
	_ = json.Unmarshal(root["mcpServers"], &mcp)
	if _, ok := mcp["atlassian-mcp"]; ok {
		t.Error("connector entry should be gone")
	}
	if _, ok := mcp["other-server"]; !ok {
		t.Error("other-server should be preserved")
	}
}

func TestRemoveFrom_NotPresentAndMissing(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(configPath, []byte(`{"mcpServers":{"other":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if removed, err := cursor.RemoveFrom(configPath); err != nil || removed {
		t.Errorf("not present: removed=%v err=%v", removed, err)
	}
	if removed, err := cursor.RemoveFrom(filepath.Join(dir, "nope.json")); err != nil || removed {
		t.Errorf("missing file: removed=%v err=%v", removed, err)
	}
}
