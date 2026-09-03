// Package cursor manages the Cursor MCP client configuration file (~/.cursor/mcp.json).
package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPEntry describes an MCP server entry in the Cursor config.
type MCPEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// GlobalPath returns the absolute path to the Cursor global MCP config file.
// Cursor stores MCP servers at ~/.cursor/mcp.json on all platforms.
func GlobalPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cursor", "mcp.json")
}

// LocalPath returns the path to the Cursor project-level MCP config.
// Cursor loads .cursor/mcp.json from the current working directory.
func LocalPath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".cursor", "mcp.json")
}

// SaveLocal writes the MCP entry to the local project config (./.cursor/mcp.json).
func SaveLocal(entry MCPEntry) error {
	return SaveTo(LocalPath(), entry)
}

// SaveWithArgsLocal writes with custom args to the local project config.
func SaveWithArgsLocal(args []string) error {
	return SaveWithArgsEnvLocal(args, nil)
}

// SaveWithArgsEnvLocal writes with custom args and environment variables
// (e.g. env{"ENABLE_WRITE":"true"}) to the local project config.
func SaveWithArgsEnvLocal(args []string, env map[string]string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	return SaveLocal(MCPEntry{Command: exe, Args: args, Env: env})
}

// Save writes entry under mcpServers.atlassian in the GlobalPath() file,
// merging with any existing content and preserving unrelated keys.
// Cursor format: {"mcpServers": {"atlassian-mcp": {"command":"exe","args":["mcp"]}}}
func Save(entry MCPEntry) error {
	return SaveTo(GlobalPath(), entry)
}

// SaveTo is the testable form of Save — uses configPath instead of GlobalPath().
func SaveTo(configPath string, entry MCPEntry) error {
	var root map[string]json.RawMessage

	existingData, readErr := os.ReadFile(configPath)
	if readErr == nil {
		if unmarshalErr := json.Unmarshal(existingData, &root); unmarshalErr != nil {
			return fmt.Errorf("parse existing cursor config %s: %w", configPath, unmarshalErr)
		}
	} else if !os.IsNotExist(readErr) {
		return fmt.Errorf("reading cursor config %s: %w", configPath, readErr)
	} else {
		root = make(map[string]json.RawMessage)
	}

	// Cursor uses "mcpServers" key (same as Claude Code, different from OpenCode).
	var mcpServers map[string]json.RawMessage
	if rawMCP, ok := root["mcpServers"]; ok {
		if unmarshalErr := json.Unmarshal(rawMCP, &mcpServers); unmarshalErr != nil {
			mcpServers = make(map[string]json.RawMessage)
		}
	} else {
		mcpServers = make(map[string]json.RawMessage)
	}

	entryBytes, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return fmt.Errorf("marshal cursor MCP entry: %w", marshalErr)
	}
	mcpServers[serverName] = json.RawMessage(entryBytes)
	delete(mcpServers, legacyServerName) // drop any pre-rename entry

	mcpBytes, err := json.Marshal(mcpServers)
	if err != nil {
		return fmt.Errorf("marshal mcpServers: %w", err)
	}
	root["mcpServers"] = json.RawMessage(mcpBytes)

	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cursor config: %w", err)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(configPath), 0o755); mkdirErr != nil {
		return fmt.Errorf("create cursor config directory: %w", mkdirErr)
	}

	if writeErr := os.WriteFile(configPath, output, 0o644); writeErr != nil {
		return fmt.Errorf("write cursor config %s: %w", configPath, writeErr)
	}

	return nil
}

// SaveWithArgs writes the MCP entry with custom args (e.g. ["mcp", "--enable", "jira"]).
// It resolves the current binary path automatically.
func SaveWithArgs(args []string) error {
	return SaveWithArgsEnv(args, nil)
}

// SaveWithArgsEnv writes the MCP entry with custom args and environment variables
// (e.g. env{"ENABLE_WRITE":"true"}). It resolves the current binary path automatically.
func SaveWithArgsEnv(args []string, env map[string]string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	return Save(MCPEntry{Command: exe, Args: args, Env: env})
}

// serverName is the MCP entry key this tool registers under.
// legacyServerName is the pre-rename key, cleaned up on save/remove.
const (
	serverName       = "atlassian-mcp"
	legacyServerName = "atlassian-platform-connector"
)

// Remove deletes the atlassian-mcp entry from GlobalPath().
func Remove() (bool, error) { return RemoveFrom(GlobalPath()) }

// RemoveLocal deletes the entry from the local project config (./.cursor/mcp.json).
func RemoveLocal() (bool, error) { return RemoveFrom(LocalPath()) }

// RemoveFrom deletes the connector entry from configPath under the "mcpServers"
// key (both the current name and the legacy pre-rename name), preserving all
// other servers. If mcpServers becomes empty it is removed. Returns (removed,
// error); removed is false when the file or entry is absent.
func RemoveFrom(configPath string) (bool, error) {
	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return false, nil
		}
		return false, fmt.Errorf("reading cursor config %s: %w", configPath, readErr)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("parse cursor config %s: %w", configPath, err)
	}

	rawMCP, ok := root["mcpServers"]
	if !ok {
		return false, nil
	}
	var mcpServers map[string]json.RawMessage
	if err := json.Unmarshal(rawMCP, &mcpServers); err != nil {
		return false, fmt.Errorf("parse mcpServers: %w", err)
	}
	_, hasCurrent := mcpServers[serverName]
	_, hasLegacy := mcpServers[legacyServerName]
	if !hasCurrent && !hasLegacy {
		return false, nil
	}
	delete(mcpServers, serverName)
	delete(mcpServers, legacyServerName)

	if len(mcpServers) == 0 {
		delete(root, "mcpServers")
	} else {
		b, err := json.Marshal(mcpServers)
		if err != nil {
			return false, fmt.Errorf("marshal mcpServers: %w", err)
		}
		root["mcpServers"] = json.RawMessage(b)
	}

	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal cursor config: %w", err)
	}
	if err := os.WriteFile(configPath, output, 0o644); err != nil {
		return false, fmt.Errorf("write cursor config %s: %w", configPath, err)
	}
	return true, nil
}
