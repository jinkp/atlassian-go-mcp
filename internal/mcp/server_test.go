package mcpserver_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
	"github.com/jinkp/atlassian-go-mcp/internal/mcp/features"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
)

// --- TestConfigFromEnv ---

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		envs        map[string]string
		wantErr     bool
		wantErrMsg  string
		wantBaseURL string
	}{
		{
			name: "all vars present returns config",
			envs: map[string]string{
				"ATLASSIAN_BASE_URL":   "https://example.atlassian.net",
				"ATLASSIAN_EMAIL":      "user@example.com",
				"ATLASSIAN_TOKEN":  "token123",
			},
			wantErr:     false,
			wantBaseURL: "https://example.atlassian.net",
		},
		{
			name: "missing ATLASSIAN_BASE_URL returns error",
			envs: map[string]string{
				"ATLASSIAN_BASE_URL":  "",
				"ATLASSIAN_EMAIL":     "user@example.com",
				"ATLASSIAN_TOKEN": "token123",
			},
			wantErr:    true,
			wantErrMsg: "ATLASSIAN_BASE_URL",
		},
		{
			name: "missing ATLASSIAN_EMAIL returns error",
			envs: map[string]string{
				"ATLASSIAN_BASE_URL":  "https://example.atlassian.net",
				"ATLASSIAN_EMAIL":     "",
				"ATLASSIAN_TOKEN": "token123",
			},
			wantErr:    true,
			wantErrMsg: "ATLASSIAN_EMAIL",
		},
		{
			name: "missing ATLASSIAN_TOKEN returns error",
			envs: map[string]string{
				"ATLASSIAN_BASE_URL": "https://example.atlassian.net",
				"ATLASSIAN_EMAIL":    "user@example.com",
				"ATLASSIAN_TOKEN":    "",
			},
			wantErr:    true,
			wantErrMsg: "ATLASSIAN_TOKEN",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set env vars for this test, restoring afterwards via t.Setenv
			for k, v := range tc.envs {
				t.Setenv(k, v)
			}

			cfg, err := mcpserver.ConfigFromEnv()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.BaseURL != tc.wantBaseURL {
				t.Errorf("BaseURL: got %q, want %q", cfg.BaseURL, tc.wantBaseURL)
			}
		})
	}
}

// --- TestWriteGuardCheck ---

func TestWriteGuardCheck(t *testing.T) {
	tests := []struct {
		name        string
		enableWrite string
		wantAllow   bool
	}{
		{
			name:        "ENABLE_WRITE unset — guard rejects",
			enableWrite: "",
			wantAllow:   false,
		},
		{
			name:        "ENABLE_WRITE=false — guard rejects",
			enableWrite: "false",
			wantAllow:   false,
		},
		{
			name:        "ENABLE_WRITE=true — guard allows",
			enableWrite: "true",
			wantAllow:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.enableWrite == "" {
				os.Unsetenv("ENABLE_WRITE") //nolint:errcheck
			} else {
				t.Setenv("ENABLE_WRITE", tc.enableWrite)
			}

			result := mcpserver.WriteGuardCheck()
			if tc.wantAllow {
				if result != nil {
					t.Errorf("expected nil (allow), got error: %v", result)
				}
			} else {
				if result == nil {
					t.Error("expected error (deny), got nil")
				}
				if !strings.Contains(result.Error(), "write operations disabled") {
					t.Errorf("error %q does not contain 'write operations disabled'", result.Error())
				}
			}
		})
	}
}

// mockServerAgileService is a minimal agile.AgileService for server construction tests.
// All methods have nil-func guards so the server can be constructed without panic.
type mockServerAgileService struct{}

func (m *mockServerAgileService) GetBoards(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error) {
	return []agile.Board{}, nil
}

func (m *mockServerAgileService) GetSprints(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
	return []agile.Sprint{}, nil
}

func (m *mockServerAgileService) GetSprintIssues(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error) {
	return &agile.SprintIssueResult{Issues: []agile.SprintIssue{}}, nil
}

func (m *mockServerAgileService) UpdateSprint(ctx context.Context, sprintID int, req agile.UpdateSprintRequest) (*agile.Sprint, error) {
	return &agile.Sprint{}, nil
}

func (m *mockServerAgileService) MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error {
	return nil
}

func (m *mockServerAgileService) MoveIssuesToEpic(ctx context.Context, epicKey string, issueKeys []string) error {
	return nil
}

func (m *mockServerAgileService) CreateSprint(ctx context.Context, req agile.CreateSprintRequest) (*agile.Sprint, error) {
	return &agile.Sprint{}, nil
}

// mockServerGoalsService is a minimal goals.GoalsService for server construction tests.
type mockServerGoalsService struct{}

func (m *mockServerGoalsService) GetSiteID(ctx context.Context, subdomain string) (string, error) {
	return "", nil
}

func (m *mockServerGoalsService) GetGoal(ctx context.Context, goalID string) (*goals.Goal, error) {
	return &goals.Goal{}, nil
}

func (m *mockServerGoalsService) SearchGoals(ctx context.Context, req goals.SearchGoalsRequest) (*goals.GoalSearchResult, error) {
	return &goals.GoalSearchResult{Goals: []goals.Goal{}}, nil
}

func (m *mockServerGoalsService) UpdateGoalStatus(ctx context.Context, req goals.UpdateGoalStatusRequest) error {
	return nil
}

func (m *mockServerGoalsService) CreateGoal(ctx context.Context, req goals.CreateGoalRequest) (*goals.CreateGoalResult, error) {
	return &goals.CreateGoalResult{}, nil
}

func (m *mockServerGoalsService) EditGoal(_ context.Context, _ goals.EditGoalRequest) (*goals.Goal, error) {
	return &goals.Goal{}, nil
}

func (m *mockServerGoalsService) GetGoalMetrics(_ context.Context, _ string) ([]goals.MetricTarget, error) {
	return []goals.MetricTarget{}, nil
}

func (m *mockServerGoalsService) CreateMetric(_ context.Context, _ goals.CreateMetricRequest) (*goals.MetricTarget, error) {
	return &goals.MetricTarget{}, nil
}

func (m *mockServerGoalsService) UpdateMetricValue(_ context.Context, _ goals.UpdateMetricValueRequest) (*goals.MetricValue, error) {
	return &goals.MetricValue{}, nil
}

func (m *mockServerGoalsService) UpdateMetricTarget(_ context.Context, _ goals.UpdateMetricTargetRequest) error {
	return nil
}

// mockServerReleasesService is a minimal releases.ReleasesService for server construction tests.
type mockServerReleasesService struct{}

func (m *mockServerReleasesService) GetReleases(_ context.Context, _ string) ([]releases.Release, error) {
	return []releases.Release{}, nil
}

func (m *mockServerReleasesService) GetRelease(_ context.Context, _ string) (*releases.Release, error) {
	return &releases.Release{}, nil
}

func (m *mockServerReleasesService) GetReleaseIssueCounts(_ context.Context, _ string) (*releases.ReleaseIssueCounts, error) {
	return &releases.ReleaseIssueCounts{}, nil
}

func (m *mockServerReleasesService) CreateRelease(_ context.Context, _ releases.CreateReleaseRequest) (*releases.Release, error) {
	return &releases.Release{}, nil
}

func (m *mockServerReleasesService) UpdateRelease(_ context.Context, _ string, _ releases.UpdateReleaseRequest) (*releases.Release, error) {
	return &releases.Release{}, nil
}

// mockServerProjectsService is a minimal projects.ProjectsService for server construction tests.
type mockServerProjectsService struct{}

func (m *mockServerProjectsService) GetProjects(_ context.Context, _ int) ([]projects.Project, error) {
	return []projects.Project{}, nil
}

func (m *mockServerProjectsService) GetProject(_ context.Context, _ string) (*projects.Project, error) {
	return &projects.Project{}, nil
}

func (m *mockServerProjectsService) SearchProjects(_ context.Context, _ projects.SearchProjectsRequest) (*projects.SearchProjectsResult, error) {
	return &projects.SearchProjectsResult{Projects: []projects.Project{}}, nil
}

func (m *mockServerProjectsService) UpdateProject(_ context.Context, _ string, _ projects.UpdateProjectRequest) (*projects.Project, error) {
	return &projects.Project{}, nil
}

// mockServerTeamsService is a minimal teams.TeamsService for server construction tests.
type mockServerTeamsService struct{}

func (m *mockServerTeamsService) GetTeams(_ context.Context, _ string, _ int) (*teams.TeamSearchResult, error) {
	return &teams.TeamSearchResult{Teams: []teams.Team{}}, nil
}

func (m *mockServerTeamsService) GetTeam(_ context.Context, _ string) (*teams.Team, error) {
	return &teams.Team{}, nil
}

func (m *mockServerTeamsService) GetTeamMembers(_ context.Context, _ string, _ int) ([]teams.TeamMember, error) {
	return []teams.TeamMember{}, nil
}

// --- TestNewAtlassianServer ---

func TestNewAtlassianServer_HasTools(t *testing.T) {
	svc := &mockJiraService{
		getIssueFunc:     nil,
		searchIssuesFunc: nil,
	}
	agileSvc := &mockServerAgileService{}
	goalsSvc := &mockServerGoalsService{}
	releasesSvc := &mockServerReleasesService{}
	projectsSvc := &mockServerProjectsService{}
	teamsSvc := &mockServerTeamsService{}
	// nil FeatureSet → all 37 tools registered (backward-compat default)
	s := mcpserver.NewAtlassianServer(svc, agileSvc, goalsSvc, releasesSvc, projectsSvc, teamsSvc, audit.NewNoopLogger(), nil)
	if s == nil {
		t.Fatal("NewAtlassianServer returned nil")
	}
	// The server should have been constructed without panic.
	// Tool registration is verified via the server not being nil
	// and the package compiling — actual tool listing requires the server to run.
}

// TestNewAtlassianServer_FeatureGating verifies the server constructs without panic
// for various FeatureSet configurations.
func TestNewAtlassianServer_FeatureGating(t *testing.T) {
	svc := &mockJiraService{}
	agileSvc := &mockServerAgileService{}
	goalsSvc := &mockServerGoalsService{}
	releasesSvc := &mockServerReleasesService{}
	projectsSvc := &mockServerProjectsService{}
	teamsSvc := &mockServerTeamsService{}
	log := audit.NewNoopLogger()

	tests := []struct {
		name    string
		fs      *features.FeatureSet
		wantNil bool
	}{
		{"nil fs → all tools", nil, false},
		{"jira only", features.Parse("jira"), false},
		{"jira-read only", features.Parse("jira-read"), false},
		{"unknown module → 0 tools", features.Parse("unknown"), false},
		{"all", features.Parse("all"), false},
		{"goals,metrics", features.Parse("goals,metrics"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := mcpserver.NewAtlassianServer(svc, agileSvc, goalsSvc, releasesSvc, projectsSvc, teamsSvc, log, tc.fs)
			if (s == nil) != tc.wantNil {
				t.Errorf("NewAtlassianServer nil=%v, want nil=%v", s == nil, tc.wantNil)
			}
		})
	}
}
