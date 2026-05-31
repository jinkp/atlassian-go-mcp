package mcpserver_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

// mockTeamsService implements teams.TeamsService for testing.
// Each method delegates to a stored func field; guards against nil with safe defaults.
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
	return &teams.Team{}, nil
}

func (m *mockTeamsService) GetTeamMembers(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error) {
	if m.getTeamMembersFunc != nil {
		return m.getTeamMembersFunc(ctx, teamID, maxResults)
	}
	return []teams.TeamMember{}, nil
}

// --- TestToolSearchTeams ---

func TestToolSearchTeams(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, query string, maxResults int) (*teams.TeamSearchResult, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns teams JSON with display_name field",
			args: map[string]any{},
			mockFn: func(ctx context.Context, query string, maxResults int) (*teams.TeamSearchResult, error) {
				return &teams.TeamSearchResult{
					Teams: []teams.Team{
						{ID: "T1", DisplayName: "Alpha", State: "ACTIVE"},
						{ID: "T2", DisplayName: "Beta", State: "ACTIVE"},
					},
				}, nil
			},
			wantIsError: false,
			wantContain: `"display_name"`,
		},
		{
			name: "max_results defaults to 50 when not provided",
			args: map[string]any{},
			mockFn: func(ctx context.Context, query string, maxResults int) (*teams.TeamSearchResult, error) {
				if maxResults != 50 {
					t.Errorf("expected maxResults=50, got %d", maxResults)
				}
				return &teams.TeamSearchResult{Teams: []teams.Team{}}, nil
			},
			wantIsError: false,
			wantContain: `"teams"`,
		},
		{
			name: "query and max_results are forwarded",
			args: map[string]any{"query": "alpha", "max_results": float64(10)},
			mockFn: func(ctx context.Context, query string, maxResults int) (*teams.TeamSearchResult, error) {
				if query != "alpha" {
					t.Errorf("expected query=alpha, got %q", query)
				}
				if maxResults != 10 {
					t.Errorf("expected maxResults=10, got %d", maxResults)
				}
				return &teams.TeamSearchResult{Teams: []teams.Team{}}, nil
			},
			wantIsError: false,
			wantContain: `"teams"`,
		},
		{
			name: "service error is returned as tool error",
			args: map[string]any{},
			mockFn: func(ctx context.Context, query string, maxResults int) (*teams.TeamSearchResult, error) {
				return nil, fmt.Errorf("upstream error")
			},
			wantIsError: true,
			wantContain: "upstream error",
		},
		{
			name: "empty teams returns teams array not null",
			args: map[string]any{},
			mockFn: func(ctx context.Context, query string, maxResults int) (*teams.TeamSearchResult, error) {
				return &teams.TeamSearchResult{Teams: []teams.Team{}}, nil
			},
			wantIsError: false,
			wantContain: `"teams":[]`,
		},
		{
			name: "unauthorized error forwarded",
			args: map[string]any{},
			mockFn: func(ctx context.Context, query string, maxResults int) (*teams.TeamSearchResult, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockTeamsService{getTeamsFunc: tc.mockFn}
			handler := mcpserver.ToolSearchTeams(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			if tc.wantIsError {
				if !result.IsError {
					t.Errorf("expected IsError=true, got false")
				}
				if tc.wantContain != "" {
					text := getResultText(t, result)
					if !strings.Contains(text, tc.wantContain) {
						t.Errorf("error text %q does not contain %q", text, tc.wantContain)
					}
				}
				return
			}
			if result.IsError {
				t.Errorf("expected IsError=false, got true; text: %s", getResultText(t, result))
			}
			if tc.wantContain != "" {
				text := getResultText(t, result)
				if !strings.Contains(text, tc.wantContain) {
					t.Errorf("result text %q does not contain %q", text, tc.wantContain)
				}
			}
		})
	}
}

// --- TestToolGetTeam ---

func TestToolGetTeam(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, teamID string) (*teams.Team, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns team JSON with id field",
			args: map[string]any{"team_id": "T1"},
			mockFn: func(ctx context.Context, teamID string) (*teams.Team, error) {
				if teamID != "T1" {
					t.Errorf("expected teamID=T1, got %q", teamID)
				}
				return &teams.Team{ID: "T1", DisplayName: "Alpha", State: "ACTIVE"}, nil
			},
			wantIsError: false,
			wantContain: `"id"`,
		},
		{
			name:        "missing team_id returns tool error",
			args:        map[string]any{},
			wantIsError: true,
			wantContain: "team_id is required",
		},
		{
			name: "not found error forwarded",
			args: map[string]any{"team_id": "MISSING"},
			mockFn: func(ctx context.Context, teamID string) (*teams.Team, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
		},
		{
			name: "service error forwarded",
			args: map[string]any{"team_id": "T1"},
			mockFn: func(ctx context.Context, teamID string) (*teams.Team, error) {
				return nil, fmt.Errorf("connection error")
			},
			wantIsError: true,
			wantContain: "connection error",
		},
		{
			name: "display_name in response",
			args: map[string]any{"team_id": "T1"},
			mockFn: func(ctx context.Context, teamID string) (*teams.Team, error) {
				return &teams.Team{ID: "T1", DisplayName: "Design Team"}, nil
			},
			wantIsError: false,
			wantContain: "Design Team",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockTeamsService{getTeamFunc: tc.mockFn}
			handler := mcpserver.ToolGetTeam(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			if tc.wantIsError {
				if !result.IsError {
					t.Errorf("expected IsError=true, got false")
				}
				if tc.wantContain != "" {
					text := getResultText(t, result)
					if !strings.Contains(text, tc.wantContain) {
						t.Errorf("error text %q does not contain %q", text, tc.wantContain)
					}
				}
				return
			}
			if result.IsError {
				t.Errorf("expected IsError=false, got true; text: %s", getResultText(t, result))
			}
			if tc.wantContain != "" {
				text := getResultText(t, result)
				if !strings.Contains(text, tc.wantContain) {
					t.Errorf("result text %q does not contain %q", text, tc.wantContain)
				}
			}
		})
	}
}

// --- TestToolGetTeamMembers ---

func TestToolGetTeamMembers(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns members JSON array",
			args: map[string]any{"team_id": "T1"},
			mockFn: func(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error) {
				return []teams.TeamMember{
					{AccountID: "ACC1"},
					{AccountID: "ACC2"},
				}, nil
			},
			wantIsError: false,
			wantContain: "ACC1",
		},
		{
			name:        "missing team_id returns tool error",
			args:        map[string]any{},
			wantIsError: true,
			wantContain: "team_id is required",
		},
		{
			name: "max_results defaults to 50",
			args: map[string]any{"team_id": "T1"},
			mockFn: func(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error) {
				if maxResults != 50 {
					t.Errorf("expected maxResults=50, got %d", maxResults)
				}
				return []teams.TeamMember{}, nil
			},
			wantIsError: false,
		},
		{
			name: "max_results forwarded",
			args: map[string]any{"team_id": "T1", "max_results": float64(25)},
			mockFn: func(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error) {
				if maxResults != 25 {
					t.Errorf("expected maxResults=25, got %d", maxResults)
				}
				return []teams.TeamMember{}, nil
			},
			wantIsError: false,
		},
		{
			name: "service error forwarded",
			args: map[string]any{"team_id": "T1"},
			mockFn: func(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error) {
				return nil, fmt.Errorf("upstream failure")
			},
			wantIsError: true,
			wantContain: "upstream failure",
		},
		{
			name: "empty members returns array not null",
			args: map[string]any{"team_id": "T1"},
			mockFn: func(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error) {
				return []teams.TeamMember{}, nil
			},
			wantIsError: false,
			wantContain: `[]`,
		},
		{
			name: "nil members coerced to empty array",
			args: map[string]any{"team_id": "T1"},
			mockFn: func(ctx context.Context, teamID string, maxResults int) ([]teams.TeamMember, error) {
				return nil, nil
			},
			wantIsError: false,
			wantContain: `[]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockTeamsService{getTeamMembersFunc: tc.mockFn}
			handler := mcpserver.ToolGetTeamMembers(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			if tc.wantIsError {
				if !result.IsError {
					t.Errorf("expected IsError=true, got false")
				}
				if tc.wantContain != "" {
					text := getResultText(t, result)
					if !strings.Contains(text, tc.wantContain) {
						t.Errorf("error text %q does not contain %q", text, tc.wantContain)
					}
				}
				return
			}
			if result.IsError {
				t.Errorf("expected IsError=false, got true; text: %s", getResultText(t, result))
			}
			if tc.wantContain != "" {
				text := getResultText(t, result)
				if !strings.Contains(text, tc.wantContain) {
					t.Errorf("result text %q does not contain %q", text, tc.wantContain)
				}
			}
		})
	}
}
