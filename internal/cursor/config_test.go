package cursor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/cursor"
)

func TestGlobalPath(t *testing.T) {
	p := cursor.GlobalPath()
	if p == "" {
		t.Fatal("GlobalPath returned empty string")
	}
	if filepath.Base(p) != "mcp.json" {
		t.Errorf("expected mcp.json, got %s", filepath.Base(p))
	}
	if filepath.Base(filepath.Dir(p)) != ".cursor" {
		t.Errorf("expected .cursor parent dir, got %s", filepath.Base(filepath.Dir(p)))
	}
}

func TestSaveTo_FreshFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	entry := cursor.MCPEntry{
		Command: "/usr/local/bin/atlassian-mcp",
		Args:    []string{"mcp"},
	}
	if err := cursor.SaveTo(path, entry); err != nil {
		t.Fatalf("SaveTo error: %v", err)
	}

	data, _ := os.ReadFile(path)
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("output is invalid JSON: %v", err)
	}
	if _, ok := root["mcpServers"]; !ok {
		t.Fatal("missing mcpServers key")
	}
}

func TestSaveTo_MergePreservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	// Write existing file with another server
	existing := `{"mcpServers":{"other-server":{"command":"other","args":["run"]}}}`
	_ = os.WriteFile(path, []byte(existing), 0o644)

	entry := cursor.MCPEntry{Command: "/bin/atlassian-mcp", Args: []string{"mcp"}}
	if err := cursor.SaveTo(path, entry); err != nil {
		t.Fatalf("SaveTo error: %v", err)
	}

	data, _ := os.ReadFile(path)
	var root struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := root.MCPServers["other-server"]; !ok {
		t.Error("existing server was overwritten")
	}
	if _, ok := root.MCPServers["atlassian-platform-connector"]; !ok {
		t.Error("atlassian entry not written")
	}
}

func TestSaveTo_OverwriteStaleEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	entry1 := cursor.MCPEntry{Command: "/old/path/atlassian-mcp", Args: []string{"mcp"}}
	_ = cursor.SaveTo(path, entry1)

	entry2 := cursor.MCPEntry{Command: "/new/path/atlassian-mcp", Args: []string{"mcp", "--enable", "jira"}}
	_ = cursor.SaveTo(path, entry2)

	data, _ := os.ReadFile(path)
	if string(data) == "" {
		t.Fatal("empty file")
	}
	// Should contain new path
	if !contains(string(data), "/new/path/atlassian-mcp") {
		t.Error("new command not found in config")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
