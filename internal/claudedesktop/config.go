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
//	    "atlassian-mcp": {"command": "exe", "args": ["mcp"]}
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
	mcpServers[serverName] = json.RawMessage(entryBytes)
	delete(mcpServers, legacyServerName) // drop any pre-rename entry

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

// Remove deletes the atlassian-mcp entry from GlobalPath()
// (Claude Desktop is global-only). Preserves all other keys (preferences, etc.)
// and other MCP servers. Returns (removed, error).
func Remove() (bool, error) { return RemoveFrom(GlobalPath()) }

// RemoveFrom deletes the connector entry from configPath under "mcpServers"
// (both the current name and the legacy pre-rename name), preserving every other
// key. If mcpServers becomes empty it is removed.
func RemoveFrom(configPath string) (bool, error) {
	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return false, nil
		}
		return false, fmt.Errorf("reading claude desktop config %s: %w", configPath, readErr)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("parse claude desktop config %s: %w", configPath, err)
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
		return false, fmt.Errorf("marshal claude desktop config: %w", err)
	}
	if err := os.WriteFile(configPath, output, 0o644); err != nil {
		return false, fmt.Errorf("write claude desktop config %s: %w", configPath, err)
	}
	return true, nil
}
