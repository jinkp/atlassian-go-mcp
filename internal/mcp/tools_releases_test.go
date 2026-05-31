package mcpserver_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

// mockReleasesService implements releases.ReleasesService for testing.
// Each method delegates to a stored func field; guards against nil with safe defaults.
type mockReleasesService struct {
	getReleasesFunc          func(ctx context.Context, projectKey string) ([]releases.Release, error)
	getReleaseFunc           func(ctx context.Context, releaseID string) (*releases.Release, error)
	getReleaseIssueCountsFunc func(ctx context.Context, releaseID string) (*releases.ReleaseIssueCounts, error)
	createReleaseFunc        func(ctx context.Context, req releases.CreateReleaseRequest) (*releases.Release, error)
	updateReleaseFunc        func(ctx context.Context, releaseID string, req releases.UpdateReleaseRequest) (*releases.Release, error)
}

func (m *mockReleasesService) GetReleases(ctx context.Context, projectKey string) ([]releases.Release, error) {
	if m.getReleasesFunc != nil {
		return m.getReleasesFunc(ctx, projectKey)
	}
	return []releases.Release{}, nil
}

func (m *mockReleasesService) GetRelease(ctx context.Context, releaseID string) (*releases.Release, error) {
	if m.getReleaseFunc != nil {
		return m.getReleaseFunc(ctx, releaseID)
	}
	return &releases.Release{}, nil
}

func (m *mockReleasesService) GetReleaseIssueCounts(ctx context.Context, releaseID string) (*releases.ReleaseIssueCounts, error) {
	if m.getReleaseIssueCountsFunc != nil {
		return m.getReleaseIssueCountsFunc(ctx, releaseID)
	}
	return &releases.ReleaseIssueCounts{}, nil
}

func (m *mockReleasesService) CreateRelease(ctx context.Context, req releases.CreateReleaseRequest) (*releases.Release, error) {
	if m.createReleaseFunc != nil {
		return m.createReleaseFunc(ctx, req)
	}
	return &releases.Release{}, nil
}

func (m *mockReleasesService) UpdateRelease(ctx context.Context, releaseID string, req releases.UpdateReleaseRequest) (*releases.Release, error) {
	if m.updateReleaseFunc != nil {
		return m.updateReleaseFunc(ctx, releaseID, req)
	}
	return &releases.Release{}, nil
}

// --- TestToolSearchReleases ---

func TestToolSearchReleases(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, projectKey string) ([]releases.Release, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns releases JSON array",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string) ([]releases.Release, error) {
				return []releases.Release{
					{ID: "10001", Name: "v1.0", Released: true, ProjectID: "10000"},
				}, nil
			},
			wantIsError: false,
			wantContain: `"id"`,
		},
		{
			name: "returns JSON with name field",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string) ([]releases.Release, error) {
				return []releases.Release{{ID: "10001", Name: "v1.0", ProjectID: "10000"}}, nil
			},
			wantIsError: false,
			wantContain: `"name"`,
		},
		{
			name:        "missing project_key returns IsError true",
			args:        map[string]any{},
			wantIsError: true,
			wantContain: "project_key",
		},
		{
			name: "service ErrUnauthorized returns IsError true",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string) ([]releases.Release, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name: "empty result returns valid JSON array",
			args: map[string]any{"project_key": "EMPTY"},
			mockFn: func(ctx context.Context, projectKey string) ([]releases.Release, error) {
				return []releases.Release{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockReleasesService{getReleasesFunc: tc.mockFn}
			handler := mcpserver.ToolSearchReleases(svc)
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

// --- TestToolGetRelease ---

func TestToolGetRelease(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, releaseID string) (*releases.Release, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns release JSON",
			args: map[string]any{"release_id": "10001"},
			mockFn: func(ctx context.Context, releaseID string) (*releases.Release, error) {
				return &releases.Release{ID: "10001", Name: "v1.0", Released: true, ProjectID: "10000"}, nil
			},
			wantIsError: false,
			wantContain: `"id":"10001"`,
		},
		{
			name:        "missing release_id returns IsError true",
			args:        map[string]any{},
			wantIsError: true,
			wantContain: "release_id",
		},
		{
			name: "service ErrNotFound returns IsError true",
			args: map[string]any{"release_id": "99999"},
			mockFn: func(ctx context.Context, releaseID string) (*releases.Release, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name: "service ErrUnauthorized returns IsError true",
			args: map[string]any{"release_id": "10001"},
			mockFn: func(ctx context.Context, releaseID string) (*releases.Release, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockReleasesService{getReleaseFunc: tc.mockFn}
			handler := mcpserver.ToolGetRelease(svc)
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

// --- TestToolGetReleaseIssues ---

func TestToolGetReleaseIssues(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, releaseID string) (*releases.ReleaseIssueCounts, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns issue counts JSON",
			args: map[string]any{"release_id": "10001"},
			mockFn: func(ctx context.Context, releaseID string) (*releases.ReleaseIssueCounts, error) {
				return &releases.ReleaseIssueCounts{FixVersion: 5, AffectsVersion: 3}, nil
			},
			wantIsError: false,
			wantContain: `"fix_version"`,
		},
		{
			name: "JSON contains affects_version field",
			args: map[string]any{"release_id": "10001"},
			mockFn: func(ctx context.Context, releaseID string) (*releases.ReleaseIssueCounts, error) {
				return &releases.ReleaseIssueCounts{FixVersion: 5, AffectsVersion: 3}, nil
			},
			wantIsError: false,
			wantContain: `"affects_version"`,
		},
		{
			name:        "missing release_id returns IsError true",
			args:        map[string]any{},
			wantIsError: true,
			wantContain: "release_id",
		},
		{
			name: "service ErrNotFound returns IsError true",
			args: map[string]any{"release_id": "99999"},
			mockFn: func(ctx context.Context, releaseID string) (*releases.ReleaseIssueCounts, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockReleasesService{getReleaseIssueCountsFunc: tc.mockFn}
			handler := mcpserver.ToolGetReleaseIssues(svc)
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

// --- TestToolCreateRelease ---

func TestToolCreateRelease(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, req releases.CreateReleaseRequest) (*releases.Release, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:        "WriteGuard blocked when ENABLE_WRITE unset",
			args:        map[string]any{"project_id": "10000", "name": "v1.0.0"},
			envWrite:    "",
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing project_id returns IsError true",
			args:        map[string]any{"name": "v1.0.0"},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "project_id",
		},
		{
			name:        "missing name returns IsError true",
			args:        map[string]any{"project_id": "10000"},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "name",
		},
		{
			name:     "success returns release JSON with id",
			args:     map[string]any{"project_id": "10000", "name": "v1.0.0"},
			envWrite: "true",
			mockFn: func(ctx context.Context, req releases.CreateReleaseRequest) (*releases.Release, error) {
				if req.ProjectID != "10000" {
					t.Errorf("req.ProjectID: got %q, want 10000", req.ProjectID)
				}
				if req.Name != "v1.0.0" {
					t.Errorf("req.Name: got %q, want v1.0.0", req.Name)
				}
				return &releases.Release{ID: "10001", Name: "v1.0.0", ProjectID: "10000"}, nil
			},
			wantIsError: false,
			wantContain: `"id":"10001"`,
		},
		{
			name:     "service error propagates as IsError true",
			args:     map[string]any{"project_id": "10000", "name": "v1.0.0"},
			envWrite: "true",
			mockFn: func(ctx context.Context, req releases.CreateReleaseRequest) (*releases.Release, error) {
				return nil, fmt.Errorf("Name is required")
			},
			wantIsError: true,
			wantContain: "Name is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}
			svc := &mockReleasesService{createReleaseFunc: tc.mockFn}
			handler := mcpserver.ToolCreateRelease(svc, audit.NewNoopLogger())
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

// TestToolCreateRelease_OptionalFieldsPassThrough verifies optional fields pass to service.
func TestToolCreateRelease_OptionalFieldsPassThrough(t *testing.T) {
	t.Setenv("ENABLE_WRITE", "true")
	var gotReq releases.CreateReleaseRequest
	svc := &mockReleasesService{
		createReleaseFunc: func(ctx context.Context, req releases.CreateReleaseRequest) (*releases.Release, error) {
			gotReq = req
			return &releases.Release{ID: "10001"}, nil
		},
	}
	handler := mcpserver.ToolCreateRelease(svc, audit.NewNoopLogger())
	req := makeCallToolRequest(map[string]any{
		"project_id":   "10000",
		"name":         "v1.0.0",
		"description":  "First release",
		"release_date": "2026-06-01",
	})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if gotReq.Description != "First release" {
		t.Errorf("Description: got %q, want 'First release'", gotReq.Description)
	}
	if gotReq.ReleaseDate != "2026-06-01" {
		t.Errorf("ReleaseDate: got %q, want '2026-06-01'", gotReq.ReleaseDate)
	}
}

// --- TestToolUpdateRelease ---

func TestToolUpdateRelease(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, releaseID string, req releases.UpdateReleaseRequest) (*releases.Release, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:        "WriteGuard blocked when ENABLE_WRITE unset",
			args:        map[string]any{"release_id": "10001"},
			envWrite:    "",
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing release_id returns IsError true",
			args:        map[string]any{},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "release_id",
		},
		{
			name:     "success with name update returns release JSON",
			args:     map[string]any{"release_id": "10001", "name": "v1.1.0"},
			envWrite: "true",
			mockFn: func(ctx context.Context, releaseID string, req releases.UpdateReleaseRequest) (*releases.Release, error) {
				if releaseID != "10001" {
					t.Errorf("releaseID: got %q, want 10001", releaseID)
				}
				if req.Name == nil || *req.Name != "v1.1.0" {
					t.Errorf("req.Name: got %v, want v1.1.0", req.Name)
				}
				return &releases.Release{ID: "10001", Name: "v1.1.0", ProjectID: "10000"}, nil
			},
			wantIsError: false,
			wantContain: "v1.1.0",
		},
		{
			name:     "released=true sets Released pointer",
			args:     map[string]any{"release_id": "10001", "released": "true"},
			envWrite: "true",
			mockFn: func(ctx context.Context, releaseID string, req releases.UpdateReleaseRequest) (*releases.Release, error) {
				if req.Released == nil || !*req.Released {
					t.Errorf("req.Released: got %v, want true", req.Released)
				}
				return &releases.Release{ID: "10001", Name: "v1.0", Released: true, ProjectID: "10000"}, nil
			},
			wantIsError: false,
			wantContain: `"id":"10001"`,
		},
		{
			name:     "service ErrNotFound returns IsError true",
			args:     map[string]any{"release_id": "99999", "name": "v2.0"},
			envWrite: "true",
			mockFn: func(ctx context.Context, releaseID string, req releases.UpdateReleaseRequest) (*releases.Release, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}
			svc := &mockReleasesService{updateReleaseFunc: tc.mockFn}
			handler := mcpserver.ToolUpdateRelease(svc, audit.NewNoopLogger())
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

// TestToolUpdateRelease_ArchivedFalseSetsPointer verifies archived=false correctly sets pointer to false.
func TestToolUpdateRelease_ArchivedFalseSetsPointer(t *testing.T) {
	t.Setenv("ENABLE_WRITE", "true")
	var gotReq releases.UpdateReleaseRequest
	svc := &mockReleasesService{
		updateReleaseFunc: func(ctx context.Context, releaseID string, req releases.UpdateReleaseRequest) (*releases.Release, error) {
			gotReq = req
			return &releases.Release{ID: "10001", Archived: false}, nil
		},
	}
	handler := mcpserver.ToolUpdateRelease(svc, audit.NewNoopLogger())
	req := makeCallToolRequest(map[string]any{"release_id": "10001", "archived": "false"})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if gotReq.Archived == nil {
		t.Fatal("Archived should not be nil when archived=false provided")
	}
	if *gotReq.Archived != false {
		t.Errorf("Archived: got %v, want false", *gotReq.Archived)
	}
}
