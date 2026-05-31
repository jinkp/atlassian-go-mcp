package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
)

// ToolEditGoal returns an MCP tool handler that edits structural fields of an Atlassian Goal.
// Required: WriteGuardCheck, goal_id. Optional: name, target_date, confidence, archive (bool).
// Returns the updated Goal as JSON.
func ToolEditGoal(goalsSvc goals.GoalsService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		goalID := mcp.ParseString(req, "goal_id", "")
		if goalID == "" {
			return mcp.NewToolResultError("goal_id is required"), nil
		}

		editReq := goals.EditGoalRequest{GoalID: goalID}

		if n := mcp.ParseString(req, "name", ""); n != "" {
			editReq.Name = &n
		}
		if td := mcp.ParseString(req, "target_date", ""); td != "" {
			editReq.TargetDate = &td
		}
		if c := mcp.ParseString(req, "confidence", ""); c != "" {
			editReq.Confidence = &c
		}
		// archive is a bool — only set if explicitly passed as true/false string
		if a := mcp.ParseString(req, "archive", ""); a != "" {
			b := a == "true"
			editReq.Archive = &b
		}

		result, svcErr := goalsSvc.EditGoal(ctx, editReq)
		log.Log(audit.NewEntry("edit_goal", "goals",
			map[string]any{"goal_id": goalID}, svcErr))
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

// ToolGetSiteID returns an MCP tool handler that resolves a subdomain to a cloudId.
// Required: subdomain (e.g. "myorg"). Returns JSON {"cloud_id": "..."}.
func ToolGetSiteID(goalsSvc goals.GoalsService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		subdomain := mcp.ParseString(req, "subdomain", "")
		if subdomain == "" {
			return mcp.NewToolResultError("subdomain is required"), nil
		}

		cloudID, svcErr := goalsSvc.GetSiteID(ctx, subdomain)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		out := map[string]string{"cloud_id": cloudID}
		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetGoal returns an MCP tool handler that fetches a single goal by ARI.
// Required: goal_id. Returns Goal JSON.
func ToolGetGoal(goalsSvc goals.GoalsService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		goalID := mcp.ParseString(req, "goal_id", "")
		if goalID == "" {
			return mcp.NewToolResultError("goal_id is required"), nil
		}

		goal, svcErr := goalsSvc.GetGoal(ctx, goalID)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(goal)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolSearchGoals returns an MCP tool handler that searches goals.
// Required: site_id. Optional: search_string, max_results, cursor.
// Returns GoalSearchResult JSON.
func ToolSearchGoals(goalsSvc goals.GoalsService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		siteID := mcp.ParseString(req, "site_id", "")
		if siteID == "" {
			return mcp.NewToolResultError("site_id is required"), nil
		}

		searchString := mcp.ParseString(req, "search_string", "")
		maxResults := int(mcp.ParseInt(req, "max_results", 0))
		cursor := mcp.ParseString(req, "cursor", "")

		searchReq := goals.SearchGoalsRequest{
			SiteID:       siteID,
			SearchString: searchString,
			MaxResults:   maxResults,
			Cursor:       cursor,
		}

		result, svcErr := goalsSvc.SearchGoals(ctx, searchReq)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure Goals is never null in JSON output
		if result.Goals == nil {
			result.Goals = []goals.Goal{}
		}

		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolCreateGoal returns an MCP tool handler that creates a new Atlassian Goal.
// Required: WriteGuardCheck, site_id, name, goal_type_id, target_date.
// Optional: confidence (default "QUARTER"), description.
// Returns JSON {"id":"...","name":"..."}.
func ToolCreateGoal(goalsSvc goals.GoalsService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		siteID := mcp.ParseString(req, "site_id", "")
		if siteID == "" {
			return mcp.NewToolResultError("site_id is required"), nil
		}

		name := mcp.ParseString(req, "name", "")
		if name == "" {
			return mcp.NewToolResultError("name is required"), nil
		}

		goalTypeID := mcp.ParseString(req, "goal_type_id", "")
		if goalTypeID == "" {
			return mcp.NewToolResultError("goal_type_id is required"), nil
		}

		targetDate := mcp.ParseString(req, "target_date", "")
		if targetDate == "" {
			return mcp.NewToolResultError("target_date is required"), nil
		}

		confidence := mcp.ParseString(req, "confidence", "QUARTER")
		description := mcp.ParseString(req, "description", "")

		createReq := goals.CreateGoalRequest{
			SiteID:      siteID,
			Name:        name,
			GoalTypeID:  goalTypeID,
			TargetDate:  targetDate,
			Confidence:  confidence,
			Description: description,
		}

		result, svcErr := goalsSvc.CreateGoal(ctx, createReq)
		log.Log(audit.NewEntry("create_goal", "goals",
			map[string]any{"site_id": siteID, "name": name}, svcErr))
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

// ToolUpdateGoalStatus returns an MCP tool handler that posts a goal status check-in.
// Required: WriteGuardCheck, goal_id, status. Optional: score, summary.
// Returns "ok" on success.
func ToolUpdateGoalStatus(goalsSvc goals.GoalsService, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		goalID := mcp.ParseString(req, "goal_id", "")
		if goalID == "" {
			return mcp.NewToolResultError("goal_id is required"), nil
		}

		status := mcp.ParseString(req, "status", "")
		if status == "" {
			return mcp.NewToolResultError("status is required"), nil
		}

		score := int(mcp.ParseInt(req, "score", 0))
		summary := mcp.ParseString(req, "summary", "")

		updateReq := goals.UpdateGoalStatusRequest{
			GoalID:  goalID,
			Status:  status,
			Score:   score,
			Summary: summary,
		}

		svcErr := goalsSvc.UpdateGoalStatus(ctx, updateReq)
		log.Log(audit.NewEntry("update_goal_status", "goals",
			map[string]any{"goal_id": goalID, "status": status}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}
		return mcp.NewToolResultText("ok"), nil
	}
}
