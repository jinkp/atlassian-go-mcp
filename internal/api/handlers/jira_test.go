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

	"github.com/jinkp/atlassian-go-mcp/internal/api"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// writeGuardForTest wraps a mux with WriteGuardMiddleware, matching the production setup.
// readOnly=false means write is allowed when X-Enable-Write: true header is present.
func writeGuardForTest(readOnly bool, next http.Handler) http.Handler {
	return api.WriteGuardMiddleware(readOnly, next)
}

// mockJiraService implements jira.Service for testing.
type mockJiraService struct {
	getIssueFunc              func(ctx context.Context, key string) (*jira.Issue, error)
	searchIssuesFunc          func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error)
	createIssueFunc           func(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error)
	updateIssueFunc           func(ctx context.Context, key string, req jira.UpdateIssueRequest) error
	getTransitionsFunc        func(ctx context.Context, key string) ([]jira.Transition, error)
	transitionIssueFunc       func(ctx context.Context, key string, transitionID string) error
	lookupAccountIDFunc       func(ctx context.Context, query string, maxResults int) ([]jira.User, error)
	addCommentFunc            func(ctx context.Context, key string, body string) (*jira.Comment, error)
	getCommentsFunc           func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error)
	linkIssuesFunc            func(ctx context.Context, inward, outward, linkTypeName string) error
	getIssueLinkTypesFunc     func(ctx context.Context) ([]jira.IssueLinkType, error)
	addWorklogFunc            func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error)
	getIssueTypeMetadataFunc  func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error)
}

func (m *mockJiraService) GetIssue(ctx context.Context, key string) (*jira.Issue, error) {
	if m.getIssueFunc != nil {
		return m.getIssueFunc(ctx, key)
	}
	return nil, nil
}
func (m *mockJiraService) SearchIssues(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
	if m.searchIssuesFunc != nil {
		return m.searchIssuesFunc(ctx, jql, maxResults)
	}
	return &jira.SearchResult{Issues: []jira.Issue{}, Total: 0}, nil
}
func (m *mockJiraService) CreateIssue(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error) {
	if m.createIssueFunc != nil {
		return m.createIssueFunc(ctx, req)
	}
	return nil, nil
}
func (m *mockJiraService) UpdateIssue(ctx context.Context, key string, req jira.UpdateIssueRequest) error {
	if m.updateIssueFunc != nil {
		return m.updateIssueFunc(ctx, key, req)
	}
	return nil
}
func (m *mockJiraService) GetTransitions(ctx context.Context, key string) ([]jira.Transition, error) {
	if m.getTransitionsFunc != nil {
		return m.getTransitionsFunc(ctx, key)
	}
	return []jira.Transition{}, nil
}
func (m *mockJiraService) TransitionIssue(ctx context.Context, key string, transitionID string) error {
	if m.transitionIssueFunc != nil {
		return m.transitionIssueFunc(ctx, key, transitionID)
	}
	return nil
}
func (m *mockJiraService) LookupAccountID(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
	if m.lookupAccountIDFunc != nil {
		return m.lookupAccountIDFunc(ctx, query, maxResults)
	}
	return []jira.User{}, nil
}
func (m *mockJiraService) AddComment(ctx context.Context, key string, body string) (*jira.Comment, error) {
	if m.addCommentFunc != nil {
		return m.addCommentFunc(ctx, key, body)
	}
	return nil, nil
}
func (m *mockJiraService) GetComments(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
	if m.getCommentsFunc != nil {
		return m.getCommentsFunc(ctx, key, maxResults)
	}
	return []jira.Comment{}, nil
}
func (m *mockJiraService) LinkIssues(ctx context.Context, inward, outward, linkTypeName string) error {
	if m.linkIssuesFunc != nil {
		return m.linkIssuesFunc(ctx, inward, outward, linkTypeName)
	}
	return nil
}
func (m *mockJiraService) GetIssueLinkTypes(ctx context.Context) ([]jira.IssueLinkType, error) {
	if m.getIssueLinkTypesFunc != nil {
		return m.getIssueLinkTypesFunc(ctx)
	}
	return []jira.IssueLinkType{}, nil
}
func (m *mockJiraService) AddWorklog(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
	if m.addWorklogFunc != nil {
		return m.addWorklogFunc(ctx, key, req)
	}
	return nil, nil
}
func (m *mockJiraService) GetIssueTypeMetadata(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
	if m.getIssueTypeMetadataFunc != nil {
		return m.getIssueTypeMetadataFunc(ctx, projectKey)
	}
	return []jira.IssueTypeMeta{}, nil
}

// captureLogger records audit entries.
type captureLogger struct{ entries []audit.Entry }

func (c *captureLogger) Log(e audit.Entry) { c.entries = append(c.entries, e) }

func TestJiraGetIssue(t *testing.T) {
	tests := []struct {
		name        string
		pathKey     string
		mockFn      func(ctx context.Context, key string) (*jira.Issue, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:    "success returns issue JSON",
			pathKey: "PROJ-1",
			mockFn: func(ctx context.Context, key string) (*jira.Issue, error) {
				return &jira.Issue{Key: "PROJ-1", Summary: "Fix login", Labels: []string{}}, nil
			},
			wantStatus:  200,
			wantContain: `"Key":"PROJ-1"`,
		},
		{
			name:    "not found returns 404",
			pathKey: "PROJ-999",
			mockFn: func(ctx context.Context, key string) (*jira.Issue, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
		{
			name:    "unauthorized returns 401",
			pathKey: "PROJ-1",
			mockFn: func(ctx context.Context, key string) (*jira.Issue, error) {
				return nil, jira.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewJiraHandler(&mockJiraService{getIssueFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /jira/issues/{key}", h.GetIssue)

			req := httptest.NewRequest(http.MethodGet, "/jira/issues/"+tc.pathKey, nil)
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

func TestJiraSearchIssues(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		mockFn      func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:  "success returns list response",
			query: "jql=project%3DPROJ",
			mockFn: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
				return &jira.SearchResult{
					Issues: []jira.Issue{{Key: "PROJ-1", Labels: []string{}}},
					Total:  1,
				}, nil
			},
			wantStatus:  200,
			wantContain: `"total":1`,
		},
		{
			name:  "invalid JQL returns 400",
			query: "jql=INVALID",
			mockFn: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
				return nil, jira.ErrInvalidJQL
			},
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewJiraHandler(&mockJiraService{searchIssuesFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /jira/issues", h.SearchIssues)

			req := httptest.NewRequest(http.MethodGet, "/jira/issues?"+tc.query, nil)
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

func TestJiraCreateIssue(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]any
		mockFn      func(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error)
		wantStatus  int
		wantContain string
		wantAudit   bool
	}{
		{
			name: "success returns 201 with key",
			body: map[string]any{"project_key": "PROJ", "issue_type": "Task", "summary": "New task"},
			mockFn: func(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error) {
				return &jira.CreateIssueResponse{Key: "PROJ-42", ID: "10042"}, nil
			},
			wantStatus:  201,
			wantContain: "PROJ-42",
			wantAudit:   true,
		},
		{
			name:        "missing project_key returns 400",
			body:        map[string]any{"issue_type": "Task", "summary": "x"},
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
			wantAudit:   false,
		},
		{
			name: "unauthorized returns 401 with audit",
			body: map[string]any{"project_key": "PROJ", "issue_type": "Task", "summary": "x"},
			mockFn: func(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error) {
				return nil, jira.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
			wantAudit:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := &captureLogger{}
			h := NewJiraHandler(&mockJiraService{createIssueFunc: tc.mockFn}, logger)

			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/jira/issues", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.CreateIssue(w, req)

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

func TestJiraUpdateIssue(t *testing.T) {
	tests := []struct {
		name        string
		pathKey     string
		body        map[string]any
		mockFn      func(ctx context.Context, key string, req jira.UpdateIssueRequest) error
		wantStatus  int
		wantContain string
	}{
		{
			name:    "success returns updated issue",
			pathKey: "PROJ-1",
			body:    map[string]any{"summary": "Updated"},
			mockFn: func(ctx context.Context, key string, req jira.UpdateIssueRequest) error {
				return nil
			},
			wantStatus:  200,
			wantContain: "updated",
		},
		{
			name:    "not found returns 404",
			pathKey: "PROJ-999",
			body:    map[string]any{"summary": "x"},
			mockFn: func(ctx context.Context, key string, req jira.UpdateIssueRequest) error {
				return jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewJiraHandler(&mockJiraService{updateIssueFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("PUT /jira/issues/{key}", h.UpdateIssue)

			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPut, "/jira/issues/"+tc.pathKey, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(strings.ToLower(w.Body.String()), strings.ToLower(tc.wantContain)) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

func TestJiraGetTransitions(t *testing.T) {
	tests := []struct {
		name        string
		pathKey     string
		mockFn      func(ctx context.Context, key string) ([]jira.Transition, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:    "success returns transitions",
			pathKey: "PROJ-1",
			mockFn: func(ctx context.Context, key string) ([]jira.Transition, error) {
				return []jira.Transition{{ID: "11", Name: "In Progress", StatusCategory: "indeterminate"}}, nil
			},
			wantStatus:  200,
			wantContain: `"id":"11"`,
		},
		{
			name:    "not found returns 404",
			pathKey: "PROJ-999",
			mockFn: func(ctx context.Context, key string) ([]jira.Transition, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewJiraHandler(&mockJiraService{getTransitionsFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /jira/issues/{key}/transitions", h.GetTransitions)

			req := httptest.NewRequest(http.MethodGet, "/jira/issues/"+tc.pathKey+"/transitions", nil)
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

func TestJiraTransitionIssue(t *testing.T) {
	tests := []struct {
		name        string
		pathKey     string
		body        map[string]any
		mockFn      func(ctx context.Context, key string, transitionID string) error
		wantStatus  int
		wantContain string
		wantAudit   bool
	}{
		{
			name:    "success returns status transitioned",
			pathKey: "PROJ-1",
			body:    map[string]any{"transition_id": "21"},
			mockFn: func(ctx context.Context, key string, transitionID string) error {
				return nil
			},
			wantStatus:  200,
			wantContain: "transitioned",
			wantAudit:   true,
		},
		{
			name:        "missing transition_id returns 400",
			pathKey:     "PROJ-1",
			body:        map[string]any{},
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
			wantAudit:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := &captureLogger{}
			h := NewJiraHandler(&mockJiraService{transitionIssueFunc: tc.mockFn}, logger)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /jira/issues/{key}/transitions", h.TransitionIssue)

			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/jira/issues/"+tc.pathKey+"/transitions", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(strings.ToLower(w.Body.String()), strings.ToLower(tc.wantContain)) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
			if tc.wantAudit && len(logger.entries) == 0 {
				t.Error("expected audit entry, got none")
			}
		})
	}
}

// --- Block 3 tests: 7 new handlers ---

func TestJiraSearchUsers(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		mockFn      func(ctx context.Context, query string, maxResults int) ([]jira.User, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:  "success returns user array",
			query: "query=Jane",
			mockFn: func(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
				return []jira.User{
					{AccountID: "abc123", DisplayName: "Jane Doe", Email: "jane@example.com", Active: true},
				}, nil
			},
			wantStatus:  200,
			wantContain: "abc123",
		},
		{
			name:  "empty result returns empty array",
			query: "query=NoMatch",
			mockFn: func(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
				return []jira.User{}, nil
			},
			wantStatus:  200,
			wantContain: "[]",
		},
		{
			name:        "missing query returns 400",
			query:       "",
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
		},
		{
			name:  "unauthorized returns 401",
			query: "query=Jane",
			mockFn: func(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
				return nil, jira.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name:  "rate limited returns 429",
			query: "query=Jane",
			mockFn: func(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
				return nil, jira.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewJiraHandler(&mockJiraService{lookupAccountIDFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /jira/users/search", h.SearchUsers)

			url := "/jira/users/search"
			if tc.query != "" {
				url += "?" + tc.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
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

func TestJiraAddComment(t *testing.T) {
	tests := []struct {
		name        string
		pathKey     string
		body        map[string]any
		writeHeader bool // set X-Enable-Write: true
		mockFn      func(ctx context.Context, key string, body string) (*jira.Comment, error)
		wantStatus  int
		wantContain string
		wantAudit   bool
	}{
		{
			name:        "success returns 201 with comment",
			pathKey:     "PROJ-1",
			body:        map[string]any{"body": "Looks good"},
			writeHeader: true,
			mockFn: func(ctx context.Context, key string, body string) (*jira.Comment, error) {
				return &jira.Comment{ID: "100", Author: "Alice", Body: "Looks good", Created: time.Time{}}, nil
			},
			wantStatus:  201,
			wantContain: `"100"`,
			wantAudit:   true,
		},
		{
			name:        "missing body field returns 400",
			pathKey:     "PROJ-1",
			body:        map[string]any{},
			writeHeader: true,
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
			wantAudit:   false,
		},
		{
			name:        "not found returns 404",
			pathKey:     "PROJ-999",
			body:        map[string]any{"body": "hi"},
			writeHeader: true,
			mockFn: func(ctx context.Context, key string, body string) (*jira.Comment, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
			wantAudit:   true,
		},
		{
			name:        "unauthorized returns 401",
			pathKey:     "PROJ-1",
			body:        map[string]any{"body": "hi"},
			writeHeader: true,
			mockFn: func(ctx context.Context, key string, body string) (*jira.Comment, error) {
				return nil, jira.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
			wantAudit:   true,
		},
		{
			name:        "rate limited returns 429",
			pathKey:     "PROJ-1",
			body:        map[string]any{"body": "hi"},
			writeHeader: true,
			mockFn: func(ctx context.Context, key string, body string) (*jira.Comment, error) {
				return nil, jira.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
			wantAudit:   true,
		},
		{
			// Write-guard is enforced by WriteGuardMiddleware at the mux level (not the handler).
			// Without X-Enable-Write header the middleware returns 403 before the handler runs.
			name:        "write guard blocks when no header",
			pathKey:     "PROJ-1",
			body:        map[string]any{"body": "hi"},
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
			h := NewJiraHandler(&mockJiraService{addCommentFunc: tc.mockFn}, logger)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /jira/issues/{key}/comments", h.AddComment)

			// Wrap with WriteGuardMiddleware to test the guard exactly as in production.
			handler := writeGuardForTest(false, mux)

			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/jira/issues/"+tc.pathKey+"/comments", bytes.NewReader(bodyBytes))
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

func TestJiraGetComments(t *testing.T) {
	tests := []struct {
		name        string
		pathKey     string
		mockFn      func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:    "success returns comment array",
			pathKey: "PROJ-1",
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return []jira.Comment{
					{ID: "10", Author: "Bob", Body: "First comment"},
					{ID: "11", Author: "Alice", Body: "Second comment"},
				}, nil
			},
			wantStatus:  200,
			wantContain: `"10"`,
		},
		{
			name:    "empty comments returns empty array",
			pathKey: "PROJ-1",
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return []jira.Comment{}, nil
			},
			wantStatus:  200,
			wantContain: "[]",
		},
		{
			name:    "not found returns 404",
			pathKey: "PROJ-999",
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
		{
			name:    "unauthorized returns 401",
			pathKey: "PROJ-1",
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return nil, jira.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name:    "rate limited returns 429",
			pathKey: "PROJ-1",
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return nil, jira.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewJiraHandler(&mockJiraService{getCommentsFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /jira/issues/{key}/comments", h.GetComments)

			req := httptest.NewRequest(http.MethodGet, "/jira/issues/"+tc.pathKey+"/comments", nil)
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

func TestJiraLinkIssues(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]any
		writeHeader bool
		mockFn      func(ctx context.Context, inward, outward, linkTypeName string) error
		wantStatus  int
		wantContain string
		wantAudit   bool
	}{
		{
			name:        "success returns 201 linked",
			body:        map[string]any{"inward_issue": "PROJ-1", "outward_issue": "PROJ-2", "link_type": "Blocks"},
			writeHeader: true,
			mockFn: func(ctx context.Context, inward, outward, linkTypeName string) error {
				return nil
			},
			wantStatus:  201,
			wantContain: "linked",
			wantAudit:   true,
		},
		{
			name:        "missing fields returns 400",
			body:        map[string]any{"inward_issue": "PROJ-1"},
			writeHeader: true,
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
			wantAudit:   false,
		},
		{
			name:        "not found returns 404",
			body:        map[string]any{"inward_issue": "PROJ-999", "outward_issue": "PROJ-2", "link_type": "Blocks"},
			writeHeader: true,
			mockFn: func(ctx context.Context, inward, outward, linkTypeName string) error {
				return jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
			wantAudit:   true,
		},
		{
			name:        "unauthorized returns 401",
			body:        map[string]any{"inward_issue": "PROJ-1", "outward_issue": "PROJ-2", "link_type": "Blocks"},
			writeHeader: true,
			mockFn: func(ctx context.Context, inward, outward, linkTypeName string) error {
				return jira.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
			wantAudit:   true,
		},
		{
			name:        "rate limited returns 429",
			body:        map[string]any{"inward_issue": "PROJ-1", "outward_issue": "PROJ-2", "link_type": "Blocks"},
			writeHeader: true,
			mockFn: func(ctx context.Context, inward, outward, linkTypeName string) error {
				return jira.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
			wantAudit:   true,
		},
		{
			name:        "write guard blocks when no header",
			body:        map[string]any{"inward_issue": "PROJ-1", "outward_issue": "PROJ-2", "link_type": "Blocks"},
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
			h := NewJiraHandler(&mockJiraService{linkIssuesFunc: tc.mockFn}, logger)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /jira/issues/links", h.LinkIssues)
			handler := writeGuardForTest(false, mux)

			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/jira/issues/links", bytes.NewReader(bodyBytes))
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

func TestJiraGetIssueLinkTypes(t *testing.T) {
	tests := []struct {
		name        string
		mockFn      func(ctx context.Context) ([]jira.IssueLinkType, error)
		wantStatus  int
		wantContain string
	}{
		{
			name: "success returns link types array",
			mockFn: func(ctx context.Context) ([]jira.IssueLinkType, error) {
				return []jira.IssueLinkType{
					{ID: "1", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
				}, nil
			},
			wantStatus:  200,
			wantContain: "Blocks",
		},
		{
			name: "unauthorized returns 401",
			mockFn: func(ctx context.Context) ([]jira.IssueLinkType, error) {
				return nil, jira.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name: "rate limited returns 429",
			mockFn: func(ctx context.Context) ([]jira.IssueLinkType, error) {
				return nil, jira.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewJiraHandler(&mockJiraService{getIssueLinkTypesFunc: tc.mockFn}, audit.NewNoopLogger())

			req := httptest.NewRequest(http.MethodGet, "/jira/issues/link-types", nil)
			w := httptest.NewRecorder()
			h.GetIssueLinkTypes(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

func TestJiraAddWorklog(t *testing.T) {
	tests := []struct {
		name        string
		pathKey     string
		body        map[string]any
		writeHeader bool
		mockFn      func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error)
		wantStatus  int
		wantContain string
		wantAudit   bool
	}{
		{
			name:        "success returns 201 with worklog",
			pathKey:     "PROJ-1",
			body:        map[string]any{"time_spent": "2h"},
			writeHeader: true,
			mockFn: func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
				return &jira.Worklog{ID: "200", TimeSpentSeconds: 7200, Author: "Alice"}, nil
			},
			wantStatus:  201,
			wantContain: `"200"`,
			wantAudit:   true,
		},
		{
			name:        "missing time_spent returns 400",
			pathKey:     "PROJ-1",
			body:        map[string]any{"comment": "no time"},
			writeHeader: true,
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
			wantAudit:   false,
		},
		{
			name:        "not found returns 404",
			pathKey:     "PROJ-999",
			body:        map[string]any{"time_spent": "1h"},
			writeHeader: true,
			mockFn: func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
			wantAudit:   true,
		},
		{
			name:        "unauthorized returns 401",
			pathKey:     "PROJ-1",
			body:        map[string]any{"time_spent": "1h"},
			writeHeader: true,
			mockFn: func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
				return nil, jira.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
			wantAudit:   true,
		},
		{
			name:        "rate limited returns 429",
			pathKey:     "PROJ-1",
			body:        map[string]any{"time_spent": "1h"},
			writeHeader: true,
			mockFn: func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
				return nil, jira.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
			wantAudit:   true,
		},
		{
			name:        "write guard blocks when no header",
			pathKey:     "PROJ-1",
			body:        map[string]any{"time_spent": "1h"},
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
			h := NewJiraHandler(&mockJiraService{addWorklogFunc: tc.mockFn}, logger)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /jira/issues/{key}/worklogs", h.AddWorklog)
			handler := writeGuardForTest(false, mux)

			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/jira/issues/"+tc.pathKey+"/worklogs", bytes.NewReader(bodyBytes))
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

func TestJiraGetIssueTypeMetadata(t *testing.T) {
	tests := []struct {
		name        string
		pathKey     string
		mockFn      func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:    "success returns issue types array",
			pathKey: "PROJ",
			mockFn: func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
				return []jira.IssueTypeMeta{
					{ID: "1", Name: "Story", Desc: "A user story", Subtask: false},
					{ID: "2", Name: "Sub-task", Desc: "A sub-task", Subtask: true},
				}, nil
			},
			wantStatus:  200,
			wantContain: "Story",
		},
		{
			name:    "project not found returns 404",
			pathKey: "NOTEXIST",
			mockFn: func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
		{
			name:    "unauthorized returns 401",
			pathKey: "PROJ",
			mockFn: func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
				return nil, jira.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
		{
			name:    "rate limited returns 429",
			pathKey: "PROJ",
			mockFn: func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
				return nil, jira.ErrRateLimit
			},
			wantStatus:  429,
			wantContain: `"code":"RATE_LIMITED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewJiraHandler(&mockJiraService{getIssueTypeMetadataFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /jira/projects/{key}/issue-types", h.GetIssueTypeMetadata)

			req := httptest.NewRequest(http.MethodGet, "/jira/projects/"+tc.pathKey+"/issue-types", nil)
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
