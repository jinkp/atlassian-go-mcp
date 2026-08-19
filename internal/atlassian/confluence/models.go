// Package confluence provides a client SDK for the Confluence Cloud REST API.
// v2 (/wiki/api/v2/) is used for CRUD operations; v1 (/wiki/rest/api/search)
// is used for CQL search. Authentication reuses the same BasicAuth credentials
// as the Jira package — no new environment variables are required.
package confluence

import (
	"context"
	"errors"
	"html/template"
	"net/url"
	"time"
)

// confluenceTimeLayout is the time format returned by the Confluence v2 API.
const confluenceTimeLayout = "2006-01-02T15:04:05.000Z"

// Sentinel errors returned by ConfluenceService. Callers can use errors.Is to
// branch on specific error types without string matching.
var (
	// ErrNotFound is returned when a requested resource does not exist (HTTP 404).
	ErrNotFound = errors.New("confluence: not found")

	// ErrUnauthorized is returned on 401/403 — bad credentials or insufficient permissions.
	ErrUnauthorized = errors.New("confluence: unauthorized")

	// ErrRateLimit is returned on 429 after all retries are exhausted.
	ErrRateLimit = errors.New("confluence: rate limited")

	// ErrConflict is returned on 409 — version conflict on page update.
	ErrConflict = errors.New("confluence: version conflict")
)

// --- Domain types ---

// Page is the canonical domain model for a Confluence page.
type Page struct {
	ID            string
	Title         string
	SpaceID       string
	Status        string
	VersionNumber int
	Body          string // storage/XHTML value; pass-through
	CreatedAt     time.Time
	WebURL        string
}

// PageRef is a lightweight reference to a page, used in lists and descendant calls.
type PageRef struct {
	ID     string
	Title  string
	Status string
	Type   string
}

// Space is the canonical domain model for a Confluence space.
type Space struct {
	ID     string
	Key    string
	Name   string
	Type   string
	Status string
}

// Comment is the canonical domain model for a Confluence comment (footer or inline).
type Comment struct {
	ID            string
	Body          string
	VersionNumber int
	CreatedAt     time.Time
}

// SearchResult represents a single item from a CQL search response.
type SearchResult struct {
	ContentID string
	Title     string
	Type      string
	SpaceKey  string
	Excerpt   string
}

// --- Paginated list types ---

// PageList holds a page of Page results plus a cursor for the next page.
type PageList struct {
	Results    []Page
	NextCursor string
}

// PageRefList holds a page of PageRef results plus a cursor for the next page.
type PageRefList struct {
	Results    []PageRef
	NextCursor string
}

// SpaceList holds a page of Space results plus a cursor for the next page.
type SpaceList struct {
	Results    []Space
	NextCursor string
}

// CommentList holds a page of Comment results plus a cursor for the next page.
type CommentList struct {
	Results    []Comment
	NextCursor string
}

// --- Request types ---

// CreatePageRequest contains the parameters for creating a new Confluence page.
type CreatePageRequest struct {
	SpaceID  string
	Title    string
	Body     string // storage/XHTML; pass-through verbatim
	ParentID string // optional
	Status   string // default "current"
}

// UpdatePageRequest contains the parameters for updating an existing Confluence page.
type UpdatePageRequest struct {
	PageID        string
	Title         string
	Body          string
	Status        string
	VersionNumber *int // nil = fetch current version internally and increment
}

// CreateCommentRequest contains the parameters for creating a footer comment.
type CreateCommentRequest struct {
	PageID          string
	Body            string
	ParentCommentID string // optional; for threaded replies
}

// CreateInlineCommentRequest contains the parameters for creating an inline comment.
// TextSelection is required; the MCP/handler layer enforces this before calling the service.
type CreateInlineCommentRequest struct {
	PageID                  string
	Body                    string
	TextSelection           string // required
	TextSelectionMatchCount int    // optional
	TextSelectionMatchIndex int    // optional
}

// --- Service interface ---

// Service defines all Confluence operations available across MCP, CLI, and REST surfaces.
type Service interface {
	// Read (7)
	GetPage(ctx context.Context, pageID string, bodyFormat string) (*Page, error)
	GetPagesInSpace(ctx context.Context, spaceID string, limit int, cursor string) (*PageList, error)
	GetSpaces(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*SpaceList, error)
	GetPageDescendants(ctx context.Context, pageID string, limit int, cursor string) (*PageRefList, error)
	GetFooterComments(ctx context.Context, pageID string, limit int, cursor string) (*CommentList, error)
	GetInlineComments(ctx context.Context, pageID string, limit int, cursor string) (*CommentList, error)
	GetCommentChildren(ctx context.Context, commentID string, limit int, cursor string) (*CommentList, error)
	// Write (4)
	CreatePage(ctx context.Context, req CreatePageRequest) (*Page, error)
	UpdatePage(ctx context.Context, req UpdatePageRequest) (*Page, error)
	CreateFooterComment(ctx context.Context, req CreateCommentRequest) (*Comment, error)
	CreateInlineComment(ctx context.Context, req CreateInlineCommentRequest) (*Comment, error)
	// Search (1)
	SearchContent(ctx context.Context, cql string, limit int) ([]SearchResult, error)
}

// --- Helpers ---

// plainTextToStorage wraps plain text in minimal Confluence storage XHTML.
// Use this when the caller wants to post plain text without hand-writing XHTML.
// The text is HTML-escaped to prevent injection. For raw XHTML, pass it verbatim
// without calling this helper.
func plainTextToStorage(text string) string {
	return "<p>" + string(template.HTMLEscapeString(text)) + "</p>"
}

// extractCursor parses the `cursor=` query param from a Confluence _links.next URL.
// Returns "" when no next page exists or the URL is malformed.
func extractCursor(next string) string {
	if next == "" {
		return ""
	}
	u, err := url.Parse(next)
	if err != nil {
		return ""
	}
	return u.Query().Get("cursor")
}

// --- Raw API response structs (unexported) ---

// confLinks is the _links envelope present on list responses.
type confLinks struct {
	Next string `json:"next"`
}

// pageAPIResponse maps the raw JSON from GET /wiki/api/v2/pages/{id}.
type pageAPIResponse struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	SpaceID string `json:"spaceId"`
	Status  string `json:"status"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
	CreatedAt string `json:"createdAt"`
}

// ToPage converts the raw API response into the domain Page model.
func (r pageAPIResponse) ToPage() Page {
	p := Page{
		ID:            r.ID,
		Title:         r.Title,
		SpaceID:       r.SpaceID,
		Status:        r.Status,
		VersionNumber: r.Version.Number,
		Body:          r.Body.Storage.Value,
		WebURL:        r.Links.WebUI,
	}
	if t, err := time.Parse(confluenceTimeLayout, r.CreatedAt); err == nil {
		p.CreatedAt = t.UTC()
	}
	return p
}

// pageListAPIResponse wraps a list of pages plus _links for cursor extraction.
type pageListAPIResponse struct {
	Results []pageAPIResponse `json:"results"`
	Links   confLinks         `json:"_links"`
}

// pageRefAPIResponse maps a lightweight page reference in list/descendants responses.
type pageRefAPIResponse struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

// ToPageRef converts the raw API response into the domain PageRef model.
func (r pageRefAPIResponse) ToPageRef() PageRef {
	return PageRef{
		ID:     r.ID,
		Title:  r.Title,
		Status: r.Status,
		Type:   r.Type,
	}
}

// pageRefListAPIResponse wraps a list of page refs plus _links.
type pageRefListAPIResponse struct {
	Results []pageRefAPIResponse `json:"results"`
	Links   confLinks            `json:"_links"`
}

// spaceAPIResponse maps the raw JSON for a single space.
type spaceAPIResponse struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// ToSpace converts the raw API response into the domain Space model.
func (r spaceAPIResponse) ToSpace() Space {
	return Space{
		ID:     r.ID,
		Key:    r.Key,
		Name:   r.Name,
		Type:   r.Type,
		Status: r.Status,
	}
}

// spaceListAPIResponse wraps a list of spaces plus _links.
type spaceListAPIResponse struct {
	Results []spaceAPIResponse `json:"results"`
	Links   confLinks          `json:"_links"`
}

// commentAPIResponse maps the raw JSON for a single comment (footer or inline).
type commentAPIResponse struct {
	ID      string `json:"id"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	CreatedAt string `json:"createdAt"`
}

// ToComment converts the raw API response into the domain Comment model.
func (r commentAPIResponse) ToComment() Comment {
	c := Comment{
		ID:            r.ID,
		Body:          r.Body.Storage.Value,
		VersionNumber: r.Version.Number,
	}
	if t, err := time.Parse(confluenceTimeLayout, r.CreatedAt); err == nil {
		c.CreatedAt = t.UTC()
	}
	return c
}

// commentListAPIResponse wraps a list of comments plus _links.
type commentListAPIResponse struct {
	Results []commentAPIResponse `json:"results"`
	Links   confLinks            `json:"_links"`
}

// searchItemAPIResponse maps a single item from the CQL search response.
type searchItemAPIResponse struct {
	Content struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"content"`
	Title   string `json:"title"`
	Excerpt string `json:"excerpt"`
	Space   struct {
		Key string `json:"key"`
	} `json:"space"`
}

// ToSearchResult converts the raw API response into the domain SearchResult model.
func (r searchItemAPIResponse) ToSearchResult() SearchResult {
	return SearchResult{
		ContentID: r.Content.ID,
		Title:     r.Title,
		Type:      r.Content.Type,
		SpaceKey:  r.Space.Key,
		Excerpt:   r.Excerpt,
	}
}

// searchAPIResponse wraps a list of search results.
type searchAPIResponse struct {
	Results []searchItemAPIResponse `json:"results"`
}
