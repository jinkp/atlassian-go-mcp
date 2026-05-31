// Package envstore manages persistent credentials for atlassian-mcp.
// Credentials are stored in ~/.mcp/atlassian/.env (never committed to git).
package envstore

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	KeyBaseURL = "ATLASSIAN_BASE_URL"
	KeyEmail   = "ATLASSIAN_EMAIL"
	KeyToken   = "ATLASSIAN_TOKEN"
	KeyOrgID   = "ATLASSIAN_ORG_ID"
)

// Credentials holds all Atlassian connection settings.
type Credentials struct {
	BaseURL string
	Email   string
	Token   string
	OrgID   string // optional — only needed for Teams module
}

// Path returns the path to the .env file: ~/.mcp/atlassian/.env
func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mcp", "atlassian", ".env")
}

// Load reads credentials from the .env file, then overlays any existing
// environment variables (env vars always win over the file).
func Load() Credentials {
	fileVars := readFile(Path())

	get := func(key string) string {
		// env var wins
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fileVars[key]
	}

	return Credentials{
		BaseURL: get(KeyBaseURL),
		Email:   get(KeyEmail),
		Token:   get(KeyToken),
		OrgID:   get(KeyOrgID),
	}
}

// Save writes credentials to ~/.mcp/atlassian/.env.
// Empty values are omitted. The file is chmod 0600.
func Save(c Credentials) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("envstore: create dir: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# Atlassian Platform Connector — credentials\n")
	sb.WriteString("# This file is auto-generated. Do not commit to git.\n\n")

	write := func(key, val string) {
		if val != "" {
			sb.WriteString(key + "=" + val + "\n")
		}
	}
	write(KeyBaseURL, c.BaseURL)
	write(KeyEmail, c.Email)
	write(KeyToken, c.Token)
	write(KeyOrgID, c.OrgID)

	if err := os.WriteFile(p, []byte(sb.String()), 0o600); err != nil {
		return fmt.Errorf("envstore: write: %w", err)
	}
	return nil
}

// Apply sets the credentials as OS environment variables in the current process.
// Call this before starting the MCP server so services pick them up.
func Apply(c Credentials) {
	set := func(key, val string) {
		if val != "" {
			_ = os.Setenv(key, val)
		}
	}
	set(KeyBaseURL, c.BaseURL)
	set(KeyEmail, c.Email)
	set(KeyToken, c.Token)
	set(KeyOrgID, c.OrgID)
}

// readFile parses KEY=VALUE lines from path, ignoring comments and blank lines.
func readFile(path string) map[string]string {
	vars := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return vars // file may not exist yet
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip optional surrounding quotes
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		vars[key] = val
	}
	return vars
}
