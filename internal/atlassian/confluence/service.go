package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
)

const defaultLimit = 25

// ConfluenceService implements Service against the Confluence Cloud REST API.
// v2 endpoints (/wiki/api/v2/) are used for CRUD; v1 (/wiki/rest/api/search)
// is used for CQL search. The same baseURL (site root) is used for both.
type ConfluenceService struct {
	doer    client.HTTPDoer
	baseURL string
}

// NewService constructs a ConfluenceService. The doer is typically a *http.Client
// from httptest in tests, or a *client.Client in production.
func NewService(doer client.HTTPDoer, baseURL string) *ConfluenceService {
	return &ConfluenceService{
		doer:    doer,
		baseURL: baseURL,
	}
}

// wikiV2 returns the base URL for Confluence v2 API endpoints.
func (s *ConfluenceService) wikiV2() string {
	return s.baseURL + "/wiki/api/v2"
}

// mapError converts an HTTP response status to a sentinel error.
// It does NOT close the body — the caller is responsible.
func mapError(resp *http.Response, prefix string) error {
	switch resp.StatusCode {
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	case http.StatusTooManyRequests:
		return ErrRateLimit
	case http.StatusConflict:
		return ErrConflict
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: unexpected status %d: %s", prefix, resp.StatusCode, string(body))
	}
}

// resolveLimit returns limit if > 0, otherwise returns defaultLimit.
func resolveLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	return limit
}

// --- Read methods ---

// GetPage fetches a single Confluence page by ID.
// bodyFormat controls the body representation (default "storage" if empty).
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *ConfluenceService) GetPage(ctx context.Context, pageID string, bodyFormat string) (*Page, error) {
	if bodyFormat == "" {
		bodyFormat = "storage"
	}

	params := url.Values{}
	params.Set("body-format", bodyFormat)
	endpoint := s.wikiV2() + "/pages/" + url.PathEscape(pageID) + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("confluence: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, mapError(resp, "confluence: GetPage")
	}

	var raw pageAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("confluence: decoding page response: %w", err)
	}
	page := raw.ToPage()
	return &page, nil
}

// GetPagesInSpace returns a page of pages in a given space.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *ConfluenceService) GetPagesInSpace(ctx context.Context, spaceID string, limit int, cursor string) (*PageList, error) {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(resolveLimit(limit)))
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	endpoint := s.wikiV2() + "/spaces/" + url.PathEscape(spaceID) + "/pages?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("confluence: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, mapError(resp, "confluence: GetPagesInSpace")
	}

	var raw pageListAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("confluence: decoding pages-in-space response: %w", err)
	}

	results := make([]Page, len(raw.Results))
	for i, r := range raw.Results {
		results[i] = r.ToPage()
	}
	return &PageList{Results: results, NextCursor: extractCursor(raw.Links.Next)}, nil
}

// GetSpaces returns a page of spaces, optionally filtered by keys and/or type.
// Returns ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *ConfluenceService) GetSpaces(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*SpaceList, error) {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(resolveLimit(limit)))
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	for _, k := range keys {
		params.Add("keys", k)
	}
	if spaceType != "" {
		params.Set("type", spaceType)
	}
	endpoint := s.wikiV2() + "/spaces?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("confluence: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, mapError(resp, "confluence: GetSpaces")
	}

	var raw spaceListAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("confluence: decoding spaces response: %w", err)
	}

	results := make([]Space, len(raw.Results))
	for i, r := range raw.Results {
		results[i] = r.ToSpace()
	}
	return &SpaceList{Results: results, NextCursor: extractCursor(raw.Links.Next)}, nil
}

// GetPageDescendants returns a page of descendant page references.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *ConfluenceService) GetPageDescendants(ctx context.Context, pageID string, limit int, cursor string) (*PageRefList, error) {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(resolveLimit(limit)))
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	endpoint := s.wikiV2() + "/pages/" + url.PathEscape(pageID) + "/descendants?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("confluence: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, mapError(resp, "confluence: GetPageDescendants")
	}

	var raw pageRefListAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("confluence: decoding descendants response: %w", err)
	}

	results := make([]PageRef, len(raw.Results))
	for i, r := range raw.Results {
		results[i] = r.ToPageRef()
	}
	return &PageRefList{Results: results, NextCursor: extractCursor(raw.Links.Next)}, nil
}

// GetFooterComments returns a page of footer comments on a page.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *ConfluenceService) GetFooterComments(ctx context.Context, pageID string, limit int, cursor string) (*CommentList, error) {
	return s.getCommentList(ctx, "/pages/"+url.PathEscape(pageID)+"/footer-comments", limit, cursor, "GetFooterComments")
}

// GetInlineComments returns a page of inline comments on a page.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *ConfluenceService) GetInlineComments(ctx context.Context, pageID string, limit int, cursor string) (*CommentList, error) {
	return s.getCommentList(ctx, "/pages/"+url.PathEscape(pageID)+"/inline-comments", limit, cursor, "GetInlineComments")
}

// GetCommentChildren returns a page of child comments (replies) for a footer comment.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *ConfluenceService) GetCommentChildren(ctx context.Context, commentID string, limit int, cursor string) (*CommentList, error) {
	return s.getCommentList(ctx, "/footer-comments/"+url.PathEscape(commentID)+"/children", limit, cursor, "GetCommentChildren")
}

// getCommentList is the shared implementation for all comment-list endpoints.
func (s *ConfluenceService) getCommentList(ctx context.Context, path string, limit int, cursor string, opName string) (*CommentList, error) {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(resolveLimit(limit)))
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	endpoint := s.wikiV2() + path + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("confluence: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, mapError(resp, "confluence: "+opName)
	}

	var raw commentListAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("confluence: decoding %s response: %w", opName, err)
	}

	results := make([]Comment, len(raw.Results))
	for i, r := range raw.Results {
		results[i] = r.ToComment()
	}
	return &CommentList{Results: results, NextCursor: extractCursor(raw.Links.Next)}, nil
}

// --- Write methods ---

// CreatePage creates a new Confluence page.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *ConfluenceService) CreatePage(ctx context.Context, r CreatePageRequest) (*Page, error) {
	status := r.Status
	if status == "" {
		status = "current"
	}

	apiBody := map[string]interface{}{
		"spaceId": r.SpaceID,
		"status":  status,
		"title":   r.Title,
		"body": map[string]interface{}{
			"representation": "storage",
			"value":          r.Body,
		},
	}
	if r.ParentID != "" {
		apiBody["parentId"] = r.ParentID
	}

	encoded, err := json.Marshal(apiBody)
	if err != nil {
		return nil, fmt.Errorf("confluence: marshaling create page request: %w", err)
	}

	endpoint := s.wikiV2() + "/pages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("confluence: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("confluence: create page 400: %s", string(body))
		}
		return nil, mapError(resp, "confluence: CreatePage")
	}

	var raw pageAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("confluence: decoding create page response: %w", err)
	}
	page := raw.ToPage()
	return &page, nil
}

// UpdatePage updates an existing Confluence page.
// Decision 2: when req.VersionNumber is nil, an internal GET is performed first to
// read the current version; the PUT is then sent with current+1. When provided,
// the version is used as-is. A 409 from the API maps to ErrConflict.
func (s *ConfluenceService) UpdatePage(ctx context.Context, r UpdatePageRequest) (*Page, error) {
	version := 0
	if r.VersionNumber == nil {
		// Fetch current version to compute next.
		current, err := s.GetPage(ctx, r.PageID, "")
		if err != nil {
			return nil, fmt.Errorf("confluence: UpdatePage version fetch: %w", err)
		}
		version = current.VersionNumber + 1
	} else {
		version = *r.VersionNumber
	}

	status := r.Status
	if status == "" {
		status = "current"
	}

	apiBody := map[string]interface{}{
		"id":     r.PageID,
		"status": status,
		"title":  r.Title,
		"body": map[string]interface{}{
			"representation": "storage",
			"value":          r.Body,
		},
		"version": map[string]interface{}{
			"number": version,
		},
	}

	encoded, err := json.Marshal(apiBody)
	if err != nil {
		return nil, fmt.Errorf("confluence: marshaling update page request: %w", err)
	}

	endpoint := s.wikiV2() + "/pages/" + url.PathEscape(r.PageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("confluence: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("confluence: update page 400: %s", string(body))
		}
		return nil, mapError(resp, "confluence: UpdatePage")
	}

	var raw pageAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("confluence: decoding update page response: %w", err)
	}
	page := raw.ToPage()
	return &page, nil
}

// CreateFooterComment creates a footer comment on a page, optionally as a reply.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *ConfluenceService) CreateFooterComment(ctx context.Context, r CreateCommentRequest) (*Comment, error) {
	apiBody := map[string]interface{}{
		"pageId": r.PageID,
		"body": map[string]interface{}{
			"representation": "storage",
			"value":          r.Body,
		},
	}
	if r.ParentCommentID != "" {
		apiBody["parentCommentId"] = r.ParentCommentID
	}

	encoded, err := json.Marshal(apiBody)
	if err != nil {
		return nil, fmt.Errorf("confluence: marshaling create footer comment request: %w", err)
	}

	endpoint := s.wikiV2() + "/footer-comments"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("confluence: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("confluence: create footer comment 400: %s", string(body))
		}
		return nil, mapError(resp, "confluence: CreateFooterComment")
	}

	var raw commentAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("confluence: decoding create footer comment response: %w", err)
	}
	comment := raw.ToComment()
	return &comment, nil
}

// CreateInlineComment creates an inline comment anchored to a text selection.
// The TextSelection in the request is required; primary validation is expected
// at the MCP/handler layer. The service forwards the request as-is.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *ConfluenceService) CreateInlineComment(ctx context.Context, r CreateInlineCommentRequest) (*Comment, error) {
	inlineProps := map[string]interface{}{
		"textSelection": r.TextSelection,
	}
	if r.TextSelectionMatchCount > 0 {
		inlineProps["textSelectionMatchCount"] = r.TextSelectionMatchCount
	}
	if r.TextSelectionMatchIndex > 0 {
		inlineProps["textSelectionMatchIndex"] = r.TextSelectionMatchIndex
	}

	apiBody := map[string]interface{}{
		"pageId": r.PageID,
		"body": map[string]interface{}{
			"representation": "storage",
			"value":          r.Body,
		},
		"inlineCommentProperties": inlineProps,
	}

	encoded, err := json.Marshal(apiBody)
	if err != nil {
		return nil, fmt.Errorf("confluence: marshaling create inline comment request: %w", err)
	}

	endpoint := s.wikiV2() + "/inline-comments"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("confluence: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("confluence: create inline comment 400: %s", string(body))
		}
		return nil, mapError(resp, "confluence: CreateInlineComment")
	}

	var raw commentAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("confluence: decoding create inline comment response: %w", err)
	}
	comment := raw.ToComment()
	return &comment, nil
}

// --- Search method ---

// SearchContent searches Confluence content using CQL (Confluence Query Language).
// Uses the v1 search API: GET /wiki/rest/api/search?cql=&limit=
// Returns ErrUnauthorized on 401/403, ErrRateLimit on 429.
// Returns a non-nil empty slice when no results are found.
func (s *ConfluenceService) SearchContent(ctx context.Context, cql string, limit int) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("cql", cql)
	params.Set("limit", strconv.Itoa(resolveLimit(limit)))
	endpoint := s.baseURL + "/wiki/rest/api/search?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("confluence: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("confluence: search 400: %s", string(body))
		}
		return nil, mapError(resp, "confluence: SearchContent")
	}

	var raw searchAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("confluence: decoding search response: %w", err)
	}

	results := make([]SearchResult, len(raw.Results))
	for i, r := range raw.Results {
		results[i] = r.ToSearchResult()
	}
	return results, nil
}
