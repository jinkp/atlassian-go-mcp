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
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
)

// mockProjectsService implements projects.ProjectsService for testing.
type mockProjectsService struct {
	getProjectsFunc    func(ctx context.Context, maxResults int) ([]projects.Project, error)
	getProjectFunc     func(ctx context.Context, projectKey string) (*projects.Project, error)
	searchProjectsFunc func(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error)
	updateProjectFunc  func(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error)
}

func (m *mockProjectsService) GetProjects(ctx context.Context, maxResults int) ([]projects.Project, error) {
	if m.getProjectsFunc != nil {
		return m.getProjectsFunc(ctx, maxResults)
	}
	return []projects.Project{}, nil
}
func (m *mockProjectsService) GetProject(ctx context.Context, projectKey string) (*projects.Project, error) {
	if m.getProjectFunc != nil {
		return m.getProjectFunc(ctx, projectKey)
	}
	return &projects.Project{Key: projectKey, Name: "Test Project"}, nil
}
func (m *mockProjectsService) SearchProjects(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error) {
	if m.searchProjectsFunc != nil {
		return m.searchProjectsFunc(ctx, req)
	}
	return &projects.SearchProjectsResult{Projects: []projects.Project{}, Total: 0}, nil
}
func (m *mockProjectsService) UpdateProject(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error) {
	if m.updateProjectFunc != nil {
		return m.updateProjectFunc(ctx, projectKey, req)
	}
	return &projects.Project{Key: projectKey}, nil
}

func TestProjectsSearchProjects(t *testing.T) {
	t.Run("success returns list", func(t *testing.T) {
		h := NewProjectsHandler(&mockProjectsService{
			searchProjectsFunc: func(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error) {
				return &projects.SearchProjectsResult{
					Projects: []projects.Project{{Key: "PROJ", Name: "My Project"}},
					Total:    1,
				}, nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("GET /projects", h.SearchProjects)

		req := httptest.NewRequest(http.MethodGet, "/projects?query=my", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"total":1`) {
			t.Errorf("body %q does not contain total:1", w.Body.String())
		}
	})
}

func TestProjectsGetProject(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		mockFn      func(ctx context.Context, projectKey string) (*projects.Project, error)
		wantStatus  int
		wantContain string
	}{
		{
			name: "success returns project",
			key:  "PROJ",
			mockFn: func(ctx context.Context, projectKey string) (*projects.Project, error) {
				return &projects.Project{Key: "PROJ", Name: "My Project"}, nil
			},
			wantStatus:  200,
			wantContain: "My Project",
		},
		{
			name: "not found returns 404",
			key:  "NOTEXIST",
			mockFn: func(ctx context.Context, projectKey string) (*projects.Project, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
		{
			name: "unauthorized returns 401",
			key:  "PROJ",
			mockFn: func(ctx context.Context, projectKey string) (*projects.Project, error) {
				return nil, jira.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewProjectsHandler(&mockProjectsService{getProjectFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /projects/{key}", h.GetProject)

			req := httptest.NewRequest(http.MethodGet, "/projects/"+tc.key, nil)
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

func TestProjectsUpdateProject(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		body        map[string]any
		mockFn      func(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error)
		wantStatus  int
		wantContain string
	}{
		{
			name: "success returns updated project",
			key:  "PROJ",
			body: map[string]any{"name": "New Name"},
			mockFn: func(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error) {
				return &projects.Project{Key: projectKey, Name: "New Name"}, nil
			},
			wantStatus:  200,
			wantContain: "New Name",
		},
		{
			name: "not found returns 404",
			key:  "NOTEXIST",
			body: map[string]any{"name": "x"},
			mockFn: func(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewProjectsHandler(&mockProjectsService{updateProjectFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("PUT /projects/{key}", h.UpdateProject)

			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPut, "/projects/"+tc.key, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
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
