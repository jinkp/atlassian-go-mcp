package bitbucket

import (
	"context"
	"fmt"
	"strings"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/spf13/cobra"
)

// splitCSVOrNil splits a comma-separated string into trimmed non-empty values,
// returning nil when the input is empty (so update can distinguish "unchanged").
func splitCSVOrNil(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func newPRCreateCmd(svc bitbucket.BitbucketService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		title, source, destination, description, reviewers string
		closeSourceBranch                                  bool
		outputFormat                                       string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pull request",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would create PR in %s/%s: %q (%s -> %s)\n", ws, repo, title, source, destination)
				return nil
			}
			req := bitbucket.CreatePRRequest{
				Title:             title,
				SourceBranch:      source,
				DestinationBranch: destination,
				Description:       description,
				Reviewers:         splitCSVOrNil(reviewers),
				CloseSourceBranch: closeSourceBranch,
			}
			pr, err := svc.CreatePullRequest(context.Background(), ws, repo, req)
			auditLog.Log(audit.NewEntry("bb_create_pr", "bitbucket",
				map[string]any{"workspace": ws, "repo": repo, "source": source, "destination": destination}, err))
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, pr)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Pull request title (required)")
	cmd.Flags().StringVar(&source, "source", "", "Source branch name (required)")
	cmd.Flags().StringVar(&destination, "destination", "", "Destination branch name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Pull request description (optional)")
	cmd.Flags().StringVar(&reviewers, "reviewers", "", "Comma-separated reviewer nicknames or {uuid} values (optional)")
	cmd.Flags().BoolVar(&closeSourceBranch, "close-source-branch", false, "Close source branch after merge (optional)")
	outputFlag(cmd, &outputFormat)
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("source")
	_ = cmd.MarkFlagRequired("destination")
	return cmd
}

func newPRCommentCmd(svc bitbucket.BitbucketService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		message      string
		outputFormat string
	)
	cmd := &cobra.Command{
		Use:   "comment <pr-id>",
		Short: "Add a comment to a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			id, err := parsePRID(args[0])
			if err != nil {
				fail(err)
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would comment on PR #%d in %s/%s\n", id, ws, repo)
				return nil
			}
			comment, err := svc.AddPRComment(context.Background(), ws, repo, id, message)
			auditLog.Log(audit.NewEntry("bb_comment_pr", "bitbucket",
				map[string]any{"workspace": ws, "repo": repo, "pr_id": id}, err))
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, comment)
			return nil
		},
	}
	cmd.Flags().StringVar(&message, "message", "", "Comment text (required)")
	outputFlag(cmd, &outputFormat)
	_ = cmd.MarkFlagRequired("message")
	return cmd
}

func newPRUpdateCmd(svc bitbucket.BitbucketService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		title, description, destination, reviewers string
		outputFormat                               string
	)
	cmd := &cobra.Command{
		Use:   "update <pr-id>",
		Short: "Update a pull request (at least one field required)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			id, err := parsePRID(args[0])
			if err != nil {
				fail(err)
			}
			if strings.TrimSpace(title) == "" && strings.TrimSpace(description) == "" &&
				strings.TrimSpace(destination) == "" && strings.TrimSpace(reviewers) == "" {
				fail(fmt.Errorf("at least one of --title, --description, --destination, --reviewers is required"))
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would update PR #%d in %s/%s\n", id, ws, repo)
				return nil
			}
			req := bitbucket.UpdatePRRequest{
				Title:             title,
				Description:       description,
				DestinationBranch: destination,
				Reviewers:         splitCSVOrNil(reviewers),
			}
			pr, err := svc.UpdatePullRequest(context.Background(), ws, repo, id, req)
			auditLog.Log(audit.NewEntry("bb_update_pr", "bitbucket",
				map[string]any{"workspace": ws, "repo": repo, "pr_id": id}, err))
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, pr)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "New pull request title (optional)")
	cmd.Flags().StringVar(&description, "description", "", "New pull request description (optional)")
	cmd.Flags().StringVar(&destination, "destination", "", "New destination branch name (optional)")
	cmd.Flags().StringVar(&reviewers, "reviewers", "", "Comma-separated reviewers; replaces existing (optional)")
	outputFlag(cmd, &outputFormat)
	return cmd
}

func newPRApproveCmd(svc bitbucket.BitbucketService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve <pr-id>",
		Short: "Approve a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			id, err := parsePRID(args[0])
			if err != nil {
				fail(err)
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would approve PR #%d in %s/%s\n", id, ws, repo)
				return nil
			}
			err = svc.ApprovePullRequest(context.Background(), ws, repo, id)
			auditLog.Log(audit.NewEntry("bb_approve_pr", "bitbucket",
				map[string]any{"workspace": ws, "repo": repo, "pr_id": id}, err))
			if err != nil {
				fail(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "PR #%d approved\n", id)
			return nil
		},
	}
	return cmd
}

func newPRTaskCreateCmd(svc bitbucket.BitbucketService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		message      string
		outputFormat string
	)
	cmd := &cobra.Command{
		Use:   "create <pr-id>",
		Short: "Create a task on a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			id, err := parsePRID(args[0])
			if err != nil {
				fail(err)
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would create task on PR #%d in %s/%s\n", id, ws, repo)
				return nil
			}
			task, err := svc.CreatePRTask(context.Background(), ws, repo, id, message)
			auditLog.Log(audit.NewEntry("bb_create_pr_task", "bitbucket",
				map[string]any{"workspace": ws, "repo": repo, "pr_id": id}, err))
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, task)
			return nil
		},
	}
	cmd.Flags().StringVar(&message, "message", "", "Task description text (required)")
	outputFlag(cmd, &outputFormat)
	_ = cmd.MarkFlagRequired("message")
	return cmd
}

func newPRDeclineCmd(svc bitbucket.BitbucketService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decline <pr-id>",
		Short: "Decline (reject) a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			id, err := parsePRID(args[0])
			if err != nil {
				fail(err)
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would decline PR #%d in %s/%s\n", id, ws, repo)
				return nil
			}
			err = svc.DeclinePullRequest(context.Background(), ws, repo, id)
			auditLog.Log(audit.NewEntry("bb_decline_pr", "bitbucket",
				map[string]any{"workspace": ws, "repo": repo, "pr_id": id}, err))
			if err != nil {
				fail(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "PR #%d declined\n", id)
			return nil
		},
	}
	return cmd
}

func newPRMergeCmd(svc bitbucket.BitbucketService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		strategy, message string
		outputFormat      string
	)
	cmd := &cobra.Command{
		Use:   "merge <pr-id>",
		Short: "Merge a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			id, err := parsePRID(args[0])
			if err != nil {
				fail(err)
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would merge PR #%d in %s/%s (strategy=%q)\n", id, ws, repo, strategy)
				return nil
			}
			pr, err := svc.MergePullRequest(context.Background(), ws, repo, id, strategy, message)
			auditLog.Log(audit.NewEntry("bb_merge_pr", "bitbucket",
				map[string]any{"workspace": ws, "repo": repo, "pr_id": id, "strategy": strategy}, err))
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, pr)
			return nil
		},
	}
	cmd.Flags().StringVar(&strategy, "strategy", "", "Merge strategy: merge_commit, squash, fast_forward, squash_fast_forward, rebase_fast_forward, rebase_merge (optional)")
	cmd.Flags().StringVar(&message, "message", "", "Custom merge commit message (optional)")
	outputFlag(cmd, &outputFormat)
	return cmd
}

func newPipelineRunCmd(svc bitbucket.BitbucketService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		branch       string
		outputFormat string
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Trigger a pipeline for a branch",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would run pipeline on branch %q in %s/%s\n", branch, ws, repo)
				return nil
			}
			pipeline, err := svc.RunPipeline(context.Background(), ws, repo, branch)
			auditLog.Log(audit.NewEntry("bb_run_pipeline", "bitbucket",
				map[string]any{"workspace": ws, "repo": repo, "branch": branch}, err))
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, pipeline)
			return nil
		},
	}
	cmd.Flags().StringVar(&branch, "branch", "", "Branch to run the pipeline on (required)")
	outputFlag(cmd, &outputFormat)
	_ = cmd.MarkFlagRequired("branch")
	return cmd
}

func newPRTaskResolveCmd(svc bitbucket.BitbucketService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var taskID int
	cmd := &cobra.Command{
		Use:   "resolve <pr-id>",
		Short: "Resolve a task on a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			id, err := parsePRID(args[0])
			if err != nil {
				fail(err)
			}
			if taskID <= 0 {
				fail(fmt.Errorf("--task must be a positive integer"))
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would resolve task #%d on PR #%d in %s/%s\n", taskID, id, ws, repo)
				return nil
			}
			err = svc.ResolvePRTask(context.Background(), ws, repo, id, taskID)
			auditLog.Log(audit.NewEntry("bb_resolve_pr_task", "bitbucket",
				map[string]any{"workspace": ws, "repo": repo, "pr_id": id, "task_id": taskID}, err))
			if err != nil {
				fail(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Task #%d on PR #%d resolved\n", taskID, id)
			return nil
		},
	}
	cmd.Flags().IntVar(&taskID, "task", 0, "Task ID to resolve (required)")
	_ = cmd.MarkFlagRequired("task")
	return cmd
}
