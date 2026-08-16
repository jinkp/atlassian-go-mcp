// Package opencode manages the OpenCode MCP client configuration file.
package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPEntry describes an MCP server entry in the OpenCode config.
type MCPEntry struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// GlobalPath returns the path to the OpenCode global configuration file.
// OpenCode stores its config at ~/.config/opencode/opencode.json on all platforms.
func GlobalPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// LocalPath returns the path to the OpenCode project-level configuration file.
// OpenCode loads ./opencode.json from the current working directory.
func LocalPath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "opencode.json")
}

// SaveLocal writes the MCP entry to the local project config (./opencode.json).
func SaveLocal(entry MCPEntry) error {
	return SaveTo(LocalPath(), entry)
}

// SaveWithArgsLocal writes with custom args to the local project config.
func SaveWithArgsLocal(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	return SaveLocal(MCPEntry{Type: "local", Command: exe, Args: args})
}

// opencodeMCPEntry is the format OpenCode expects under the "mcp" key.
// Command is a []string (binary + args combined), not a separate args field.
// enabled:true is required for OpenCode to load the server on startup.
// OpenCode uses "environment" (not "env") for environment variables.
type opencodeMCPEntry struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command"`
	Enabled     bool              `json:"enabled"`
	Environment map[string]string `json:"environment,omitempty"`
}

// Save writes entry under mcp.atlassian in the GlobalPath() file,
// merging with any existing content and preserving unrelated keys.
// OpenCode format: {"mcp": {"atlassian-platform-connector": {"type":"local","command":["exe","mcp"]}}}
func Save(entry MCPEntry) error {
	return SaveTo(GlobalPath(), entry)
}

// SaveTo is the testable form of Save — uses configPath instead of GlobalPath().
func SaveTo(configPath string, entry MCPEntry) error {
	// Read existing file or start with empty object.
	// Use a raw bytes approach to avoid losing keys on re-marshal.
	var root map[string]json.RawMessage

	existingData, readErr := os.ReadFile(configPath)
	if readErr == nil {
		if unmarshalErr := json.Unmarshal(existingData, &root); unmarshalErr != nil {
			return fmt.Errorf("parse existing config %s: %w", configPath, unmarshalErr)
		}
	} else if !os.IsNotExist(readErr) {
		return fmt.Errorf("reading config %s: %w", configPath, readErr)
	} else {
		root = make(map[string]json.RawMessage)
	}

	// OpenCode uses "mcp" key (not "mcpServers").
	var mcpSection map[string]json.RawMessage
	if rawMCP, ok := root["mcp"]; ok {
		if unmarshalErr := json.Unmarshal(rawMCP, &mcpSection); unmarshalErr != nil {
			return fmt.Errorf("parse mcp section: %w", unmarshalErr)
		}
	} else {
		mcpSection = make(map[string]json.RawMessage)
	}

	// Build command as []string — OpenCode requires binary + args in one array.
	cmd := []string{entry.Command}
	cmd = append(cmd, entry.Args...)

	ocEntry := opencodeMCPEntry{
		Type:        "local",
		Command:     cmd,
		Enabled:     true,
		Environment: entry.Env,
	}

	entryBytes, marshalErr := json.Marshal(ocEntry)
	if marshalErr != nil {
		return fmt.Errorf("marshal MCP entry: %w", marshalErr)
	}
	mcpSection["atlassian-platform-connector"] = json.RawMessage(entryBytes)

	mcpBytes, err := json.Marshal(mcpSection)
	if err != nil {
		return fmt.Errorf("marshal mcp section: %w", err)
	}
	root["mcp"] = json.RawMessage(mcpBytes)

	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(configPath), 0o755); mkdirErr != nil {
		return fmt.Errorf("create config directory: %w", mkdirErr)
	}

	if writeErr := os.WriteFile(configPath, output, 0o644); writeErr != nil {
		return fmt.Errorf("write config %s: %w", configPath, writeErr)
	}

	return nil
}

// SaveWithArgs writes the MCP entry with custom args (e.g. ["mcp", "--enable", "jira"]).
// It resolves the current binary path automatically.
func SaveWithArgs(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	return Save(MCPEntry{Type: "local", Command: exe, Args: args})
}
