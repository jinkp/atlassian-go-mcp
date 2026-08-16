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
// OpenCode format: {"mcp": {"atlassian-mcp": {"type":"local","command":["exe","mcp"]}}}
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
	mcpSection[serverName] = json.RawMessage(entryBytes)
	delete(mcpSection, legacyServerName) // drop any pre-rename entry

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

// serverName is the MCP entry key this tool registers under.
// legacyServerName is the pre-rename key, cleaned up on save/remove.
const (
	serverName       = "atlassian-mcp"
	legacyServerName = "atlassian-platform-connector"
)

// Remove deletes the atlassian-mcp entry from GlobalPath().
func Remove() (bool, error) { return RemoveFrom(GlobalPath()) }

// RemoveLocal deletes the entry from the local project config (./opencode.json).
func RemoveLocal() (bool, error) { return RemoveFrom(LocalPath()) }

// RemoveFrom deletes the connector entry from configPath under the "mcp" key
// (both the current name and the legacy pre-rename name), preserving all other
// keys and MCP servers. If the "mcp" section becomes empty it is removed too.
// Returns (removed, error); removed is false when the file or the entry does not
// exist.
func RemoveFrom(configPath string) (bool, error) {
	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return false, nil
		}
		return false, fmt.Errorf("reading config %s: %w", configPath, readErr)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("parse config %s: %w", configPath, err)
	}

	rawMCP, ok := root["mcp"]
	if !ok {
		return false, nil
	}
	var mcpSection map[string]json.RawMessage
	if err := json.Unmarshal(rawMCP, &mcpSection); err != nil {
		return false, fmt.Errorf("parse mcp section: %w", err)
	}
	_, hasCurrent := mcpSection[serverName]
	_, hasLegacy := mcpSection[legacyServerName]
	if !hasCurrent && !hasLegacy {
		return false, nil
	}
	delete(mcpSection, serverName)
	delete(mcpSection, legacyServerName)

	if len(mcpSection) == 0 {
		delete(root, "mcp")
	} else {
		b, err := json.Marshal(mcpSection)
		if err != nil {
			return false, fmt.Errorf("marshal mcp section: %w", err)
		}
		root["mcp"] = json.RawMessage(b)
	}

	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, output, 0o644); err != nil {
		return false, fmt.Errorf("write config %s: %w", configPath, err)
	}
	return true, nil
}
