package bitbucket

import (
	"context"
	"fmt"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/spf13/cobra"
)

func newReposCmd(svc bitbucket.BitbucketService) *cobra.Command {
	var outputFormat string
	cmd := &cobra.Command{
		Use:   "repos",
		Short: "List repositories in a workspace",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, err := resolveWorkspace(cmd)
			if err != nil {
				fail(err)
			}
			repos, err := svc.ListRepositories(context.Background(), ws)
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, repos)
			return nil
		},
	}
	outputFlag(cmd, &outputFormat)
	return cmd
}

func newBranchesCmd(svc bitbucket.BitbucketService) *cobra.Command {
	var outputFormat string
	cmd := &cobra.Command{
		Use:   "branches",
		Short: "List branches in a repository",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			branches, err := svc.ListBranches(context.Background(), ws, repo)
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, branches)
			return nil
		},
	}
	outputFlag(cmd, &outputFormat)
	return cmd
}

func newStaleBranchesCmd(svc bitbucket.BitbucketService) *cobra.Command {
	var (
		outputFormat string
		days         int
	)
	cmd := &cobra.Command{
		Use:   "stale-branches",
		Short: "List branches with no commits in the last N days",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			branches, err := svc.StaleBranches(context.Background(), ws, repo, days)
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, branches)
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Number of days to consider stale")
	outputFlag(cmd, &outputFormat)
	return cmd
}

func newPipelineListCmd(svc bitbucket.BitbucketService) *cobra.Command {
	var outputFormat string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent pipelines for a repository",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			pipelines, err := svc.ListPipelines(context.Background(), ws, repo)
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, pipelines)
			return nil
		},
	}
	outputFlag(cmd, &outputFormat)
	return cmd
}

func newPRListCmd(svc bitbucket.BitbucketService) *cobra.Command {
	var (
		outputFormat string
		state        string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pull requests for a repository",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			prs, err := svc.ListPullRequests(context.Background(), ws, repo, state)
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, prs)
			return nil
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "PR state filter: OPEN, MERGED, DECLINED, SUPERSEDED (default OPEN)")
	outputFlag(cmd, &outputFormat)
	return cmd
}

// prReadIDCmd builds a read command that takes a single <pr-id> argument and
// invokes fn with the resolved workspace/repo and parsed ID.
func prReadIDCmd(svc bitbucket.BitbucketService, use, short string, fn func(ctx context.Context, ws, repo string, id int) (any, error)) *cobra.Command {
	var outputFormat string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
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
			result, err := fn(context.Background(), ws, repo, id)
			if err != nil {
				fail(err)
			}
			emit(cmd, outputFormat, result)
			return nil
		},
	}
	outputFlag(cmd, &outputFormat)
	return cmd
}

func newPRGetCmd(svc bitbucket.BitbucketService) *cobra.Command {
	return prReadIDCmd(svc, "get <pr-id>", "Get details of a pull request",
		func(ctx context.Context, ws, repo string, id int) (any, error) {
			return svc.GetPullRequest(ctx, ws, repo, id)
		})
}

func newPRCommentsCmd(svc bitbucket.BitbucketService) *cobra.Command {
	return prReadIDCmd(svc, "comments <pr-id>", "List comments on a pull request",
		func(ctx context.Context, ws, repo string, id int) (any, error) {
			return svc.ListPRComments(ctx, ws, repo, id)
		})
}

func newPRCommitsCmd(svc bitbucket.BitbucketService) *cobra.Command {
	return prReadIDCmd(svc, "commits <pr-id>", "List commits in a pull request",
		func(ctx context.Context, ws, repo string, id int) (any, error) {
			return svc.ListPRCommits(ctx, ws, repo, id)
		})
}

func newPRFilesCmd(svc bitbucket.BitbucketService) *cobra.Command {
	return prReadIDCmd(svc, "files <pr-id>", "List changed files (diffstat) in a pull request",
		func(ctx context.Context, ws, repo string, id int) (any, error) {
			return svc.ListPRFiles(ctx, ws, repo, id)
		})
}

func newPRChecksCmd(svc bitbucket.BitbucketService) *cobra.Command {
	return prReadIDCmd(svc, "checks <pr-id>", "List build/pipeline status checks for a pull request",
		func(ctx context.Context, ws, repo string, id int) (any, error) {
			return svc.ListPRChecks(ctx, ws, repo, id)
		})
}

func newPRReviewersCmd(svc bitbucket.BitbucketService) *cobra.Command {
	return prReadIDCmd(svc, "reviewers <pr-id>", "List reviewers of a pull request",
		func(ctx context.Context, ws, repo string, id int) (any, error) {
			return svc.ListPRReviewers(ctx, ws, repo, id)
		})
}

// newPRDiffCmd prints the raw unified diff (text, not table/json).
func newPRDiffCmd(svc bitbucket.BitbucketService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <pr-id>",
		Short: "Get the raw diff for a pull request",
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
			diff, err := svc.GetPRDiff(context.Background(), ws, repo, id)
			if err != nil {
				fail(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), diff)
			return nil
		},
	}
	return cmd
}
