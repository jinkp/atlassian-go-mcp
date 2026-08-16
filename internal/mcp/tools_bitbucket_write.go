package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
)

// splitCSVOrNil splits a comma-separated string into trimmed, non-empty values.
// Returns nil when the input is empty or whitespace-only, so callers can treat
// "field not provided" (nil) distinctly from "replace with empty list".
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

// ToolBitbucketCreatePR creates a pull request. Requires ENABLE_WRITE=true.
func ToolBitbucketCreatePR(svc bitbucket.BitbucketService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		title := strings.TrimSpace(mcp.ParseString(req, "title", ""))
		source := strings.TrimSpace(mcp.ParseString(req, "source", ""))
		destination := strings.TrimSpace(mcp.ParseString(req, "destination", ""))
		if title == "" {
			return mcp.NewToolResultError("title is required"), nil
		}
		if source == "" {
			return mcp.NewToolResultError("source is required"), nil
		}
		if destination == "" {
			return mcp.NewToolResultError("destination is required"), nil
		}

		createReq := bitbucket.CreatePRRequest{
			Title:             title,
			SourceBranch:      source,
			DestinationBranch: destination,
			Description:       mcp.ParseString(req, "description", ""),
			Reviewers:         splitCSVOrNil(mcp.ParseString(req, "reviewers", "")),
			CloseSourceBranch: mcp.ParseBoolean(req, "close_source_branch", false),
		}

		pr, svcErr := svc.CreatePullRequest(ctx, ws, repo, createReq)
		log.Log(audit.NewEntry("bb_create_pr", "bitbucket",
			map[string]any{"workspace": ws, "repo": repo, "source": source, "destination": destination}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(pr)
	}
}

// ToolBitbucketCommentPR adds a comment to a pull request. Requires ENABLE_WRITE=true.
func ToolBitbucketCommentPR(svc bitbucket.BitbucketService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}
		message := strings.TrimSpace(mcp.ParseString(req, "message", ""))
		if message == "" {
			return mcp.NewToolResultError("message is required"), nil
		}

		comment, svcErr := svc.AddPRComment(ctx, ws, repo, prID, message)
		log.Log(audit.NewEntry("bb_comment_pr", "bitbucket",
			map[string]any{"workspace": ws, "repo": repo, "pr_id": prID}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(comment)
	}
}

// ToolBitbucketUpdatePR updates a pull request. Requires ENABLE_WRITE=true.
func ToolBitbucketUpdatePR(svc bitbucket.BitbucketService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}

		title := mcp.ParseString(req, "title", "")
		description := mcp.ParseString(req, "description", "")
		destination := mcp.ParseString(req, "destination", "")
		reviewersRaw := mcp.ParseString(req, "reviewers", "")
		if strings.TrimSpace(title) == "" && strings.TrimSpace(description) == "" &&
			strings.TrimSpace(destination) == "" && strings.TrimSpace(reviewersRaw) == "" {
			return mcp.NewToolResultError("bb_update_pr requires at least one of: title, description, destination, reviewers"), nil
		}

		updateReq := bitbucket.UpdatePRRequest{
			Title:             title,
			Description:       description,
			DestinationBranch: destination,
			Reviewers:         splitCSVOrNil(reviewersRaw), // nil when empty
		}

		pr, svcErr := svc.UpdatePullRequest(ctx, ws, repo, prID, updateReq)
		log.Log(audit.NewEntry("bb_update_pr", "bitbucket",
			map[string]any{"workspace": ws, "repo": repo, "pr_id": prID}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(pr)
	}
}

// ToolBitbucketApprovePR approves a pull request. Requires ENABLE_WRITE=true.
func ToolBitbucketApprovePR(svc bitbucket.BitbucketService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}

		svcErr := svc.ApprovePullRequest(ctx, ws, repo, prID)
		log.Log(audit.NewEntry("bb_approve_pr", "bitbucket",
			map[string]any{"workspace": ws, "repo": repo, "pr_id": prID}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("PR #%d approved", prID)), nil
	}
}

// ToolBitbucketDeclinePR declines (rejects) a pull request. Requires ENABLE_WRITE=true.
func ToolBitbucketDeclinePR(svc bitbucket.BitbucketService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}

		svcErr := svc.DeclinePullRequest(ctx, ws, repo, prID)
		log.Log(audit.NewEntry("bb_decline_pr", "bitbucket",
			map[string]any{"workspace": ws, "repo": repo, "pr_id": prID}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("PR #%d declined", prID)), nil
	}
}

// ToolBitbucketMergePR merges a pull request. Requires ENABLE_WRITE=true.
// Optional strategy and message customize the merge.
func ToolBitbucketMergePR(svc bitbucket.BitbucketService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}
		strategy := mcp.ParseString(req, "strategy", "")
		message := mcp.ParseString(req, "message", "")

		pr, svcErr := svc.MergePullRequest(ctx, ws, repo, prID, strategy, message)
		log.Log(audit.NewEntry("bb_merge_pr", "bitbucket",
			map[string]any{"workspace": ws, "repo": repo, "pr_id": prID, "strategy": strategy}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(pr)
	}
}

// ToolBitbucketRunPipeline triggers a pipeline for a branch. Requires ENABLE_WRITE=true.
func ToolBitbucketRunPipeline(svc bitbucket.BitbucketService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		branch := strings.TrimSpace(mcp.ParseString(req, "branch", ""))
		if branch == "" {
			return mcp.NewToolResultError("branch is required"), nil
		}

		pipeline, svcErr := svc.RunPipeline(ctx, ws, repo, branch)
		log.Log(audit.NewEntry("bb_run_pipeline", "bitbucket",
			map[string]any{"workspace": ws, "repo": repo, "branch": branch}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(pipeline)
	}
}

// ToolBitbucketCreatePRTask creates a task on a pull request. Requires ENABLE_WRITE=true.
func ToolBitbucketCreatePRTask(svc bitbucket.BitbucketService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}
		message := strings.TrimSpace(mcp.ParseString(req, "message", ""))
		if message == "" {
			return mcp.NewToolResultError("message is required"), nil
		}

		task, svcErr := svc.CreatePRTask(ctx, ws, repo, prID, message)
		log.Log(audit.NewEntry("bb_create_pr_task", "bitbucket",
			map[string]any{"workspace": ws, "repo": repo, "pr_id": prID}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return bitbucketJSON(task)
	}
}

// ToolBitbucketResolvePRTask resolves a task on a pull request. Requires ENABLE_WRITE=true.
func ToolBitbucketResolvePRTask(svc bitbucket.BitbucketService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		ws, repo, err := resolveBitbucketWorkspaceRepo(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prID, errResult := requirePRID(req)
		if errResult != nil {
			return errResult, nil
		}
		taskID := int(mcp.ParseInt(req, "task_id", 0))
		if taskID <= 0 {
			return mcp.NewToolResultError("task_id must be a positive integer"), nil
		}

		svcErr := svc.ResolvePRTask(ctx, ws, repo, prID, taskID)
		log.Log(audit.NewEntry("bb_resolve_pr_task", "bitbucket",
			map[string]any{"workspace": ws, "repo": repo, "pr_id": prID, "task_id": taskID}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Task #%d on PR #%d resolved", taskID, prID)), nil
	}
}
