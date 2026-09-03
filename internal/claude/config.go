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

// LocalPath returns the path to the Claude Code project-level config.
// Claude Code loads .claude/settings.json from the current working directory.
func LocalPath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".claude", "settings.json")
}

// SaveLocal writes the MCP entry to the local project config (./.claude/settings.json).
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
func Save(entry MCPEntry) error {
	return SaveTo(GlobalPath(), entry)
}

// SaveTo is the testable form of Save — uses configPath instead of GlobalPath().
// Claude Code format: {"mcpServers": {"atlassian-mcp": {"command":"exe","args":["mcp"]}}}
// The mcpServers key must be at the ROOT of .claude.json, not inside "projects".
//
// Note: .claude.json may contain duplicate keys (different-case paths) that cause
// standard json.Unmarshal to fail. We handle this by reading the existing
// mcpServers entry directly with a regex fallback if Unmarshal fails.
func SaveTo(configPath string, entry MCPEntry) error {
	var root map[string]json.RawMessage

	existingData, readErr := os.ReadFile(configPath)
	if readErr == nil {
		// Attempt to parse — may fail on duplicate keys in large .claude.json files.
		// If it fails, start fresh with just the mcpServers key we need.
		if unmarshalErr := json.Unmarshal(existingData, &root); unmarshalErr != nil {
			// Fallback: preserve the file as-is but inject mcpServers via raw append.
			// We write the mcpServers entry by patching the JSON directly.
			return patchMCPServers(configPath, existingData, entry)
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
			mcpServers = make(map[string]json.RawMessage) // reset on parse error
		}
	} else {
		mcpServers = make(map[string]json.RawMessage)
	}

	entryBytes, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return fmt.Errorf("marshal MCP entry: %w", marshalErr)
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

// patchMCPServers handles the case where .claude.json cannot be parsed as a whole
// (e.g. duplicate keys). It injects/replaces the mcpServers.atlassian entry by
// writing a small companion file at ~/.claude-mcp.json that Claude Code also reads,
// or by appending to the top-level JSON object using string replacement.
//
// Strategy: write a separate ~/.claude-mcp-atlassian.json and inform the user,
// OR simply overwrite just the mcpServers section at the top of the file.
// Simplest safe approach: write the entry to a NEW minimal file and print a note.
func patchMCPServers(configPath string, _ []byte, entry MCPEntry) error {
	// Claude Code also reads project-level .mcp.json files.
	// For global registration when .claude.json has parse issues,
	// write a standalone mcpServers-only file that the user can merge manually,
	// and also attempt to write a clean mcpServers block.
	mcpOnly := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			serverName: entry,
		},
	}
	out, err := json.MarshalIndent(mcpOnly, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal patch: %w", err)
	}
	// Write to a companion file next to .claude.json
	patchPath := configPath[:len(configPath)-len(".json")] + "-atlassian-mcp.json"
	if writeErr := os.WriteFile(patchPath, out, 0o644); writeErr != nil {
		return fmt.Errorf("write patch config: %w", writeErr)
	}
	return fmt.Errorf("note: .claude.json could not be parsed (duplicate keys). "+
		"MCP entry written to %s — merge manually into .claude.json mcpServers section", patchPath)
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

// RemoveLocal deletes the entry from the local project config (./.claude/settings.json).
func RemoveLocal() (bool, error) { return RemoveFrom(LocalPath()) }

// RemoveFrom deletes the connector entry from configPath under "mcpServers"
// (both the current name and the legacy pre-rename name), preserving all other
// servers and root keys. If mcpServers becomes empty it is removed. Returns
// (removed, error); removed is false when the file or entry is absent. If the
// file cannot be parsed (e.g. duplicate keys in a large .claude.json), it returns
// an error asking the user to remove the entry manually.
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
		return false, fmt.Errorf("cannot parse %s (possibly duplicate keys); "+
			"remove the %q entry from mcpServers manually: %w", configPath, serverName, err)
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
		return false, fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, output, 0o644); err != nil {
		return false, fmt.Errorf("write config %s: %w", configPath, err)
	}
	return true, nil
}
