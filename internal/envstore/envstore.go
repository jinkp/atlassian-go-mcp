// Package envstore manages persistent credentials for Atlassian tooling.
//
// Homologation: credentials live in a SINGLE shared file
// (~/.atlassian/credentials.env, override with ATLASSIAN_SHARED_CONFIG) that is
// shared across ALL Atlassian MCP tools (bbk, atlassian-go-mcp). Writes are
// merge-by-line so tools never clobber each other's keys — e.g. saving Jira
// credentials preserves the BITBUCKET_* keys written by bbk, and vice versa.
package envstore

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	KeyBaseURL = "ATLASSIAN_BASE_URL"
	KeyEmail   = "ATLASSIAN_EMAIL"
	// KeyJiraToken is the homologated shared-file key for the Jira/Confluence API
	// token. It matches the key bbk preserves in the shared file.
	KeyJiraToken = "JIRA_API_TOKEN"
	KeyOrgID     = "ATLASSIAN_ORG_ID"

	// legacyKeyToken is the pre-homologation token key. It is still honored when
	// READING (env var + legacy file) for backward compatibility, and it is the
	// env var the MCP server + services expect at runtime via ConfigFromEnv.
	legacyKeyToken = "ATLASSIAN_TOKEN"
)

// canonicalOrder defines the deterministic order used when appending new keys
// to the shared file. Unknown keys are appended afterwards, sorted.
var canonicalOrder = []string{
	KeyBaseURL, KeyEmail, KeyJiraToken, KeyOrgID,
	KeyBitbucketUsername, KeyBitbucketAPIToken, KeyBitbucketWorkspace, KeyBitbucketRepo,
}

// Credentials holds all Atlassian (Jira/Confluence/Teams) connection settings.
type Credentials struct {
	BaseURL string
	Email   string
	Token   string
	OrgID   string // optional — only needed for Teams module
}

// Path returns the homologated shared credentials file. It is the single source
// of truth for every Atlassian tool. See SharedConfigPath (envstore_bitbucket.go).
func Path() string { return SharedConfigPath() }

// LegacyPath returns the pre-homologation store (~/.mcp/atlassian/.env), kept
// only as a migration source and backup.
func LegacyPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mcp", "atlassian", ".env")
}

// Load reads credentials from the shared file, then overlays any existing
// environment variables (env vars always win over the file).
func Load() Credentials {
	fileVars := readFile(Path())

	get := func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fileVars[key]
	}

	// Token: prefer the homologated key, fall back to the legacy key so existing
	// setups (env var or legacy file already migrated in place) keep working.
	token := get(KeyJiraToken)
	if token == "" {
		token = get(legacyKeyToken)
	}

	return Credentials{
		BaseURL: get(KeyBaseURL),
		Email:   get(KeyEmail),
		Token:   token,
		OrgID:   get(KeyOrgID),
	}
}

// Save merges Atlassian (Jira/Confluence/Teams) credentials into the shared
// file, preserving every other line — comments, blank lines, and the
// BITBUCKET_* keys owned by bbk. Empty values are skipped (never written).
func Save(c Credentials) error {
	return mergeEnvFile(Path(), map[string]string{
		KeyBaseURL:   c.BaseURL,
		KeyEmail:     c.Email,
		KeyJiraToken: c.Token,
		KeyOrgID:     c.OrgID,
	})
}

// Apply exports the credentials as the runtime environment variables that the
// MCP server + services read. The token is exported under ATLASSIAN_TOKEN (what
// ConfigFromEnv expects) even though it is stored under JIRA_API_TOKEN.
func Apply(c Credentials) {
	set := func(key, val string) {
		if val != "" {
			_ = os.Setenv(key, val)
		}
	}
	set(KeyBaseURL, c.BaseURL)
	set(KeyEmail, c.Email)
	set(legacyKeyToken, c.Token) // runtime env var read by ConfigFromEnv
	set(KeyOrgID, c.OrgID)
}

// MigrateLegacy performs a one-time, non-destructive migration from the legacy
// store (~/.mcp/atlassian/.env) into the shared file. The legacy ATLASSIAN_TOKEN
// is remapped to JIRA_API_TOKEN. Values already present in the shared file are
// never overwritten, and the legacy file is left untouched as a backup. Safe to
// call on every startup (idempotent).
func MigrateLegacy() error {
	legacy := readFile(LegacyPath())
	if len(legacy) == 0 {
		return nil // nothing to migrate
	}

	shared := readFile(Path())
	updates := map[string]string{}
	consider := func(sharedKey, legacyKey string) {
		if shared[sharedKey] != "" {
			return // already set in shared → do not clobber
		}
		if v := legacy[legacyKey]; v != "" {
			updates[sharedKey] = v
		}
	}
	consider(KeyBaseURL, KeyBaseURL)
	consider(KeyEmail, KeyEmail)
	consider(KeyJiraToken, legacyKeyToken)
	consider(KeyOrgID, KeyOrgID)

	if len(updates) == 0 {
		return nil
	}
	return mergeEnvFile(Path(), updates)
}

// mergeEnvFile writes KEY=VALUE updates into path, preserving all other lines
// (comments, blank lines, and keys not in updates). Keys with empty values are
// skipped — neither written nor removed. Existing keys are updated in place;
// new keys are appended in canonical order. Creates the file (0600) if absent.
func mergeEnvFile(path string, updates map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("envstore: create dir: %w", err)
	}

	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		lines = strings.Split(string(data), "\n")
		// Drop the trailing empty element produced by a final newline.
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("envstore: read %s: %w", path, err)
	}

	applied := make(map[string]bool, len(updates))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.IndexByte(trimmed, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val, ok := updates[key]
		if !ok {
			continue
		}
		if val == "" {
			continue // empty update → leave the existing line as-is
		}
		lines[i] = key + "=" + val
		applied[key] = true
	}

	for _, key := range orderedKeys(updates) {
		val := updates[key]
		if val == "" || applied[key] {
			continue
		}
		lines = append(lines, key+"="+val)
	}

	out := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
		return fmt.Errorf("envstore: write %s: %w", path, err)
	}
	return nil
}

// orderedKeys returns the keys of updates in canonical order, with any unknown
// keys appended in sorted order for deterministic output.
func orderedKeys(updates map[string]string) []string {
	seen := make(map[string]bool, len(updates))
	out := make([]string, 0, len(updates))
	for _, k := range canonicalOrder {
		if _, ok := updates[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	rest := make([]string, 0)
	for k := range updates {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
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
