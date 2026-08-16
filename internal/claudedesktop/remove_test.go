package claudedesktop_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/claudedesktop"
)

func TestRemoveFrom_PreservesPreferencesAndServers(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "claude_desktop_config.json")
	seed := `{
  "preferences": {"theme": "dark"},
  "mcpServers": {
    "atlassian-platform-connector": {"command":"exe","args":["mcp"]},
    "other-server": {"command":"other"}
  }
}`
	if err := os.WriteFile(configPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := claudedesktop.RemoveFrom(configPath)
	if err != nil || !removed {
		t.Fatalf("RemoveFrom: removed=%v err=%v", removed, err)
	}

	var root map[string]json.RawMessage
	data, _ := os.ReadFile(configPath)
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("result not valid JSON: %v", err)
	}
	if _, ok := root["preferences"]; !ok {
		t.Error("preferences must be preserved")
	}
	var mcp map[string]json.RawMessage
	_ = json.Unmarshal(root["mcpServers"], &mcp)
	if _, ok := mcp["atlassian-platform-connector"]; ok {
		t.Error("connector entry should be gone")
	}
	if _, ok := mcp["other-server"]; !ok {
		t.Error("other-server should be preserved")
	}
}

func TestRemoveFrom_MissingFile(t *testing.T) {
	removed, err := claudedesktop.RemoveFrom(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil || removed {
		t.Errorf("missing file: removed=%v err=%v", removed, err)
	}
}
