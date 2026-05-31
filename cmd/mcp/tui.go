package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/jinkp/atlassian-go-mcp/internal/tui"
)

// NewTUICmd returns the `tui` subcommand that launches the interactive
// Bubbletea TUI for configuring and registering the MCP server.
func NewTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:           "tui",
		Short:         "Interactive TUI to configure and register the MCP server",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			m := tui.NewModel()
			p := tea.NewProgram(m, tea.WithAltScreen())
			_, err := p.Run()
			return err
		},
	}
}
