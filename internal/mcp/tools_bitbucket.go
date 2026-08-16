package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
)

// resolveBitbucketWorkspace resolves the workspace from: tool param → BITBUCKET_WORKSPACE env.
// git-remote inference is intentionally NOT supported (an MCP server does not run inside a repo).
func resolveBitbucketWorkspace(req mcp.CallToolRequest) (string, error) {
	ws := strings.TrimSpace(mcp.ParseString(req, "workspace", ""))
	if ws == "" {
		ws = strings.TrimSpace(os.Getenv("BITBUCKET_WORKSPACE"))
	}
	if ws == "" {
		return "", fmt.Errorf("no workspace configured — pass the 'workspace' param or set BITBUCKET_WORKSPACE")
	}
	return ws, nil
}

// resolveBitbucketWorkspaceRepo resolves both workspace and repo from: tool params → env.
func resolveBitbucketWorkspaceRepo(req mcp.CallToolRequest) (workspace, repo string, err error) {
	workspace, err = resolveBitbucketWorkspace(req)
	if err != nil {
		return "", "", err
	}
	repo = strings.TrimSpace(mcp.ParseString(req, "repo", ""))
	if repo == "" {
		repo = strings.TrimSpace(os.Getenv("BITBUCKET_REPO"))
	}
	if repo == "" {
		return "", "", fmt.Errorf("no repository configured — pass the 'repo' param or set BITBUCKET_REPO")
	}
	return workspace, repo, nil
}

// bitbucketJSON marshals v to a JSON tool result, or an error result on failure.
func bitbucketJSON(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// requirePRID reads and validates the pr_id argument.
func requirePRID(req mcp.CallToolRequest) (int, *mcp.CallToolResult) {
	prID := int(mcp.ParseInt(req, "pr_id", 0))
	if prID <= 0 {
		return 0, mcp.NewToolResultError("pr_id must be a positive integer")
	}
	return prID, nil
}

// ToolBitbucketListRepos lists repositories in a workspace.
func ToolBitbucketListRepos(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, err := resolveBitbucketWorkspace(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		repos, svcErr := svc.ListRepositories(ctx, ws)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(repos)
	}
}

// ToolBitbucketListPRs lists pull requests for a repository, optionally filtered by state.
func ToolBitbucketListPRs(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		state := mcp.ParseString(req, "state", "")
		prs, svcErr := svc.ListPullRequests(ctx, ws, repo, state)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(prs)
	}
}

// ToolBitbucketGetPR fetches a single pull request by ID.
func ToolBitbucketGetPR(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}
		pr, svcErr := svc.GetPullRequest(ctx, ws, repo, prID)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(pr)
	}
}

// ToolBitbucketPRComments lists comments on a pull request.
func ToolBitbucketPRComments(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}
		comments, svcErr := svc.ListPRComments(ctx, ws, repo, prID)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(comments)
	}
}

// ToolBitbucketPRCommits lists commits in a pull request.
func ToolBitbucketPRCommits(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}
		commits, svcErr := svc.ListPRCommits(ctx, ws, repo, prID)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(commits)
	}
}

// ToolBitbucketPRFiles lists changed files (diffstat) in a pull request.
func ToolBitbucketPRFiles(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}
		files, svcErr := svc.ListPRFiles(ctx, ws, repo, prID)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(files)
	}
}

// ToolBitbucketPRDiff returns the raw diff for a pull request.
func ToolBitbucketPRDiff(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}
		diff, svcErr := svc.GetPRDiff(ctx, ws, repo, prID)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return mcp.NewToolResultText(diff), nil
	}
}

// ToolBitbucketPRChecks lists build/pipeline status checks for a pull request.
func ToolBitbucketPRChecks(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}
		checks, svcErr := svc.ListPRChecks(ctx, ws, repo, prID)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(checks)
	}
}

// ToolBitbucketPRReviewers lists reviewers of a pull request.
func ToolBitbucketPRReviewers(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}
		reviewers, svcErr := svc.ListPRReviewers(ctx, ws, repo, prID)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(reviewers)
	}
}

// ToolBitbucketListBranches lists branches in a repository.
func ToolBitbucketListBranches(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		branches, svcErr := svc.ListBranches(ctx, ws, repo)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(branches)
	}
}

// ToolBitbucketStaleBranches lists branches with no commits in the last N days (default 30).
func ToolBitbucketStaleBranches(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		days := int(mcp.ParseInt(req, "days", 30))
		branches, svcErr := svc.StaleBranches(ctx, ws, repo, days)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(branches)
	}
}

// ToolBitbucketListPipelines lists the most recent pipelines for a repository.
func ToolBitbucketListPipelines(svc bitbucket.BitbucketService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		pipelines, svcErr := svc.ListPipelines(ctx, ws, repo)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(pipelines)
	}
}
