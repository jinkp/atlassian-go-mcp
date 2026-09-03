package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jinkp/atlassian-go-mcp/internal/claude"
	"github.com/jinkp/atlassian-go-mcp/internal/claudedesktop"
	"github.com/jinkp/atlassian-go-mcp/internal/claudeplugin"
	"github.com/jinkp/atlassian-go-mcp/internal/cursor"
	"github.com/jinkp/atlassian-go-mcp/internal/envstore"
	"github.com/jinkp/atlassian-go-mcp/internal/opencode"
)

// Update handles all Bubbletea messages and returns the updated model + next cmd.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case testConnMsg:
		m.testResults = msg.results
		m.testRunning = false
		m.testDone = true
		return m, nil

	case tea.KeyMsg:
		switch m.screen {
		case ScreenModules:
			return m.handleModulesKey(msg)
		case ScreenCredentials:
			return m.handleCredentialsKey(msg)
		case ScreenTest:
			return m.handleTestKey(msg)
		case ScreenRegister:
			return m.handleRegisterKey(msg)
		case ScreenDone:
			return m.handleDoneKey(msg)
		}
	}
	return m, nil
}

// ── Screen 1: Modules ────────────────────────────────────────────────────────

func (m Model) handleModulesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	mods := make([]ModuleConfig, len(m.modules))
	copy(mods, m.modules)
	m.modules = mods

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		m.cursor--
		if m.cursor < 0 {
			m.cursor = len(m.modules) - 1
		}

	case "down", "j":
		m.cursor++
		if m.cursor >= len(m.modules) {
			m.cursor = 0
		}

	case " ":
		m.modules[m.cursor].Enabled = !m.modules[m.cursor].Enabled
		m.preview = m.buildPreview()

	case "r":
		if m.modules[m.cursor].Enabled {
			if m.modules[m.cursor].Access == AccessReadWrite {
				m.modules[m.cursor].Access = AccessRead
			} else {
				m.modules[m.cursor].Access = AccessReadWrite
			}
			m.preview = m.buildPreview()
		}

	case "a":
		allEnabled := true
		for _, mod := range m.modules {
			if !mod.Enabled {
				allEnabled = false
				break
			}
		}
		for i := range m.modules {
			if allEnabled {
				m.modules[i].Enabled = false
			} else {
				m.modules[i].Enabled = true
				m.modules[i].Access = AccessReadWrite
			}
		}
		m.preview = m.buildPreview()

	case "enter":
		// Advance to credentials screen
		m.screen = ScreenCredentials
		m.credCursor = 0
		m.inputActive = false
	}

	return m, nil
}

// ── Screen 2: Credentials ────────────────────────────────────────────────────

func (m Model) handleCredentialsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fields := make([]credField, len(m.credFields))
	copy(fields, m.credFields)
	m.credFields = fields

	if m.inputActive {
		// Typing mode — capture characters
		switch msg.String() {
		case "enter", "tab":
			// Commit field, move to next or stop
			m.inputActive = false
			m.credCursor++
			if m.credCursor < len(m.credFields) {
				m.inputActive = true
			} else {
				m.credCursor = len(m.credFields) - 1
			}

		case "esc":
			m.inputActive = false

		case "backspace", "ctrl+h":
			if len(m.credFields[m.credCursor].Value) > 0 {
				v := m.credFields[m.credCursor].Value
				m.credFields[m.credCursor].Value = v[:len(v)-1]
			}

		default:
			// Append printable character
			ch := msg.String()
			if len(ch) == 1 && ch[0] >= 32 && ch[0] < 127 {
				m.credFields[m.credCursor].Value += ch
			}
		}
		return m, nil
	}

	// Navigation mode
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.screen = ScreenModules

	case "up", "k":
		m.credCursor--
		if m.credCursor < 0 {
			m.credCursor = len(m.credFields) - 1
		}

	case "down", "j", "tab":
		m.credCursor++
		if m.credCursor >= len(m.credFields) {
			m.credCursor = 0
		}

	case "enter", " ":
		// Enter typing mode for current field
		m.inputActive = true

	case "s":
		// Skip — proceed without saving
		m.screen = ScreenTest
		m.testDone = false
		m.testRunning = true
		m.testResults = nil
		creds := m.currentCreds()
		bbCreds := m.currentBitbucketCreds()
		fs := toFeatureSet(m.modules)
		return m, func() tea.Msg { return runConnectivityTests(creds, bbCreds, fs)() }

	case "ctrl+s":
		// Save both Atlassian (Jira) and Bitbucket credentials into the shared
		// file (each merge-by-line, preserving the other's keys), then test.
		var saveErrs []string
		if err := envstore.Save(m.currentCreds()); err != nil {
			saveErrs = append(saveErrs, err.Error())
		}
		if err := envstore.SaveBitbucket(m.currentBitbucketCreds()); err != nil {
			saveErrs = append(saveErrs, err.Error())
		}
		if len(saveErrs) > 0 {
			m.errMsg = "Could not save credentials: " + strings.Join(saveErrs, "; ")
		} else {
			m.errMsg = ""
		}
		m.screen = ScreenTest
		m.testDone = false
		m.testRunning = true
		m.testResults = nil
		creds := m.currentCreds()
		bbCreds := m.currentBitbucketCreds()
		fs := toFeatureSet(m.modules)
		return m, func() tea.Msg { return runConnectivityTests(creds, bbCreds, fs)() }
	}

	return m, nil
}

// ── Screen 3: Test connection ────────────────────────────────────────────────

func (m Model) handleTestKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.testRunning {
		// Only allow quit while tests are running
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.screen = ScreenCredentials

	case "r":
		// Retry
		m.testDone = false
		m.testRunning = true
		m.testResults = nil
		creds := m.currentCreds()
		bbCreds := m.currentBitbucketCreds()
		fs := toFeatureSet(m.modules)
		return m, func() tea.Msg { return runConnectivityTests(creds, bbCreds, fs)() }

	case "enter":
		// Continue to registration regardless of test results
		m.screen = ScreenRegister
		m.regCursor = 0
	}

	return m, nil
}

// ── Screen 4: Register ───────────────────────────────────────────────────────

func (m Model) handleRegisterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	opts := make([]RegOption, len(m.regOpts))
	copy(opts, m.regOpts)
	m.regOpts = opts

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		m.regCursor--
		if m.regCursor < 0 {
			m.regCursor = len(m.regOpts) - 1
		}

	case "down", "j":
		m.regCursor++
		if m.regCursor >= len(m.regOpts) {
			m.regCursor = 0
		}

	case "s", "tab":
		// Toggle scope on current option
		cur := &m.regOpts[m.regCursor]
		if !cur.GlobalOnly {
			if cur.Scope == ScopeGlobal {
				cur.Scope = ScopeLocal
			} else {
				cur.Scope = ScopeGlobal
			}
		}

	case "esc":
		m.screen = ScreenTest

	case "enter":
		m = m.executeRegistration()
		m.screen = ScreenDone
	}

	return m, nil
}

// ── Screen 5: Done ───────────────────────────────────────────────────────────

func (m Model) handleDoneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "enter":
		return m, tea.Quit
	}
	return m, nil
}

// ── Registration logic ───────────────────────────────────────────────────────

func (m Model) executeRegistration() Model {
	enableVal := m.preview
	if enableVal == "" {
		enableVal = "all"
	}
	args := []string{"mcp", "--enable", enableVal}

	// Write tools are registered via --enable but ALSO gated at runtime by the
	// ENABLE_WRITE guard (see internal/mcp/server.go WriteGuardCheck). When any
	// enabled module has read+write access, inject ENABLE_WRITE=true into the
	// generated client config so write tools actually work — not just register.
	var env map[string]string
	if m.WriteEnabled() {
		env = map[string]string{"ENABLE_WRITE": "true"}
	}
	m.doneMsg = fmt.Sprintf("atlassian-mcp mcp --enable %s", enableVal)

	opt := m.regOpts[m.regCursor]

	var errs []string
	addErr := func(msg string) { errs = append(errs, msg) }

	save := func(clientName string, scope Scope) {
		switch clientName {
		case "opencode":
			var err error
			if scope == ScopeLocal {
				err = opencode.SaveWithArgsEnvLocal(args, env)
			} else {
				err = opencode.SaveWithArgsEnv(args, env)
			}
			if err != nil {
				addErr(fmt.Sprintf("OpenCode: %v", err))
			}
		case "claude":
			var err error
			if scope == ScopeLocal {
				err = claude.SaveWithArgsEnvLocal(args, env)
			} else {
				err = claude.SaveWithArgsEnv(args, env)
			}
			if err != nil {
				addErr(fmt.Sprintf("Claude Code: %v", err))
			}
		case "claude-plugin":
			dir := claudeplugin.GlobalDir()
			if scope == ScopeLocal {
				dir = claudeplugin.LocalDir()
			}
			// version "" → manifest omits it (fine for a skills-dir plugin; the
			// TUI has no build version injected). Write access flows via env.
			if err := claudeplugin.SaveWithArgsEnv(dir, "", args, env); err != nil {
				addErr(fmt.Sprintf("Claude Plugin: %v", err))
			}
		case "claude-desktop":
			if err := claudedesktop.SaveWithArgsEnv(args, env); err != nil {
				addErr(fmt.Sprintf("Claude Desktop: %v", err))
			}
		case "cursor":
			var err error
			if scope == ScopeLocal {
				err = cursor.SaveWithArgsEnvLocal(args, env)
			} else {
				err = cursor.SaveWithArgsEnv(args, env)
			}
			if err != nil {
				addErr(fmt.Sprintf("Cursor: %v", err))
			}
		}
	}

	switch opt.Client {
	case "all":
		save("opencode", opt.Scope)
		save("claude-desktop", ScopeGlobal)
		save("cursor", opt.Scope)
	case "skip":
		// no-op
	default:
		save(opt.Client, opt.Scope)
	}

	if len(errs) > 0 {
		m.errMsg = errs[0]
		for _, e := range errs[1:] {
			m.errMsg += "; " + e
		}
	}

	return m
}
