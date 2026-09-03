// Package claudeplugin generates a self-contained Claude Code plugin that bundles
// the atlassian-mcp server. Unlike the plain MCP registration (internal/claude),
// this produces a plugin directory Claude Code auto-loads as a skills-directory
// plugin:
//
//	<dir>/.claude-plugin/plugin.json   # plugin manifest (name/description/version)
//	<dir>/.mcp.json                    # bundled MCP server (command/args/env)
//
// A folder under ~/.claude/skills/<name>/ that contains .claude-plugin/plugin.json
// loads as "<name>@skills-dir" on the next session — no marketplace, no install.
// Personal scope (~/.claude/skills) has no MCP approval restrictions.
//
// Reference: https://docs.claude.com/en/docs/claude-code/plugins-reference
package claudeplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PluginName is the plugin identifier (kebab-case) and also the bundled MCP
// server key. Claude Code namespaces the plugin's tools under this name.
const PluginName = "atlassian-mcp"

// Author is the optional manifest author block.
type Author struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// Manifest is the .claude-plugin/plugin.json document. Only Name is required.
type Manifest struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"displayName,omitempty"`
	Description string  `json:"description,omitempty"`
	Version     string  `json:"version,omitempty"`
	Author      *Author `json:"author,omitempty"`
	Repository  string  `json:"repository,omitempty"`
}

// MCPEntry describes the bundled MCP server in .mcp.json (standard MCP format).
type MCPEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// mcpFile is the top-level .mcp.json document.
type mcpFile struct {
	MCPServers map[string]MCPEntry `json:"mcpServers"`
}

// GlobalDir returns the personal skills-directory plugin path.
// A plugin here auto-loads in every project as "atlassian-mcp@skills-dir".
func GlobalDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "skills", PluginName)
}

// LocalDir returns the project-scoped skills-directory plugin path.
// Project-scope plugins load only after the workspace trust dialog is accepted,
// and their MCP server goes through the same per-server approval as a project
// .mcp.json. Launch Claude Code from the repository root for it to be discovered.
func LocalDir() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".claude", "skills", PluginName)
}

// Save writes the plugin manifest and bundled .mcp.json into dir. Existing files
// are overwritten (the plugin is fully owned by this tool). version is embedded
// in the manifest unless it is empty or the sentinel "dev".
func Save(dir, version string, entry MCPEntry) error {
	manifest := Manifest{
		Name:        PluginName,
		DisplayName: "Atlassian MCP",
		Description: "Atlassian Platform Connector — Jira, Agile, Bitbucket, Confluence and more as MCP tools.",
		Author:      &Author{Name: "jinkp", URL: "https://github.com/jinkp/atlassian-go-mcp"},
		Repository:  "https://github.com/jinkp/atlassian-go-mcp",
	}
	if version != "" && version != "dev" {
		manifest.Version = version
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugin manifest: %w", err)
	}

	mcp := mcpFile{MCPServers: map[string]MCPEntry{PluginName: entry}}
	mcpBytes, err := json.MarshalIndent(mcp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .mcp.json: %w", err)
	}

	manifestDir := filepath.Join(dir, ".claude-plugin")
	if mkdirErr := os.MkdirAll(manifestDir, 0o755); mkdirErr != nil {
		return fmt.Errorf("create plugin dir %s: %w", manifestDir, mkdirErr)
	}

	manifestPath := filepath.Join(manifestDir, "plugin.json")
	if writeErr := os.WriteFile(manifestPath, manifestBytes, 0o644); writeErr != nil {
		return fmt.Errorf("write %s: %w", manifestPath, writeErr)
	}

	mcpPath := filepath.Join(dir, ".mcp.json")
	if writeErr := os.WriteFile(mcpPath, mcpBytes, 0o644); writeErr != nil {
		return fmt.Errorf("write %s: %w", mcpPath, writeErr)
	}

	return nil
}

// SaveWithArgsEnv resolves the current binary path and writes the plugin with the
// given args (e.g. ["mcp", "--enable", "jira"]) and env (e.g. ENABLE_WRITE=true).
func SaveWithArgsEnv(dir, version string, args []string, env map[string]string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	return Save(dir, version, MCPEntry{Command: exe, Args: args, Env: env})
}

// Remove deletes the plugin directory, but ONLY when it is our plugin — verified
// by reading .claude-plugin/plugin.json and confirming name == PluginName. This
// guards against nuking an unrelated folder that happens to share the path.
// Returns (removed, error); removed is false when the plugin is absent.
func Remove(dir string) (bool, error) {
	manifestPath := filepath.Join(dir, ".claude-plugin", "plugin.json")
	data, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return false, nil
		}
		return false, fmt.Errorf("reading plugin manifest %s: %w", manifestPath, readErr)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return false, fmt.Errorf("parse plugin manifest %s: %w", manifestPath, err)
	}
	if manifest.Name != PluginName {
		// Not ours — refuse to delete.
		return false, fmt.Errorf("refusing to remove %s: manifest name %q is not %q", dir, manifest.Name, PluginName)
	}

	if err := os.RemoveAll(dir); err != nil {
		return false, fmt.Errorf("remove plugin dir %s: %w", dir, err)
	}
	return true, nil
}
