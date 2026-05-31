package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
)

// ToolListProjects returns an MCP tool handler that lists all Jira projects.
// Accepts optional max_results (default 50). Returns a JSON array of projects.
// All errors are returned as mcp.NewToolResultError — never as Go errors.
func ToolListProjects(svc projects.ProjectsService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		maxResults := int(mcp.ParseInt(req, "max_results", 0))
		if maxResults <= 0 {
			maxResults = 50
		}

		result, svcErr := svc.GetProjects(ctx, maxResults)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure never null in JSON output
		if result == nil {
			result = []projects.Project{}
		}

		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetProject returns an MCP tool handler that fetches a single Jira project by key.
// Requires project_key (string). Returns the project as JSON.
func ToolGetProject(svc projects.ProjectsService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectKey, err := req.RequireString("project_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project_key is required: %v", err)), nil
		}

		project, svcErr := svc.GetProject(ctx, projectKey)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(project)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolSearchProjects returns an MCP tool handler that searches Jira projects.
// Accepts optional query (string) and max_results (default 50).
// Returns a JSON object with projects array and pagination metadata.
func ToolSearchProjects(svc projects.ProjectsService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := mcp.ParseString(req, "query", "")
		maxResults := int(mcp.ParseInt(req, "max_results", 0))
		if maxResults <= 0 {
			maxResults = 50
		}

		searchReq := projects.SearchProjectsRequest{
			Query:      query,
			MaxResults: maxResults,
		}

		result, svcErr := svc.SearchProjects(ctx, searchReq)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolUpdateProject returns an MCP tool handler that updates a Jira project.
// Requires ENABLE_WRITE=true and project_key (string).
// Accepts optional name, description, and lead (accountId) as strings.
// Returns the updated project as JSON.
func ToolUpdateProject(svc projects.ProjectsService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		projectKey, err := req.RequireString("project_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project_key is required: %v", err)), nil
		}

		updateReq := projects.UpdateProjectRequest{}

		if s := mcp.ParseString(req, "name", ""); s != "" {
			updateReq.Name = &s
		}
		if s := mcp.ParseString(req, "description", ""); s != "" {
			updateReq.Description = &s
		}
		if s := mcp.ParseString(req, "lead", ""); s != "" {
			updateReq.Lead = &s
		}

		project, svcErr := svc.UpdateProject(ctx, projectKey, updateReq)
		log.Log(audit.NewEntry("update_project", "projects",
			map[string]any{"project_key": projectKey}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(project)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
