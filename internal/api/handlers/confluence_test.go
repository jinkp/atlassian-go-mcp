package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
)

// mockConfluenceService implements confluence.Service for testing.
// All func fields default to nil; nil-safe defaults return zero values.
type mockConfluenceService struct {
	getPageFunc             func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error)
	getPagesInSpaceFunc     func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error)
	getSpacesFunc           func(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluence.SpaceList, error)
	getPageDescendantsFunc  func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.PageRefList, error)
	getFooterCommentsFunc   func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error)
	getInlineCommentsFunc   func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error)
	getCommentChildrenFunc  func(ctx context.Context, commentID string, limit int, cursor string) (*confluence.CommentList, error)
	createPageFunc          func(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error)
	updatePageFunc          func(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error)
	createFooterCommentFunc func(ctx context.Context, req confluence.CreateCommentRequest) (*confluence.Comment, error)
	createInlineCommentFunc func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error)
	searchContentFunc       func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error)
}

func (m *mockConfluenceService) GetPage(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error) {
	if m.getPageFunc != nil {
		return m.getPageFunc(ctx, pageID, bodyFormat)
	}
	return nil, nil
}

func (m *mockConfluenceService) GetPagesInSpace(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error) {
	if m.getPagesInSpaceFunc != nil {
		return m.getPagesInSpaceFunc(ctx, spaceID, limit, cursor)
	}
	return &confluence.PageList{Results: []confluence.Page{}}, nil
}

func (m *mockConfluenceService) GetSpaces(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluence.SpaceList, error) {
	if m.getSpacesFunc != nil {
		return m.getSpacesFunc(ctx, limit, cursor, keys, spaceType)
	}
	return &confluence.SpaceList{Results: []confluence.Space{}}, nil
}

func (m *mockConfluenceService) GetPageDescendants(ctx context.Context, pageID string, limit int, cursor string) (*confluence.PageRefList, error) {
	if m.getPageDescendantsFunc != nil {
		return m.getPageDescendantsFunc(ctx, pageID, limit, cursor)
	}
	return &confluence.PageRefList{Results: []confluence.PageRef{}}, nil
}

func (m *mockConfluenceService) GetFooterComments(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
	if m.getFooterCommentsFunc != nil {
		return m.getFooterCommentsFunc(ctx, pageID, limit, cursor)
	}
	return &confluence.CommentList{Results: []confluence.Comment{}}, nil
}

func (m *mockConfluenceService) GetInlineComments(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
	if m.getInlineCommentsFunc != nil {
		return m.getInlineCommentsFunc(ctx, pageID, limit, cursor)
	}
	return &confluence.CommentList{Results: []confluence.Comment{}}, nil
}

func (m *mockConfluenceService) GetCommentChildren(ctx context.Context, commentID string, limit int, cursor string) (*confluence.CommentList, error) {
	if m.getCommentChildrenFunc != nil {
		return m.getCommentChildrenFunc(ctx, commentID, limit, cursor)
	}
	return &confluence.CommentList{Results: []confluence.Comment{}}, nil
}

func (m *mockConfluenceService) CreatePage(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error) {
	if m.createPageFunc != nil {
		return m.createPageFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockConfluenceService) UpdatePage(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error) {
	if m.updatePageFunc != nil {
		return m.updatePageFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockConfluenceService) CreateFooterComment(ctx context.Context, req confluence.CreateCommentRequest) (*confluence.Comment, error) {
	if m.createFooterCommentFunc != nil {
		return m.createFooterCommentFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockConfluenceService) CreateInlineComment(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error) {
	if m.createInlineCommentFunc != nil {
		return m.createInlineCommentFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockConfluenceService) SearchContent(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error) {
	if m.searchContentFunc != nil {
		return m.searchContentFunc(ctx, cql, limit)
	}
	return []confluence.SearchResult{}, nil
}

// samplePage returns a minimal Page for use in tests.
func samplePage() *confluence.Page {
	return &confluence.Page{
		ID:            "123456",
		Title:         "Test Page",
		SpaceID:       "SPACE1",
		Status:        "current",
		VersionNumber: 3,
		Body:          "<p>hello</p>",
		CreatedAt:     time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
		WebURL:        "/wiki/spaces/SPACE1/pages/123456",
	}
}

// sampleComment returns a minimal Comment for use in tests.
func sampleComment() *confluence.Comment {
	return &confluence.Comment{
		ID:            "c99",
		Body:          "<p>nice</p>",
		VersionNumber: 1,
		CreatedAt:     time.Date(2024, 6, 1, 11, 0, 0, 0, time.UTC),
	}
}

// ---- GetPage ----

func TestConfluenceGetPage(t *testing.T) {
	tests := []struct {
		name        string
		pageID      string
		mockFn      func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:   "success returns page JSON",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error) {
				return samplePage(), nil
			},
			wantStatus:  200,
			wantContain: `"id":"123456"`,
		},
		{
			name:   "not found returns 404",
			pageID: "999",
			mockFn: func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error) {
				return nil, confluence.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
		{
			name:   "unauthorized returns 401",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name:   "rate limited returns 429",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewConfluenceHandler(&mockConfluenceService{getPageFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /confluence/pages/{pageId}", h.GetPage)

			req := httptest.NewRequest(http.MethodGet, "/confluence/pages/"+tc.pageID, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

// ---- GetPagesInSpace ----

func TestConfluenceGetPagesInSpace(t *testing.T) {
	tests := []struct {
		name        string
		spaceID     string
		mockFn      func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:    "success returns list with next_cursor",
			spaceID: "SPACE1",
			mockFn: func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error) {
				return &confluence.PageList{
					Results:    []confluence.Page{*samplePage()},
					NextCursor: "abc",
				}, nil
			},
			wantStatus:  200,
			wantContain: `"next_cursor":"abc"`,
		},
		{
			name:    "empty space returns empty results array",
			spaceID: "EMPTY",
			mockFn: func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error) {
				return &confluence.PageList{Results: []confluence.Page{}}, nil
			},
			wantStatus:  200,
			wantContain: `"results":[]`,
		},
		{
			name:    "not found returns 404",
			spaceID: "GONE",
			mockFn: func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error) {
				return nil, confluence.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
		{
			name:    "unauthorized returns 401",
			spaceID: "SPACE1",
			mockFn: func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name:    "rate limited returns 429",
			spaceID: "SPACE1",
			mockFn: func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewConfluenceHandler(&mockConfluenceService{getPagesInSpaceFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /confluence/spaces/{spaceId}/pages", h.GetPagesInSpace)

			req := httptest.NewRequest(http.MethodGet, "/confluence/spaces/"+tc.spaceID+"/pages", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

// ---- GetSpaces ----

func TestConfluenceGetSpaces(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		mockFn      func(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluence.SpaceList, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:  "success returns space list",
			query: "limit=10",
			mockFn: func(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluence.SpaceList, error) {
				return &confluence.SpaceList{
					Results: []confluence.Space{{ID: "1", Key: "TS", Name: "Test Space", Type: "global", Status: "current"}},
				}, nil
			},
			wantStatus:  200,
			wantContain: `"key":"TS"`,
		},
		{
			name:  "unauthorized returns 401",
			query: "",
			mockFn: func(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluence.SpaceList, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name:  "rate limited returns 429",
			query: "",
			mockFn: func(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluence.SpaceList, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewConfluenceHandler(&mockConfluenceService{getSpacesFunc: tc.mockFn}, audit.NewNoopLogger())

			url := "/confluence/spaces"
			if tc.query != "" {
				url += "?" + tc.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			h.GetSpaces(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

// ---- GetPageDescendants ----

func TestConfluenceGetPageDescendants(t *testing.T) {
	tests := []struct {
		name        string
		pageID      string
		mockFn      func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.PageRefList, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:   "success returns descendants",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.PageRefList, error) {
				return &confluence.PageRefList{
					Results: []confluence.PageRef{{ID: "child1", Title: "Child Page", Status: "current", Type: "page"}},
				}, nil
			},
			wantStatus:  200,
			wantContain: `"id":"child1"`,
		},
		{
			name:   "not found returns 404",
			pageID: "999",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.PageRefList, error) {
				return nil, confluence.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
		{
			name:   "unauthorized returns 401",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.PageRefList, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name:   "rate limited returns 429",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.PageRefList, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewConfluenceHandler(&mockConfluenceService{getPageDescendantsFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /confluence/pages/{pageId}/descendants", h.GetPageDescendants)

			req := httptest.NewRequest(http.MethodGet, "/confluence/pages/"+tc.pageID+"/descendants", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

// ---- GetFooterComments ----

func TestConfluenceGetFooterComments(t *testing.T) {
	tests := []struct {
		name        string
		pageID      string
		mockFn      func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:   "success returns comments with next_cursor",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return &confluence.CommentList{
					Results:    []confluence.Comment{*sampleComment()},
					NextCursor: "tok1",
				}, nil
			},
			wantStatus:  200,
			wantContain: `"next_cursor":"tok1"`,
		},
		{
			name:   "not found returns 404",
			pageID: "999",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return nil, confluence.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
		{
			name:   "unauthorized returns 401",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name:   "rate limited returns 429",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewConfluenceHandler(&mockConfluenceService{getFooterCommentsFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /confluence/pages/{pageId}/footer-comments", h.GetFooterComments)

			req := httptest.NewRequest(http.MethodGet, "/confluence/pages/"+tc.pageID+"/footer-comments", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

// ---- GetInlineComments ----

func TestConfluenceGetInlineComments(t *testing.T) {
	tests := []struct {
		name        string
		pageID      string
		mockFn      func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:   "success returns inline comments",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return &confluence.CommentList{Results: []confluence.Comment{*sampleComment()}}, nil
			},
			wantStatus:  200,
			wantContain: `"id":"c99"`,
		},
		{
			name:   "not found returns 404",
			pageID: "999",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return nil, confluence.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
		{
			name:   "unauthorized returns 401",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name:   "rate limited returns 429",
			pageID: "123456",
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewConfluenceHandler(&mockConfluenceService{getInlineCommentsFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /confluence/pages/{pageId}/inline-comments", h.GetInlineComments)

			req := httptest.NewRequest(http.MethodGet, "/confluence/pages/"+tc.pageID+"/inline-comments", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

// ---- GetCommentChildren ----

func TestConfluenceGetCommentChildren(t *testing.T) {
	tests := []struct {
		name        string
		commentID   string
		mockFn      func(ctx context.Context, commentID string, limit int, cursor string) (*confluence.CommentList, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:      "success returns children",
			commentID: "c99",
			mockFn: func(ctx context.Context, commentID string, limit int, cursor string) (*confluence.CommentList, error) {
				return &confluence.CommentList{Results: []confluence.Comment{{ID: "child1", Body: "reply"}}}, nil
			},
			wantStatus:  200,
			wantContain: `"id":"child1"`,
		},
		{
			name:      "not found returns 404",
			commentID: "gone",
			mockFn: func(ctx context.Context, commentID string, limit int, cursor string) (*confluence.CommentList, error) {
				return nil, confluence.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
		{
			name:      "unauthorized returns 401",
			commentID: "c99",
			mockFn: func(ctx context.Context, commentID string, limit int, cursor string) (*confluence.CommentList, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name:      "rate limited returns 429",
			commentID: "c99",
			mockFn: func(ctx context.Context, commentID string, limit int, cursor string) (*confluence.CommentList, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewConfluenceHandler(&mockConfluenceService{getCommentChildrenFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /confluence/comments/{commentId}/children", h.GetCommentChildren)

			req := httptest.NewRequest(http.MethodGet, "/confluence/comments/"+tc.commentID+"/children", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

// ---- SearchContent ----

func TestConfluenceSearchContent(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		mockFn      func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:  "success returns search results",
			query: "cql=type%3Dpage",
			mockFn: func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error) {
				return []confluence.SearchResult{
					{ContentID: "123", Title: "Found Page", Type: "page", SpaceKey: "TS", Excerpt: "Some excerpt"},
				}, nil
			},
			wantStatus:  200,
			wantContain: `"content_id":"123"`,
		},
		{
			name:        "missing cql returns 400",
			query:       "",
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
		},
		{
			name:  "unauthorized returns 401",
			query: "cql=type%3Dpage",
			mockFn: func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name:  "rate limited returns 429",
			query: "cql=type%3Dpage",
			mockFn: func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewConfluenceHandler(&mockConfluenceService{searchContentFunc: tc.mockFn}, audit.NewNoopLogger())

			url := "/confluence/search"
			if tc.query != "" {
				url += "?" + tc.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			h.SearchContent(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

// ---- CreatePage ----

func TestConfluenceCreatePage(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]any
		writeHeader bool
		mockFn      func(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error)
		wantStatus  int
		wantContain string
		wantAudit   bool
	}{
		{
			name:        "success returns 201 with page",
			body:        map[string]any{"space_id": "SPACE1", "title": "New Page", "body": "<p>hi</p>"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error) {
				return samplePage(), nil
			},
			wantStatus:  201,
			wantContain: `"id":"123456"`,
			wantAudit:   true,
		},
		{
			name:        "missing space_id returns 400",
			body:        map[string]any{"title": "x", "body": "y"},
			writeHeader: true,
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
			wantAudit:   false,
		},
		{
			name:        "missing title returns 400",
			body:        map[string]any{"space_id": "SPACE1", "body": "y"},
			writeHeader: true,
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
			wantAudit:   false,
		},
		{
			name:        "not found returns 404 with audit",
			body:        map[string]any{"space_id": "GONE", "title": "x"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error) {
				return nil, confluence.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
			wantAudit:   true,
		},
		{
			name:        "unauthorized returns 401 with audit",
			body:        map[string]any{"space_id": "SPACE1", "title": "x"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
			wantAudit:   true,
		},
		{
			name:        "rate limited returns 429 with audit",
			body:        map[string]any{"space_id": "SPACE1", "title": "x"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
			wantAudit:   true,
		},
		{
			name:        "write guard blocks when no header",
			body:        map[string]any{"space_id": "SPACE1", "title": "x"},
			writeHeader: false,
			mockFn:      nil,
			wantStatus:  403,
			wantContain: `"code":"WRITE_DISABLED"`,
			wantAudit:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := &captureLogger{}
			h := NewConfluenceHandler(&mockConfluenceService{createPageFunc: tc.mockFn}, logger)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /confluence/pages", h.CreatePage)
			handler := writeGuardForTest(false, mux)

			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/confluence/pages", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			if tc.writeHeader {
				req.Header.Set("X-Enable-Write", "true")
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
			if tc.wantAudit && len(logger.entries) == 0 {
				t.Error("expected audit log entry but got none")
			}
			if !tc.wantAudit && len(logger.entries) > 0 {
				t.Error("expected no audit log entry but got one")
			}
		})
	}
}

// ---- UpdatePage ----

func TestConfluenceUpdatePage(t *testing.T) {
	tests := []struct {
		name        string
		pageID      string
		body        map[string]any
		writeHeader bool
		mockFn      func(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error)
		wantStatus  int
		wantContain string
		wantAudit   bool
	}{
		{
			name:        "success returns 200 with page",
			pageID:      "123456",
			body:        map[string]any{"title": "Updated", "body": "<p>new</p>"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error) {
				return samplePage(), nil
			},
			wantStatus:  200,
			wantContain: `"id":"123456"`,
			wantAudit:   true,
		},
		{
			name:        "not found returns 404",
			pageID:      "999",
			body:        map[string]any{"title": "x"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error) {
				return nil, confluence.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
			wantAudit:   true,
		},
		{
			name:        "unauthorized returns 401",
			pageID:      "123456",
			body:        map[string]any{"title": "x"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
			wantAudit:   true,
		},
		{
			name:        "conflict returns 409 (stale version)",
			pageID:      "123456",
			body:        map[string]any{"title": "x", "version_number": 1},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error) {
				return nil, confluence.ErrConflict
			},
			wantStatus:  409,
			wantContain: `"code":"CONFLICT"`,
			wantAudit:   true,
		},
		{
			name:        "rate limited returns 429",
			pageID:      "123456",
			body:        map[string]any{"title": "x"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
			wantAudit:   true,
		},
		{
			name:        "write guard blocks when no header",
			pageID:      "123456",
			body:        map[string]any{"title": "x"},
			writeHeader: false,
			mockFn:      nil,
			wantStatus:  403,
			wantContain: `"code":"WRITE_DISABLED"`,
			wantAudit:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := &captureLogger{}
			h := NewConfluenceHandler(&mockConfluenceService{updatePageFunc: tc.mockFn}, logger)

			mux := http.NewServeMux()
			mux.HandleFunc("PUT /confluence/pages/{pageId}", h.UpdatePage)
			handler := writeGuardForTest(false, mux)

			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPut, "/confluence/pages/"+tc.pageID, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			if tc.writeHeader {
				req.Header.Set("X-Enable-Write", "true")
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
			if tc.wantAudit && len(logger.entries) == 0 {
				t.Error("expected audit log entry but got none")
			}
			if !tc.wantAudit && len(logger.entries) > 0 {
				t.Error("expected no audit log entry but got one")
			}
		})
	}
}

// ---- CreateFooterComment ----

func TestConfluenceCreateFooterComment(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]any
		writeHeader bool
		mockFn      func(ctx context.Context, req confluence.CreateCommentRequest) (*confluence.Comment, error)
		wantStatus  int
		wantContain string
		wantAudit   bool
	}{
		{
			name:        "success returns 201 with comment",
			body:        map[string]any{"page_id": "123456", "body": "<p>comment</p>"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreateCommentRequest) (*confluence.Comment, error) {
				return sampleComment(), nil
			},
			wantStatus:  201,
			wantContain: `"id":"c99"`,
			wantAudit:   true,
		},
		{
			name:        "missing page_id returns 400",
			body:        map[string]any{"body": "hi"},
			writeHeader: true,
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
			wantAudit:   false,
		},
		{
			name:        "missing body returns 400",
			body:        map[string]any{"page_id": "123456"},
			writeHeader: true,
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
			wantAudit:   false,
		},
		{
			name:        "not found returns 404",
			body:        map[string]any{"page_id": "999", "body": "hi"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreateCommentRequest) (*confluence.Comment, error) {
				return nil, confluence.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
			wantAudit:   true,
		},
		{
			name:        "unauthorized returns 401",
			body:        map[string]any{"page_id": "123456", "body": "hi"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreateCommentRequest) (*confluence.Comment, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
			wantAudit:   true,
		},
		{
			name:        "rate limited returns 429",
			body:        map[string]any{"page_id": "123456", "body": "hi"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreateCommentRequest) (*confluence.Comment, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
			wantAudit:   true,
		},
		{
			name:        "write guard blocks when no header",
			body:        map[string]any{"page_id": "123456", "body": "hi"},
			writeHeader: false,
			mockFn:      nil,
			wantStatus:  403,
			wantContain: `"code":"WRITE_DISABLED"`,
			wantAudit:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := &captureLogger{}
			h := NewConfluenceHandler(&mockConfluenceService{createFooterCommentFunc: tc.mockFn}, logger)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /confluence/footer-comments", h.CreateFooterComment)
			handler := writeGuardForTest(false, mux)

			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/confluence/footer-comments", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			if tc.writeHeader {
				req.Header.Set("X-Enable-Write", "true")
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
			if tc.wantAudit && len(logger.entries) == 0 {
				t.Error("expected audit log entry but got none")
			}
			if !tc.wantAudit && len(logger.entries) > 0 {
				t.Error("expected no audit log entry but got one")
			}
		})
	}
}

// ---- CreateInlineComment ----

func TestConfluenceCreateInlineComment(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]any
		writeHeader bool
		mockFn      func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error)
		wantStatus  int
		wantContain string
		wantAudit   bool
	}{
		{
			name:        "success returns 201 with comment",
			body:        map[string]any{"page_id": "123456", "body": "note", "text_selection": "selected text"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error) {
				return sampleComment(), nil
			},
			wantStatus:  201,
			wantContain: `"id":"c99"`,
			wantAudit:   true,
		},
		{
			// text_selection is required — handler must return 400 BEFORE calling the service.
			name:        "missing text_selection returns 400 before service call",
			body:        map[string]any{"page_id": "123456", "body": "note"},
			writeHeader: true,
			mockFn:      nil, // nil means service would panic if called
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
			wantAudit:   false,
		},
		{
			name:        "missing page_id returns 400",
			body:        map[string]any{"body": "note", "text_selection": "hi"},
			writeHeader: true,
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
			wantAudit:   false,
		},
		{
			name:        "not found returns 404",
			body:        map[string]any{"page_id": "999", "body": "note", "text_selection": "hi"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error) {
				return nil, confluence.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
			wantAudit:   true,
		},
		{
			name:        "unauthorized returns 401",
			body:        map[string]any{"page_id": "123456", "body": "note", "text_selection": "hi"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
			wantAudit:   true,
		},
		{
			name:        "rate limited returns 429",
			body:        map[string]any{"page_id": "123456", "body": "note", "text_selection": "hi"},
			writeHeader: true,
			mockFn: func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error) {
				return nil, confluence.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
			wantAudit:   true,
		},
		{
			name:        "write guard blocks when no header",
			body:        map[string]any{"page_id": "123456", "body": "note", "text_selection": "hi"},
			writeHeader: false,
			mockFn:      nil,
			wantStatus:  403,
			wantContain: `"code":"WRITE_DISABLED"`,
			wantAudit:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := &captureLogger{}
			h := NewConfluenceHandler(&mockConfluenceService{createInlineCommentFunc: tc.mockFn}, logger)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /confluence/inline-comments", h.CreateInlineComment)
			handler := writeGuardForTest(false, mux)

			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/confluence/inline-comments", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			if tc.writeHeader {
				req.Header.Set("X-Enable-Write", "true")
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
			if tc.wantAudit && len(logger.entries) == 0 {
				t.Error("expected audit log entry but got none")
			}
			if !tc.wantAudit && len(logger.entries) > 0 {
				t.Error("expected no audit log entry but got one")
			}
		})
	}
}
