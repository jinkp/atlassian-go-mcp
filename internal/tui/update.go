package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jinkp/atlassian-go-mcp/internal/claude"
	"github.com/jinkp/atlassian-go-mcp/internal/opencode"
)

// Update handles all Bubbletea messages and returns the updated model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.screen {
		case ScreenModules:
			return m.handleModulesKey(msg)
		case ScreenRegister:
			return m.handleRegisterKey(msg)
		case ScreenDone:
			return m.handleDoneKey(msg)
		}
	}
	return m, nil
}

func (m Model) handleModulesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Always copy modules slice to avoid mutating shared backing array
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
		// Toggle all: if all enabled → disable all; otherwise enable all at RW
		allEnabled := true
		for _, mod := range m.modules {
			if !mod.Enabled {
				allEnabled = false
				break
			}
		}
		if allEnabled {
			for i := range m.modules {
				m.modules[i].Enabled = false
			}
		} else {
			for i := range m.modules {
				m.modules[i].Enabled = true
				m.modules[i].Access = AccessReadWrite
			}
		}
		m.preview = m.buildPreview()

	case "enter":
		m.screen = ScreenRegister
		m.regCursor = 0
	}

	return m, nil
}

func (m Model) handleRegisterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

	case "esc":
		m.screen = ScreenModules

	case "enter":
		m = m.executeRegistration()
		m.screen = ScreenDone
	}

	return m, nil
}

func (m Model) handleDoneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "enter":
		return m, tea.Quit
	}
	return m, nil
}

// executeRegistration writes to OpenCode / Claude config based on regCursor selection.
// Returns updated model with doneMsg or errMsg set.
func (m Model) executeRegistration() Model {
	enableVal := m.preview
	if enableVal == "" {
		enableVal = "all"
	}
	args := []string{"mcp", "--enable", enableVal}
	cmd := fmt.Sprintf("atlassian-mcp mcp --enable %s", enableVal)
	m.doneMsg = cmd

	selected := m.regOpts[m.regCursor]

	switch selected {
	case "OpenCode":
		if err := opencode.SaveWithArgs(args); err != nil {
			m.errMsg = fmt.Sprintf("OpenCode registration failed: %v", err)
		}
	case "Claude Code":
		if err := claude.SaveWithArgs(args); err != nil {
			m.errMsg = fmt.Sprintf("Claude registration failed: %v", err)
		}
	case "Both":
		if err := opencode.SaveWithArgs(args); err != nil {
			m.errMsg = fmt.Sprintf("OpenCode registration failed: %v", err)
		}
		if err := claude.SaveWithArgs(args); err != nil {
			if m.errMsg != "" {
				m.errMsg += "; "
			}
			m.errMsg += fmt.Sprintf("Claude registration failed: %v", err)
		}
	case "Skip (show command)":
		// no file I/O — just show the command
	}

	return m
}
