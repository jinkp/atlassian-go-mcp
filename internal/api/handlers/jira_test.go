package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// mockJiraService implements jira.Service for testing.
type mockJiraService struct {
	getIssueFunc        func(ctx context.Context, key string) (*jira.Issue, error)
	searchIssuesFunc    func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error)
	createIssueFunc     func(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error)
	updateIssueFunc     func(ctx context.Context, key string, req jira.UpdateIssueRequest) error
	getTransitionsFunc  func(ctx context.Context, key string) ([]jira.Transition, error)
	transitionIssueFunc func(ctx context.Context, key string, transitionID string) error
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
