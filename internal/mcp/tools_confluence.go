// Package mcpserver — Confluence tool handlers (Block 2).
// Implements 12 MCP tool handlers: 8 read + 4 write.
// Pattern mirrors tools_jira_extra.go exactly.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	confluence "github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
)

// --- MCP output structs (snake_case, all in this file) ---

type mcpPageJSON struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	SpaceID       string `json:"space_id"`
	Status        string `json:"status"`
	VersionNumber int    `json:"version_number"`
	Body          string `json:"body,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	WebURL        string `json:"web_url,omitempty"`
}

type mcpPageRefJSON struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

type mcpSpaceJSON struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type mcpCommentJSON struct {
	ID            string `json:"id"`
	Body          string `json:"body"`
	VersionNumber int    `json:"version_number"`
	CreatedAt     string `json:"created_at,omitempty"`
}

type mcpSearchResultJSON struct {
	ContentID string `json:"content_id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	SpaceKey  string `json:"space_key"`
	Excerpt   string `json:"excerpt"`
}

type mcpPageListJSON struct {
	Results    []mcpPageRefJSON `json:"results"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type mcpSpaceListJSON struct {
	Results    []mcpSpaceJSON `json:"results"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

type mcpCommentListJSON struct {
	Results    []mcpCommentJSON `json:"results"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

// confluenceTimeFmt is the timestamp format for MCP output, matching existing tools.
const confluenceTimeFmt = "2006-01-02T15:04:05Z07:00"

// toMCPPageJSON converts a domain Page to the MCP output struct.
func toMCPPageJSON(p confluence.Page) mcpPageJSON {
	out := mcpPageJSON{
		ID:            p.ID,
		Title:         p.Title,
		SpaceID:       p.SpaceID,
		Status:        p.Status,
		VersionNumber: p.VersionNumber,
		Body:          p.Body,
		WebURL:        p.WebURL,
	}
	if !p.CreatedAt.IsZero() {
		out.CreatedAt = p.CreatedAt.Format(confluenceTimeFmt)
	}
	return out
}

// toMCPCommentJSON converts a domain Comment to the MCP output struct.
func toMCPCommentJSON(c confluence.Comment) mcpCommentJSON {
	out := mcpCommentJSON{
		ID:            c.ID,
		Body:          c.Body,
		VersionNumber: c.VersionNumber,
	}
	if !c.CreatedAt.IsZero() {
		out.CreatedAt = c.CreatedAt.Format(confluenceTimeFmt)
	}
	return out
}

// --- READ HANDLERS (8) ---

// ToolGetConfluencePage returns an MCP tool handler that fetches a Confluence page by ID.
// READ — no WriteGuardCheck required.
// Input: page_id (required), body_format (optional, default "storage").
// Output: mcpPageJSON.
func ToolGetConfluencePage(svc confluence.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pageID, err := req.RequireString("page_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("page_id is required: %v", err)), nil
		}

		bodyFormat := mcp.ParseString(req, "body_format", "storage")

		page, svcErr := svc.GetPage(ctx, pageID, bodyFormat)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(toMCPPageJSON(*page))
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetPagesInSpace returns an MCP tool handler that lists pages in a Confluence space.
// READ — no WriteGuardCheck required.
// Input: space_id (required), limit (optional), cursor (optional).
// Output: mcpPageListJSON — empty results serialize as [].
func ToolGetPagesInSpace(svc confluence.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		spaceID, err := req.RequireString("space_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("space_id is required: %v", err)), nil
		}

		limit := int(mcp.ParseInt(req, "limit", 0))
		cursor := mcp.ParseString(req, "cursor", "")

		result, svcErr := svc.GetPagesInSpace(ctx, spaceID, limit, cursor)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure empty slice serializes as [] not null.
		// GetPagesInSpace returns []Page (full pages); we project to the slim list shape.
		refs := make([]mcpPageRefJSON, len(result.Results))
		for i, p := range result.Results {
			refs[i] = mcpPageRefJSON{
				ID:     p.ID,
				Title:  p.Title,
				Status: p.Status,
				Type:   "", // Page (not PageRef) has no Type field
			}
		}

		out := mcpPageListJSON{
			Results:    refs,
			NextCursor: result.NextCursor,
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetConfluenceSpaces returns an MCP tool handler that lists Confluence spaces.
// READ — no WriteGuardCheck required.
// Input: limit/cursor/keys/type (all optional).
// Output: mcpSpaceListJSON — empty results serialize as [].
func ToolGetConfluenceSpaces(svc confluence.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := int(mcp.ParseInt(req, "limit", 0))
		cursor := mcp.ParseString(req, "cursor", "")
		keysStr := mcp.ParseString(req, "keys", "")
		spaceType := mcp.ParseString(req, "type", "")

		// Parse keys as comma-separated list
		var keys []string
		if keysStr != "" {
			keys = splitCSV(keysStr)
		}

		result, svcErr := svc.GetSpaces(ctx, limit, cursor, keys, spaceType)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure empty slice serializes as [] not null
		spaces := make([]mcpSpaceJSON, len(result.Results))
		for i, s := range result.Results {
			spaces[i] = mcpSpaceJSON{
				ID:     s.ID,
				Key:    s.Key,
				Name:   s.Name,
				Type:   s.Type,
				Status: s.Status,
			}
		}

		out := mcpSpaceListJSON{
			Results:    spaces,
			NextCursor: result.NextCursor,
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetPageDescendants returns an MCP tool handler that lists descendant pages of a page.
// READ — no WriteGuardCheck required.
// Input: page_id (required), limit/cursor (optional).
// Output: mcpPageListJSON — empty results serialize as [].
func ToolGetPageDescendants(svc confluence.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pageID, err := req.RequireString("page_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("page_id is required: %v", err)), nil
		}

		limit := int(mcp.ParseInt(req, "limit", 0))
		cursor := mcp.ParseString(req, "cursor", "")

		result, svcErr := svc.GetPageDescendants(ctx, pageID, limit, cursor)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure empty slice serializes as [] not null
		refs := make([]mcpPageRefJSON, len(result.Results))
		for i, p := range result.Results {
			refs[i] = mcpPageRefJSON{
				ID:     p.ID,
				Title:  p.Title,
				Status: p.Status,
				Type:   p.Type,
			}
		}

		out := mcpPageListJSON{
			Results:    refs,
			NextCursor: result.NextCursor,
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetPageFooterComments returns an MCP tool handler that lists footer comments on a page.
// READ — no WriteGuardCheck required.
// Input: page_id (required), limit/cursor (optional).
// Output: mcpCommentListJSON — empty results serialize as [].
func ToolGetPageFooterComments(svc confluence.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pageID, err := req.RequireString("page_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("page_id is required: %v", err)), nil
		}

		limit := int(mcp.ParseInt(req, "limit", 0))
		cursor := mcp.ParseString(req, "cursor", "")

		result, svcErr := svc.GetFooterComments(ctx, pageID, limit, cursor)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure empty slice serializes as [] not null
		comments := make([]mcpCommentJSON, len(result.Results))
		for i, c := range result.Results {
			comments[i] = toMCPCommentJSON(c)
		}

		out := mcpCommentListJSON{
			Results:    comments,
			NextCursor: result.NextCursor,
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetPageInlineComments returns an MCP tool handler that lists inline comments on a page.
// READ — no WriteGuardCheck required.
// Input: page_id (required), limit/cursor (optional).
// Output: mcpCommentListJSON — empty results serialize as [].
func ToolGetPageInlineComments(svc confluence.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pageID, err := req.RequireString("page_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("page_id is required: %v", err)), nil
		}

		limit := int(mcp.ParseInt(req, "limit", 0))
		cursor := mcp.ParseString(req, "cursor", "")

		result, svcErr := svc.GetInlineComments(ctx, pageID, limit, cursor)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure empty slice serializes as [] not null
		comments := make([]mcpCommentJSON, len(result.Results))
		for i, c := range result.Results {
			comments[i] = toMCPCommentJSON(c)
		}

		out := mcpCommentListJSON{
			Results:    comments,
			NextCursor: result.NextCursor,
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolGetCommentChildren returns an MCP tool handler that lists child comments of a comment.
// READ — no WriteGuardCheck required.
// Input: comment_id (required), limit/cursor (optional).
// Output: mcpCommentListJSON — empty results serialize as [].
func ToolGetCommentChildren(svc confluence.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		commentID, err := req.RequireString("comment_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("comment_id is required: %v", err)), nil
		}

		limit := int(mcp.ParseInt(req, "limit", 0))
		cursor := mcp.ParseString(req, "cursor", "")

		result, svcErr := svc.GetCommentChildren(ctx, commentID, limit, cursor)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure empty slice serializes as [] not null
		comments := make([]mcpCommentJSON, len(result.Results))
		for i, c := range result.Results {
			comments[i] = toMCPCommentJSON(c)
		}

		out := mcpCommentListJSON{
			Results:    comments,
			NextCursor: result.NextCursor,
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolSearchConfluence returns an MCP tool handler that searches Confluence with CQL.
// READ — no WriteGuardCheck required.
// Input: cql (required), limit (optional).
// Output: []mcpSearchResultJSON — empty results serialize as [].
func ToolSearchConfluence(svc confluence.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cql, err := req.RequireString("cql")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("cql is required: %v", err)), nil
		}

		limit := int(mcp.ParseInt(req, "limit", 0))

		results, svcErr := svc.SearchContent(ctx, cql, limit)
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		// Ensure empty slice serializes as [] not null
		out := make([]mcpSearchResultJSON, len(results))
		for i, r := range results {
			out[i] = mcpSearchResultJSON{
				ContentID: r.ContentID,
				Title:     r.Title,
				Type:      r.Type,
				SpaceKey:  r.SpaceKey,
				Excerpt:   r.Excerpt,
			}
		}

		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- WRITE HANDLERS (4) ---

// ToolCreateConfluencePage returns an MCP tool handler that creates a new Confluence page.
// WRITE — calls WriteGuardCheck() first, then audit logs.
// Input: space_id (required), title (required), body (required), parent_id (optional), status (optional).
// Output: mcpPageJSON of the created page.
func ToolCreateConfluencePage(svc confluence.Service, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		spaceID, err := req.RequireString("space_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("space_id is required: %v", err)), nil
		}
		title, err := req.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("title is required: %v", err)), nil
		}
		body, err := req.RequireString("body")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("body is required: %v", err)), nil
		}

		parentID := mcp.ParseString(req, "parent_id", "")
		status := mcp.ParseString(req, "status", "current")

		createReq := confluence.CreatePageRequest{
			SpaceID:  spaceID,
			Title:    title,
			Body:     body,
			ParentID: parentID,
			Status:   status,
		}

		page, svcErr := svc.CreatePage(ctx, createReq)
		log.Log(audit.NewEntry("create_confluence_page", "confluence",
			map[string]any{"space_id": spaceID, "title": title}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(toMCPPageJSON(*page))
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolUpdateConfluencePage returns an MCP tool handler that updates an existing Confluence page.
// WRITE — calls WriteGuardCheck() first, then audit logs.
// Input: page_id (required), title (required), body (required), status (optional), version_number (optional int).
// Output: mcpPageJSON of the updated page.
func ToolUpdateConfluencePage(svc confluence.Service, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pageID, err := req.RequireString("page_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("page_id is required: %v", err)), nil
		}
		title, err := req.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("title is required: %v", err)), nil
		}
		body, err := req.RequireString("body")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("body is required: %v", err)), nil
		}

		status := mcp.ParseString(req, "status", "current")

		// version_number is optional; 0 means "not supplied → service auto-increments"
		versionRaw := int(mcp.ParseInt(req, "version_number", 0))
		var versionPtr *int
		if versionRaw > 0 {
			v := versionRaw
			versionPtr = &v
		}

		updateReq := confluence.UpdatePageRequest{
			PageID:        pageID,
			Title:         title,
			Body:          body,
			Status:        status,
			VersionNumber: versionPtr,
		}

		page, svcErr := svc.UpdatePage(ctx, updateReq)
		log.Log(audit.NewEntry("update_confluence_page", "confluence",
			map[string]any{"page_id": pageID, "title": title}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(toMCPPageJSON(*page))
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolCreateFooterComment returns an MCP tool handler that creates a footer comment on a Confluence page.
// WRITE — calls WriteGuardCheck() first, then audit logs.
// Input: page_id (required), body (required), parent_comment_id (optional).
// Output: mcpCommentJSON of the created comment.
func ToolCreateFooterComment(svc confluence.Service, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pageID, err := req.RequireString("page_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("page_id is required: %v", err)), nil
		}
		body, err := req.RequireString("body")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("body is required: %v", err)), nil
		}

		parentCommentID := mcp.ParseString(req, "parent_comment_id", "")

		commentReq := confluence.CreateCommentRequest{
			PageID:          pageID,
			Body:            body,
			ParentCommentID: parentCommentID,
		}

		comment, svcErr := svc.CreateFooterComment(ctx, commentReq)
		log.Log(audit.NewEntry("create_footer_comment", "confluence",
			map[string]any{"page_id": pageID}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(toMCPCommentJSON(*comment))
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// ToolCreateInlineComment returns an MCP tool handler that creates an inline comment anchored
// to a text selection on a Confluence page.
// WRITE — calls WriteGuardCheck() first, then validates text_selection (REQUIRED per spec),
// then calls service, then audit logs.
// Input: page_id (required), body (required), text_selection (required — validated before service call),
// text_selection_match_count (optional), text_selection_match_index (optional).
// Output: mcpCommentJSON of the created comment.
func ToolCreateInlineComment(svc confluence.Service, log audit.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := WriteGuardCheck(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pageID, err := req.RequireString("page_id")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("page_id is required: %v", err)), nil
		}
		body, err := req.RequireString("body")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("body is required: %v", err)), nil
		}

		// text_selection is required for inline comments — validate BEFORE any service call.
		// Per spec: "Inline comments MUST have an anchor; the text_selection field is REQUIRED."
		textSelection, err := req.RequireString("text_selection")
		if err != nil || textSelection == "" {
			return mcp.NewToolResultError("text_selection is required for inline comments: the comment must be anchored to selected text"), nil
		}

		matchCount := int(mcp.ParseInt(req, "text_selection_match_count", 0))
		matchIndex := int(mcp.ParseInt(req, "text_selection_match_index", 0))

		inlineReq := confluence.CreateInlineCommentRequest{
			PageID:                  pageID,
			Body:                    body,
			TextSelection:           textSelection,
			TextSelectionMatchCount: matchCount,
			TextSelectionMatchIndex: matchIndex,
		}

		comment, svcErr := svc.CreateInlineComment(ctx, inlineReq)
		log.Log(audit.NewEntry("create_inline_comment", "confluence",
			map[string]any{"page_id": pageID, "text_selection": textSelection}, svcErr))
		if svcErr != nil {
			return mcp.NewToolResultError(svcErr.Error()), nil
		}

		data, marshalErr := json.Marshal(toMCPCommentJSON(*comment))
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
