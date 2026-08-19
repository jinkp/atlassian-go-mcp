// Package tui provides an interactive Bubbletea TUI for configuring and
// registering the atlassian-mcp MCP server.
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jinkp/atlassian-go-mcp/internal/envstore"
	"github.com/jinkp/atlassian-go-mcp/internal/mcp/features"
)

// Screen identifies which screen is currently displayed.
type Screen int

const (
	ScreenModules     Screen = iota // 1. module checkbox selector
	ScreenCredentials               // 2. enter/review credentials
	ScreenTest                      // 3. test connection per module
	ScreenRegister                  // 4. client + scope selector
	ScreenDone                      // 5. success + command display
)

// AccessLevel for the TUI — read-only or read+write.
type AccessLevel int

const (
	AccessRead      AccessLevel = 1
	AccessReadWrite AccessLevel = 3
)

// Scope for registration — global or local (project-level).
type Scope int

const (
	ScopeGlobal Scope = iota
	ScopeLocal
)

func (s Scope) String() string {
	if s == ScopeLocal {
		return "local"
	}
	return "global"
}

// credField is a single input field on the credentials screen.
type credField struct {
	Key    string
	Label  string
	Value  string
	Masked bool // true for token — display as ****
}

// ModuleConfig holds the display state of a single module row.
type ModuleConfig struct {
	Name    string
	Enabled bool
	Access  AccessLevel
}

// RegOption holds a registration target with its configurable scope.
type RegOption struct {
	Label      string
	Client     string
	Scope      Scope
	GlobalOnly bool
}

// Model is the Bubbletea model for the TUI.
type Model struct {
	screen    Screen
	modules   []ModuleConfig
	cursor    int

	// credentials screen
	credFields  []credField
	credCursor  int  // which field is focused
	inputActive bool // are we typing into a field?

	// test screen
	testRunning bool
	testResults []TestResult
	testDone    bool

	// register screen
	regOpts   []RegOption
	regCursor int

	preview  string
	doneMsg  string
	errMsg   string
	width    int
	height   int
}

var defaultModules = []ModuleConfig{
	{Name: features.ModuleJira, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleAgile, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleGoals, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleMetrics, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleReleases, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleProjects, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleTeams, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleBitbucket, Enabled: true, Access: AccessReadWrite},
	// Confluence is off by default: lean install. Enable via 'a' or space, or pass --enable all.
	{Name: features.ModuleConfluence, Enabled: false, Access: AccessReadWrite},
}

var defaultRegOpts = []RegOption{
	{Label: "OpenCode", Client: "opencode", Scope: ScopeGlobal},
	{Label: "Claude Code (CLI)", Client: "claude", Scope: ScopeGlobal},
	{Label: "Claude Desktop", Client: "claude-desktop", Scope: ScopeGlobal, GlobalOnly: true},
	{Label: "Cursor", Client: "cursor", Scope: ScopeGlobal},
	{Label: "All (OpenCode + Claude Desktop + Cursor)", Client: "all", Scope: ScopeGlobal},
	{Label: "Skip (show command)", Client: "skip", Scope: ScopeGlobal, GlobalOnly: true},
}

// NewModel creates a fresh Model. Credentials are pre-loaded from .env / env vars.
func NewModel() Model {
	opts := make([]RegOption, len(defaultRegOpts))
	copy(opts, defaultRegOpts)

	creds := envstore.Load()
	bb := envstore.LoadBitbucket()

	m := Model{
		screen:  ScreenModules,
		modules: make([]ModuleConfig, len(defaultModules)),
		regOpts: opts,
		credFields: []credField{
			{Key: envstore.KeyBaseURL, Label: "ATLASSIAN_BASE_URL", Value: creds.BaseURL},
			{Key: envstore.KeyEmail, Label: "ATLASSIAN_EMAIL", Value: creds.Email},
			{Key: envstore.KeyJiraToken, Label: "JIRA_API_TOKEN", Value: creds.Token, Masked: true},
			{Key: envstore.KeyOrgID, Label: "ATLASSIAN_ORG_ID (optional — Teams only)", Value: creds.OrgID},
			{Key: envstore.KeyBitbucketUsername, Label: "BITBUCKET_USERNAME (Bitbucket only)", Value: bb.Username},
			{Key: envstore.KeyBitbucketAPIToken, Label: "BITBUCKET_API_TOKEN (Bitbucket only)", Value: bb.APIToken, Masked: true},
			{Key: envstore.KeyBitbucketWorkspace, Label: "BITBUCKET_WORKSPACE (optional default)", Value: bb.Workspace},
			{Key: envstore.KeyBitbucketRepo, Label: "BITBUCKET_REPO (optional default)", Value: bb.Repo},
		},
	}
	copy(m.modules, defaultModules)
	m.preview = m.buildPreview()
	return m
}

// Init satisfies tea.Model — starts a no-op.
func (m Model) Init() tea.Cmd { return nil }

// buildPreview constructs the --enable flag value from current module state.
func (m Model) buildPreview() string {
	var parts []string
	allRW := true
	anyEnabled := false

	for _, mod := range m.modules {
		if !mod.Enabled {
			allRW = false
			continue
		}
		anyEnabled = true
		switch mod.Access {
		case AccessReadWrite:
			parts = append(parts, mod.Name)
		case AccessRead:
			parts = append(parts, mod.Name+"-read")
			allRW = false
		}
	}

	if !anyEnabled {
		return ""
	}
	if allRW && len(parts) == len(defaultModules) {
		return "all"
	}
	return strings.Join(parts, ",")
}

// currentCreds assembles Atlassian (Jira) Credentials from the credFields state.
func (m Model) currentCreds() envstore.Credentials {
	c := envstore.Credentials{}
	for _, f := range m.credFields {
		switch f.Key {
		case envstore.KeyBaseURL:
			c.BaseURL = f.Value
		case envstore.KeyEmail:
			c.Email = f.Value
		case envstore.KeyJiraToken:
			c.Token = f.Value
		case envstore.KeyOrgID:
			c.OrgID = f.Value
		}
	}
	return c
}

// currentBitbucketCreds assembles BitbucketCredentials from the credFields state.
func (m Model) currentBitbucketCreds() envstore.BitbucketCredentials {
	c := envstore.BitbucketCredentials{}
	for _, f := range m.credFields {
		switch f.Key {
		case envstore.KeyBitbucketUsername:
			c.Username = f.Value
		case envstore.KeyBitbucketAPIToken:
			c.APIToken = f.Value
		case envstore.KeyBitbucketWorkspace:
			c.Workspace = f.Value
		case envstore.KeyBitbucketRepo:
			c.Repo = f.Value
		}
	}
	return c
}

// --- Exported accessors for tests ---

func (m Model) Screen() Screen       { return m.screen }
func (m Model) Cursor() int          { return m.cursor }
func (m Model) Preview() string      { return m.preview }
func (m Model) RegCursor() int       { return m.regCursor }
func (m Model) DoneMsg() string      { return m.doneMsg }
func (m Model) ErrMsg() string       { return m.errMsg }
func (m Model) CredCursor() int      { return m.credCursor }
func (m Model) InputActive() bool    { return m.inputActive }
func (m Model) TestDone() bool       { return m.testDone }
func (m Model) TestResults() []TestResult { return m.testResults }

func (m Model) CredFields() []credField {
	cp := make([]credField, len(m.credFields))
	copy(cp, m.credFields)
	return cp
}

func (m Model) Modules() []ModuleConfig {
	cp := make([]ModuleConfig, len(m.modules))
	copy(cp, m.modules)
	return cp
}

func (m Model) RegOpts() []RegOption {
	cp := make([]RegOption, len(m.regOpts))
	copy(cp, m.regOpts)
	return cp
}

// SimulateTestResults injects a testConnMsg into the model, allowing external
// tests to bypass the async connectivity check. Returns the updated model.
func (m Model) SimulateTestResults(results []TestResult) Model {
	m2, _ := m.Update(testConnMsg{results: results})
	return m2.(Model)
}
