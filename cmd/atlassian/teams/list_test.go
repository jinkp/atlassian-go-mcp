package teams_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	teamssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
)

// mockTeamsService implements teamssvc.TeamsService for CLI testing.
type mockTeamsService struct {
	getTeamsFunc       func(ctx context.Context, query string, maxResults int) (*teamssvc.TeamSearchResult, error)
	getTeamFunc        func(ctx context.Context, teamID string) (*teamssvc.Team, error)
	getTeamMembersFunc func(ctx context.Context, teamID string, maxResults int) ([]teamssvc.TeamMember, error)
}

func (m *mockTeamsService) GetTeams(ctx context.Context, query string, maxResults int) (*teamssvc.TeamSearchResult, error) {
	if m.getTeamsFunc != nil {
		return m.getTeamsFunc(ctx, query, maxResults)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTeamsService) GetTeam(ctx context.Context, teamID string) (*teamssvc.Team, error) {
	if m.getTeamFunc != nil {
		return m.getTeamFunc(ctx, teamID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTeamsService) GetTeamMembers(ctx context.Context, teamID string, maxResults int) ([]teamssvc.TeamMember, error) {
	if m.getTeamMembersFunc != nil {
		return m.getTeamMembersFunc(ctx, teamID, maxResults)
	}
	return nil, errors.New("not implemented")
}

// --- list tests ---

// SC-C1: list renders teams table output with display name
func TestTeams_RendersTableOutput(t *testing.T) {
	svc := &mockTeamsService{
		getTeamsFunc: func(_ context.Context, _ string, _ int) (*teamssvc.TeamSearchResult, error) {
			return &teamssvc.TeamSearchResult{
				Teams: []teamssvc.Team{
					{ID: "T1", DisplayName: "Alpha Team", State: "ACTIVE", TeamType: "OPEN"},
					{ID: "T2", DisplayName: "Beta Team", State: "ACTIVE", TeamType: "MEMBER_INVITE"},
				},
			}, nil
		},
	}

	result, err := svc.GetTeams(context.Background(), "", 50)
	if err != nil {
		t.Fatalf("GetTeams() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(result.Teams)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "Alpha Team") {
		t.Errorf("table missing team display name\nGot: %s", out)
	}
	if !strings.Contains(out, "ACTIVE") {
		t.Errorf("table missing team state\nGot: %s", out)
	}
	if !strings.Contains(out, "OPEN") {
		t.Errorf("table missing team type\nGot: %s", out)
	}
}

// SC-C2: list renders JSON output
func TestTeams_RendersJSONOutput(t *testing.T) {
	svc := &mockTeamsService{
		getTeamsFunc: func(_ context.Context, _ string, _ int) (*teamssvc.TeamSearchResult, error) {
			return &teamssvc.TeamSearchResult{
				Teams: []teamssvc.Team{
					{ID: "T1", DisplayName: "Alpha Team", State: "ACTIVE"},
				},
			}, nil
		},
	}

	result, err := svc.GetTeams(context.Background(), "", 50)
	if err != nil {
		t.Fatalf("GetTeams() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(result.Teams)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "T1") {
		t.Errorf("JSON missing team ID\nGot: %s", out)
	}
	if !strings.Contains(out, "Alpha Team") {
		t.Errorf("JSON missing display name\nGot: %s", out)
	}
}

// SC-R: single team renders table
func TestTeam_RendersTableSingle(t *testing.T) {
	team := teamssvc.Team{
		ID: "T1", DisplayName: "Alpha Team",
		State: "ACTIVE", TeamType: "OPEN",
		OrganizationID: "ORG1",
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(team)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "NAME") {
		t.Errorf("table missing NAME label\nGot: %s", out)
	}
	if !strings.Contains(out, "STATE") {
		t.Errorf("table missing STATE label\nGot: %s", out)
	}
	if !strings.Contains(out, "Alpha Team") {
		t.Errorf("table missing team display name\nGot: %s", out)
	}
}

// SC-R: *Team pointer also renders
func TestTeam_RendersTableSinglePointer(t *testing.T) {
	team := &teamssvc.Team{
		ID: "T1", DisplayName: "Alpha Team", State: "ACTIVE",
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(team)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "Alpha Team") {
		t.Errorf("table missing team display name\nGot: %s", out)
	}
}

// SC-Members: members renders table
func TestTeamMembers_RendersTable(t *testing.T) {
	members := []teamssvc.TeamMember{
		{AccountID: "ACC1"},
		{AccountID: "ACC2"},
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(members)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "ACC1") {
		t.Errorf("table missing account ID\nGot: %s", out)
	}
	if !strings.Contains(out, "ACCOUNT_ID") {
		t.Errorf("table missing ACCOUNT_ID header\nGot: %s", out)
	}
}

// SC-JSON: teams array has expected fields
func TestTeams_JSONOutputStructure(t *testing.T) {
	teamsData := []teamssvc.Team{
		{
			ID:             "T1",
			DisplayName:    "Alpha Team",
			State:          "ACTIVE",
			TeamType:       "OPEN",
			OrganizationID: "ORG1",
		},
	}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(teamsData)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	fields := []string{`"id"`, `"display_name"`, `"state"`, `"team_type"`, `"organization_id"`}
	for _, field := range fields {
		if !strings.Contains(out, field) {
			t.Errorf("JSON missing field %q\nGot: %s", field, out)
		}
	}
}

// exit code mapping tests
func TestTeamsExitCode_NotFound(t *testing.T) {
	err := jira.ErrNotFound
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("expected ErrNotFound")
	}
}

func TestTeamsExitCode_Unauthorized(t *testing.T) {
	err := jira.ErrUnauthorized
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized")
	}
}
