// Package claude manages the Claude Code MCP client configuration file (~/.claude.json).
package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPEntry describes an MCP server entry in the Claude Code config.
type MCPEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// GlobalPath returns the absolute path to the Claude Code global config file.
// On all platforms: ~/.claude.json
func GlobalPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude.json")
}

// Save writes entry under mcpServers.atlassian in the GlobalPath() file,
// merging with any existing content and preserving unrelated keys.
func Save(entry MCPEntry) error {
	return SaveTo(GlobalPath(), entry)
}

// SaveTo is the testable form of Save — uses configPath instead of GlobalPath().
func SaveTo(configPath string, entry MCPEntry) error {
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

	// Parse existing mcpServers map (or create empty)
	var mcpServers map[string]json.RawMessage
	if rawMCP, ok := root["mcpServers"]; ok {
		if unmarshalErr := json.Unmarshal(rawMCP, &mcpServers); unmarshalErr != nil {
			return fmt.Errorf("parse mcpServers: %w", unmarshalErr)
		}
	} else {
		mcpServers = make(map[string]json.RawMessage)
	}

	// Marshal the new entry
	entryBytes, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return fmt.Errorf("marshal MCP entry: %w", marshalErr)
	}
	mcpServers["atlassian"] = json.RawMessage(entryBytes)

	// Write updated mcpServers back into root
	mcpBytes, err := json.Marshal(mcpServers)
	if err != nil {
		return fmt.Errorf("marshal mcpServers: %w", err)
	}
	root["mcpServers"] = json.RawMessage(mcpBytes)

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
