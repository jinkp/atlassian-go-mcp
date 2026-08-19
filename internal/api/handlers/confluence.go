package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/jinkp/atlassian-go-mcp/internal/api"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
)

// ConfluenceHandler handles all /confluence/* routes.
type ConfluenceHandler struct {
	svc      confluence.Service
	auditLog audit.Logger
}

// NewConfluenceHandler constructs a ConfluenceHandler.
func NewConfluenceHandler(svc confluence.Service, auditLog audit.Logger) *ConfluenceHandler {
	return &ConfluenceHandler{svc: svc, auditLog: auditLog}
}

// confluencePageJSON is the REST JSON shape for a Confluence page.
type confluencePageJSON struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	SpaceID       string `json:"space_id"`
	Status        string `json:"status"`
	VersionNumber int    `json:"version_number"`
	Body          string `json:"body,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	WebURL        string `json:"web_url,omitempty"`
}

// confluencePageRefJSON is the REST JSON shape for a lightweight page reference.
type confluencePageRefJSON struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

// confluenceSpaceJSON is the REST JSON shape for a Confluence space.
type confluenceSpaceJSON struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// confluenceCommentJSON is the REST JSON shape for a Confluence comment.
type confluenceCommentJSON struct {
	ID            string `json:"id"`
	Body          string `json:"body"`
	VersionNumber int    `json:"version_number"`
	CreatedAt     string `json:"created_at,omitempty"`
}

// confluenceSearchResultJSON is the REST JSON shape for a CQL search result.
type confluenceSearchResultJSON struct {
	ContentID string `json:"content_id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	SpaceKey  string `json:"space_key"`
	Excerpt   string `json:"excerpt"`
}

// confluenceListResponse wraps paginated list results with a next_cursor field.
type confluenceListResponse struct {
	Results    any    `json:"results"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// toPageJSON converts a domain Page to the REST JSON shape.
func toPageJSON(p confluence.Page) confluencePageJSON {
	out := confluencePageJSON{
		ID:            p.ID,
		Title:         p.Title,
		SpaceID:       p.SpaceID,
		Status:        p.Status,
		VersionNumber: p.VersionNumber,
		Body:          p.Body,
		WebURL:        p.WebURL,
	}
	if !p.CreatedAt.IsZero() {
		out.CreatedAt = p.CreatedAt.Format("2006-01-02T15:04:05Z")
	}
	return out
}

// toCommentJSON converts a domain Comment to the REST JSON shape.
func toCommentJSON(c confluence.Comment) confluenceCommentJSON {
	out := confluenceCommentJSON{
		ID:            c.ID,
		Body:          c.Body,
		VersionNumber: c.VersionNumber,
	}
	if !c.CreatedAt.IsZero() {
		out.CreatedAt = c.CreatedAt.Format("2006-01-02T15:04:05Z")
	}
	return out
}

// parseLimit extracts an integer ?limit= query param, defaulting to def when absent or invalid.
func parseLimit(r *http.Request, def int) int {
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// GetPage handles GET /confluence/pages/{pageId}.
// Query: body_format (optional, defaults to "storage").
func (h *ConfluenceHandler) GetPage(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")
	bodyFormat := r.URL.Query().Get("body_format")

	page, err := h.svc.GetPage(r.Context(), pageID, bodyFormat)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, toPageJSON(*page))
}

// GetPagesInSpace handles GET /confluence/spaces/{spaceId}/pages.
// Query: limit (optional), cursor (optional).
func (h *ConfluenceHandler) GetPagesInSpace(w http.ResponseWriter, r *http.Request) {
	spaceID := r.PathValue("spaceId")
	limit := parseLimit(r, 0)
	cursor := r.URL.Query().Get("cursor")

	list, err := h.svc.GetPagesInSpace(r.Context(), spaceID, limit, cursor)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}

	results := make([]confluencePageJSON, len(list.Results))
	for i, p := range list.Results {
		results[i] = toPageJSON(p)
	}
	api.RespondJSON(w, http.StatusOK, confluenceListResponse{Results: results, NextCursor: list.NextCursor})
}

// GetSpaces handles GET /confluence/spaces.
// Query: limit, cursor, keys (comma-separated), type.
func (h *ConfluenceHandler) GetSpaces(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 0)
	cursor := r.URL.Query().Get("cursor")
	spaceType := r.URL.Query().Get("type")

	var keys []string
	if k := r.URL.Query().Get("keys"); k != "" {
		for _, part := range strings.Split(k, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				keys = append(keys, part)
			}
		}
	}

	list, err := h.svc.GetSpaces(r.Context(), limit, cursor, keys, spaceType)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}

	results := make([]confluenceSpaceJSON, len(list.Results))
	for i, s := range list.Results {
		results[i] = confluenceSpaceJSON{
			ID:     s.ID,
			Key:    s.Key,
			Name:   s.Name,
			Type:   s.Type,
			Status: s.Status,
		}
	}
	api.RespondJSON(w, http.StatusOK, confluenceListResponse{Results: results, NextCursor: list.NextCursor})
}

// GetPageDescendants handles GET /confluence/pages/{pageId}/descendants.
// Query: limit, cursor.
func (h *ConfluenceHandler) GetPageDescendants(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")
	limit := parseLimit(r, 0)
	cursor := r.URL.Query().Get("cursor")

	list, err := h.svc.GetPageDescendants(r.Context(), pageID, limit, cursor)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}

	results := make([]confluencePageRefJSON, len(list.Results))
	for i, ref := range list.Results {
		results[i] = confluencePageRefJSON{
			ID:     ref.ID,
			Title:  ref.Title,
			Status: ref.Status,
			Type:   ref.Type,
		}
	}
	api.RespondJSON(w, http.StatusOK, confluenceListResponse{Results: results, NextCursor: list.NextCursor})
}

// GetFooterComments handles GET /confluence/pages/{pageId}/footer-comments.
// Query: limit, cursor.
func (h *ConfluenceHandler) GetFooterComments(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")
	limit := parseLimit(r, 0)
	cursor := r.URL.Query().Get("cursor")

	list, err := h.svc.GetFooterComments(r.Context(), pageID, limit, cursor)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}

	results := make([]confluenceCommentJSON, len(list.Results))
	for i, c := range list.Results {
		results[i] = toCommentJSON(c)
	}
	api.RespondJSON(w, http.StatusOK, confluenceListResponse{Results: results, NextCursor: list.NextCursor})
}

// GetInlineComments handles GET /confluence/pages/{pageId}/inline-comments.
// Query: limit, cursor.
func (h *ConfluenceHandler) GetInlineComments(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")
	limit := parseLimit(r, 0)
	cursor := r.URL.Query().Get("cursor")

	list, err := h.svc.GetInlineComments(r.Context(), pageID, limit, cursor)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}

	results := make([]confluenceCommentJSON, len(list.Results))
	for i, c := range list.Results {
		results[i] = toCommentJSON(c)
	}
	api.RespondJSON(w, http.StatusOK, confluenceListResponse{Results: results, NextCursor: list.NextCursor})
}

// GetCommentChildren handles GET /confluence/comments/{commentId}/children.
// Query: limit, cursor.
func (h *ConfluenceHandler) GetCommentChildren(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	limit := parseLimit(r, 0)
	cursor := r.URL.Query().Get("cursor")

	list, err := h.svc.GetCommentChildren(r.Context(), commentID, limit, cursor)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}

	results := make([]confluenceCommentJSON, len(list.Results))
	for i, c := range list.Results {
		results[i] = toCommentJSON(c)
	}
	api.RespondJSON(w, http.StatusOK, confluenceListResponse{Results: results, NextCursor: list.NextCursor})
}

// SearchContent handles GET /confluence/search.
// Query: cql (required), limit.
func (h *ConfluenceHandler) SearchContent(w http.ResponseWriter, r *http.Request) {
	cql := r.URL.Query().Get("cql")
	if cql == "" {
		api.RespondError(w, http.StatusBadRequest, "cql is required", api.ErrCodeBadRequest)
		return
	}
	limit := parseLimit(r, 0)

	items, err := h.svc.SearchContent(r.Context(), cql, limit)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}

	results := make([]confluenceSearchResultJSON, len(items))
	for i, item := range items {
		results[i] = confluenceSearchResultJSON{
			ContentID: item.ContentID,
			Title:     item.Title,
			Type:      item.Type,
			SpaceKey:  item.SpaceKey,
			Excerpt:   item.Excerpt,
		}
	}
	api.RespondJSON(w, http.StatusOK, confluenceListResponse{Results: results})
}

// --- Write handlers ---

// createPageBody is the JSON request body for CreatePage.
type createPageBody struct {
	SpaceID  string `json:"space_id"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	ParentID string `json:"parent_id"`
	Status   string `json:"status"`
}

// CreatePage handles POST /confluence/pages.
// Write-guarded globally by WriteGuardMiddleware.
func (h *ConfluenceHandler) CreatePage(w http.ResponseWriter, r *http.Request) {
	var body createPageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.SpaceID == "" {
		api.RespondError(w, http.StatusBadRequest, "space_id is required", api.ErrCodeBadRequest)
		return
	}
	if body.Title == "" {
		api.RespondError(w, http.StatusBadRequest, "title is required", api.ErrCodeBadRequest)
		return
	}

	req := confluence.CreatePageRequest{
		SpaceID:  body.SpaceID,
		Title:    body.Title,
		Body:     body.Body,
		ParentID: body.ParentID,
		Status:   body.Status,
	}

	page, err := h.svc.CreatePage(r.Context(), req)
	h.auditLog.Log(audit.NewEntry("create_page", "confluence", map[string]any{"space_id": body.SpaceID, "title": body.Title}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusCreated, toPageJSON(*page))
}

// updatePageBody is the JSON request body for UpdatePage.
type updatePageBody struct {
	Title         string `json:"title"`
	Body          string `json:"body"`
	Status        string `json:"status"`
	VersionNumber *int   `json:"version_number"`
}

// UpdatePage handles PUT /confluence/pages/{pageId}.
// Write-guarded globally by WriteGuardMiddleware.
func (h *ConfluenceHandler) UpdatePage(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")

	var body updatePageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}

	req := confluence.UpdatePageRequest{
		PageID:        pageID,
		Title:         body.Title,
		Body:          body.Body,
		Status:        body.Status,
		VersionNumber: body.VersionNumber,
	}

	page, err := h.svc.UpdatePage(r.Context(), req)
	h.auditLog.Log(audit.NewEntry("update_page", "confluence", map[string]any{"page_id": pageID}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, toPageJSON(*page))
}

// createFooterCommentBody is the JSON request body for CreateFooterComment.
type createFooterCommentBody struct {
	PageID          string `json:"page_id"`
	Body            string `json:"body"`
	ParentCommentID string `json:"parent_comment_id"`
}

// CreateFooterComment handles POST /confluence/footer-comments.
// Write-guarded globally by WriteGuardMiddleware.
func (h *ConfluenceHandler) CreateFooterComment(w http.ResponseWriter, r *http.Request) {
	var body createFooterCommentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.PageID == "" {
		api.RespondError(w, http.StatusBadRequest, "page_id is required", api.ErrCodeBadRequest)
		return
	}
	if body.Body == "" {
		api.RespondError(w, http.StatusBadRequest, "body is required", api.ErrCodeBadRequest)
		return
	}

	req := confluence.CreateCommentRequest{
		PageID:          body.PageID,
		Body:            body.Body,
		ParentCommentID: body.ParentCommentID,
	}

	comment, err := h.svc.CreateFooterComment(r.Context(), req)
	h.auditLog.Log(audit.NewEntry("create_footer_comment", "confluence", map[string]any{"page_id": body.PageID}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusCreated, toCommentJSON(*comment))
}

// createInlineCommentBody is the JSON request body for CreateInlineComment.
type createInlineCommentBody struct {
	PageID                  string `json:"page_id"`
	Body                    string `json:"body"`
	TextSelection           string `json:"text_selection"`
	TextSelectionMatchCount int    `json:"text_selection_match_count"`
	TextSelectionMatchIndex int    `json:"text_selection_match_index"`
}

// CreateInlineComment handles POST /confluence/inline-comments.
// text_selection is required — returns 400 before calling the service if missing.
// Write-guarded globally by WriteGuardMiddleware.
func (h *ConfluenceHandler) CreateInlineComment(w http.ResponseWriter, r *http.Request) {
	var body createInlineCommentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.TextSelection == "" {
		api.RespondError(w, http.StatusBadRequest, "text_selection is required", api.ErrCodeBadRequest)
		return
	}
	if body.PageID == "" {
		api.RespondError(w, http.StatusBadRequest, "page_id is required", api.ErrCodeBadRequest)
		return
	}

	req := confluence.CreateInlineCommentRequest{
		PageID:                  body.PageID,
		Body:                    body.Body,
		TextSelection:           body.TextSelection,
		TextSelectionMatchCount: body.TextSelectionMatchCount,
		TextSelectionMatchIndex: body.TextSelectionMatchIndex,
	}

	comment, err := h.svc.CreateInlineComment(r.Context(), req)
	h.auditLog.Log(audit.NewEntry("create_inline_comment", "confluence", map[string]any{"page_id": body.PageID, "text_selection": body.TextSelection}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusCreated, toCommentJSON(*comment))
}
