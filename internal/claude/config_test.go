package claude_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/claude"
)

// --- TestGlobalPath ---

func TestGlobalPath(t *testing.T) {
	path := claude.GlobalPath()
	if path == "" {
		t.Fatal("GlobalPath() returned empty string")
	}
	// Must end with .claude.json on all platforms
	if !strings.HasSuffix(path, ".claude.json") {
		t.Errorf("GlobalPath() %q does not end with '.claude.json'", path)
	}
}

// --- TestSave ---

func TestSave(t *testing.T) {
	entry := claude.MCPEntry{
		Command: "/usr/local/bin/atlassian-mcp",
		Args:    []string{"mcp"},
	}

	t.Run("fresh file creation", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".claude.json")

		err := claude.SaveTo(configPath, entry)
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

		mcpRaw, ok := parsed["mcpServers"]
		if !ok {
			t.Fatal("written JSON does not contain 'mcpServers' key")
		}
		var mcpServers map[string]json.RawMessage
		if jsonErr := json.Unmarshal(mcpRaw, &mcpServers); jsonErr != nil {
			t.Fatalf("mcpServers is not a JSON object: %v", jsonErr)
		}
		if _, ok := mcpServers["atlassian-platform-connector"]; !ok {
			t.Error("mcpServers does not contain 'atlassian' entry")
		}
	})

	t.Run("merge preserves existing keys", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".claude.json")

		existing := `{"mcpServers":{"existing":{"command":"x"}},"other":"val"}`
		if writeErr := os.WriteFile(configPath, []byte(existing), 0o644); writeErr != nil {
			t.Fatalf("setup: WriteFile: %v", writeErr)
		}

		err := claude.SaveTo(configPath, entry)
		if err != nil {
			t.Fatalf("SaveTo() unexpected error: %v", err)
		}

		data, _ := os.ReadFile(configPath)
		var parsed map[string]json.RawMessage
		_ = json.Unmarshal(data, &parsed)

		// other key preserved
		if _, ok := parsed["other"]; !ok {
			t.Error("merge destroyed 'other' key")
		}

		// existing server preserved
		mcpRaw := parsed["mcpServers"]
		var mcpServers map[string]json.RawMessage
		_ = json.Unmarshal(mcpRaw, &mcpServers)
		if _, ok := mcpServers["existing"]; !ok {
			t.Error("merge destroyed 'mcpServers.existing' entry")
		}
		if _, ok := mcpServers["atlassian-platform-connector"]; !ok {
			t.Error("merge did not add 'mcpServers.atlassian' entry")
		}
	})

	t.Run("double registration is idempotent", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".claude.json")

		if err := claude.SaveTo(configPath, entry); err != nil {
			t.Fatalf("first SaveTo: %v", err)
		}
		if err := claude.SaveTo(configPath, entry); err != nil {
			t.Fatalf("second SaveTo: %v", err)
		}

		data, _ := os.ReadFile(configPath)
		var parsed map[string]json.RawMessage
		_ = json.Unmarshal(data, &parsed)
		mcpRaw := parsed["mcpServers"]
		var mcpServers map[string]json.RawMessage
		_ = json.Unmarshal(mcpRaw, &mcpServers)
		if count := len(mcpServers); count != 1 {
			t.Errorf("expected 1 mcpServers entry after double save, got %d", count)
		}
	})

	t.Run("corrupted file returns error without modifying file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".claude.json")

		corrupt := []byte("{invalid json")
		if writeErr := os.WriteFile(configPath, corrupt, 0o644); writeErr != nil {
			t.Fatalf("setup: WriteFile: %v", writeErr)
		}

		err := claude.SaveTo(configPath, entry)
		if err == nil {
			t.Fatal("expected error for corrupted file, got nil")
		}
		data, _ := os.ReadFile(configPath)
		if string(data) != string(corrupt) {
			t.Error("corrupted file was modified despite parse error")
		}
	})
}
