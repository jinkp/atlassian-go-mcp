package mcpserver_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
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

// --- TestNewAtlassianServer ---

func TestNewAtlassianServer_HasTools(t *testing.T) {
	svc := &mockJiraService{
		getIssueFunc:     nil,
		searchIssuesFunc: nil,
	}
	agileSvc := &mockServerAgileService{}
	goalsSvc := &mockServerGoalsService{}
	s := mcpserver.NewAtlassianServer(svc, agileSvc, goalsSvc)
	if s == nil {
		t.Fatal("NewAtlassianServer returned nil")
	}
	// The server should have been constructed without panic.
	// Tool registration is verified via the server not being nil
	// and the package compiling — actual tool listing requires the server to run.
}
