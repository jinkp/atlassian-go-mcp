package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
)

// releaseIssuesMaxResults caps how many issues are pulled for a release when
// validating or generating notes.
const releaseIssuesMaxResults = 200

// splitEnvList parses a comma-separated env var into a trimmed, non-empty slice.
// Returns nil when the variable is unset or empty so callers fall back to defaults.
func splitEnvList(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// fetchReleaseIssues resolves the issues linked to a release via the fixVersion JQL.
func fetchReleaseIssues(ctx context.Context, svc jira.Service, projectKey, releaseName string) ([]jira.Issue, error) {
	jql := fmt.Sprintf(`project = "%s" AND fixVersion = "%s"`, projectKey, releaseName)
	result, err := svc.SearchIssues(ctx, jql, releaseIssuesMaxResults)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return []jira.Issue{}, nil
	}
	return result.Issues, nil
}

// ToolValidateReleaseForDeploy returns an MCP tool handler that evaluates deploy-readiness
// rules against the issues linked to a release. Read-only (no ENABLE_WRITE required).
// Requires project_key and release_name. Optional rules (comma-separated) overrides the
// default rule set. Done statuses and critical labels are read from the environment.
func ToolValidateReleaseForDeploy(svc jira.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectKey, err := req.RequireString("project_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project_key is required: %v", err)), nil
		}
		releaseName, err := req.RequireString("release_name")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("release_name is required: %v", err)), nil
		}

		var ruleNames []string
		if raw := strings.TrimSpace(mcp.ParseString(req, "rules", "")); raw != "" {
			for _, r := range strings.Split(raw, ",") {
				if v := strings.TrimSpace(r); v != "" {
					ruleNames = append(ruleNames, v)
				}
			}
		}

		issues, svcErr := fetchReleaseIssues(ctx, svc, projectKey, releaseName)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		engine := releases.NewValidationEngine(
			splitEnvList("ATLASSIAN_DONE_STATUSES"),
			splitEnvList("ATLASSIAN_CRITICAL_LABELS"),
		)
		result := engine.Evaluate(issues, ruleNames)

		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGenerateReleaseNotes returns an MCP tool handler that produces Markdown release
// notes grouped by issue type for a release. Read-only (no ENABLE_WRITE required).
// Requires project_key and release_name. Returns raw Markdown text.
func ToolGenerateReleaseNotes(svc jira.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectKey, err := req.RequireString("project_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project_key is required: %v", err)), nil
		}
		releaseName, err := req.RequireString("release_name")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("release_name is required: %v", err)), nil
		}

		issues, svcErr := fetchReleaseIssues(ctx, svc, projectKey, releaseName)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		md := releases.GenerateNotes(issues, releaseName)
		return mcp.NewToolResultText(md), nil
	}
}
