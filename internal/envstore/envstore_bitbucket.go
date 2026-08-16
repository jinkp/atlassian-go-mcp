package envstore

import (
	"os"
	"path/filepath"
)

// Bitbucket credential env keys. These live in the SHARED credentials file
// (~/.atlassian/credentials.env) that is homologated across Atlassian tooling
// (e.g. bbkit), NOT in the atlassian-mcp .env at Path().
const (
	KeyBitbucketUsername  = "BITBUCKET_USERNAME"
	KeyBitbucketAPIToken  = "BITBUCKET_API_TOKEN"
	KeyBitbucketWorkspace = "BITBUCKET_WORKSPACE"
	KeyBitbucketRepo      = "BITBUCKET_REPO"
)

// BitbucketCredentials holds Bitbucket Cloud connection settings.
// Auth against api.bitbucket.org uses base64(Username:APIToken).
type BitbucketCredentials struct {
	Username  string
	APIToken  string
	Workspace string // optional default; may be overridden per tool call
	Repo      string // optional default; may be overridden per tool call
}

// SharedConfigPath returns the path to the shared Atlassian credentials file.
// Precedence: ATLASSIAN_SHARED_CONFIG env var → ~/.atlassian/credentials.env.
// This mirrors bbkit so both tools read the same homologated store.
func SharedConfigPath() string {
	if p := os.Getenv("ATLASSIAN_SHARED_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".atlassian", "credentials.env")
}

// LoadBitbucket reads Bitbucket credentials from the shared credentials file,
// then overlays environment variables (env vars always win over the file).
func LoadBitbucket() BitbucketCredentials {
	fileVars := readFile(SharedConfigPath())

	get := func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fileVars[key]
	}

	return BitbucketCredentials{
		Username:  get(KeyBitbucketUsername),
		APIToken:  get(KeyBitbucketAPIToken),
		Workspace: get(KeyBitbucketWorkspace),
		Repo:      get(KeyBitbucketRepo),
	}
}
