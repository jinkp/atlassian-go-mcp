package claudeplugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/claudeplugin"
)

func TestSave_WritesManifestAndMCP(t *testing.T) {
	dir := t.TempDir()

	entry := claudeplugin.MCPEntry{
		Command: "/usr/local/bin/atlassian-mcp",
		Args:    []string{"mcp", "--enable", "jira,agile"},
		Env:     map[string]string{"ENABLE_WRITE": "true"},
	}
	if err := claudeplugin.Save(dir, "v1.2.3", entry); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Manifest exists with correct name/version
	manifestPath := filepath.Join(dir, ".claude-plugin", "plugin.json")
	mData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	var manifest claudeplugin.Manifest
	if err := json.Unmarshal(mData, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if manifest.Name != claudeplugin.PluginName {
		t.Errorf("manifest name: got %q, want %q", manifest.Name, claudeplugin.PluginName)
	}
	if manifest.Version != "v1.2.3" {
		t.Errorf("manifest version: got %q, want v1.2.3", manifest.Version)
	}

	// .mcp.json exists with the bundled server + env
	mcpPath := filepath.Join(dir, ".mcp.json")
	cData, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("reading .mcp.json: %v", err)
	}
	var parsed struct {
		MCPServers map[string]claudeplugin.MCPEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(cData, &parsed); err != nil {
		t.Fatalf("parse .mcp.json: %v", err)
	}
	got, ok := parsed.MCPServers[claudeplugin.PluginName]
	if !ok {
		t.Fatalf(".mcp.json missing server key %q, got: %s", claudeplugin.PluginName, cData)
	}
	if got.Env["ENABLE_WRITE"] != "true" {
		t.Errorf("ENABLE_WRITE: got %q, want true", got.Env["ENABLE_WRITE"])
	}
	if len(got.Args) != 3 || got.Args[0] != "mcp" {
		t.Errorf("args: got %v, want [mcp --enable jira,agile]", got.Args)
	}
}

func TestSave_OmitsDevVersion(t *testing.T) {
	dir := t.TempDir()
	if err := claudeplugin.Save(dir, "dev", claudeplugin.MCPEntry{Command: "x"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	mData, _ := os.ReadFile(filepath.Join(dir, ".claude-plugin", "plugin.json"))
	var manifest claudeplugin.Manifest
	_ = json.Unmarshal(mData, &manifest)
	if manifest.Version != "" {
		t.Errorf("dev version should be omitted, got %q", manifest.Version)
	}
}

func TestSave_ReadOnlyOmitsEnv(t *testing.T) {
	dir := t.TempDir()
	if err := claudeplugin.SaveWithArgsEnv(dir, "", []string{"mcp"}, nil); err != nil {
		t.Fatalf("SaveWithArgsEnv() error: %v", err)
	}
	cData, _ := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if json.Valid(cData) == false {
		t.Fatalf(".mcp.json invalid: %s", cData)
	}
	// env with omitempty should not appear when nil
	if string(cData) == "" {
		t.Fatal("empty .mcp.json")
	}
	var parsed struct {
		MCPServers map[string]claudeplugin.MCPEntry `json:"mcpServers"`
	}
	_ = json.Unmarshal(cData, &parsed)
	if got := parsed.MCPServers[claudeplugin.PluginName]; len(got.Env) != 0 {
		t.Errorf("read-only should have no env, got %v", got.Env)
	}
}

func TestRemove_OnlyDeletesOwnPlugin(t *testing.T) {
	t.Run("removes our plugin", func(t *testing.T) {
		dir := t.TempDir()
		pluginDir := filepath.Join(dir, "atlassian-mcp")
		if err := claudeplugin.Save(pluginDir, "", claudeplugin.MCPEntry{Command: "x"}); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		removed, err := claudeplugin.Remove(pluginDir)
		if err != nil {
			t.Fatalf("Remove() error: %v", err)
		}
		if !removed {
			t.Error("expected removed=true")
		}
		if _, statErr := os.Stat(pluginDir); !os.IsNotExist(statErr) {
			t.Error("plugin dir should be gone")
		}
	})

	t.Run("absent plugin returns false", func(t *testing.T) {
		removed, err := claudeplugin.Remove(filepath.Join(t.TempDir(), "nope"))
		if err != nil {
			t.Fatalf("Remove() unexpected error: %v", err)
		}
		if removed {
			t.Error("expected removed=false for absent plugin")
		}
	})

	t.Run("refuses foreign manifest", func(t *testing.T) {
		dir := t.TempDir()
		manifestDir := filepath.Join(dir, ".claude-plugin")
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			t.Fatal(err)
		}
		foreign := []byte(`{"name":"some-other-plugin"}`)
		if err := os.WriteFile(filepath.Join(manifestDir, "plugin.json"), foreign, 0o644); err != nil {
			t.Fatal(err)
		}
		removed, err := claudeplugin.Remove(dir)
		if err == nil {
			t.Error("expected error refusing to remove foreign plugin")
		}
		if removed {
			t.Error("must not remove a foreign plugin")
		}
		// Directory must still exist
		if _, statErr := os.Stat(dir); statErr != nil {
			t.Error("foreign plugin dir must be preserved")
		}
	})
}
