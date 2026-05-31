package opencode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/opencode"
)

// --- TestGlobalPath ---

func TestGlobalPath(t *testing.T) {
	path := opencode.GlobalPath()
	if path == "" {
		t.Fatal("GlobalPath() returned empty string")
	}
	// Must end with opencode.json
	if !strings.HasSuffix(path, "opencode.json") {
		t.Errorf("GlobalPath() %q does not end with 'opencode.json'", path)
	}
	// Always under ~/.config/opencode/ on all platforms
	if !strings.Contains(path, ".config") {
		t.Errorf("GlobalPath() %q does not contain '.config'", path)
	}
}

// --- TestSave ---
// OpenCode format: {"mcp": {"atlassian": {"type":"local","command":["exe","mcp"]}}}

func TestSave(t *testing.T) {
	entry := opencode.MCPEntry{
		Type:    "local",
		Command: "/usr/local/bin/atlassian-mcp",
		Args:    []string{"mcp"},
	}

	t.Run("fresh file creation", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "opencode.json")

		err := opencode.SaveTo(configPath, entry)
		if err != nil {
			t.Fatalf("SaveTo() unexpected error: %v", err)
		}

		data, readErr := os.ReadFile(configPath)
		if readErr != nil {
			t.Fatalf("ReadFile after SaveTo: %v", readErr)
		}

		var parsed map[string]json.RawMessage
		if jsonErr := json.Unmarshal(data, &parsed); jsonErr != nil {
			t.Fatalf("written file is not valid JSON: %v", jsonErr)
		}

		// OpenCode uses "mcp" key (not "mcpServers")
		mcpRaw, ok := parsed["mcp"]
		if !ok {
			t.Fatal("written JSON does not contain 'mcp' key")
		}
		var mcpSection map[string]json.RawMessage
		if jsonErr := json.Unmarshal(mcpRaw, &mcpSection); jsonErr != nil {
			t.Fatalf("mcp is not a JSON object: %v", jsonErr)
		}
		if _, ok := mcpSection["atlassian"]; !ok {
			t.Error("mcp does not contain 'atlassian' entry")
		}

		// Verify command is an array (not a string)
		var atlEntry struct {
			Type    string   `json:"type"`
			Command []string `json:"command"`
		}
		if err := json.Unmarshal(mcpSection["atlassian"], &atlEntry); err != nil {
			t.Fatalf("atlassian entry is not valid JSON: %v", err)
		}
		if atlEntry.Type != "local" {
			t.Errorf("expected type=local, got %q", atlEntry.Type)
		}
		if len(atlEntry.Command) < 2 {
			t.Errorf("command should be [exe, mcp, ...], got %v", atlEntry.Command)
		}
	})

	t.Run("merge preserves existing keys", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "opencode.json")

		// Write an existing config with unrelated keys and another mcp server
		existing := `{"theme":"dark","mcp":{"other-tool":{"type":"local","command":["other"]}}}`
		if writeErr := os.WriteFile(configPath, []byte(existing), 0o644); writeErr != nil {
			t.Fatalf("setup: WriteFile: %v", writeErr)
		}

		err := opencode.SaveTo(configPath, entry)
		if err != nil {
			t.Fatalf("SaveTo() unexpected error: %v", err)
		}

		data, _ := os.ReadFile(configPath)
		var parsed map[string]json.RawMessage
		_ = json.Unmarshal(data, &parsed)

		// theme key must be preserved
		if _, ok := parsed["theme"]; !ok {
			t.Error("merge destroyed 'theme' key")
		}

		// other-tool must be preserved
		mcpRaw := parsed["mcp"]
		var mcpSection map[string]json.RawMessage
		_ = json.Unmarshal(mcpRaw, &mcpSection)
		if _, ok := mcpSection["other-tool"]; !ok {
			t.Error("merge destroyed 'mcp.other-tool' entry")
		}
		if _, ok := mcpSection["atlassian"]; !ok {
			t.Error("merge did not add 'mcp.atlassian' entry")
		}
	})

	t.Run("overwrite stale entry is idempotent", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "opencode.json")

		// Run twice
		if err := opencode.SaveTo(configPath, entry); err != nil {
			t.Fatalf("first SaveTo: %v", err)
		}
		if err := opencode.SaveTo(configPath, entry); err != nil {
			t.Fatalf("second SaveTo: %v", err)
		}

		data, _ := os.ReadFile(configPath)
		var parsed map[string]json.RawMessage
		_ = json.Unmarshal(data, &parsed)

		mcpRaw := parsed["mcp"]
		var mcpSection map[string]json.RawMessage
		_ = json.Unmarshal(mcpRaw, &mcpSection)

		// Exactly one atlassian entry — not duplicated
		if count := len(mcpSection); count != 1 {
			t.Errorf("expected exactly 1 mcp entry after double save, got %d", count)
		}
	})

	t.Run("corrupted file returns error without modifying file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "opencode.json")

		corrupt := []byte("{invalid json")
		if writeErr := os.WriteFile(configPath, corrupt, 0o644); writeErr != nil {
			t.Fatalf("setup: WriteFile: %v", writeErr)
		}

		err := opencode.SaveTo(configPath, entry)
		if err == nil {
			t.Fatal("expected error for corrupted file, got nil")
		}
		// File must not be modified
		data, _ := os.ReadFile(configPath)
		if string(data) != string(corrupt) {
			t.Error("corrupted file was modified despite parse error")
		}
	})
}
