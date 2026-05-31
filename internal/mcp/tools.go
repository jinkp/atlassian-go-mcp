// Package mcpserver wires jira.Service into MCP tool handlers.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// splitCSV splits a comma-separated string into trimmed, non-empty tokens.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// issueJSON is the snake_case JSON representation of a jira.Issue.
// Ensures empty labels serialize as [] not null.
type issueJSON struct {
	Key      string   `json:"key"`
	Summary  string   `json:"summary"`
	Status   string   `json:"status"`
	Assignee string   `json:"assignee"`
	Priority string   `json:"priority"`
	Labels   []string `json:"labels"`
	Created  string   `json:"created,omitempty"`
	Updated  string   `json:"updated,omitempty"`
}

func toIssueJSON(issue *jira.Issue) issueJSON {
	labels := issue.Labels
	if labels == nil {
		labels = []string{}
	}
	created := ""
	if !issue.Created.IsZero() {
		created = issue.Created.Format("2006-01-02T15:04:05Z07:00")
	}
	updated := ""
	if !issue.Updated.IsZero() {
		updated = issue.Updated.Format("2006-01-02T15:04:05Z07:00")
	}
	return issueJSON{
		Key:      issue.Key,
		Summary:  issue.Summary,
		Status:   issue.Status,
		Assignee: issue.Assignee,
		Priority: issue.Priority,
		Labels:   labels,
		Created:  created,
		Updated:  updated,
	}
}

// searchResultJSON is the JSON shape returned by search_jira_issues.
type searchResultJSON struct {
	Issues     []issueJSON `json:"issues"`
	Total      int         `json:"total"`
	StartAt    int         `json:"start_at"`
	MaxResults int         `json:"max_results"`
}

// ToolGetJiraIssue returns an MCP tool handler that wraps jira.Service.GetIssue.
// All errors are returned as mcp.NewToolResultError — never as Go errors.
func ToolGetJiraIssue(svc jira.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("issue_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("issue_key is required: %v", err)), nil
		}

		issue, err := svc.GetIssue(ctx, key)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		data, marshalErr := json.Marshal(toIssueJSON(issue))
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// transitionJSON is the snake_case JSON representation of a jira.Transition.
// Ensures the status_category field uses snake_case as per MCP contract.
type transitionJSON struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	StatusCategory string `json:"status_category"`
}

// ToolCreateJiraIssue returns an MCP tool handler that wraps jira.Service.CreateIssue.
// All errors are returned as mcp.NewToolResultError — never as Go errors.
func ToolCreateJiraIssue(svc jira.Service, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		projectKey, err := req.RequireString("project_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project_key is required: %v", err)), nil
		}
		issueType, err := req.RequireString("issue_type")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("issue_type is required: %v", err)), nil
		}
		summary, err := req.RequireString("summary")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("summary is required: %v", err)), nil
		}

		// Parse optional labels (comma-separated string → []string)
		var labels []string
		if rawLabels := mcp.ParseString(req, "labels", ""); rawLabels != "" {
			for _, l := range splitCSV(rawLabels) {
				if l != "" {
					labels = append(labels, l)
				}
			}
		}

		createReq := jira.CreateIssueRequest{
			ProjectKey:   projectKey,
			IssueType:    issueType,
			Summary:      summary,
			Description:  mcp.ParseString(req, "description", ""),
			AssigneeID:   mcp.ParseString(req, "assignee_id", ""),
			PriorityName: mcp.ParseString(req, "priority", ""),
			Labels:       labels,
		}

		resp, svcErr := svc.CreateIssue(ctx, createReq)
		log.Log(audit.NewEntry("create_jira_issue", "jira",
			map[string]any{"project_key": projectKey, "issue_type": issueType, "summary": summary}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(resp)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolUpdateJiraIssue returns an MCP tool handler that wraps jira.Service.UpdateIssue.
// All errors are returned as mcp.NewToolResultError — never as Go errors.
func ToolUpdateJiraIssue(svc jira.Service, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		issueKey, err := req.RequireString("issue_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("issue_key is required: %v", err)), nil
		}

		updateReq := jira.UpdateIssueRequest{}

		if s := mcp.ParseString(req, "summary", ""); s != "" {
			updateReq.Summary = &s
		}
		if d := mcp.ParseString(req, "description", ""); d != "" {
			updateReq.Description = &d
		}
		if a := mcp.ParseString(req, "assignee_id", ""); a != "" {
			updateReq.AssigneeID = &a
		}
		if p := mcp.ParseString(req, "priority", ""); p != "" {
			updateReq.PriorityName = &p
		}

		svcErr := svc.UpdateIssue(ctx, issueKey, updateReq)
		log.Log(audit.NewEntry("update_jira_issue", "jira",
			map[string]any{"issue_key": issueKey}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		return mcp.NewToolResultText("issue updated successfully"), nil
	}
}

// ToolGetJiraTransitions returns an MCP tool handler that wraps jira.Service.GetTransitions.
// Returns a JSON array of transitions in snake_case format.
func ToolGetJiraTransitions(svc jira.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		issueKey, err := req.RequireString("issue_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("issue_key is required: %v", err)), nil
		}

		transitions, svcErr := svc.GetTransitions(ctx, issueKey)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Convert to snake_case JSON — ensure empty slice serializes as [] not null
		out := make([]transitionJSON, len(transitions))
		for i, t := range transitions {
			out[i] = transitionJSON{
				ID:             t.ID,
				Name:           t.Name,
				StatusCategory: t.StatusCategory,
			}
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolTransitionJiraIssue returns an MCP tool handler that wraps jira.Service.TransitionIssue.
// All errors are returned as mcp.NewToolResultError — never as Go errors.
func ToolTransitionJiraIssue(svc jira.Service, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		issueKey, err := req.RequireString("issue_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("issue_key is required: %v", err)), nil
		}
		transitionID, err := req.RequireString("transition_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("transition_id is required: %v", err)), nil
		}

		svcErr := svc.TransitionIssue(ctx, issueKey, transitionID)
		log.Log(audit.NewEntry("transition_jira_issue", "jira",
			map[string]any{"issue_key": issueKey, "transition_id": transitionID}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		return mcp.NewToolResultText("issue transitioned successfully"), nil
	}
}

// ToolSearchJiraIssues returns an MCP tool handler that wraps jira.Service.SearchIssues.
// max_results defaults to 50 when not provided or zero.
func ToolSearchJiraIssues(svc jira.Service) server.ToolHandlerFunc {
	const defaultMaxResults = 50

	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jql, err := req.RequireString("jql")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("jql is required: %v", err)), nil
		}

		maxResults := int(mcp.ParseInt(req, "max_results", 0))
		if maxResults <= 0 {
			maxResults = defaultMaxResults
		}

		result, err := svc.SearchIssues(ctx, jql, maxResults)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		issues := make([]issueJSON, len(result.Issues))
		for i, iss := range result.Issues {
			issCopy := iss
			issues[i] = toIssueJSON(&issCopy)
		}

		out := searchResultJSON{
			Issues:     issues,
			Total:      result.Total,
			StartAt:    result.StartAt,
			MaxResults: result.MaxResults,
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
