package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
)

// mockTeamsService implements teams.TeamsService for testing.
type mockTeamsService struct {
	getTeamsFunc       func(ctx context.Context, query string, maxResults int) (*teams.TeamSearchResult, error)
	getTeamFunc        func(ctx context.Context, teamID string) (*teams.Team, error)
	getTeamMembersFunc func(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error)
}

func (m *mockTeamsService) GetTeams(ctx context.Context, query string, maxResults int) (*teams.TeamSearchResult, error) {
	if m.getTeamsFunc != nil {
		return m.getTeamsFunc(ctx, query, maxResults)
	}
	return &teams.TeamSearchResult{Teams: []teams.Team{}}, nil
}
func (m *mockTeamsService) GetTeam(ctx context.Context, teamID string) (*teams.Team, error) {
	if m.getTeamFunc != nil {
		return m.getTeamFunc(ctx, teamID)
	}
	return &teams.Team{ID: teamID, DisplayName: "Test Team"}, nil
}
func (m *mockTeamsService) GetTeamMembers(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error) {
	if m.getTeamMembersFunc != nil {
		return m.getTeamMembersFunc(ctx, teamID, maxResults)
	}
	return []teams.TeamMember{}, nil
}

func TestTeamsGetTeams(t *testing.T) {
	t.Run("success returns teams list", func(t *testing.T) {
		h := NewTeamsHandler(&mockTeamsService{
			getTeamsFunc: func(ctx context.Context, query string, maxResults int) (*teams.TeamSearchResult, error) {
				return &teams.TeamSearchResult{
					Teams: []teams.Team{{ID: "team-1", DisplayName: "Engineering"}},
				}, nil
			},
		})

		mux := http.NewServeMux()
		mux.HandleFunc("GET /teams", h.GetTeams)

		req := httptest.NewRequest(http.MethodGet, "/teams?query=eng", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "Engineering") {
			t.Errorf("body %q does not contain 'Engineering'", w.Body.String())
		}
	})

	t.Run("unauthorized returns 401", func(t *testing.T) {
		h := NewTeamsHandler(&mockTeamsService{
			getTeamsFunc: func(ctx context.Context, query string, maxResults int) (*teams.TeamSearchResult, error) {
				return nil, jira.ErrUnauthorized
			},
		})

		mux := http.NewServeMux()
		mux.HandleFunc("GET /teams", h.GetTeams)

		req := httptest.NewRequest(http.MethodGet, "/teams", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 401 {
			t.Errorf("status: got %d, want 401", w.Code)
		}
	})
}

func TestTeamsGetTeam(t *testing.T) {
	tests := []struct {
		name        string
		teamID      string
		mockFn      func(ctx context.Context, teamID string) (*teams.Team, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:   "success returns team",
			teamID: "team-123",
			mockFn: func(ctx context.Context, teamID string) (*teams.Team, error) {
				return &teams.Team{ID: teamID, DisplayName: "My Team"}, nil
			},
			wantStatus:  200,
			wantContain: "My Team",
		},
		{
			name:   "not found returns 404",
			teamID: "team-999",
			mockFn: func(ctx context.Context, teamID string) (*teams.Team, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewTeamsHandler(&mockTeamsService{getTeamFunc: tc.mockFn})

			mux := http.NewServeMux()
			mux.HandleFunc("GET /teams/{teamId}", h.GetTeam)

			req := httptest.NewRequest(http.MethodGet, "/teams/"+tc.teamID, nil)
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

func TestTeamsGetTeamMembers(t *testing.T) {
	tests := []struct {
		name        string
		teamID      string
		mockFn      func(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:   "success returns members",
			teamID: "team-1",
			mockFn: func(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error) {
				return []teams.TeamMember{{AccountID: "acc-1"}, {AccountID: "acc-2"}}, nil
			},
			wantStatus:  200,
			wantContain: `"total":2`,
		},
		{
			name:   "not found returns 404",
			teamID: "team-999",
			mockFn: func(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewTeamsHandler(&mockTeamsService{getTeamMembersFunc: tc.mockFn})

			mux := http.NewServeMux()
			mux.HandleFunc("GET /teams/{teamId}/members", h.GetTeamMembers)

			req := httptest.NewRequest(http.MethodGet, "/teams/"+tc.teamID+"/members", nil)
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

// compile-time check: TeamsHandler doesn't need audit.Logger (read-only service)
func init() {
	_ = audit.NewNoopLogger() // ensure audit package is used
}
