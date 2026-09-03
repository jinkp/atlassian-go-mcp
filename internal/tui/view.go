package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleTitle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleHelp        = lipgloss.NewStyle().Faint(true)
	styleCursor      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	styleChecked     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleUnchecked   = lipgloss.NewStyle().Faint(true)
	stylePreview     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleError       = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleSuccess     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleWarning     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleCmd         = lipgloss.NewStyle().Bold(true)
	styleScopeGlobal = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Faint(true)
	styleScopeLocal  = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	styleScopeFixed  = lipgloss.NewStyle().Faint(true)
	styleFieldActive = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	styleFieldLabel  = lipgloss.NewStyle().Faint(true)
	styleMask        = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleStep        = lipgloss.NewStyle().Faint(true)
	styleSpinner     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleLogo        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleLogoSub     = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Faint(true)
)

// logoArt is the compact ASCII wordmark shown at the top of the first screen.
const logoArt = ` █████╗ ████████╗██╗
██╔══██╗╚══██╔══╝██║
███████║   ██║   ██║
██╔══██║   ██║   ██║
██║  ██║   ██║   ███████╗
╚═╝  ╚═╝   ╚═╝   ╚══════╝`

// renderLogo returns the styled logo block plus the product tagline.
func renderLogo() string {
	return styleLogo.Render(logoArt) + "\n" +
		styleLogoSub.Render("Atlassian Platform Connector  ·  MCP · CLI · REST") + "\n\n"
}

// View renders the current screen.
func (m Model) View() string {
	switch m.screen {
	case ScreenModules:
		return m.viewModules()
	case ScreenCredentials:
		return m.viewCredentials()
	case ScreenTest:
		return m.viewTest()
	case ScreenRegister:
		return m.viewRegister()
	case ScreenDone:
		return m.viewDone()
	}
	return ""
}

// ── Screen 1: Modules ────────────────────────────────────────────────────────

func (m Model) viewModules() string {
	var b strings.Builder

	b.WriteString(renderLogo())
	b.WriteString(styleTitle.Render("Configure Tools"))
	b.WriteString("  " + styleStep.Render("(1/4)"))
	b.WriteString("\n\n")
	b.WriteString("Select modules to enable:\n\n")

	for i, mod := range m.modules {
		cur := "  "
		if i == m.cursor {
			cur = styleCursor.Render("> ")
		}

		var checkbox string
		if mod.Enabled {
			checkbox = styleChecked.Render("[x]")
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

		b.WriteString(fmt.Sprintf("%s%s %-10s%s\n", cur, checkbox, mod.Name, accessStr))
	}

	b.WriteString("\n")
	previewLine := fmt.Sprintf("Preview: --enable %s", m.preview)
	if m.preview == "" {
		previewLine = "Preview: (no modules selected)"
	}
	b.WriteString(stylePreview.Render(previewLine))
	b.WriteString("\n")

	// Write guard status — makes it explicit that read+write also writes
	// ENABLE_WRITE=true into the client config (otherwise writes stay blocked).
	if m.WriteEnabled() {
		b.WriteString(styleSuccess.Render("Write guard: ENABLE_WRITE=true will be written to the client config"))
	} else {
		b.WriteString(styleHelp.Render("Write guard: read-only (no ENABLE_WRITE set)"))
	}
	b.WriteString("\n\n")
	b.WriteString(styleHelp.Render("[jk] Move  [space] Toggle  [r] Read-only  [a] All  [enter] Next  [q] Quit"))

	return b.String()
}

// ── Screen 2: Credentials ────────────────────────────────────────────────────

func (m Model) viewCredentials() string {
	var b strings.Builder

	b.WriteString(styleTitle.Render("Atlassian MCP — Credentials"))
	b.WriteString("  " + styleStep.Render("(2/4)"))
	b.WriteString("\n\n")

	for i, f := range m.credFields {
		isCurrent := i == m.credCursor

		// Label
		label := styleFieldLabel.Render(f.Label)
		if isCurrent {
			label = styleFieldActive.Render(f.Label)
		}
		b.WriteString(label + "\n")

		// Value line
		displayVal := f.Value
		if f.Masked && len(f.Value) > 0 {
			displayVal = strings.Repeat("*", len(f.Value))
		}
		if displayVal == "" {
			displayVal = styleUnchecked.Render("(empty)")
		}

		cursor := "  "
		if isCurrent && m.inputActive {
			cursor = styleCursor.Render("> ")
			displayVal += styleFieldActive.Render("_") // blinking cursor hint
		} else if isCurrent {
			cursor = "  "
		}

		b.WriteString(fmt.Sprintf("  %s%s\n\n", cursor, displayVal))
	}

	// Saved indicator
	if m.errMsg != "" {
		b.WriteString(styleError.Render("  "+m.errMsg) + "\n\n")
	}

	if m.inputActive {
		b.WriteString(styleHelp.Render("[type] Edit  [enter/tab] Next field  [esc] Stop editing  [backspace] Delete"))
	} else {
		b.WriteString(styleHelp.Render("[jk] Move  [enter] Edit field  [ctrl+s] Save & test  [s] Skip  [esc] Back  [q] Quit"))
	}

	return b.String()
}

// ── Screen 3: Test connection ─────────────────────────────────────────────────

func (m Model) viewTest() string {
	var b strings.Builder

	b.WriteString(styleTitle.Render("Atlassian MCP — Test Connection"))
	b.WriteString("  " + styleStep.Render("(3/4)"))
	b.WriteString("\n\n")

	if m.testRunning {
		b.WriteString(styleSpinner.Render("  Testing connectivity...") + "\n\n")
		b.WriteString(styleHelp.Render("[q] Quit"))
		return b.String()
	}

	if len(m.testResults) == 0 {
		b.WriteString(styleHelp.Render("  No tests ran.") + "\n\n")
	} else {
		for _, r := range m.testResults {
			var icon, msg string
			if r.OK {
				icon = styleSuccess.Render("  ok  ")
				msg = styleHelp.Render(r.Message)
			} else {
				icon = styleError.Render("  xx  ")
				msg = styleWarning.Render(r.Message)
			}
			b.WriteString(fmt.Sprintf("%s %-12s  %s\n", icon, r.Module, msg))
		}
	}

	b.WriteString("\n")

	// Summary line
	failed := 0
	for _, r := range m.testResults {
		if !r.OK {
			failed++
		}
	}
	if failed == 0 {
		b.WriteString(styleSuccess.Render("All checks passed!"))
	} else {
		b.WriteString(styleWarning.Render(fmt.Sprintf("%d check(s) failed — you can continue or fix credentials.", failed)))
	}

	b.WriteString("\n\n")
	b.WriteString(styleHelp.Render("[enter] Continue  [r] Retry  [esc] Edit credentials  [q] Quit"))

	return b.String()
}

// ── Screen 4: Register ───────────────────────────────────────────────────────

func (m Model) viewRegister() string {
	var b strings.Builder

	b.WriteString(styleTitle.Render("Atlassian MCP — Register"))
	b.WriteString("  " + styleStep.Render("(4/4)"))
	b.WriteString("\n\n")

	for i, opt := range m.regOpts {
		isCurrent := i == m.regCursor

		cur := "  "
		if isCurrent {
			cur = styleCursor.Render("> ")
		}

		var scopeBadge string
		switch {
		case opt.Client == "skip":
			scopeBadge = ""
		case opt.GlobalOnly:
			scopeBadge = styleScopeFixed.Render(" [global only]")
		case opt.Scope == ScopeLocal:
			scopeBadge = styleScopeLocal.Render(" [local]")
		default:
			scopeBadge = styleScopeGlobal.Render(" [global]")
		}

		b.WriteString(fmt.Sprintf("%s%-42s%s\n", cur, opt.Label, scopeBadge))
	}

	b.WriteString("\n")
	b.WriteString(styleHelp.Render("[jk] Move  [s/tab] Toggle scope  [enter] Register  [esc] Back  [q] Quit"))

	return b.String()
}

// ── Screen 5: Done ───────────────────────────────────────────────────────────

func (m Model) viewDone() string {
	var b strings.Builder

	if m.errMsg != "" {
		b.WriteString(styleError.Render("Warning: " + m.errMsg))
	} else {
		b.WriteString(styleSuccess.Render("Done! Registered atlassian-mcp"))
	}

	b.WriteString("\n\nCommand to run the MCP server:\n")
	b.WriteString(styleCmd.Render("  " + m.doneMsg))

	// Show the write guard env var that was written into the client config so the
	// user understands write access is enabled (and how to reproduce it manually).
	if m.WriteEnabled() {
		b.WriteString("\n\nEnvironment written to client config:\n")
		b.WriteString(styleSuccess.Render("  ENABLE_WRITE=true"))
	} else {
		b.WriteString("\n\n")
		b.WriteString(styleHelp.Render("Read-only: set ENABLE_WRITE=true to enable write tools."))
	}
	b.WriteString("\n\n")
	b.WriteString(styleHelp.Render("[enter/q] Quit"))

	return b.String()
}
