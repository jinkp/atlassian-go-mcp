// Package bitbucket provides cobra commands for Bitbucket Cloud operations
// (repositories, pull requests, branches, pipelines) via the Bitbucket REST API v2.0.
package bitbucket

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewBitbucketCmd returns the "bitbucket" subcommand group with persistent
// --workspace and --repo flags shared by all children.
func NewBitbucketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bitbucket",
		Short: "Interact with Bitbucket Cloud (repos, pull requests, branches, pipelines)",
		Long:  "Read and manage Bitbucket Cloud resources using the Bitbucket REST API v2.0.",
	}
	cmd.PersistentFlags().String("workspace", "", "Bitbucket workspace slug (overrides BITBUCKET_WORKSPACE env)")
	cmd.PersistentFlags().String("repo", "", "Repository slug (overrides BITBUCKET_REPO env)")
	return cmd
}

// RegisterCommands attaches all Bitbucket sub-commands to root.
func RegisterCommands(root *cobra.Command, svc bitbucket.BitbucketService, auditLog audit.Logger, dryRun bool) {
	root.AddCommand(newReposCmd(svc))
	root.AddCommand(newBranchesCmd(svc))
	root.AddCommand(newStaleBranchesCmd(svc))

	pipeline := &cobra.Command{
		Use:   "pipeline",
		Short: "Pipeline operations",
	}
	pipeline.AddCommand(newPipelineListCmd(svc))
	pipeline.AddCommand(newPipelineRunCmd(svc, auditLog, dryRun))
	root.AddCommand(pipeline)

	pr := &cobra.Command{
		Use:   "pr",
		Short: "Pull request operations",
	}
	// read
	pr.AddCommand(newPRListCmd(svc))
	pr.AddCommand(newPRGetCmd(svc))
	pr.AddCommand(newPRCommentsCmd(svc))
	pr.AddCommand(newPRCommitsCmd(svc))
	pr.AddCommand(newPRFilesCmd(svc))
	pr.AddCommand(newPRDiffCmd(svc))
	pr.AddCommand(newPRChecksCmd(svc))
	pr.AddCommand(newPRReviewersCmd(svc))
	// write
	pr.AddCommand(newPRCreateCmd(svc, auditLog, dryRun))
	pr.AddCommand(newPRCommentCmd(svc, auditLog, dryRun))
	pr.AddCommand(newPRUpdateCmd(svc, auditLog, dryRun))
	pr.AddCommand(newPRApproveCmd(svc, auditLog, dryRun))
	pr.AddCommand(newPRDeclineCmd(svc, auditLog, dryRun))
	pr.AddCommand(newPRMergeCmd(svc, auditLog, dryRun))
	// local (git / browser)
	pr.AddCommand(newPRCheckoutCmd(svc))
	pr.AddCommand(newPROpenCmd(svc))

	task := &cobra.Command{
		Use:   "task",
		Short: "Pull request task operations",
	}
	task.AddCommand(newPRTaskCreateCmd(svc, auditLog, dryRun))
	task.AddCommand(newPRTaskResolveCmd(svc, auditLog, dryRun))
	pr.AddCommand(task)

	root.AddCommand(pr)
}

// --- shared helpers ---

// resolveWorkspace reads the workspace from the --workspace flag (inherited from
// the bitbucket root) then falls back to BITBUCKET_WORKSPACE.
func resolveWorkspace(cmd *cobra.Command) (string, error) {
	ws, _ := cmd.Flags().GetString("workspace")
	if ws == "" {
		ws = os.Getenv("BITBUCKET_WORKSPACE")
	}
	if ws == "" {
		return "", fmt.Errorf("no workspace configured — pass --workspace or set BITBUCKET_WORKSPACE")
	}
	return ws, nil
}

// resolveWorkspaceRepo resolves both workspace and repo (flag → env).
func resolveWorkspaceRepo(cmd *cobra.Command) (workspace, repo string, err error) {
	workspace, err = resolveWorkspace(cmd)
	if err != nil {
		return "", "", err
	}
	repo, _ = cmd.Flags().GetString("repo")
	if repo == "" {
		repo = os.Getenv("BITBUCKET_REPO")
	}
	if repo == "" {
		return "", "", fmt.Errorf("no repository configured — pass --repo or set BITBUCKET_REPO")
	}
	return workspace, repo, nil
}

// parsePRID parses a positive pull request ID from a CLI argument.
func parsePRID(arg string) (int, error) {
	id, err := strconv.Atoi(arg)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("pull request id must be a positive integer, got %q", arg)
	}
	return id, nil
}

// emit formats v with the requested output format and writes it to stdout.
// Exits the process on formatter/serialization errors.
func emit(cmd *cobra.Command, format string, v any) {
	formatter, err := output.NewFormatter(format)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	data, err := formatter.Format(v)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error formatting output:", err)
		os.Exit(2)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
}

// fail prints an error to stderr and exits with a mapped exit code.
func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(bitbucketExitCode(err))
}

// bitbucketExitCode maps sentinel errors to POSIX-style exit codes.
func bitbucketExitCode(err error) int {
	if err == nil {
		return 0
	}
	switch {
	case errors.Is(err, bitbucket.ErrNotFound):
		return 3
	case errors.Is(err, bitbucket.ErrUnauthorized):
		return 2
	case errors.Is(err, bitbucket.ErrRateLimit):
		return 2
	default:
		return 2
	}
}

// outputFlag registers the shared -o/--output flag on a command.
func outputFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVarP(target, "output", "o", "table", "Output format: table, json, yaml")
}
