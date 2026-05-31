// Package tui provides an interactive Bubbletea TUI for configuring and
// registering the atlassian-mcp MCP server.
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jinkp/atlassian-go-mcp/internal/mcp/features"
)

// Screen identifies which screen is currently displayed.
type Screen int

const (
	ScreenModules  Screen = iota // module checkbox selector
	ScreenRegister               // OpenCode / Claude / Both / Skip
	ScreenDone                   // success + command display
)

// AccessLevel for the TUI — read-only or read+write.
// (Write-only not supported in TUI.)
type AccessLevel int

const (
	AccessRead      AccessLevel = 1
	AccessReadWrite AccessLevel = 3
)

// ModuleConfig holds the display state of a single module row.
type ModuleConfig struct {
	Name    string
	Enabled bool
	Access  AccessLevel
}

// Model is the Bubbletea model for the TUI.
type Model struct {
	screen    Screen
	modules   []ModuleConfig
	cursor    int
	regOpts   []string
	regCursor int
	preview   string
	doneMsg   string
	errMsg    string
	width     int
	height    int
}

// defaultModules lists all 7 modules in canonical order, all enabled at RW.
var defaultModules = []ModuleConfig{
	{Name: features.ModuleJira, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleAgile, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleGoals, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleMetrics, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleReleases, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleProjects, Enabled: true, Access: AccessReadWrite},
	{Name: features.ModuleTeams, Enabled: true, Access: AccessReadWrite},
}

var defaultRegOpts = []string{
	"OpenCode",
	"Claude Code (CLI)",
	"Claude Desktop",
	"Cursor",
	"All (OpenCode + Claude Desktop + Cursor)",
	"Skip (show command)",
}

// NewModel creates and returns a fresh Model with all modules enabled at RW.
func NewModel() Model {
	m := Model{
		screen:  ScreenModules,
		modules: make([]ModuleConfig, len(defaultModules)),
		regOpts: defaultRegOpts,
	}
	copy(m.modules, defaultModules)
	m.preview = m.buildPreview()
	return m
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// buildPreview constructs the --enable flag value from current module state.
// Returns "all" when all modules are enabled at RW, "" when none enabled,
// or a comma-joined token list in allModules order.
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

// --- Exported accessors for tests ---

// Screen returns the current screen.
func (m Model) Screen() Screen { return m.screen }

// Cursor returns the current row cursor in the module list.
func (m Model) Cursor() int { return m.cursor }

// Preview returns the current --enable preview string.
func (m Model) Preview() string { return m.preview }

// Modules returns a copy of the module configurations.
func (m Model) Modules() []ModuleConfig {
	cp := make([]ModuleConfig, len(m.modules))
	copy(cp, m.modules)
	return cp
}

// RegOpts returns the registration option labels.
func (m Model) RegOpts() []string { return m.regOpts }

// RegCursor returns the current register screen cursor.
func (m Model) RegCursor() int { return m.regCursor }

// DoneMsg returns the done-screen message string.
func (m Model) DoneMsg() string { return m.doneMsg }

// ErrMsg returns the error message (if any).
func (m Model) ErrMsg() string { return m.errMsg }
