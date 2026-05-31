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
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
)

// mockReleasesService implements releases.ReleasesService for testing.
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
	return &releases.Release{ID: releaseID, Name: "v1.0"}, nil
}
func (m *mockReleasesService) GetReleaseIssueCounts(ctx context.Context, releaseID string) (*releases.ReleaseIssueCounts, error) {
	if m.getReleaseIssueCountsFunc != nil {
		return m.getReleaseIssueCountsFunc(ctx, releaseID)
	}
	return &releases.ReleaseIssueCounts{FixVersion: 5}, nil
}
func (m *mockReleasesService) CreateRelease(ctx context.Context, req releases.CreateReleaseRequest) (*releases.Release, error) {
	if m.createReleaseFunc != nil {
		return m.createReleaseFunc(ctx, req)
	}
	return &releases.Release{ID: "10001", Name: req.Name}, nil
}
func (m *mockReleasesService) UpdateRelease(ctx context.Context, releaseID string, req releases.UpdateReleaseRequest) (*releases.Release, error) {
	if m.updateReleaseFunc != nil {
		return m.updateReleaseFunc(ctx, releaseID, req)
	}
	return &releases.Release{ID: releaseID}, nil
}

func TestReleasesGetReleases(t *testing.T) {
	t.Run("success returns list", func(t *testing.T) {
		h := NewReleasesHandler(&mockReleasesService{
			getReleasesFunc: func(ctx context.Context, projectKey string) ([]releases.Release, error) {
				return []releases.Release{{ID: "10001", Name: "v1.0", ProjectID: "PROJ"}}, nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("GET /releases", h.GetReleases)

		req := httptest.NewRequest(http.MethodGet, "/releases?project=PROJ", nil)
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

func TestReleasesGetRelease(t *testing.T) {
	tests := []struct {
		name        string
		releaseID   string
		mockFn      func(ctx context.Context, releaseID string) (*releases.Release, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:      "success returns release",
			releaseID: "10001",
			mockFn: func(ctx context.Context, releaseID string) (*releases.Release, error) {
				return &releases.Release{ID: releaseID, Name: "v1.0"}, nil
			},
			wantStatus:  200,
			wantContain: "v1.0",
		},
		{
			name:      "not found returns 404",
			releaseID: "99999",
			mockFn: func(ctx context.Context, releaseID string) (*releases.Release, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewReleasesHandler(&mockReleasesService{getReleaseFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /releases/{releaseId}", h.GetRelease)

			req := httptest.NewRequest(http.MethodGet, "/releases/"+tc.releaseID, nil)
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

func TestReleasesGetReleaseIssues(t *testing.T) {
	t.Run("success returns issue counts", func(t *testing.T) {
		h := NewReleasesHandler(&mockReleasesService{
			getReleaseIssueCountsFunc: func(ctx context.Context, releaseID string) (*releases.ReleaseIssueCounts, error) {
				return &releases.ReleaseIssueCounts{FixVersion: 3, AffectsVersion: 1}, nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("GET /releases/{releaseId}/issues", h.GetReleaseIssues)

		req := httptest.NewRequest(http.MethodGet, "/releases/10001/issues", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "fix_version") {
			t.Errorf("body %q does not contain 'fix_version'", w.Body.String())
		}
	})
}

func TestReleasesCreateRelease(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]any
		mockFn      func(ctx context.Context, req releases.CreateReleaseRequest) (*releases.Release, error)
		wantStatus  int
		wantContain string
	}{
		{
			name: "success returns 201",
			body: map[string]any{"project_id": "10000", "name": "v2.0"},
			mockFn: func(ctx context.Context, req releases.CreateReleaseRequest) (*releases.Release, error) {
				return &releases.Release{ID: "10002", Name: req.Name}, nil
			},
			wantStatus:  201,
			wantContain: "v2.0",
		},
		{
			name:        "missing name returns 400",
			body:        map[string]any{"project_id": "10000"},
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewReleasesHandler(&mockReleasesService{createReleaseFunc: tc.mockFn}, audit.NewNoopLogger())
			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/releases", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.CreateRelease(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

func TestReleasesUpdateRelease(t *testing.T) {
	t.Run("success returns updated release", func(t *testing.T) {
		h := NewReleasesHandler(&mockReleasesService{
			updateReleaseFunc: func(ctx context.Context, releaseID string, req releases.UpdateReleaseRequest) (*releases.Release, error) {
				return &releases.Release{ID: releaseID, Name: "v1.1"}, nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("PUT /releases/{releaseId}", h.UpdateRelease)

		name := "v1.1"
		body, _ := json.Marshal(map[string]any{"name": name})
		req := httptest.NewRequest(http.MethodPut, "/releases/10001", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
	})
}
