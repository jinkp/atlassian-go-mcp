package mcpserver_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

// mockProjectsService implements projects.ProjectsService for testing.
// Each method delegates to a stored func field; guards against nil with safe defaults.
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
	return &projects.Project{}, nil
}

func (m *mockProjectsService) SearchProjects(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error) {
	if m.searchProjectsFunc != nil {
		return m.searchProjectsFunc(ctx, req)
	}
	return &projects.SearchProjectsResult{Projects: []projects.Project{}}, nil
}

func (m *mockProjectsService) UpdateProject(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error) {
	if m.updateProjectFunc != nil {
		return m.updateProjectFunc(ctx, projectKey, req)
	}
	return &projects.Project{}, nil
}

// --- TestToolListProjects ---

func TestToolListProjects(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, maxResults int) ([]projects.Project, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns projects JSON array with key field",
			args: map[string]any{},
			mockFn: func(ctx context.Context, maxResults int) ([]projects.Project, error) {
				return []projects.Project{
					{ID: "10000", Key: "PROJ", Name: "My Project", ProjectType: "software"},
					{ID: "10001", Key: "PROJ2", Name: "Second Project", ProjectType: "business"},
				}, nil
			},
			wantIsError: false,
			wantContain: `"key"`,
		},
		{
			name: "max_results defaults to 50 when not provided",
			args: map[string]any{},
			mockFn: func(ctx context.Context, maxResults int) ([]projects.Project, error) {
				if maxResults != 50 {
					t.Errorf("expected maxResults=50, got %d", maxResults)
				}
				return []projects.Project{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
		{
			name: "max_results=25 passed through to service",
			args: map[string]any{"max_results": float64(25)},
			mockFn: func(ctx context.Context, maxResults int) ([]projects.Project, error) {
				if maxResults != 25 {
					t.Errorf("expected maxResults=25, got %d", maxResults)
				}
				return []projects.Project{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
		{
			name: "service ErrUnauthorized returns IsError true",
			args: map[string]any{},
			mockFn: func(ctx context.Context, maxResults int) ([]projects.Project, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name: "service generic error returns IsError true",
			args: map[string]any{},
			mockFn: func(ctx context.Context, maxResults int) ([]projects.Project, error) {
				return nil, fmt.Errorf("network timeout")
			},
			wantIsError: true,
			wantContain: "network timeout",
		},
		{
			name: "empty list returns valid JSON array",
			args: map[string]any{},
			mockFn: func(ctx context.Context, maxResults int) ([]projects.Project, error) {
				return []projects.Project{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockProjectsService{getProjectsFunc: tc.mockFn}
			handler := mcpserver.ToolListProjects(svc)
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

// --- TestToolGetProject ---

func TestToolGetProject(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, projectKey string) (*projects.Project, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns project JSON with key field",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string) (*projects.Project, error) {
				return &projects.Project{ID: "10000", Key: "PROJ", Name: "My Project", ProjectType: "software"}, nil
			},
			wantIsError: false,
			wantContain: `"key"`,
		},
		{
			name: "project_key passed to service correctly",
			args: map[string]any{"project_key": "MYPROJ"},
			mockFn: func(ctx context.Context, projectKey string) (*projects.Project, error) {
				if projectKey != "MYPROJ" {
					t.Errorf("projectKey: got %q, want MYPROJ", projectKey)
				}
				return &projects.Project{ID: "10000", Key: "MYPROJ", Name: "My Project"}, nil
			},
			wantIsError: false,
			wantContain: "MYPROJ",
		},
		{
			name:        "missing project_key returns IsError true",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "project_key",
		},
		{
			name: "service ErrNotFound returns IsError true",
			args: map[string]any{"project_key": "MISSING"},
			mockFn: func(ctx context.Context, projectKey string) (*projects.Project, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name: "service ErrUnauthorized returns IsError true",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string) (*projects.Project, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockProjectsService{getProjectFunc: tc.mockFn}
			handler := mcpserver.ToolGetProject(svc)
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

// --- TestToolSearchProjects ---

func TestToolSearchProjects(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "no args returns result with total field",
			args: map[string]any{},
			mockFn: func(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error) {
				return &projects.SearchProjectsResult{
					Projects:   []projects.Project{{ID: "10000", Key: "PROJ", Name: "My Project"}},
					Total:      1,
					MaxResults: 50,
				}, nil
			},
			wantIsError: false,
			wantContain: "total",
		},
		{
			name: "max_results defaults to 50 when not provided",
			args: map[string]any{},
			mockFn: func(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error) {
				if req.MaxResults != 50 {
					t.Errorf("expected MaxResults=50, got %d", req.MaxResults)
				}
				return &projects.SearchProjectsResult{Projects: []projects.Project{}, Total: 0, MaxResults: 50}, nil
			},
			wantIsError: false,
			wantContain: "total",
		},
		{
			name: "query and max_results forwarded to service",
			args: map[string]any{"query": "PROJ", "max_results": float64(10)},
			mockFn: func(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error) {
				if req.Query != "PROJ" {
					t.Errorf("Query: got %q, want PROJ", req.Query)
				}
				if req.MaxResults != 10 {
					t.Errorf("MaxResults: got %d, want 10", req.MaxResults)
				}
				return &projects.SearchProjectsResult{Projects: []projects.Project{}, Total: 0, MaxResults: 10}, nil
			},
			wantIsError: false,
			wantContain: "total",
		},
		{
			name: "service ErrUnauthorized returns IsError true",
			args: map[string]any{},
			mockFn: func(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name: "empty results returns IsError false with total field",
			args: map[string]any{},
			mockFn: func(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error) {
				return &projects.SearchProjectsResult{Projects: []projects.Project{}, Total: 0, MaxResults: 50}, nil
			},
			wantIsError: false,
			wantContain: "total",
		},
		{
			name: "handler never returns Go error",
			args: map[string]any{},
			mockFn: func(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error) {
				return nil, fmt.Errorf("some error")
			},
			wantIsError: true,
			wantContain: "some error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockProjectsService{searchProjectsFunc: tc.mockFn}
			handler := mcpserver.ToolSearchProjects(svc)
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

// --- TestToolUpdateProject ---

func TestToolUpdateProject(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:        "WriteGuard blocked when ENABLE_WRITE unset",
			args:        map[string]any{"project_key": "PROJ"},
			envWrite:    "",
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing project_key returns IsError true",
			args:        map[string]any{},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "project_key",
		},
		{
			name:     "success with name update returns project JSON",
			args:     map[string]any{"project_key": "PROJ", "name": "New Name"},
			envWrite: "true",
			mockFn: func(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error) {
				if projectKey != "PROJ" {
					t.Errorf("projectKey: got %q, want PROJ", projectKey)
				}
				if req.Name == nil || *req.Name != "New Name" {
					t.Errorf("req.Name: got %v, want New Name", req.Name)
				}
				return &projects.Project{ID: "10000", Key: "PROJ", Name: "New Name"}, nil
			},
			wantIsError: false,
			wantContain: `"key"`,
		},
		{
			name:     "lead field passed as UpdateProjectRequest.Lead pointer",
			args:     map[string]any{"project_key": "PROJ", "lead": "acc123"},
			envWrite: "true",
			mockFn: func(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error) {
				if req.Lead == nil || *req.Lead != "acc123" {
					t.Errorf("req.Lead: got %v, want acc123", req.Lead)
				}
				return &projects.Project{ID: "10000", Key: "PROJ", Lead: "acc123"}, nil
			},
			wantIsError: false,
			wantContain: `"key"`,
		},
		{
			name:     "optional fields not provided — fields remain nil",
			args:     map[string]any{"project_key": "PROJ"},
			envWrite: "true",
			mockFn: func(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error) {
				if req.Name != nil {
					t.Errorf("Name should be nil, got %v", req.Name)
				}
				if req.Description != nil {
					t.Errorf("Description should be nil, got %v", req.Description)
				}
				if req.Lead != nil {
					t.Errorf("Lead should be nil, got %v", req.Lead)
				}
				return &projects.Project{ID: "10000", Key: "PROJ"}, nil
			},
			wantIsError: false,
			wantContain: `"key"`,
		},
		{
			name:     "service ErrNotFound returns IsError true",
			args:     map[string]any{"project_key": "MISSING"},
			envWrite: "true",
			mockFn: func(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name:     "service ErrUnauthorized returns IsError true",
			args:     map[string]any{"project_key": "PROJ"},
			envWrite: "true",
			mockFn: func(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}
			svc := &mockProjectsService{updateProjectFunc: tc.mockFn}
			handler := mcpserver.ToolUpdateProject(svc, audit.NewNoopLogger())
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

// TestToolListProjects_MaxResultsPassthrough verifies max_results passes correctly.
func TestToolListProjects_MaxResultsPassthrough(t *testing.T) {
	var gotMaxResults int
	svc := &mockProjectsService{
		getProjectsFunc: func(ctx context.Context, maxResults int) ([]projects.Project, error) {
			gotMaxResults = maxResults
			return []projects.Project{}, nil
		},
	}
	handler := mcpserver.ToolListProjects(svc)
	req := makeCallToolRequest(map[string]any{"max_results": float64(15)})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if gotMaxResults != 15 {
		t.Errorf("maxResults: got %d, want 15", gotMaxResults)
	}
}

// TestToolSearchProjects_QueryPassthrough verifies query is forwarded.
func TestToolSearchProjects_QueryPassthrough(t *testing.T) {
	var gotReq projects.SearchProjectsRequest
	svc := &mockProjectsService{
		searchProjectsFunc: func(ctx context.Context, req projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error) {
			gotReq = req
			return &projects.SearchProjectsResult{Projects: []projects.Project{}, Total: 0}, nil
		},
	}
	handler := mcpserver.ToolSearchProjects(svc)
	req := makeCallToolRequest(map[string]any{"query": "engineering", "max_results": float64(20)})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if gotReq.Query != "engineering" {
		t.Errorf("Query: got %q, want engineering", gotReq.Query)
	}
	if gotReq.MaxResults != 20 {
		t.Errorf("MaxResults: got %d, want 20", gotReq.MaxResults)
	}
}

// TestToolUpdateProject_DescriptionPassthrough verifies description is forwarded.
func TestToolUpdateProject_DescriptionPassthrough(t *testing.T) {
	t.Setenv("ENABLE_WRITE", "true")
	var gotReq projects.UpdateProjectRequest
	svc := &mockProjectsService{
		updateProjectFunc: func(ctx context.Context, projectKey string, req projects.UpdateProjectRequest) (*projects.Project, error) {
			gotReq = req
			return &projects.Project{ID: "10000", Key: "PROJ"}, nil
		},
	}
	handler := mcpserver.ToolUpdateProject(svc, audit.NewNoopLogger())
	req := makeCallToolRequest(map[string]any{
		"project_key": "PROJ",
		"description": "Updated description",
	})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if gotReq.Description == nil || *gotReq.Description != "Updated description" {
		t.Errorf("Description: got %v, want 'Updated description'", gotReq.Description)
	}
}
