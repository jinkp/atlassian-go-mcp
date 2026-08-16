package envstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withSharedConfig points SharedConfigPath at a temp file for the test and
// returns the path. It restores the env var on cleanup.
func withSharedConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.env")
	t.Setenv("ATLASSIAN_SHARED_CONFIG", path)
	return path
}

// clearEnv unsets any credential env vars so file reads are deterministic.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		KeyBaseURL, KeyEmail, KeyJiraToken, KeyOrgID, legacyKeyToken,
		KeyBitbucketUsername, KeyBitbucketAPIToken, KeyBitbucketWorkspace, KeyBitbucketRepo,
	} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

// --- Save merges, preserving Bitbucket keys written by bbk ---

func TestSave_PreservesBitbucketKeys(t *testing.T) {
	clearEnv(t)
	path := withSharedConfig(t)
	writeFile(t, path,
		"BITBUCKET_USERNAME=joel@example.com\n"+
			"BITBUCKET_API_TOKEN=bb-secret\n")

	err := Save(Credentials{
		BaseURL: "https://acme.atlassian.net",
		Email:   "joel@example.com",
		Token:   "jira-secret",
		OrgID:   "org-123",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := readFileString(t, path)
	// Jira keys written with homologated names
	assertContains(t, got, "ATLASSIAN_BASE_URL=https://acme.atlassian.net")
	assertContains(t, got, "ATLASSIAN_EMAIL=joel@example.com")
	assertContains(t, got, "JIRA_API_TOKEN=jira-secret")
	assertContains(t, got, "ATLASSIAN_ORG_ID=org-123")
	// Bitbucket keys preserved untouched
	assertContains(t, got, "BITBUCKET_USERNAME=joel@example.com")
	assertContains(t, got, "BITBUCKET_API_TOKEN=bb-secret")
	// No legacy token key leaked into the file
	if strings.Contains(got, "ATLASSIAN_TOKEN=") {
		t.Errorf("file must not contain legacy ATLASSIAN_TOKEN key:\n%s", got)
	}
}

// --- SaveBitbucket merges, preserving Jira keys ---

func TestSaveBitbucket_PreservesJiraKeys(t *testing.T) {
	clearEnv(t)
	path := withSharedConfig(t)
	writeFile(t, path,
		"ATLASSIAN_BASE_URL=https://acme.atlassian.net\n"+
			"JIRA_API_TOKEN=jira-secret\n")

	err := SaveBitbucket(BitbucketCredentials{
		Username: "joel@example.com",
		APIToken: "bb-secret",
	})
	if err != nil {
		t.Fatalf("SaveBitbucket: %v", err)
	}

	got := readFileString(t, path)
	assertContains(t, got, "BITBUCKET_USERNAME=joel@example.com")
	assertContains(t, got, "BITBUCKET_API_TOKEN=bb-secret")
	// Jira keys preserved
	assertContains(t, got, "ATLASSIAN_BASE_URL=https://acme.atlassian.net")
	assertContains(t, got, "JIRA_API_TOKEN=jira-secret")
}

// --- Save updates existing keys in place (no duplicates) ---

func TestSave_UpdatesInPlace(t *testing.T) {
	clearEnv(t)
	path := withSharedConfig(t)
	writeFile(t, path,
		"# header comment\n"+
			"ATLASSIAN_BASE_URL=https://old.atlassian.net\n"+
			"JIRA_API_TOKEN=old-token\n"+
			"BITBUCKET_USERNAME=joel\n")

	if err := Save(Credentials{
		BaseURL: "https://new.atlassian.net",
		Email:   "joel@example.com",
		Token:   "new-token",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := readFileString(t, path)
	assertContains(t, got, "# header comment")
	assertContains(t, got, "ATLASSIAN_BASE_URL=https://new.atlassian.net")
	assertContains(t, got, "JIRA_API_TOKEN=new-token")
	assertContains(t, got, "BITBUCKET_USERNAME=joel")
	// No duplicate lines
	if n := strings.Count(got, "ATLASSIAN_BASE_URL="); n != 1 {
		t.Errorf("expected 1 ATLASSIAN_BASE_URL line, got %d:\n%s", n, got)
	}
	if n := strings.Count(got, "JIRA_API_TOKEN="); n != 1 {
		t.Errorf("expected 1 JIRA_API_TOKEN line, got %d:\n%s", n, got)
	}
}

// --- Save skips empty values (does not remove existing lines) ---

func TestSave_EmptyValueLeavesLine(t *testing.T) {
	clearEnv(t)
	path := withSharedConfig(t)
	writeFile(t, path, "ATLASSIAN_ORG_ID=keep-me\n")

	if err := Save(Credentials{
		BaseURL: "https://acme.atlassian.net",
		Email:   "joel@example.com",
		Token:   "jira-secret",
		// OrgID intentionally empty
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := readFileString(t, path)
	assertContains(t, got, "ATLASSIAN_ORG_ID=keep-me")
}

// --- Load reads homologated key and env overlay ---

func TestLoad_SharedFileAndEnvOverlay(t *testing.T) {
	clearEnv(t)
	path := withSharedConfig(t)
	writeFile(t, path,
		"ATLASSIAN_BASE_URL=https://acme.atlassian.net\n"+
			"ATLASSIAN_EMAIL=joel@example.com\n"+
			"JIRA_API_TOKEN=file-token\n")

	// env var wins over file
	t.Setenv(KeyJiraToken, "env-token")

	c := Load()
	if c.BaseURL != "https://acme.atlassian.net" {
		t.Errorf("BaseURL: got %q", c.BaseURL)
	}
	if c.Email != "joel@example.com" {
		t.Errorf("Email: got %q", c.Email)
	}
	if c.Token != "env-token" {
		t.Errorf("Token: env should win, got %q", c.Token)
	}
}

// --- Load falls back to legacy ATLASSIAN_TOKEN key in the file ---

func TestLoad_LegacyTokenFallback(t *testing.T) {
	clearEnv(t)
	path := withSharedConfig(t)
	writeFile(t, path, "ATLASSIAN_TOKEN=legacy-token\n")

	c := Load()
	if c.Token != "legacy-token" {
		t.Errorf("Token: expected legacy fallback, got %q", c.Token)
	}
}

// --- Apply exports the runtime env var under ATLASSIAN_TOKEN ---

func TestApply_ExportsRuntimeTokenEnv(t *testing.T) {
	clearEnv(t)
	Apply(Credentials{
		BaseURL: "https://acme.atlassian.net",
		Email:   "joel@example.com",
		Token:   "jira-secret",
	})
	t.Cleanup(func() {
		_ = os.Unsetenv(KeyBaseURL)
		_ = os.Unsetenv(KeyEmail)
		_ = os.Unsetenv(legacyKeyToken)
	})
	if got := os.Getenv(legacyKeyToken); got != "jira-secret" {
		t.Errorf("runtime ATLASSIAN_TOKEN: got %q, want jira-secret", got)
	}
	// It must NOT export under the file key.
	if got := os.Getenv(KeyJiraToken); got != "" {
		t.Errorf("must not export JIRA_API_TOKEN env, got %q", got)
	}
}

// --- MigrateLegacy copies legacy creds into the shared file, remapping token ---

func TestMigrateLegacy_CopiesAndRemaps(t *testing.T) {
	clearEnv(t)
	shared := withSharedConfig(t)

	// Point the legacy path at a temp file by overriding HOME is brittle; instead
	// we write the legacy file at its resolved location under a temp HOME.
	home := t.TempDir()
	t.Setenv("USERPROFILE", home) // Windows home
	t.Setenv("HOME", home)        // POSIX home
	legacy := filepath.Join(home, ".mcp", "atlassian", ".env")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	writeFile(t, legacy,
		"ATLASSIAN_BASE_URL=https://acme.atlassian.net\n"+
			"ATLASSIAN_EMAIL=joel@example.com\n"+
			"ATLASSIAN_TOKEN=legacy-token\n")

	if err := MigrateLegacy(); err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}

	got := readFileString(t, shared)
	assertContains(t, got, "ATLASSIAN_BASE_URL=https://acme.atlassian.net")
	assertContains(t, got, "ATLASSIAN_EMAIL=joel@example.com")
	assertContains(t, got, "JIRA_API_TOKEN=legacy-token") // remapped
	// legacy file must still exist as backup
	if _, err := os.Stat(legacy); err != nil {
		t.Errorf("legacy file should be preserved: %v", err)
	}
}

// --- MigrateLegacy does not clobber existing shared values ---

func TestMigrateLegacy_DoesNotClobber(t *testing.T) {
	clearEnv(t)
	shared := withSharedConfig(t)
	writeFile(t, shared, "JIRA_API_TOKEN=already-here\n")

	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	legacy := filepath.Join(home, ".mcp", "atlassian", ".env")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	writeFile(t, legacy, "ATLASSIAN_TOKEN=legacy-token\n")

	if err := MigrateLegacy(); err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}
	got := readFileString(t, shared)
	assertContains(t, got, "JIRA_API_TOKEN=already-here")
	if strings.Contains(got, "legacy-token") {
		t.Errorf("must not clobber existing shared value:\n%s", got)
	}
}

// --- helpers ---

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected file to contain %q, got:\n%s", needle, haystack)
	}
}
