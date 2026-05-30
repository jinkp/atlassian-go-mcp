package opencode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	// Platform-specific directory check
	if runtime.GOOS == "windows" {
		if !strings.Contains(path, "OpenCode") {
			t.Errorf("Windows GlobalPath() %q does not contain 'OpenCode'", path)
		}
	} else {
		if !strings.Contains(path, ".config") {
			t.Errorf("Unix GlobalPath() %q does not contain '.config'", path)
		}
	}
}

// --- TestSave ---

func TestSave(t *testing.T) {
	entry := opencode.MCPEntry{
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

		mcpRaw, ok := parsed["mcpServers"]
		if !ok {
			t.Fatal("written JSON does not contain 'mcpServers' key")
		}
		var mcpServers map[string]json.RawMessage
		if jsonErr := json.Unmarshal(mcpRaw, &mcpServers); jsonErr != nil {
			t.Fatalf("mcpServers is not a JSON object: %v", jsonErr)
		}
		if _, ok := mcpServers["atlassian"]; !ok {
			t.Error("mcpServers does not contain 'atlassian' entry")
		}
	})

	t.Run("merge preserves existing keys", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "opencode.json")

		// Write an existing config with unrelated keys
		existing := `{"theme":"dark","mcpServers":{"other-tool":{"command":"other"}}}`
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
		mcpRaw := parsed["mcpServers"]
		var mcpServers map[string]json.RawMessage
		_ = json.Unmarshal(mcpRaw, &mcpServers)
		if _, ok := mcpServers["other-tool"]; !ok {
			t.Error("merge destroyed 'mcpServers.other-tool' entry")
		}
		if _, ok := mcpServers["atlassian"]; !ok {
			t.Error("merge did not add 'mcpServers.atlassian' entry")
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

		mcpRaw := parsed["mcpServers"]
		var mcpServers map[string]json.RawMessage
		_ = json.Unmarshal(mcpRaw, &mcpServers)

		// Exactly one atlassian entry — not duplicated
		if count := len(mcpServers); count != 1 {
			t.Errorf("expected exactly 1 mcpServers entry after double save, got %d", count)
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
