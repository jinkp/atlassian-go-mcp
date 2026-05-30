package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

const agileDefaultMaxResults = 50

// ToolGetJiraBoards returns an MCP tool handler that lists Jira Software boards for a project.
// Requires project_key; accepts optional max_results (default 50).
// All errors are returned as mcp.NewToolResultError — never as Go errors.
func ToolGetJiraBoards(agileSvc agile.AgileService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectKey, err := req.RequireString("project_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project_key is required: %v", err)), nil
		}

		maxResults := int(mcp.ParseInt(req, "max_results", 0))
		if maxResults <= 0 {
			maxResults = agileDefaultMaxResults
		}

		boards, svcErr := agileSvc.GetBoards(ctx, projectKey, maxResults)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(boards)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetJiraSprints returns an MCP tool handler that lists sprints for a board.
// Requires board_id (number); accepts optional state (default "") and max_results (default 50).
// All errors are returned as mcp.NewToolResultError — never as Go errors.
func ToolGetJiraSprints(agileSvc agile.AgileService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boardID := int(mcp.ParseInt(req, "board_id", 0))
		if boardID == 0 {
			return mcp.NewToolResultError("board_id is required"), nil
		}

		state := mcp.ParseString(req, "state", "")

		maxResults := int(mcp.ParseInt(req, "max_results", 0))
		if maxResults <= 0 {
			maxResults = agileDefaultMaxResults
		}

		sprints, svcErr := agileSvc.GetSprints(ctx, boardID, state, maxResults)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(sprints)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetJiraActiveSprint returns an MCP tool handler that fetches the single active sprint
// for a board. Requires board_id (number). Calls GetSprints(ctx, boardID, "active", 1).
// Returns a tool error when no active sprint exists.
func ToolGetJiraActiveSprint(agileSvc agile.AgileService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boardID := int(mcp.ParseInt(req, "board_id", 0))
		if boardID == 0 {
			return mcp.NewToolResultError("board_id is required"), nil
		}

		sprints, svcErr := agileSvc.GetSprints(ctx, boardID, "active", 1)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		if len(sprints) == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("no active sprint found for board %d", boardID)), nil
		}

		data, marshalErr := json.Marshal(sprints[0])
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetJiraSprintIssues returns an MCP tool handler that fetches issues for a sprint.
// Requires sprint_id (number); accepts optional max_results (default 50).
// Returns JSON with issues array and total count. Empty sprint → {issues:[], total:0}.
// All errors are returned as mcp.NewToolResultError — never as Go errors.
func ToolGetJiraSprintIssues(agileSvc agile.AgileService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sprintID := int(mcp.ParseInt(req, "sprint_id", 0))
		if sprintID == 0 {
			return mcp.NewToolResultError("sprint_id is required"), nil
		}

		maxResults := int(mcp.ParseInt(req, "max_results", 0))
		if maxResults <= 0 {
			maxResults = agileDefaultMaxResults
		}

		result, svcErr := agileSvc.GetSprintIssues(ctx, sprintID, maxResults)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure Issues is never null in JSON output
		if result.Issues == nil {
			result.Issues = []agile.SprintIssue{}
		}

		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolUpdateSprint returns an MCP tool handler that updates a sprint's name, state, or dates.
// Requires sprint_id (number) and ENABLE_WRITE=true. Accepts optional state, name, start_date, end_date.
// Returns the updated sprint as JSON on success.
func ToolUpdateSprint(agileSvc agile.AgileService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sprintID := int(mcp.ParseInt(req, "sprint_id", 0))
		if sprintID == 0 {
			return mcp.NewToolResultError("sprint_id is required"), nil
		}

		updateReq := agile.UpdateSprintRequest{}
		if s := mcp.ParseString(req, "state", ""); s != "" {
			updateReq.State = &s
		}
		if s := mcp.ParseString(req, "name", ""); s != "" {
			updateReq.Name = &s
		}
		if s := mcp.ParseString(req, "start_date", ""); s != "" {
			updateReq.StartDate = &s
		}
		if s := mcp.ParseString(req, "end_date", ""); s != "" {
			updateReq.EndDate = &s
		}

		sprint, svcErr := agileSvc.UpdateSprint(ctx, sprintID, updateReq)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(sprint)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolMoveIssuesToSprint returns an MCP tool handler that moves issues into a sprint.
// Requires sprint_id (number) and issue_keys (comma-separated string). Requires ENABLE_WRITE=true.
func ToolMoveIssuesToSprint(agileSvc agile.AgileService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sprintID := int(mcp.ParseInt(req, "sprint_id", 0))
		if sprintID == 0 {
			return mcp.NewToolResultError("sprint_id is required"), nil
		}

		issueKeysRaw := mcp.ParseString(req, "issue_keys", "")
		if issueKeysRaw == "" {
			return mcp.NewToolResultError("issue_keys is required"), nil
		}
		issueKeys := splitCSV(issueKeysRaw)

		if svcErr := agileSvc.MoveIssuesToSprint(ctx, sprintID, issueKeys); svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return mcp.NewToolResultText("ok"), nil
	}
}

// ToolMoveIssuesToEpic returns an MCP tool handler that links issues to an epic.
// Requires epic_key (string) and issue_keys (comma-separated string). Requires ENABLE_WRITE=true.
func ToolMoveIssuesToEpic(agileSvc agile.AgileService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		epicKey := mcp.ParseString(req, "epic_key", "")
		if epicKey == "" {
			return mcp.NewToolResultError("epic_key is required"), nil
		}

		issueKeysRaw := mcp.ParseString(req, "issue_keys", "")
		if issueKeysRaw == "" {
			return mcp.NewToolResultError("issue_keys is required"), nil
		}
		issueKeys := splitCSV(issueKeysRaw)

		if svcErr := agileSvc.MoveIssuesToEpic(ctx, epicKey, issueKeys); svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return mcp.NewToolResultText("ok"), nil
	}
}

// ToolCreateSprint returns an MCP tool handler that creates a new sprint on a board.
// Requires name (string) and board_id (number). Accepts optional start_date and end_date.
// Requires ENABLE_WRITE=true. Returns JSON of the created Sprint.
func ToolCreateSprint(agileSvc agile.AgileService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		name := mcp.ParseString(req, "name", "")
		if name == "" {
			return mcp.NewToolResultError("name is required"), nil
		}

		boardID := int(mcp.ParseInt(req, "board_id", 0))
		if boardID == 0 {
			return mcp.NewToolResultError("board_id is required"), nil
		}

		createReq := agile.CreateSprintRequest{
			Name:      name,
			BoardID:   boardID,
			StartDate: mcp.ParseString(req, "start_date", ""),
			EndDate:   mcp.ParseString(req, "end_date", ""),
		}

		sprint, svcErr := agileSvc.CreateSprint(ctx, createReq)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(sprint)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetJiraEpics returns an MCP tool handler that lists epics for a project.
// Requires project_key (string); accepts optional max_results (default 50).
// Calls jira.Service.SearchIssues with JQL: project={key} AND issuetype=Epic ORDER BY created DESC.
// Returns the same searchResultJSON shape as search_jira_issues.
func ToolGetJiraEpics(jiraSvc jira.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectKey, err := req.RequireString("project_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project_key is required: %v", err)), nil
		}

		maxResults := int(mcp.ParseInt(req, "max_results", 0))
		if maxResults <= 0 {
			maxResults = agileDefaultMaxResults
		}

		jql := fmt.Sprintf("project=%s AND issuetype=Epic ORDER BY created DESC", projectKey)

		searchResult, svcErr := jiraSvc.SearchIssues(ctx, jql, maxResults)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		issues := make([]issueJSON, len(searchResult.Issues))
		for i, iss := range searchResult.Issues {
			issCopy := iss
			issues[i] = toIssueJSON(&issCopy)
		}

		out := searchResultJSON{
			Issues:     issues,
			Total:      searchResult.Total,
			StartAt:    searchResult.StartAt,
			MaxResults: searchResult.MaxResults,
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
