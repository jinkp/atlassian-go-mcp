package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
)

// ToolSearchTeams returns an MCP tool handler that lists or searches Atlassian teams.
// Accepts optional query (string) and max_results (default 50).
// Returns a JSON object with teams array and optional next_cursor.
// All errors are returned as mcp.NewToolResultError — never as Go errors.
func ToolSearchTeams(svc teams.TeamsService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := mcp.ParseString(req, "query", "")
		maxResults := int(mcp.ParseInt(req, "max_results", 0))
		if maxResults <= 0 {
			maxResults = 50
		}

		result, err := svc.GetTeams(ctx, query, maxResults)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Ensure teams is never null in JSON output
		if result.Teams == nil {
			result.Teams = []teams.Team{}
		}

		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetTeam returns an MCP tool handler that fetches a single Atlassian team by ID.
// Requires team_id (string). Returns the team as JSON.
func ToolGetTeam(svc teams.TeamsService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		teamID, err := req.RequireString("team_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("team_id is required: %v", err)), nil
		}

		team, svcErr := svc.GetTeam(ctx, teamID)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(team)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetTeamMembers returns an MCP tool handler that lists members of an Atlassian team.
// Requires team_id (string). Accepts optional max_results (default 50).
// Returns a JSON array of team members.
func ToolGetTeamMembers(svc teams.TeamsService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		teamID, err := req.RequireString("team_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("team_id is required: %v", err)), nil
		}

		maxResults := int(mcp.ParseInt(req, "max_results", 0))
		if maxResults <= 0 {
			maxResults = 50
		}

		members, svcErr := svc.GetTeamMembers(ctx, teamID, maxResults)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure never null in JSON output
		if members == nil {
			members = []teams.TeamMember{}
		}

		data, marshalErr := json.Marshal(members)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
