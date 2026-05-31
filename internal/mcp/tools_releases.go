package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
)

// ToolSearchReleases returns an MCP tool handler that lists all Jira releases for a project.
// Requires project_key (string). Returns a JSON array of releases.
// All errors are returned as mcp.NewToolResultError — never as Go errors.
func ToolSearchReleases(svc releases.ReleasesService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectKey, err := req.RequireString("project_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project_key is required: %v", err)), nil
		}

		result, svcErr := svc.GetReleases(ctx, projectKey)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure never null in JSON output
		if result == nil {
			result = []releases.Release{}
		}

		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetRelease returns an MCP tool handler that fetches a single Jira release by ID.
// Requires release_id (string). Returns JSON of the release.
func ToolGetRelease(svc releases.ReleasesService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		releaseID, err := req.RequireString("release_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("release_id is required: %v", err)), nil
		}

		release, svcErr := svc.GetRelease(ctx, releaseID)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(release)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetReleaseIssues returns an MCP tool handler that fetches issue counts for a release.
// Requires release_id (string). Returns JSON with fix_version and affects_version counts.
func ToolGetReleaseIssues(svc releases.ReleasesService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		releaseID, err := req.RequireString("release_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("release_id is required: %v", err)), nil
		}

		counts, svcErr := svc.GetReleaseIssueCounts(ctx, releaseID)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(counts)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolCreateRelease returns an MCP tool handler that creates a new Jira release.
// Requires ENABLE_WRITE=true, project_id (string), and name (string).
// Accepts optional description, start_date, and release_date.
// Returns the created release as JSON.
func ToolCreateRelease(svc releases.ReleasesService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		projectID, err := req.RequireString("project_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project_id is required: %v", err)), nil
		}

		name, err := req.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("name is required: %v", err)), nil
		}

		createReq := releases.CreateReleaseRequest{
			ProjectID:   projectID,
			Name:        name,
			Description: mcp.ParseString(req, "description", ""),
			StartDate:   mcp.ParseString(req, "start_date", ""),
			ReleaseDate: mcp.ParseString(req, "release_date", ""),
		}

		release, svcErr := svc.CreateRelease(ctx, createReq)
		log.Log(audit.NewEntry("create_release", "releases",
			map[string]any{"project_id": projectID, "name": name}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(release)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolUpdateRelease returns an MCP tool handler that updates a Jira release.
// Requires ENABLE_WRITE=true and release_id (string).
// Accepts optional name, description, release_date (strings) and released, archived ("true"/"false").
// Returns the updated release as JSON.
func ToolUpdateRelease(svc releases.ReleasesService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		releaseID, err := req.RequireString("release_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("release_id is required: %v", err)), nil
		}

		updateReq := releases.UpdateReleaseRequest{}

		if s := mcp.ParseString(req, "name", ""); s != "" {
			updateReq.Name = &s
		}
		if s := mcp.ParseString(req, "description", ""); s != "" {
			updateReq.Description = &s
		}
		if s := mcp.ParseString(req, "release_date", ""); s != "" {
			updateReq.ReleaseDate = &s
		}
		if s := mcp.ParseString(req, "released", ""); s != "" {
			b := s == "true"
			updateReq.Released = &b
		}
		if s := mcp.ParseString(req, "archived", ""); s != "" {
			b := s == "true"
			updateReq.Archived = &b
		}

		release, svcErr := svc.UpdateRelease(ctx, releaseID, updateReq)
		log.Log(audit.NewEntry("update_release", "releases",
			map[string]any{"release_id": releaseID}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(release)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
