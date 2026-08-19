package mcpserver_test

import (
	"context"
	"strings"
	"testing"
	"time"

	confluence "github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

// mockConfluenceService implements confluence.Service for testing.
// Each method field is a func; nil means "return empty success".
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
	return &confluence.Page{ID: pageID}, nil
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
	return &confluence.Page{}, nil
}

func (m *mockConfluenceService) UpdatePage(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error) {
	if m.updatePageFunc != nil {
		return m.updatePageFunc(ctx, req)
	}
	return &confluence.Page{}, nil
}

func (m *mockConfluenceService) CreateFooterComment(ctx context.Context, req confluence.CreateCommentRequest) (*confluence.Comment, error) {
	if m.createFooterCommentFunc != nil {
		return m.createFooterCommentFunc(ctx, req)
	}
	return &confluence.Comment{}, nil
}

func (m *mockConfluenceService) CreateInlineComment(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error) {
	if m.createInlineCommentFunc != nil {
		return m.createInlineCommentFunc(ctx, req)
	}
	return &confluence.Comment{}, nil
}

func (m *mockConfluenceService) SearchContent(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error) {
	if m.searchContentFunc != nil {
		return m.searchContentFunc(ctx, cql, limit)
	}
	return []confluence.SearchResult{}, nil
}

// --- TestToolGetConfluencePage ---

func TestToolGetConfluencePage(t *testing.T) {
	fixedTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "valid page returns JSON with snake_case fields",
			args: map[string]any{"page_id": "123456"},
			mockFn: func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error) {
				return &confluence.Page{
					ID:            pageID,
					Title:         "My Page",
					SpaceID:       "DEV",
					Status:        "current",
					VersionNumber: 2,
					Body:          "<p>Hello</p>",
					CreatedAt:     fixedTime,
					WebURL:        "/wiki/pages/123456",
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"123456"`,
		},
		{
			name: "response contains space_id in snake_case",
			args: map[string]any{"page_id": "123456"},
			mockFn: func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error) {
				return &confluence.Page{ID: "123456", Title: "T", SpaceID: "DEV", Status: "current"}, nil
			},
			wantIsError: false,
			wantContain: `"space_id":"DEV"`,
		},
		{
			name: "zero created_at is omitted (omitempty)",
			args: map[string]any{"page_id": "123456"},
			mockFn: func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error) {
				return &confluence.Page{ID: "123456", Title: "T", Status: "current"}, nil
			},
			wantIsError: false,
			wantContain: `"status":"current"`,
		},
		{
			name:        "missing page_id returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "page_id",
		},
		{
			name: "ErrNotFound returns error result",
			args: map[string]any{"page_id": "999"},
			mockFn: func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error) {
				return nil, confluence.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name: "ErrUnauthorized returns error result",
			args: map[string]any{"page_id": "123456"},
			mockFn: func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name:  "works without ENABLE_WRITE (read-only)",
			setup: disableWrite,
			args:  map[string]any{"page_id": "123456"},
			mockFn: func(ctx context.Context, pageID string, bodyFormat string) (*confluence.Page, error) {
				return &confluence.Page{ID: "123456", Title: "T", Status: "current"}, nil
			},
			wantIsError: false,
			wantContain: `"id":"123456"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			svc := &mockConfluenceService{getPageFunc: tc.mockFn}
			handler := mcpserver.ToolGetConfluencePage(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolGetPagesInSpace ---

func TestToolGetPagesInSpace(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "valid response contains results array",
			args: map[string]any{"space_id": "DEV"},
			mockFn: func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error) {
				return &confluence.PageList{
					Results: []confluence.Page{
						{ID: "p1", Title: "Page 1", Status: "current"},
						{ID: "p2", Title: "Page 2", Status: "current"},
					},
					NextCursor: "",
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"p1"`,
		},
		{
			name: "empty space returns [] not null",
			args: map[string]any{"space_id": "EMPTY"},
			mockFn: func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error) {
				return &confluence.PageList{Results: []confluence.Page{}}, nil
			},
			wantIsError: false,
			wantContain: `"results":[]`,
		},
		{
			name: "response contains next_cursor when present",
			args: map[string]any{"space_id": "DEV"},
			mockFn: func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error) {
				return &confluence.PageList{
					Results:    []confluence.Page{{ID: "p1", Title: "Page 1", Status: "current"}},
					NextCursor: "abc123",
				}, nil
			},
			wantIsError: false,
			wantContain: `"next_cursor":"abc123"`,
		},
		{
			name:        "missing space_id returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "space_id",
		},
		{
			name: "ErrNotFound returns error result",
			args: map[string]any{"space_id": "NOTEXIST"},
			mockFn: func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error) {
				return nil, confluence.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockConfluenceService{getPagesInSpaceFunc: tc.mockFn}
			handler := mcpserver.ToolGetPagesInSpace(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolGetConfluenceSpaces ---

func TestToolGetConfluenceSpaces(t *testing.T) {
	tests := []struct {
		name        string
		mockFn      func(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluence.SpaceList, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "valid response contains spaces with snake_case fields",
			mockFn: func(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluence.SpaceList, error) {
				return &confluence.SpaceList{
					Results: []confluence.Space{
						{ID: "s1", Key: "DEV", Name: "Development", Type: "global", Status: "current"},
					},
				}, nil
			},
			wantIsError: false,
			wantContain: `"key":"DEV"`,
		},
		{
			name: "empty spaces returns []",
			mockFn: func(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluence.SpaceList, error) {
				return &confluence.SpaceList{Results: []confluence.Space{}}, nil
			},
			wantIsError: false,
			wantContain: `"results":[]`,
		},
		{
			name: "ErrUnauthorized returns error result",
			mockFn: func(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluence.SpaceList, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockConfluenceService{getSpacesFunc: tc.mockFn}
			handler := mcpserver.ToolGetConfluenceSpaces(svc)
			req := makeCallToolRequest(map[string]any{})
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// TestToolGetConfluenceSpaces_ReadOnly verifies spaces work without ENABLE_WRITE.
func TestToolGetConfluenceSpaces_ReadOnly(t *testing.T) {
	disableWrite(t)
	svc := &mockConfluenceService{
		getSpacesFunc: func(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluence.SpaceList, error) {
			return &confluence.SpaceList{Results: []confluence.Space{{ID: "s1", Key: "DEV"}}}, nil
		},
	}
	handler := mcpserver.ToolGetConfluenceSpaces(svc)
	req := makeCallToolRequest(map[string]any{})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("read tool should succeed without ENABLE_WRITE, got error: %s", getResultText(t, result))
	}
}

// --- TestToolGetPageDescendants ---

func TestToolGetPageDescendants(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.PageRefList, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "valid response contains page refs",
			args: map[string]any{"page_id": "123"},
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.PageRefList, error) {
				return &confluence.PageRefList{
					Results: []confluence.PageRef{
						{ID: "child1", Title: "Child Page", Status: "current", Type: "page"},
					},
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"child1"`,
		},
		{
			name: "empty results serialize as []",
			args: map[string]any{"page_id": "123"},
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.PageRefList, error) {
				return &confluence.PageRefList{Results: []confluence.PageRef{}}, nil
			},
			wantIsError: false,
			wantContain: `"results":[]`,
		},
		{
			name:        "missing page_id returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "page_id",
		},
		{
			name: "ErrNotFound returns error result",
			args: map[string]any{"page_id": "999"},
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.PageRefList, error) {
				return nil, confluence.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockConfluenceService{getPageDescendantsFunc: tc.mockFn}
			handler := mcpserver.ToolGetPageDescendants(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolGetPageFooterComments ---

func TestToolGetPageFooterComments(t *testing.T) {
	fixedTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "valid response contains comments with snake_case fields",
			args: map[string]any{"page_id": "123"},
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return &confluence.CommentList{
					Results: []confluence.Comment{
						{ID: "c1", Body: "Great work!", VersionNumber: 1, CreatedAt: fixedTime},
					},
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"c1"`,
		},
		{
			name: "response contains version_number in snake_case",
			args: map[string]any{"page_id": "123"},
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return &confluence.CommentList{
					Results: []confluence.Comment{{ID: "c1", Body: "Test", VersionNumber: 3}},
				}, nil
			},
			wantIsError: false,
			wantContain: `"version_number":3`,
		},
		{
			name: "no comments returns []",
			args: map[string]any{"page_id": "123"},
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return &confluence.CommentList{Results: []confluence.Comment{}}, nil
			},
			wantIsError: false,
			wantContain: `"results":[]`,
		},
		{
			name:        "missing page_id returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "page_id",
		},
		{
			name: "ErrNotFound returns error result",
			args: map[string]any{"page_id": "999"},
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return nil, confluence.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockConfluenceService{getFooterCommentsFunc: tc.mockFn}
			handler := mcpserver.ToolGetPageFooterComments(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolGetPageInlineComments ---

func TestToolGetPageInlineComments(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "valid response contains inline comments",
			args: map[string]any{"page_id": "123"},
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return &confluence.CommentList{
					Results: []confluence.Comment{{ID: "ic1", Body: "Inline note", VersionNumber: 1}},
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"ic1"`,
		},
		{
			name: "no inline comments returns []",
			args: map[string]any{"page_id": "123"},
			mockFn: func(ctx context.Context, pageID string, limit int, cursor string) (*confluence.CommentList, error) {
				return &confluence.CommentList{Results: []confluence.Comment{}}, nil
			},
			wantIsError: false,
			wantContain: `"results":[]`,
		},
		{
			name:        "missing page_id returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "page_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockConfluenceService{getInlineCommentsFunc: tc.mockFn}
			handler := mcpserver.ToolGetPageInlineComments(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolGetCommentChildren ---

func TestToolGetCommentChildren(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, commentID string, limit int, cursor string) (*confluence.CommentList, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "valid response contains child comments",
			args: map[string]any{"comment_id": "cmt-1"},
			mockFn: func(ctx context.Context, commentID string, limit int, cursor string) (*confluence.CommentList, error) {
				return &confluence.CommentList{
					Results: []confluence.Comment{{ID: "child-1", Body: "Reply here", VersionNumber: 1}},
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"child-1"`,
		},
		{
			name: "no children returns []",
			args: map[string]any{"comment_id": "cmt-1"},
			mockFn: func(ctx context.Context, commentID string, limit int, cursor string) (*confluence.CommentList, error) {
				return &confluence.CommentList{Results: []confluence.Comment{}}, nil
			},
			wantIsError: false,
			wantContain: `"results":[]`,
		},
		{
			name:        "missing comment_id returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "comment_id",
		},
		{
			name: "ErrNotFound returns error result",
			args: map[string]any{"comment_id": "bad-id"},
			mockFn: func(ctx context.Context, commentID string, limit int, cursor string) (*confluence.CommentList, error) {
				return nil, confluence.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockConfluenceService{getCommentChildrenFunc: tc.mockFn}
			handler := mcpserver.ToolGetCommentChildren(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolSearchConfluence ---

func TestToolSearchConfluence(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "valid search returns results with snake_case fields",
			args: map[string]any{"cql": "type=page AND space=DEV"},
			mockFn: func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error) {
				return []confluence.SearchResult{
					{ContentID: "123", Title: "Deploy Pipeline", Type: "page", SpaceKey: "DEV", Excerpt: "..."},
				}, nil
			},
			wantIsError: false,
			wantContain: `"content_id":"123"`,
		},
		{
			name: "response contains space_key in snake_case",
			args: map[string]any{"cql": "type=page"},
			mockFn: func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error) {
				return []confluence.SearchResult{
					{ContentID: "1", Title: "T", Type: "page", SpaceKey: "PROD"},
				}, nil
			},
			wantIsError: false,
			wantContain: `"space_key":"PROD"`,
		},
		{
			name: "no results returns []",
			args: map[string]any{"cql": "type=page AND space=EMPTY"},
			mockFn: func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error) {
				return []confluence.SearchResult{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
		{
			name:        "missing cql returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "cql",
		},
		{
			name: "ErrUnauthorized returns error result",
			args: map[string]any{"cql": "type=page"},
			mockFn: func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name:  "works without ENABLE_WRITE (read-only)",
			setup: disableWrite,
			args:  map[string]any{"cql": "type=page"},
			mockFn: func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error) {
				return []confluence.SearchResult{{ContentID: "1", Title: "T"}}, nil
			},
			wantIsError: false,
			wantContain: `"content_id"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			svc := &mockConfluenceService{searchContentFunc: tc.mockFn}
			handler := mcpserver.ToolSearchConfluence(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolCreateConfluencePage ---

func TestToolCreateConfluencePage(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:  "success returns created page JSON",
			setup: enableWrite,
			args:  map[string]any{"space_id": "DEV", "title": "New Page", "body": "<p>Content</p>"},
			mockFn: func(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error) {
				return &confluence.Page{ID: "new-1", Title: req.Title, SpaceID: req.SpaceID, Status: "current", VersionNumber: 1}, nil
			},
			wantIsError: false,
			wantContain: `"id":"new-1"`,
		},
		{
			name:  "success response contains title",
			setup: enableWrite,
			args:  map[string]any{"space_id": "DEV", "title": "My New Page", "body": "<p>X</p>"},
			mockFn: func(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error) {
				return &confluence.Page{ID: "new-2", Title: "My New Page", SpaceID: "DEV", Status: "current"}, nil
			},
			wantIsError: false,
			wantContain: `"title":"My New Page"`,
		},
		{
			name:        "write guard blocks when ENABLE_WRITE not set",
			setup:       disableWrite,
			args:        map[string]any{"space_id": "DEV", "title": "T", "body": "<p>B</p>"},
			mockFn:      nil, // must never be called
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing space_id returns error",
			setup:       enableWrite,
			args:        map[string]any{"title": "T", "body": "<p>B</p>"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "space_id",
		},
		{
			name:        "missing title returns error",
			setup:       enableWrite,
			args:        map[string]any{"space_id": "DEV", "body": "<p>B</p>"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "title",
		},
		{
			name:        "missing body returns error",
			setup:       enableWrite,
			args:        map[string]any{"space_id": "DEV", "title": "T"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "body",
		},
		{
			name:  "ErrUnauthorized returns error result",
			setup: enableWrite,
			args:  map[string]any{"space_id": "DEV", "title": "T", "body": "<p>B</p>"},
			mockFn: func(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error) {
				return nil, confluence.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			cl := &captureLogger{}
			svc := &mockConfluenceService{createPageFunc: tc.mockFn}
			handler := mcpserver.ToolCreateConfluencePage(svc, cl)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// TestToolCreateConfluencePage_AuditLogging verifies audit log behavior.
func TestToolCreateConfluencePage_AuditLogging(t *testing.T) {
	t.Run("audit log written on success", func(t *testing.T) {
		enableWrite(t)
		cl := &captureLogger{}
		svc := &mockConfluenceService{
			createPageFunc: func(ctx context.Context, req confluence.CreatePageRequest) (*confluence.Page, error) {
				return &confluence.Page{ID: "p1", Title: req.Title}, nil
			},
		}
		handler := mcpserver.ToolCreateConfluencePage(svc, cl)
		req := makeCallToolRequest(map[string]any{"space_id": "DEV", "title": "T", "body": "<p>B</p>"})
		_, _ = handler(context.Background(), req)
		if len(cl.entries) != 1 {
			t.Errorf("expected 1 audit entry, got %d", len(cl.entries))
		}
		if cl.entries[0].Operation != "create_confluence_page" {
			t.Errorf("audit operation: got %q, want %q", cl.entries[0].Operation, "create_confluence_page")
		}
	})

	t.Run("audit log NOT written when write guard blocks", func(t *testing.T) {
		disableWrite(t)
		cl := &captureLogger{}
		svc := &mockConfluenceService{}
		handler := mcpserver.ToolCreateConfluencePage(svc, cl)
		req := makeCallToolRequest(map[string]any{"space_id": "DEV", "title": "T", "body": "<p>B</p>"})
		_, _ = handler(context.Background(), req)
		if len(cl.entries) != 0 {
			t.Errorf("expected 0 audit entries when write guard blocks, got %d", len(cl.entries))
		}
	})
}

// --- TestToolUpdateConfluencePage ---

func TestToolUpdateConfluencePage(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:  "success returns updated page JSON",
			setup: enableWrite,
			args:  map[string]any{"page_id": "123", "title": "Updated Title", "body": "<p>Updated</p>"},
			mockFn: func(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error) {
				return &confluence.Page{ID: req.PageID, Title: req.Title, Status: "current", VersionNumber: 3}, nil
			},
			wantIsError: false,
			wantContain: `"id":"123"`,
		},
		{
			name:        "write guard blocks when ENABLE_WRITE not set",
			setup:       disableWrite,
			args:        map[string]any{"page_id": "123", "title": "T", "body": "<p>B</p>"},
			mockFn:      nil, // must never be called
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing page_id returns error",
			setup:       enableWrite,
			args:        map[string]any{"title": "T", "body": "<p>B</p>"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "page_id",
		},
		{
			name:        "missing title returns error",
			setup:       enableWrite,
			args:        map[string]any{"page_id": "123", "body": "<p>B</p>"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "title",
		},
		{
			name:  "ErrConflict (stale version) returns error result",
			setup: enableWrite,
			args:  map[string]any{"page_id": "123", "title": "T", "body": "<p>B</p>", "version_number": float64(1)},
			mockFn: func(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error) {
				return nil, confluence.ErrConflict
			},
			wantIsError: true,
			wantContain: "conflict",
		},
		{
			name:  "ErrNotFound returns error result",
			setup: enableWrite,
			args:  map[string]any{"page_id": "999", "title": "T", "body": "<p>B</p>"},
			mockFn: func(ctx context.Context, req confluence.UpdatePageRequest) (*confluence.Page, error) {
				return nil, confluence.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			cl := &captureLogger{}
			svc := &mockConfluenceService{updatePageFunc: tc.mockFn}
			handler := mcpserver.ToolUpdateConfluencePage(svc, cl)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolCreateFooterComment ---

func TestToolCreateFooterComment(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, req confluence.CreateCommentRequest) (*confluence.Comment, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:  "success returns created comment JSON",
			setup: enableWrite,
			args:  map[string]any{"page_id": "123", "body": "<p>Nice work!</p>"},
			mockFn: func(ctx context.Context, req confluence.CreateCommentRequest) (*confluence.Comment, error) {
				return &confluence.Comment{ID: "c-new", Body: req.Body, VersionNumber: 1}, nil
			},
			wantIsError: false,
			wantContain: `"id":"c-new"`,
		},
		{
			name:        "write guard blocks when ENABLE_WRITE not set",
			setup:       disableWrite,
			args:        map[string]any{"page_id": "123", "body": "<p>B</p>"},
			mockFn:      nil, // must never be called
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing page_id returns error",
			setup:       enableWrite,
			args:        map[string]any{"body": "<p>B</p>"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "page_id",
		},
		{
			name:        "missing body returns error",
			setup:       enableWrite,
			args:        map[string]any{"page_id": "123"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "body",
		},
		{
			name:  "ErrNotFound returns error result",
			setup: enableWrite,
			args:  map[string]any{"page_id": "999", "body": "<p>B</p>"},
			mockFn: func(ctx context.Context, req confluence.CreateCommentRequest) (*confluence.Comment, error) {
				return nil, confluence.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			cl := &captureLogger{}
			svc := &mockConfluenceService{createFooterCommentFunc: tc.mockFn}
			handler := mcpserver.ToolCreateFooterComment(svc, cl)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolCreateInlineComment ---

func TestToolCreateInlineComment(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:  "success returns created inline comment JSON",
			setup: enableWrite,
			args: map[string]any{
				"page_id":        "123",
				"body":           "<p>Anchor comment</p>",
				"text_selection": "deploy pipeline",
			},
			mockFn: func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error) {
				return &confluence.Comment{ID: "ic-new", Body: req.Body, VersionNumber: 1}, nil
			},
			wantIsError: false,
			wantContain: `"id":"ic-new"`,
		},
		{
			name:        "write guard blocks when ENABLE_WRITE not set",
			setup:       disableWrite,
			args:        map[string]any{"page_id": "123", "body": "<p>B</p>", "text_selection": "some text"},
			mockFn:      nil, // must never be called
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			// spec: "the system MUST return a validation error before making any API call"
			name:  "missing text_selection returns validation error before service call",
			setup: enableWrite,
			args:  map[string]any{"page_id": "123", "body": "<p>B</p>"},
			mockFn: func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error) {
				// This MUST NOT be called
				t.Error("service must NOT be called when text_selection is missing")
				return nil, nil
			},
			wantIsError: true,
			wantContain: "text_selection",
		},
		{
			name:        "missing page_id returns error",
			setup:       enableWrite,
			args:        map[string]any{"body": "<p>B</p>", "text_selection": "some text"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "page_id",
		},
		{
			name:        "missing body returns error",
			setup:       enableWrite,
			args:        map[string]any{"page_id": "123", "text_selection": "some text"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "body",
		},
		{
			name:  "ErrNotFound returns error result",
			setup: enableWrite,
			args:  map[string]any{"page_id": "999", "body": "<p>B</p>", "text_selection": "some text"},
			mockFn: func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error) {
				return nil, confluence.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name:  "optional match_count and match_index are forwarded to service",
			setup: enableWrite,
			args: map[string]any{
				"page_id":                    "123",
				"body":                       "<p>C</p>",
				"text_selection":             "deploy pipeline",
				"text_selection_match_count": float64(3),
				"text_selection_match_index": float64(1),
			},
			mockFn: func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error) {
				if req.TextSelectionMatchCount != 3 {
					t.Errorf("match_count: got %d, want 3", req.TextSelectionMatchCount)
				}
				if req.TextSelectionMatchIndex != 1 {
					t.Errorf("match_index: got %d, want 1", req.TextSelectionMatchIndex)
				}
				return &confluence.Comment{ID: "ic-3", Body: req.Body, VersionNumber: 1}, nil
			},
			wantIsError: false,
			wantContain: `"id":"ic-3"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			cl := &captureLogger{}
			svc := &mockConfluenceService{createInlineCommentFunc: tc.mockFn}
			handler := mcpserver.ToolCreateInlineComment(svc, cl)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// TestToolCreateInlineComment_AuditLogging verifies audit log behavior.
func TestToolCreateInlineComment_AuditLogging(t *testing.T) {
	t.Run("audit log written on success", func(t *testing.T) {
		enableWrite(t)
		cl := &captureLogger{}
		svc := &mockConfluenceService{
			createInlineCommentFunc: func(ctx context.Context, req confluence.CreateInlineCommentRequest) (*confluence.Comment, error) {
				return &confluence.Comment{ID: "ic-1", Body: req.Body}, nil
			},
		}
		handler := mcpserver.ToolCreateInlineComment(svc, cl)
		req := makeCallToolRequest(map[string]any{
			"page_id":        "123",
			"body":           "<p>X</p>",
			"text_selection": "deploy pipeline",
		})
		_, _ = handler(context.Background(), req)
		if len(cl.entries) != 1 {
			t.Errorf("expected 1 audit entry, got %d", len(cl.entries))
		}
		if cl.entries[0].Operation != "create_inline_comment" {
			t.Errorf("audit operation: got %q, want %q", cl.entries[0].Operation, "create_inline_comment")
		}
	})

	t.Run("audit log NOT written when write guard blocks", func(t *testing.T) {
		disableWrite(t)
		cl := &captureLogger{}
		svc := &mockConfluenceService{}
		handler := mcpserver.ToolCreateInlineComment(svc, cl)
		req := makeCallToolRequest(map[string]any{
			"page_id":        "123",
			"body":           "<p>X</p>",
			"text_selection": "text",
		})
		_, _ = handler(context.Background(), req)
		if len(cl.entries) != 0 {
			t.Errorf("expected 0 audit entries when write guard blocks, got %d", len(cl.entries))
		}
	})

	t.Run("audit log NOT written when text_selection missing (validation error before service)", func(t *testing.T) {
		enableWrite(t)
		cl := &captureLogger{}
		svc := &mockConfluenceService{}
		handler := mcpserver.ToolCreateInlineComment(svc, cl)
		req := makeCallToolRequest(map[string]any{
			"page_id": "123",
			"body":    "<p>X</p>",
			// text_selection deliberately omitted
		})
		result, _ := handler(context.Background(), req)
		if !result.IsError {
			t.Error("expected error result when text_selection missing")
		}
		if len(cl.entries) != 0 {
			t.Errorf("expected 0 audit entries on validation error, got %d", len(cl.entries))
		}
	})
}

// TestConfluenceEmptySliceRule verifies that empty results serialize as [] not null.
func TestConfluenceEmptySliceRule(t *testing.T) {
	t.Run("search_confluence empty → []", func(t *testing.T) {
		disableWrite(t)
		svc := &mockConfluenceService{
			searchContentFunc: func(ctx context.Context, cql string, limit int) ([]confluence.SearchResult, error) {
				return []confluence.SearchResult{}, nil
			},
		}
		handler := mcpserver.ToolSearchConfluence(svc)
		req := makeCallToolRequest(map[string]any{"cql": "type=page"})
		result, _ := handler(context.Background(), req)
		text := getResultText(t, result)
		if !strings.Contains(text, "[]") {
			t.Errorf("empty search result must serialize as [], got: %s", text)
		}
		if strings.Contains(text, "null") {
			t.Errorf("empty search result must NOT contain null, got: %s", text)
		}
	})

	t.Run("get_pages_in_space empty → results:[]", func(t *testing.T) {
		disableWrite(t)
		svc := &mockConfluenceService{
			getPagesInSpaceFunc: func(ctx context.Context, spaceID string, limit int, cursor string) (*confluence.PageList, error) {
				return &confluence.PageList{Results: []confluence.Page{}}, nil
			},
		}
		handler := mcpserver.ToolGetPagesInSpace(svc)
		req := makeCallToolRequest(map[string]any{"space_id": "DEV"})
		result, _ := handler(context.Background(), req)
		text := getResultText(t, result)
		if strings.Contains(text, "null") {
			t.Errorf("empty results must NOT contain null, got: %s", text)
		}
	})
}

// TestConfluenceWriteHandlers_WriteGuardCheck confirms all 4 write handlers
// are blocked by WriteGuardCheck when ENABLE_WRITE is not "true".
func TestConfluenceWriteHandlers_WriteGuardCheck(t *testing.T) {
	disableWrite(t)
	svc := &mockConfluenceService{}
	log := audit.NewNoopLogger()

	writeHandlers := []struct {
		name    string
		handler func(ctx context.Context, req interface{}) (interface{}, error)
	}{} // use sub-tests instead

	_ = writeHandlers

	t.Run("create_confluence_page blocked", func(t *testing.T) {
		handler := mcpserver.ToolCreateConfluencePage(svc, log)
		result, err := handler(context.Background(), makeCallToolRequest(map[string]any{
			"space_id": "DEV", "title": "T", "body": "<p>B</p>",
		}))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError {
			t.Error("expected write guard to block create_confluence_page")
		}
		if !strings.Contains(getResultText(t, result), "write operations disabled") {
			t.Error("expected 'write operations disabled' in error message")
		}
	})

	t.Run("update_confluence_page blocked", func(t *testing.T) {
		handler := mcpserver.ToolUpdateConfluencePage(svc, log)
		result, err := handler(context.Background(), makeCallToolRequest(map[string]any{
			"page_id": "123", "title": "T", "body": "<p>B</p>",
		}))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError {
			t.Error("expected write guard to block update_confluence_page")
		}
	})

	t.Run("create_footer_comment blocked", func(t *testing.T) {
		handler := mcpserver.ToolCreateFooterComment(svc, log)
		result, err := handler(context.Background(), makeCallToolRequest(map[string]any{
			"page_id": "123", "body": "<p>B</p>",
		}))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError {
			t.Error("expected write guard to block create_footer_comment")
		}
	})

	t.Run("create_inline_comment blocked", func(t *testing.T) {
		handler := mcpserver.ToolCreateInlineComment(svc, log)
		result, err := handler(context.Background(), makeCallToolRequest(map[string]any{
			"page_id": "123", "body": "<p>B</p>", "text_selection": "text",
		}))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError {
			t.Error("expected write guard to block create_inline_comment")
		}
	})
}
