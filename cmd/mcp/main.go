// Command atlassian-mcp exposes Atlassian operations as MCP tools.
// It can also self-register into AI client config files.
//
// Usage:
//
//	atlassian-mcp mcp                            # Start the stdio MCP server
//	atlassian-mcp setup opencode [--scope local] # Register into OpenCode config
//	atlassian-mcp setup claude   [--scope local] # Register into Claude Code config
//	atlassian-mcp setup claude-desktop           # Register into Claude Desktop config
//	atlassian-mcp setup cursor   [--scope local] # Register into Cursor config
//	atlassian-mcp tui                            # Interactive TUI to configure modules
//	atlassian-mcp version                        # Show version information
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	"github.com/jinkp/atlassian-go-mcp/internal/claude"
	"github.com/jinkp/atlassian-go-mcp/internal/claudedesktop"
	"github.com/jinkp/atlassian-go-mcp/internal/cursor"
	"github.com/jinkp/atlassian-go-mcp/internal/envstore"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
	"github.com/jinkp/atlassian-go-mcp/internal/mcp/features"
	"github.com/jinkp/atlassian-go-mcp/internal/opencode"
)

// Build-time variables — injected via ldflags:
//
//	go build -ldflags "-X main.version=v1.2.0 -X main.commit=abc1234 -X main.date=2026-06-07" ./cmd/mcp
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// setupRecord is saved to engram after a successful registration.
type setupRecord struct {
	Client    string    `json:"client"`
	Scope     string    `json:"scope"`
	ConfigPath string   `json:"config_path"`
	Args      []string  `json:"args"`
	Timestamp time.Time `json:"timestamp"`
}

// saveSetupToEngram writes the registration record to ~/.mcp/atlassian/setup-history.json
// so users can recall which clients are configured.
func saveSetupToEngram(record setupRecord) {
	historyPath := func() string {
		home, _ := os.UserHomeDir()
		return fmt.Sprintf("%s/.mcp/atlassian/setup-history.json", home)
	}()

	var records []setupRecord
	if data, err := os.ReadFile(historyPath); err == nil {
		_ = json.Unmarshal(data, &records)
	}

	// Replace existing entry for same client+scope, or append
	found := false
	for i, r := range records {
		if r.Client == record.Client && r.Scope == record.Scope {
			records[i] = record
			found = true
			break
		}
	}
	if !found {
		records = append(records, record)
	}

	if data, err := json.MarshalIndent(records, "", "  "); err == nil {
		_ = os.MkdirAll(fmt.Sprintf("%s/.mcp/atlassian", func() string {
			home, _ := os.UserHomeDir()
			return home
		}()), 0o755)
		_ = os.WriteFile(historyPath, data, 0o644)
	}
}

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
	root.AddCommand(newVersionCommand())

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
			// Homologated credentials: migrate any legacy ~/.mcp/atlassian/.env into the
			// shared ~/.atlassian/credentials.env (one-time, non-destructive), then load
			// from the shared file and export as env vars (env vars still win).
			_ = envstore.MigrateLegacy()
			envstore.Apply(envstore.Load())

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

			// Bitbucket uses a DIFFERENT host (api.bitbucket.org) and DIFFERENT credentials
			// (base64 username:apiToken), loaded from the shared ~/.atlassian/credentials.env.
			// A dedicated client reuses the same transport stack (auth → idempotency → retry).
			bbCreds := envstore.LoadBitbucket()
			if bbCreds.Workspace != "" && os.Getenv("BITBUCKET_WORKSPACE") == "" {
				_ = os.Setenv("BITBUCKET_WORKSPACE", bbCreds.Workspace)
			}
			if bbCreds.Repo != "" && os.Getenv("BITBUCKET_REPO") == "" {
				_ = os.Setenv("BITBUCKET_REPO", bbCreds.Repo)
			}
			bbClient, err := client.NewClient(client.Config{
				BaseURL:  bitbucket.CloudBaseURL,
				Email:    bbCreds.Username, // BasicAuth encodes username:apiToken
				APIToken: bbCreds.APIToken,
			})
			if err != nil {
				return fmt.Errorf("building Bitbucket HTTP client: %w", err)
			}
			bitbucketSvc := bitbucket.NewService(bbClient, bitbucket.CloudBaseURL)

			auditLog := audit.NewJSONLogger(os.Stderr)
			fs := features.Parse(enableFlag)
			mcpserver.SetVersion(version)
			mcpserver.LogStartupDiagnostics(os.Stderr, fs)
			return mcpserver.StartServer(svc, agileSvc, goalsSvc, releasesSvc, projectsSvc, teamsSvc, bitbucketSvc, auditLog, fs)
		},
	}
	cmd.Flags().StringVar(&enableFlag, "enable", "all",
		"Comma-separated modules to enable: jira,agile,goals,metrics,releases,projects,teams,bitbucket\n"+
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
	setup.AddCommand(newSetupClaudeDesktopCommand())
	setup.AddCommand(newSetupCursorCommand())
	return setup
}

func newSetupOpenCodeCommand() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "Register into OpenCode config",
		Long: `Register atlassian-platform-connector into OpenCode.

  Global (default): ~/.config/opencode/opencode.json
  Local:            ./opencode.json  (current project only)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, err := resolvedBinaryPath()
			if err != nil {
				return err
			}
			entry := opencode.MCPEntry{Type: "local", Command: binPath, Args: []string{"mcp"}}
			var configPath string
			if scope == "local" {
				configPath = opencode.LocalPath()
				err = opencode.SaveLocal(entry)
			} else {
				configPath = opencode.GlobalPath()
				err = opencode.Save(entry)
			}
			if err != nil {
				return fmt.Errorf("saving OpenCode config: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Registered atlassian-platform-connector in %s\n", configPath)
			saveSetupToEngram(setupRecord{Client: "opencode", Scope: scope, ConfigPath: configPath, Args: []string{"mcp"}, Timestamp: time.Now()})
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "global", "Where to register: global (default) or local (current project)")
	return cmd
}

func newSetupClaudeCommand() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Register into Claude Code config",
		Long: `Register atlassian-platform-connector into Claude Code (CLI).

  Global (default): ~/.claude.json
  Local:            ./.claude/settings.json  (current project only)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, err := resolvedBinaryPath()
			if err != nil {
				return err
			}
			entry := claude.MCPEntry{Command: binPath, Args: []string{"mcp"}}
			var configPath string
			if scope == "local" {
				configPath = claude.LocalPath()
				err = claude.SaveLocal(entry)
			} else {
				configPath = claude.GlobalPath()
				err = claude.Save(entry)
			}
			if err != nil {
				return fmt.Errorf("saving Claude config: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Registered atlassian-platform-connector in %s\n", configPath)
			saveSetupToEngram(setupRecord{Client: "claude", Scope: scope, ConfigPath: configPath, Args: []string{"mcp"}, Timestamp: time.Now()})
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "global", "Where to register: global (default) or local (current project)")
	return cmd
}

func newSetupClaudeDesktopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "claude-desktop",
		Short: "Register into Claude Desktop (global only)",
		Long:  `Register atlassian-platform-connector into Claude Desktop. Global only: %APPDATA%\Claude\claude_desktop_config.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, err := resolvedBinaryPath()
			if err != nil {
				return err
			}
			entry := claudedesktop.MCPEntry{Command: binPath, Args: []string{"mcp"}}
			configPath := claudedesktop.GlobalPath()
			if err := claudedesktop.Save(entry); err != nil {
				return fmt.Errorf("saving Claude Desktop config: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Registered atlassian-platform-connector in %s\n", configPath)
			saveSetupToEngram(setupRecord{Client: "claude-desktop", Scope: "global", ConfigPath: configPath, Args: []string{"mcp"}, Timestamp: time.Now()})
			return nil
		},
	}
}

func newSetupCursorCommand() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "cursor",
		Short: "Register into Cursor config",
		Long: `Register atlassian-platform-connector into Cursor.

  Global (default): ~/.cursor/mcp.json
  Local:            ./.cursor/mcp.json  (current project only)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, err := resolvedBinaryPath()
			if err != nil {
				return err
			}
			entry := cursor.MCPEntry{Command: binPath, Args: []string{"mcp"}}
			var configPath string
			if scope == "local" {
				configPath = cursor.LocalPath()
				err = cursor.SaveLocal(entry)
			} else {
				configPath = cursor.GlobalPath()
				err = cursor.Save(entry)
			}
			if err != nil {
				return fmt.Errorf("saving Cursor config: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Registered atlassian-platform-connector in %s\n", configPath)
			saveSetupToEngram(setupRecord{Client: "cursor", Scope: scope, ConfigPath: configPath, Args: []string{"mcp"}, Timestamp: time.Now()})
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "global", "Where to register: global (default) or local (current project)")
	return cmd
}

// newVersionCommand returns the `version` subcommand that prints build info.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(os.Stdout, "atlassian-mcp %s\n", version)
			fmt.Fprintf(os.Stdout, "  commit: %s\n", commit)
			fmt.Fprintf(os.Stdout, "  built:  %s\n", date)
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
