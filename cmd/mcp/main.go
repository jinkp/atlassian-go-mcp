// Command atlassian-mcp exposes Atlassian operations as MCP tools.
// It can also self-register into AI client config files.
//
// Usage:
//
//	atlassian-mcp mcp               # Start the stdio MCP server
//	atlassian-mcp setup opencode    # Register into ~/.config/opencode/opencode.json
//	atlassian-mcp setup claude      # Register into ~/.claude.json
//	atlassian-mcp setup cursor      # Register into ~/.cursor/mcp.json
//	atlassian-mcp tui               # Interactive TUI to configure modules
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	"github.com/jinkp/atlassian-go-mcp/internal/claude"
	"github.com/jinkp/atlassian-go-mcp/internal/cursor"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
	"github.com/jinkp/atlassian-go-mcp/internal/mcp/features"
	"github.com/jinkp/atlassian-go-mcp/internal/opencode"
)

func main() {
	// FIRST STATEMENT — redirect all log output to stderr before any cobra wiring.
	// stdout is owned by the MCP stdio transport; any premature write corrupts JSON-RPC.
	log.SetOutput(os.Stderr)

	root := &cobra.Command{
		Use:           "atlassian-mcp",
		Short:         "Atlassian MCP server",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newMCPCommand())
	root.AddCommand(newSetupCommand())
	root.AddCommand(NewTUICmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// newMCPCommand returns the `mcp` subcommand that starts the stdio MCP server.
func newMCPCommand() *cobra.Command {
	var enableFlag string
	cmd := &cobra.Command{
		Use:           "mcp",
		Short:         "Start the Atlassian MCP stdio server",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := mcpserver.ConfigFromEnv()
			if err != nil {
				return err
			}

			httpClient, err := client.NewClient(cfg)
			if err != nil {
				return fmt.Errorf("building HTTP client: %w", err)
			}

			svc := jira.NewService(httpClient, cfg.BaseURL)
			agileSvc := agile.NewService(httpClient, cfg.BaseURL)
			goalsSvc := goals.NewService(httpClient, cfg.BaseURL)
			releasesSvc := releases.NewService(httpClient, cfg.BaseURL)
			projectsSvc := projects.NewService(httpClient, cfg.BaseURL)
			// ATLASSIAN_ORG_ID is required for teams tools. If empty, teams tools will fail
			// at invocation time with an appropriate error — non-teams tools are unaffected.
			orgID := os.Getenv("ATLASSIAN_ORG_ID")
			teamsSvc := teams.NewService(httpClient, orgID)
			auditLog := audit.NewJSONLogger(os.Stderr)
			fs := features.Parse(enableFlag)
			return mcpserver.StartServer(svc, agileSvc, goalsSvc, releasesSvc, projectsSvc, teamsSvc, auditLog, fs)
		},
	}
	cmd.Flags().StringVar(&enableFlag, "enable", "all",
		"Comma-separated modules to enable: jira,agile,goals,metrics,releases,projects,teams\n"+
			"Suffix -read or -write for access control (e.g. jira-read,agile).\n"+
			"Default 'all' enables every tool.")
	return cmd
}

// newSetupCommand returns the `setup` parent command with opencode/claude subcommands.
func newSetupCommand() *cobra.Command {
	setup := &cobra.Command{
		Use:   "setup",
		Short: "Register atlassian-mcp into an AI client config",
	}
	setup.AddCommand(newSetupOpenCodeCommand())
	setup.AddCommand(newSetupClaudeCommand())
	setup.AddCommand(newSetupCursorCommand())
	return setup
}

func newSetupOpenCodeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "opencode",
		Short: "Register into OpenCode (~/.config/opencode/opencode.json)",
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, err := resolvedBinaryPath()
			if err != nil {
				return err
			}
			entry := opencode.MCPEntry{
				Type:    "local",
				Command: binPath,
				Args:    []string{"mcp"},
			}
			if err := opencode.Save(entry); err != nil {
				return fmt.Errorf("saving OpenCode config: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Registered atlassian-mcp in %s\n", opencode.GlobalPath())
			return nil
		},
	}
}

func newSetupClaudeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "claude",
		Short: "Register into Claude Code (~/.claude.json)",
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, err := resolvedBinaryPath()
			if err != nil {
				return err
			}
			entry := claude.MCPEntry{
				Command: binPath,
				Args:    []string{"mcp"},
			}
			if err := claude.Save(entry); err != nil {
				return fmt.Errorf("saving Claude config: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Registered atlassian-mcp in %s\n", claude.GlobalPath())
			return nil
		},
	}
}

func newSetupCursorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cursor",
		Short: "Register into Cursor (~/.cursor/mcp.json)",
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, err := resolvedBinaryPath()
			if err != nil {
				return err
			}
			entry := cursor.MCPEntry{
				Command: binPath,
				Args:    []string{"mcp"},
			}
			if err := cursor.Save(entry); err != nil {
				return fmt.Errorf("saving Cursor config: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Registered atlassian-mcp in %s\n", cursor.GlobalPath())
			return nil
		},
	}
}

// resolvedBinaryPath returns the absolute path of the running binary,
// following any symlinks so the config entry always points to the real file.
func resolvedBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving binary path: %w", err)
	}
	return exe, nil
}
