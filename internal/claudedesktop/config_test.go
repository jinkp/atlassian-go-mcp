package claudedesktop_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/claudedesktop"
)

func TestGlobalPath(t *testing.T) {
	p := claudedesktop.GlobalPath()
	if p == "" {
		t.Fatal("GlobalPath returned empty string")
	}
	if !strings.HasSuffix(p, "claude_desktop_config.json") {
		t.Errorf("expected claude_desktop_config.json, got %s", filepath.Base(p))
	}
	if !strings.Contains(p, "Claude") {
		t.Errorf("expected 'Claude' in path, got %s", p)
	}
}

func TestSaveTo_FreshFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "claude_desktop_config.json")

	entry := claudedesktop.MCPEntry{
		Command: "C:\\Users\\user\\.mcp\\atlassian\\atlassian-mcp.exe",
		Args:    []string{"mcp"},
	}
	if err := claudedesktop.SaveTo(path, entry); err != nil {
		t.Fatalf("SaveTo error: %v", err)
	}

	data, _ := os.ReadFile(path)
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := root["mcpServers"]; !ok {
		t.Fatal("missing mcpServers key")
	}
	var servers map[string]json.RawMessage
	_ = json.Unmarshal(root["mcpServers"], &servers)
	if _, ok := servers["atlassian-mcp"]; !ok {
		t.Fatal("missing atlassian entry")
	}
}

func TestSaveTo_PreservesAllExistingKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "claude_desktop_config.json")

	// Simulate a real Claude Desktop config with preferences and other servers
	existing := `{
  "mcpServers": {
    "other-server": {"command": "other", "args": ["run"]}
  },
  "coworkUserFilesPath": "C:\\Users\\joel.keb\\Documents\\Claude",
  "preferences": {
    "sidebarMode": "epitaxy",
    "keepAwakeEnabled": false
  }
}`
	_ = os.WriteFile(path, []byte(existing), 0o644)

	entry := claudedesktop.MCPEntry{
		Command: "C:\\Users\\user\\.mcp\\atlassian\\atlassian-mcp.exe",
		Args:    []string{"mcp"},
	}
	if err := claudedesktop.SaveTo(path, entry); err != nil {
		t.Fatalf("SaveTo error: %v", err)
	}

	data, _ := os.ReadFile(path)
	var root map[string]json.RawMessage
	_ = json.Unmarshal(data, &root)

	// All existing top-level keys must be preserved
	for _, key := range []string{"mcpServers", "coworkUserFilesPath", "preferences"} {
		if _, ok := root[key]; !ok {
			t.Errorf("key %q was deleted", key)
		}
	}

	// other-server must still be there
	var servers map[string]json.RawMessage
	_ = json.Unmarshal(root["mcpServers"], &servers)
	if _, ok := servers["other-server"]; !ok {
		t.Error("other-server was deleted")
	}
	if _, ok := servers["atlassian-mcp"]; !ok {
		t.Error("atlassian entry not written")
	}
}

func TestSaveTo_IdempotentDoubleSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "claude_desktop_config.json")

	entry := claudedesktop.MCPEntry{Command: "/bin/atlassian-mcp", Args: []string{"mcp"}}
	_ = claudedesktop.SaveTo(path, entry)
	_ = claudedesktop.SaveTo(path, entry)

	data, _ := os.ReadFile(path)
	var root map[string]json.RawMessage
	_ = json.Unmarshal(data, &root)
	var servers map[string]json.RawMessage
	_ = json.Unmarshal(root["mcpServers"], &servers)

	if len(servers) != 1 {
		t.Errorf("expected 1 server after double save, got %d", len(servers))
	}
}
