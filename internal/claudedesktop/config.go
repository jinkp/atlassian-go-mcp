// Package claudedesktop manages the Claude Desktop MCP client configuration.
// Config path: %APPDATA%\Claude\claude_desktop_config.json (Windows)
//              ~/Library/Application Support/Claude/claude_desktop_config.json (macOS)
package claudedesktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// MCPEntry describes an MCP server entry for Claude Desktop.
// Claude Desktop uses the same mcpServers format as Cursor:
// {"command": "exe", "args": ["mcp"]}
type MCPEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// GlobalPath returns the platform-appropriate path to the Claude Desktop config file.
//
//   - Windows: %APPDATA%\Claude\claude_desktop_config.json
//   - macOS:   ~/Library/Application Support/Claude/claude_desktop_config.json
func GlobalPath() string {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, _ := os.UserHomeDir()
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Claude", "claude_desktop_config.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
}

// Save writes entry under mcpServers.atlassian in the GlobalPath() file,
// merging with any existing content and preserving ALL unrelated keys
// (preferences, coworkUserFilesPath, etc.).
//
// Claude Desktop format:
//
//	{
//	  "mcpServers": {
//	    "atlassian-platform-connector": {"command": "exe", "args": ["mcp"]}
//	  },
//	  "preferences": { ... },   <- preserved
//	  "coworkUserFilesPath": "...", <- preserved
//	  ...
//	}
func Save(entry MCPEntry) error {
	return SaveTo(GlobalPath(), entry)
}

// SaveTo is the testable form of Save — uses configPath instead of GlobalPath().
func SaveTo(configPath string, entry MCPEntry) error {
	// Use map[string]json.RawMessage to preserve ALL existing keys without
	// re-marshaling them (avoids losing unknown fields like preferences).
	var root map[string]json.RawMessage

	existingData, readErr := os.ReadFile(configPath)
	if readErr == nil {
		if unmarshalErr := json.Unmarshal(existingData, &root); unmarshalErr != nil {
			return fmt.Errorf("parse existing claude desktop config %s: %w", configPath, unmarshalErr)
		}
	} else if !os.IsNotExist(readErr) {
		return fmt.Errorf("reading claude desktop config %s: %w", configPath, readErr)
	} else {
		root = make(map[string]json.RawMessage)
	}

	// Parse existing mcpServers or create empty — preserves other servers.
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
		return fmt.Errorf("marshal claude desktop MCP entry: %w", marshalErr)
	}
	mcpServers["atlassian-platform-connector"] = json.RawMessage(entryBytes)

	mcpBytes, err := json.Marshal(mcpServers)
	if err != nil {
		return fmt.Errorf("marshal mcpServers: %w", err)
	}
	root["mcpServers"] = json.RawMessage(mcpBytes)

	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal claude desktop config: %w", err)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(configPath), 0o755); mkdirErr != nil {
		return fmt.Errorf("create claude desktop config directory: %w", mkdirErr)
	}

	if writeErr := os.WriteFile(configPath, output, 0o644); writeErr != nil {
		return fmt.Errorf("write claude desktop config %s: %w", configPath, writeErr)
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
