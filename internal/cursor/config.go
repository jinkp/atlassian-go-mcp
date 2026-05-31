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
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	return SaveLocal(MCPEntry{Command: exe, Args: args})
}

// Save writes entry under mcpServers.atlassian in the GlobalPath() file,
// merging with any existing content and preserving unrelated keys.
// Cursor format: {"mcpServers": {"atlassian-platform-connector": {"command":"exe","args":["mcp"]}}}
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
	mcpServers["atlassian-platform-connector"] = json.RawMessage(entryBytes)

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
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	return Save(MCPEntry{Command: exe, Args: args})
}
