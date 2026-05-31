package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleHelp    = lipgloss.NewStyle().Faint(true)
	styleCursor  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	styleChecked = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleUnchecked = lipgloss.NewStyle().Faint(true)
	stylePreview = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleCmd     = lipgloss.NewStyle().Bold(true)
)

// View renders the current screen.
func (m Model) View() string {
	switch m.screen {
	case ScreenModules:
		return m.viewModules()
	case ScreenRegister:
		return m.viewRegister()
	case ScreenDone:
		return m.viewDone()
	}
	return ""
}

func (m Model) viewModules() string {
	var b strings.Builder

	b.WriteString(styleTitle.Render("Atlassian MCP — Configure Tools"))
	b.WriteString("\n\n")
	b.WriteString("Select modules to enable:\n\n")

	for i, mod := range m.modules {
		cursor := "  "
		if i == m.cursor {
			cursor = styleCursor.Render("❯ ")
		}

		var checkbox string
		if mod.Enabled {
			checkbox = styleChecked.Render("[✓]")
		} else {
			checkbox = styleUnchecked.Render("[ ]")
		}

		accessStr := ""
		if mod.Enabled {
			if mod.Access == AccessRead {
				accessStr = styleHelp.Render("  read-only")
			} else {
				accessStr = styleHelp.Render("  read+write")
			}
		}

		b.WriteString(fmt.Sprintf("%s%s %-10s%s\n", cursor, checkbox, mod.Name, accessStr))
	}

	b.WriteString("\n")
	previewLine := fmt.Sprintf("Preview: --enable %s", m.preview)
	if m.preview == "" {
		previewLine = "Preview: (no modules selected)"
	}
	b.WriteString(stylePreview.Render(previewLine))
	b.WriteString("\n\n")
	b.WriteString(styleHelp.Render("[↑↓/jk] Navigate  [space] Toggle  [r] Read-only  [a] All  [enter] Continue  [q] Quit"))

	return b.String()
}

func (m Model) viewRegister() string {
	var b strings.Builder

	b.WriteString(styleTitle.Render("Register MCP Server"))
	b.WriteString("\n\n")

	for i, opt := range m.regOpts {
		if i == m.regCursor {
			b.WriteString(styleCursor.Render("❯ ") + opt)
		} else {
			b.WriteString("  " + opt)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styleHelp.Render("[↑↓] Navigate  [enter] Select  [esc] Back  [q] Quit"))

	return b.String()
}

func (m Model) viewDone() string {
	var b strings.Builder

	if m.errMsg != "" {
		b.WriteString(styleError.Render("⚠  Registration error: " + m.errMsg))
	} else {
		b.WriteString(styleSuccess.Render("✓ Done!"))
	}

	b.WriteString("\n\nCommand:\n")
	b.WriteString(styleCmd.Render("  " + m.doneMsg))
	b.WriteString("\n\n")
	b.WriteString(styleHelp.Render("[q/enter] Quit"))

	return b.String()
}
