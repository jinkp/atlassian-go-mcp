// Command atlassian-mcp exposes Atlassian operations as MCP tools.
// It can also self-register into AI client config files.
//
// Usage:
//
//	atlassian-mcp mcp                            # Start the stdio MCP server
//	atlassian-mcp setup opencode [--scope local] # Register into OpenCode config
//	atlassian-mcp setup claude   [--scope local] # Register into Claude Code config
//	atlassian-mcp setup claude-plugin [--write]  # Generate a Claude Code plugin (skills-dir)
//	atlassian-mcp setup claude-desktop           # Register into Claude Desktop config
//	atlassian-mcp setup cursor   [--scope local] # Register into Cursor config
//	atlassian-mcp tui                            # Interactive TUI to configure modules
//	atlassian-mcp version                        # Show version information
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
	confluencepkg "github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	"github.com/jinkp/atlassian-go-mcp/internal/claude"
	"github.com/jinkp/atlassian-go-mcp/internal/claudedesktop"
	"github.com/jinkp/atlassian-go-mcp/internal/claudeplugin"
	"github.com/jinkp/atlassian-go-mcp/internal/cursor"
	"github.com/jinkp/atlassian-go-mcp/internal/envstore"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
	"github.com/jinkp/atlassian-go-mcp/internal/mcp/features"
	"github.com/jinkp/atlassian-go-mcp/internal/opencode"
	"github.com/jinkp/atlassian-go-mcp/internal/updatecheck"
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

// setupHistoryPath returns the path to the registration history file.
func setupHistoryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mcp", "atlassian", "setup-history.json")
}

// loadSetupHistory reads the recorded registrations (empty slice if none).
func loadSetupHistory() []setupRecord {
	var records []setupRecord
	if data, err := os.ReadFile(setupHistoryPath()); err == nil {
		_ = json.Unmarshal(data, &records)
	}
	return records
}

// saveSetupHistory writes the registration history. If records is empty, the
// history file is removed so the store reflects a clean state.
func saveSetupHistory(records []setupRecord) {
	p := setupHistoryPath()
	if len(records) == 0 {
		_ = os.Remove(p)
		return
	}
	if data, err := json.MarshalIndent(records, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, data, 0o644)
	}
}

// saveSetupToEngram records a registration in ~/.mcp/atlassian/setup-history.json
// so users (and the uninstall command) can recall which clients are configured.
func saveSetupToEngram(record setupRecord) {
	records := loadSetupHistory()
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
	saveSetupHistory(records)
}

// removeSetupFromHistory drops the record for a given client+scope, if present.
func removeSetupFromHistory(client, scope string) {
	records := loadSetupHistory()
	out := records[:0]
	for _, r := range records {
		if r.Client == client && r.Scope == scope {
			continue
		}
		out = append(out, r)
	}
	saveSetupHistory(out)
}

// reportRemoval prints a consistent message for a single --remove operation and
// updates the setup history when something was actually removed.
func reportRemoval(client, scope, configPath string, removed bool) {
	if removed {
		fmt.Fprintf(os.Stdout, "Removed atlassian-mcp from %s\n", configPath)
		removeSetupFromHistory(client, scope)
		return
	}
	fmt.Fprintf(os.Stdout, "No atlassian-mcp entry found in %s\n", configPath)
}

// removeFromClient dispatches an unregister to the correct client config package.
func removeFromClient(client, configPath string) (bool, error) {
	switch client {
	case "opencode":
		return opencode.RemoveFrom(configPath)
	case "claude":
		return claude.RemoveFrom(configPath)
	case "claude-plugin":
		return claudeplugin.Remove(configPath)
	case "claude-desktop":
		return claudedesktop.RemoveFrom(configPath)
	case "cursor":
		return cursor.RemoveFrom(configPath)
	default:
		return false, fmt.Errorf("unknown client %q", client)
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
	root.AddCommand(newUninstallCommand())
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
			confluenceSvc := confluencepkg.NewService(httpClient, cfg.BaseURL)

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
			return mcpserver.StartServer(svc, agileSvc, goalsSvc, releasesSvc, projectsSvc, teamsSvc, bitbucketSvc, confluenceSvc, auditLog, fs)
		},
	}
	cmd.Flags().StringVar(&enableFlag, "enable", features.DefaultProfile,
		"Comma-separated modules to enable (default: all modules except Confluence).\n"+
			"Suffix -read or -write for access control (e.g. jira-read,agile).\n"+
			"Use 'all' to enable every module including Confluence (79 tools).\n"+
			"Add ',confluence' to the default list to include only the Confluence module.\n"+
			"Default lean profile: "+features.DefaultProfile)
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
	setup.AddCommand(newSetupClaudePluginCommand())
	setup.AddCommand(newSetupClaudeDesktopCommand())
	setup.AddCommand(newSetupCursorCommand())
	return setup
}

// newSetupClaudePluginCommand returns the `setup claude-plugin` command that
// generates a self-contained Claude Code plugin bundling the MCP server, instead
// of editing ~/.claude.json directly. Claude Code auto-loads it as a
// skills-directory plugin ("atlassian-mcp@skills-dir") on the next session.
func newSetupClaudePluginCommand() *cobra.Command {
	var scope string
	var remove bool
	var enable string
	var write bool
	cmd := &cobra.Command{
		Use:   "claude-plugin",
		Short: "Generate a Claude Code plugin that bundles the MCP server",
		Long: `Generate a self-contained Claude Code plugin (instead of editing ~/.claude.json).

  Global (default): ~/.claude/skills/atlassian-mcp/   (auto-loads in every project)
  Local:            ./.claude/skills/atlassian-mcp/    (current repo; needs trust dialog)

The plugin contains .claude-plugin/plugin.json and .mcp.json. Claude Code loads it
as "atlassian-mcp@skills-dir" on the next session — no marketplace, no install step.
Run /reload-plugins after changes. Credentials are still read at runtime from the
shared ~/.atlassian/credentials.env; they are NOT baked into the plugin.

Pass --write to enable write tools (adds ENABLE_WRITE=true to the plugin .mcp.json).
Pass --remove to delete the generated plugin.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := claudeplugin.GlobalDir()
			if scope == "local" {
				dir = claudeplugin.LocalDir()
			}

			if remove {
				removed, err := claudeplugin.Remove(dir)
				if err != nil {
					return fmt.Errorf("removing Claude plugin: %w", err)
				}
				reportRemoval("claude-plugin", scope, dir, removed)
				return nil
			}

			mcpArgs := []string{"mcp"}
			if enable != "" {
				mcpArgs = append(mcpArgs, "--enable", enable)
			}
			var env map[string]string
			if write {
				env = map[string]string{"ENABLE_WRITE": "true"}
			}

			if err := claudeplugin.SaveWithArgsEnv(dir, version, mcpArgs, env); err != nil {
				return fmt.Errorf("generating Claude plugin: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Generated Claude plugin at %s\n", dir)
			if write {
				fmt.Fprintln(os.Stdout, "  write tools: ENABLE_WRITE=true")
			} else {
				fmt.Fprintln(os.Stdout, "  read-only (pass --write to enable write tools)")
			}
			fmt.Fprintln(os.Stdout, "  Claude Code loads it as \"atlassian-mcp@skills-dir\" on the next session.")
			fmt.Fprintln(os.Stdout, "  Already running? Run /reload-plugins to pick it up.")
			saveSetupToEngram(setupRecord{Client: "claude-plugin", Scope: scope, ConfigPath: dir, Args: mcpArgs, Timestamp: time.Now()})
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "global", "Where to generate: global (~/.claude/skills) or local (current repo)")
	cmd.Flags().BoolVar(&remove, "remove", false, "Delete the generated plugin instead of creating it")
	cmd.Flags().StringVar(&enable, "enable", "", "Modules to enable (e.g. jira,agile). Omit for the default lean profile.")
	cmd.Flags().BoolVar(&write, "write", false, "Enable write tools (adds ENABLE_WRITE=true to the plugin)")
	return cmd
}

func newSetupOpenCodeCommand() *cobra.Command {
	var scope string
	var remove bool
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "Register into OpenCode config",
		Long: `Register atlassian-mcp into OpenCode.

  Global (default): ~/.config/opencode/opencode.json
  Local:            ./opencode.json  (current project only)

Pass --remove to unregister instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if remove {
				configPath := opencode.GlobalPath()
				fn := opencode.Remove
				if scope == "local" {
					configPath = opencode.LocalPath()
					fn = opencode.RemoveLocal
				}
				removed, err := fn()
				if err != nil {
					return fmt.Errorf("removing from OpenCode config: %w", err)
				}
				reportRemoval("opencode", scope, configPath, removed)
				return nil
			}
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
			fmt.Fprintf(os.Stdout, "Registered atlassian-mcp in %s\n", configPath)
			saveSetupToEngram(setupRecord{Client: "opencode", Scope: scope, ConfigPath: configPath, Args: []string{"mcp"}, Timestamp: time.Now()})
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "global", "Where to register: global (default) or local (current project)")
	cmd.Flags().BoolVar(&remove, "remove", false, "Unregister (remove) the entry instead of adding it")
	return cmd
}

func newSetupClaudeCommand() *cobra.Command {
	var scope string
	var remove bool
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Register into Claude Code config",
		Long: `Register atlassian-mcp into Claude Code (CLI).

  Global (default): ~/.claude.json
  Local:            ./.claude/settings.json  (current project only)

Pass --remove to unregister instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if remove {
				configPath := claude.GlobalPath()
				fn := claude.Remove
				if scope == "local" {
					configPath = claude.LocalPath()
					fn = claude.RemoveLocal
				}
				removed, err := fn()
				if err != nil {
					return fmt.Errorf("removing from Claude config: %w", err)
				}
				reportRemoval("claude", scope, configPath, removed)
				return nil
			}
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
			fmt.Fprintf(os.Stdout, "Registered atlassian-mcp in %s\n", configPath)
			saveSetupToEngram(setupRecord{Client: "claude", Scope: scope, ConfigPath: configPath, Args: []string{"mcp"}, Timestamp: time.Now()})
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "global", "Where to register: global (default) or local (current project)")
	cmd.Flags().BoolVar(&remove, "remove", false, "Unregister (remove) the entry instead of adding it")
	return cmd
}

func newSetupClaudeDesktopCommand() *cobra.Command {
	var remove bool
	cmd := &cobra.Command{
		Use:   "claude-desktop",
		Short: "Register into Claude Desktop (global only)",
		Long:  `Register atlassian-mcp into Claude Desktop. Global only: %APPDATA%\Claude\claude_desktop_config.json` + "\n\nPass --remove to unregister instead.",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := claudedesktop.GlobalPath()
			if remove {
				removed, err := claudedesktop.Remove()
				if err != nil {
					return fmt.Errorf("removing from Claude Desktop config: %w", err)
				}
				reportRemoval("claude-desktop", "global", configPath, removed)
				return nil
			}
			binPath, err := resolvedBinaryPath()
			if err != nil {
				return err
			}
			entry := claudedesktop.MCPEntry{Command: binPath, Args: []string{"mcp"}}
			if err := claudedesktop.Save(entry); err != nil {
				return fmt.Errorf("saving Claude Desktop config: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Registered atlassian-mcp in %s\n", configPath)
			saveSetupToEngram(setupRecord{Client: "claude-desktop", Scope: "global", ConfigPath: configPath, Args: []string{"mcp"}, Timestamp: time.Now()})
			return nil
		},
	}
	cmd.Flags().BoolVar(&remove, "remove", false, "Unregister (remove) the entry instead of adding it")
	return cmd
}

func newSetupCursorCommand() *cobra.Command {
	var scope string
	var remove bool
	cmd := &cobra.Command{
		Use:   "cursor",
		Short: "Register into Cursor config",
		Long: `Register atlassian-mcp into Cursor.

  Global (default): ~/.cursor/mcp.json
  Local:            ./.cursor/mcp.json  (current project only)

Pass --remove to unregister instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if remove {
				configPath := cursor.GlobalPath()
				fn := cursor.Remove
				if scope == "local" {
					configPath = cursor.LocalPath()
					fn = cursor.RemoveLocal
				}
				removed, err := fn()
				if err != nil {
					return fmt.Errorf("removing from Cursor config: %w", err)
				}
				reportRemoval("cursor", scope, configPath, removed)
				return nil
			}
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
			fmt.Fprintf(os.Stdout, "Registered atlassian-mcp in %s\n", configPath)
			saveSetupToEngram(setupRecord{Client: "cursor", Scope: scope, ConfigPath: configPath, Args: []string{"mcp"}, Timestamp: time.Now()})
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "global", "Where to register: global (default) or local (current project)")
	cmd.Flags().BoolVar(&remove, "remove", false, "Unregister (remove) the entry instead of adding it")
	return cmd
}

// newUninstallCommand returns the `uninstall` command that unregisters the
// connector from every AI client recorded in setup-history.json. It is the
// symmetric counterpart of `setup`. It NEVER touches the shared credentials file
// (~/.atlassian/credentials.env) because it is shared with other Atlassian
// tooling, and it does not remove the binaries or PATH entry (that belongs to the
// installer script; a running executable cannot delete itself on Windows).
func newUninstallCommand() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Unregister the connector from all configured AI clients",
		Long: `Unregister atlassian-mcp from every AI client recorded in
~/.mcp/atlassian/setup-history.json (OpenCode, Claude Code, Claude Plugin, Claude Desktop, Cursor).

The shared credentials file (~/.atlassian/credentials.env) is NEVER deleted — it is
shared with other Atlassian tooling (e.g. bbk). Binaries and the PATH entry are not
touched here — see the printed hint.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			records := loadSetupHistory()
			if len(records) == 0 {
				fmt.Fprintln(os.Stdout, "No setup history found — no client registrations to remove.")
			} else {
				fmt.Fprintln(os.Stdout, "Unregistering from AI clients:")
			}

			var remaining []setupRecord
			for _, r := range records {
				label := fmt.Sprintf("%s (%s) → %s", r.Client, r.Scope, r.ConfigPath)
				if dryRun {
					fmt.Fprintf(os.Stdout, "  [dry-run] would remove %s\n", label)
					remaining = append(remaining, r)
					continue
				}
				removed, err := removeFromClient(r.Client, r.ConfigPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  ! %s: %v\n", label, err)
					remaining = append(remaining, r) // keep so the user can retry/fix
					continue
				}
				if removed {
					fmt.Fprintf(os.Stdout, "  removed    %s\n", label)
				} else {
					fmt.Fprintf(os.Stdout, "  not found  %s\n", label)
				}
			}
			if !dryRun {
				saveSetupHistory(remaining)
			}

			// Credentials are ALWAYS kept — the file is a shared store and this
			// command must never risk deconfiguring other Atlassian tools.
			fmt.Fprintf(os.Stdout, "\nCredentials kept at %s (shared store — never deleted)\n", envstore.Path())

			printBinaryCleanupHint()
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be removed without changing anything")
	return cmd
}

// printBinaryCleanupHint prints the manual steps to remove the binaries and PATH
// entry, which the CLI intentionally does not do (a running executable cannot
// delete itself on Windows; PATH edits belong to the installer script).
func printBinaryCleanupHint() {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".mcp", "atlassian")
	fmt.Fprintln(os.Stdout, "\nTo remove the binaries and PATH entry, run in a NEW terminal:")
	fmt.Fprintf(os.Stdout, "  Remove-Item -Recurse -Force \"%s\"\n", dir)
	fmt.Fprintf(os.Stdout, "  # then remove \"%s\" from your user PATH\n", dir)
}

// newVersionCommand returns the `version` subcommand that prints build info.
// With --check it also queries GitHub for the latest release (read-only).
func newVersionCommand() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(os.Stdout, "atlassian-mcp %s\n", version)
			fmt.Fprintf(os.Stdout, "  commit: %s\n", commit)
			fmt.Fprintf(os.Stdout, "  built:  %s\n", date)
			if check {
				checkForUpdate()
			}
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Check GitHub for a newer release (read-only, no download)")
	return cmd
}

// repoSlug is the GitHub repository used for update checks.
const repoSlug = "jinkp/atlassian-go-mcp"

// checkForUpdate queries the GitHub Releases API and prints whether a newer
// version is available. It is best-effort: network/parse failures print a soft
// note and never fail the command.
func checkForUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	latest, err := updatecheck.FetchLatestTag(ctx, http.DefaultClient, updatecheck.DefaultAPIBase, repoSlug)
	if err != nil {
		fmt.Fprintf(os.Stdout, "  update check: could not reach GitHub (%v)\n", err)
		return
	}

	cmp, comparable := updatecheck.CompareSemver(version, latest)
	switch {
	case !comparable:
		fmt.Fprintf(os.Stdout, "  latest release: %s\n", latest)
	case cmp < 0:
		fmt.Fprintf(os.Stdout, "  update available: %s → %s\n", version, latest)
		fmt.Fprintln(os.Stdout, "  upgrade: irm https://raw.githubusercontent.com/jinkp/atlassian-go-mcp/main/install.ps1 | iex")
	case cmp == 0:
		fmt.Fprintf(os.Stdout, "  you are on the latest version (%s)\n", version)
	default:
		fmt.Fprintf(os.Stdout, "  you are ahead of the latest release (%s > %s)\n", version, latest)
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
