// Package mcpserver — jira extra tool handlers (Phase 2 expansion).
// Implements 7 new MCP tool handlers: 4 read + 3 write.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
)

// --- MCP output structs (snake_case, exactly as in design.md) ---

type userJSON struct {
	AccountID   string `json:"account_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Active      bool   `json:"active"`
}

type commentJSON struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created,omitempty"`
	Updated string `json:"updated,omitempty"`
}

type issueLinkTypeJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

type worklogJSON struct {
	ID               string `json:"id"`
	TimeSpentSeconds int    `json:"time_spent_seconds"`
	Started          string `json:"started,omitempty"`
	Author           string `json:"author"`
}

type issueTypeMetaJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Desc    string `json:"description"`
	Subtask bool   `json:"subtask"`
}

// toCommentJSON converts a domain Comment to commentJSON.
// Timestamps are formatted per toIssueJSON convention; zero times are omitted.
func toCommentJSON(c jira.Comment) commentJSON {
	const timeFmt = "2006-01-02T15:04:05Z07:00"
	created := ""
	if !c.Created.IsZero() {
		created = c.Created.Format(timeFmt)
	}
	updated := ""
	if !c.Updated.IsZero() {
		updated = c.Updated.Format(timeFmt)
	}
	return commentJSON{
		ID:      c.ID,
		Author:  c.Author,
		Body:    c.Body,
		Created: created,
		Updated: updated,
	}
}

// --- READ HANDLERS (4) ---

// ToolLookupJiraAccountID returns an MCP tool handler that searches Jira users
// by display name or email. READ — no WriteGuardCheck required.
// Input: query (required), max_results (optional int, default 10).
// Output: []userJSON — empty results serialize as [].
func ToolLookupJiraAccountID(svc jira.Service) server.ToolHandlerFunc {
	const defaultMax = 10
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query is required: %v", err)), nil
		}

		maxResults := int(mcp.ParseInt(req, "max_results", 0))
		if maxResults <= 0 {
			maxResults = defaultMax
		}

		users, svcErr := svc.LookupAccountID(ctx, query, maxResults)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure empty slice serializes as [] not null
		out := make([]userJSON, len(users))
		for i, u := range users {
			out[i] = userJSON{
				AccountID:   u.AccountID,
				DisplayName: u.DisplayName,
				Email:       u.Email,
				Active:      u.Active,
			}
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetIssueComments returns an MCP tool handler that retrieves comments for a
// Jira issue. READ — no WriteGuardCheck required.
// Input: issue_key (required), max_results (optional int).
// Output: []commentJSON — empty results serialize as [].
func ToolGetIssueComments(svc jira.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		issueKey, err := req.RequireString("issue_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("issue_key is required: %v", err)), nil
		}

		maxResults := int(mcp.ParseInt(req, "max_results", 0))

		comments, svcErr := svc.GetComments(ctx, issueKey, maxResults)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure empty slice serializes as [] not null
		out := make([]commentJSON, len(comments))
		for i, c := range comments {
			out[i] = toCommentJSON(c)
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetIssueLinkTypes returns an MCP tool handler that lists all available
// issue link types for this Jira instance. READ — no WriteGuardCheck required.
// Input: none.
// Output: []issueLinkTypeJSON — empty results serialize as [].
func ToolGetIssueLinkTypes(svc jira.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		linkTypes, svcErr := svc.GetIssueLinkTypes(ctx)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure empty slice serializes as [] not null
		out := make([]issueLinkTypeJSON, len(linkTypes))
		for i, lt := range linkTypes {
			out[i] = issueLinkTypeJSON{
				ID:      lt.ID,
				Name:    lt.Name,
				Inward:  lt.Inward,
				Outward: lt.Outward,
			}
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetIssueTypeMetadata returns an MCP tool handler that lists valid issue
// types for a given Jira project. READ — no WriteGuardCheck required.
// Input: project_key (required).
// Output: []issueTypeMetaJSON — empty results serialize as [].
func ToolGetIssueTypeMetadata(svc jira.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectKey, err := req.RequireString("project_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project_key is required: %v", err)), nil
		}

		issueTypes, svcErr := svc.GetIssueTypeMetadata(ctx, projectKey)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure empty slice serializes as [] not null
		out := make([]issueTypeMetaJSON, len(issueTypes))
		for i, it := range issueTypes {
			out[i] = issueTypeMetaJSON{
				ID:      it.ID,
				Name:    it.Name,
				Desc:    it.Desc,
				Subtask: it.Subtask,
			}
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- WRITE HANDLERS (3) ---

// ToolAddCommentToIssue returns an MCP tool handler that posts a comment on a
// Jira issue. WRITE — calls WriteGuardCheck() first, then audit logs.
// Input: issue_key (required), body (required).
// Output: commentJSON of the created comment.
func ToolAddCommentToIssue(svc jira.Service, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		issueKey, err := req.RequireString("issue_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("issue_key is required: %v", err)), nil
		}
		body, err := req.RequireString("body")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("body is required: %v", err)), nil
		}

		comment, svcErr := svc.AddComment(ctx, issueKey, body)
		log.Log(audit.NewEntry("add_comment_to_issue", "jira",
			map[string]any{"issue_key": issueKey}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(toCommentJSON(*comment))
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolLinkIssues returns an MCP tool handler that creates a link between two
// Jira issues. WRITE — calls WriteGuardCheck() first, then audit logs.
// Input: inward_issue (required), outward_issue (required), link_type (required).
// Output: plain text confirmation "issues linked successfully".
func ToolLinkIssues(svc jira.Service, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		inward, err := req.RequireString("inward_issue")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("inward_issue is required: %v", err)), nil
		}
		outward, err := req.RequireString("outward_issue")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("outward_issue is required: %v", err)), nil
		}
		linkType, err := req.RequireString("link_type")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("link_type is required: %v", err)), nil
		}

		svcErr := svc.LinkIssues(ctx, inward, outward, linkType)
		log.Log(audit.NewEntry("link_issues", "jira",
			map[string]any{"inward_issue": inward, "outward_issue": outward, "link_type": linkType}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		return mcp.NewToolResultText("issues linked successfully"), nil
	}
}

// ToolAddWorklog returns an MCP tool handler that logs time spent on a Jira
// issue. WRITE — calls WriteGuardCheck() first, then audit logs.
// Input: issue_key (required), time_spent (required), comment (optional), started (optional).
// Output: worklogJSON of the created worklog entry.
func ToolAddWorklog(svc jira.Service, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		issueKey, err := req.RequireString("issue_key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("issue_key is required: %v", err)), nil
		}
		timeSpent, err := req.RequireString("time_spent")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("time_spent is required: %v", err)), nil
		}

		worklogReq := jira.AddWorklogRequest{
			TimeSpent: timeSpent,
			Comment:   mcp.ParseString(req, "comment", ""),
			Started:   mcp.ParseString(req, "started", ""),
		}

		worklog, svcErr := svc.AddWorklog(ctx, issueKey, worklogReq)
		log.Log(audit.NewEntry("add_worklog", "jira",
			map[string]any{"issue_key": issueKey, "time_spent": timeSpent}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		const timeFmt = "2006-01-02T15:04:05Z07:00"
		started := ""
		if !worklog.Started.IsZero() {
			started = worklog.Started.Format(timeFmt)
		}
		out := worklogJSON{
			ID:               worklog.ID,
			TimeSpentSeconds: worklog.TimeSpentSeconds,
			Started:          started,
			Author:           worklog.Author,
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
