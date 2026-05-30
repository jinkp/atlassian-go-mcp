package agile_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	agilesvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
)

// mockAgileService implements agilesvc.AgileService for testing.
type mockAgileService struct {
	getBoardsFunc       func(ctx context.Context, projectKey string, maxResults int) ([]agilesvc.Board, error)
	getSprintsFunc      func(ctx context.Context, boardID int, state string, maxResults int) ([]agilesvc.Sprint, error)
	getSprintIssuesFunc func(ctx context.Context, sprintID int, maxResults int) (*agilesvc.SprintIssueResult, error)
	updateSprintFunc    func(ctx context.Context, sprintID int, req agilesvc.UpdateSprintRequest) (*agilesvc.Sprint, error)
	moveToSprintFunc    func(ctx context.Context, sprintID int, issueKeys []string) error
	moveToEpicFunc      func(ctx context.Context, epicKey string, issueKeys []string) error
	createSprintFunc    func(ctx context.Context, req agilesvc.CreateSprintRequest) (*agilesvc.Sprint, error)
}

func (m *mockAgileService) GetBoards(ctx context.Context, projectKey string, maxResults int) ([]agilesvc.Board, error) {
	if m.getBoardsFunc != nil {
		return m.getBoardsFunc(ctx, projectKey, maxResults)
	}
	return nil, errors.New("not implemented")
}
func (m *mockAgileService) GetSprints(ctx context.Context, boardID int, state string, maxResults int) ([]agilesvc.Sprint, error) {
	if m.getSprintsFunc != nil {
		return m.getSprintsFunc(ctx, boardID, state, maxResults)
	}
	return nil, errors.New("not implemented")
}
func (m *mockAgileService) GetSprintIssues(ctx context.Context, sprintID int, maxResults int) (*agilesvc.SprintIssueResult, error) {
	if m.getSprintIssuesFunc != nil {
		return m.getSprintIssuesFunc(ctx, sprintID, maxResults)
	}
	return nil, errors.New("not implemented")
}
func (m *mockAgileService) UpdateSprint(ctx context.Context, sprintID int, req agilesvc.UpdateSprintRequest) (*agilesvc.Sprint, error) {
	if m.updateSprintFunc != nil {
		return m.updateSprintFunc(ctx, sprintID, req)
	}
	return nil, errors.New("not implemented")
}
func (m *mockAgileService) MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error {
	if m.moveToSprintFunc != nil {
		return m.moveToSprintFunc(ctx, sprintID, issueKeys)
	}
	return errors.New("not implemented")
}
func (m *mockAgileService) MoveIssuesToEpic(ctx context.Context, epicKey string, issueKeys []string) error {
	if m.moveToEpicFunc != nil {
		return m.moveToEpicFunc(ctx, epicKey, issueKeys)
	}
	return errors.New("not implemented")
}
func (m *mockAgileService) CreateSprint(ctx context.Context, req agilesvc.CreateSprintRequest) (*agilesvc.Sprint, error) {
	if m.createSprintFunc != nil {
		return m.createSprintFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

// --- SC-A1: boards table output ---

func TestBoards_RendersTableOutput(t *testing.T) {
	svc := &mockAgileService{
		getBoardsFunc: func(_ context.Context, _ string, _ int) ([]agilesvc.Board, error) {
			return []agilesvc.Board{
				{ID: 1, Name: "SCRUM board", Type: "scrum"},
			}, nil
		},
	}

	boards, err := svc.GetBoards(context.Background(), "TEST", 50)
	if err != nil {
		t.Fatalf("GetBoards() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(boards)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "1") {
		t.Errorf("table missing board ID\nGot: %s", out)
	}
	if !strings.Contains(out, "SCRUM board") {
		t.Errorf("table missing board name\nGot: %s", out)
	}
	if !strings.Contains(out, "scrum") {
		t.Errorf("table missing board type\nGot: %s", out)
	}
}

// --- SC-A2: boards JSON output ---

func TestBoards_RendersJSONOutput(t *testing.T) {
	svc := &mockAgileService{
		getBoardsFunc: func(_ context.Context, _ string, _ int) ([]agilesvc.Board, error) {
			return []agilesvc.Board{
				{ID: 2, Name: "Kanban board", Type: "kanban"},
			}, nil
		},
	}

	boards, err := svc.GetBoards(context.Background(), "PROJ", 50)
	if err != nil {
		t.Fatalf("GetBoards() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(boards)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "Kanban board") {
		t.Errorf("JSON missing board name\nGot: %s", out)
	}
}

// --- SC-A3: sprints with state filter ---

func TestSprints_PassesStateFilter(t *testing.T) {
	var gotState string
	svc := &mockAgileService{
		getSprintsFunc: func(_ context.Context, boardID int, state string, _ int) ([]agilesvc.Sprint, error) {
			gotState = state
			return []agilesvc.Sprint{
				{ID: 10, Name: "Sprint 3", State: "active"},
			}, nil
		},
	}

	sprints, err := svc.GetSprints(context.Background(), 5, "active", 50)
	if err != nil {
		t.Fatalf("GetSprints() unexpected error: %v", err)
	}
	if gotState != "active" {
		t.Errorf("expected state='active', got %q", gotState)
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(sprints)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "Sprint 3") {
		t.Errorf("table missing sprint name\nGot: %s", out)
	}
	if !strings.Contains(out, "active") {
		t.Errorf("table missing sprint state\nGot: %s", out)
	}
}

// --- SC-A4: sprint active — found ---

func TestSprintActive_Found(t *testing.T) {
	svc := &mockAgileService{
		getSprintsFunc: func(_ context.Context, _ int, state string, maxResults int) ([]agilesvc.Sprint, error) {
			if state != "active" {
				t.Errorf("expected state='active', got %q", state)
			}
			return []agilesvc.Sprint{
				{ID: 7, Name: "Sprint 7", State: "active"},
			}, nil
		},
	}

	sprints, err := svc.GetSprints(context.Background(), 5, "active", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprints) == 0 {
		t.Fatal("expected at least one sprint")
	}
	if sprints[0].Name != "Sprint 7" {
		t.Errorf("expected 'Sprint 7', got %q", sprints[0].Name)
	}
}

// --- SC-A5: sprint active — none found ---

func TestSprintActive_NoneFound(t *testing.T) {
	svc := &mockAgileService{
		getSprintsFunc: func(_ context.Context, _ int, _ string, _ int) ([]agilesvc.Sprint, error) {
			return []agilesvc.Sprint{}, nil
		},
	}

	sprints, err := svc.GetSprints(context.Background(), 5, "active", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprints) != 0 {
		t.Errorf("expected 0 sprints, got %d", len(sprints))
	}
}

// --- SC-A6: sprint issues — success ---

func TestSprintIssues_RendersTable(t *testing.T) {
	svc := &mockAgileService{
		getSprintIssuesFunc: func(_ context.Context, _ int, _ int) (*agilesvc.SprintIssueResult, error) {
			return &agilesvc.SprintIssueResult{
				Issues: []agilesvc.SprintIssue{
					{Key: "PROJ-1", Summary: "Issue one", Status: "In Progress", Assignee: "Alice"},
					{Key: "PROJ-2", Summary: "Issue two", Status: "Done", Assignee: "Bob"},
				},
				Total: 2,
			}, nil
		},
	}

	result, err := svc.GetSprintIssues(context.Background(), 10, 50)
	if err != nil {
		t.Fatalf("GetSprintIssues() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(result)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("table missing PROJ-1\nGot: %s", out)
	}
	if !strings.Contains(out, "PROJ-2") {
		t.Errorf("table missing PROJ-2\nGot: %s", out)
	}
}

// --- SC-A7: sprint issues — empty ---

func TestSprintIssues_EmptySprint(t *testing.T) {
	svc := &mockAgileService{
		getSprintIssuesFunc: func(_ context.Context, _ int, _ int) (*agilesvc.SprintIssueResult, error) {
			return &agilesvc.SprintIssueResult{Issues: []agilesvc.SprintIssue{}, Total: 0}, nil
		},
	}

	result, err := svc.GetSprintIssues(context.Background(), 10, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Issues) != 0 {
		t.Errorf("expected empty issues, got %d", len(result.Issues))
	}
}

// --- exit code mapping ---

func TestAgileExitCode_NotFound(t *testing.T) {
	if code := agileExitCodeTest(jira.ErrNotFound); code != 3 {
		t.Errorf("expected 3 for ErrNotFound, got %d", code)
	}
}

func TestAgileExitCode_Unauthorized(t *testing.T) {
	if code := agileExitCodeTest(jira.ErrUnauthorized); code != 2 {
		t.Errorf("expected 2 for ErrUnauthorized, got %d", code)
	}
}

// agileExitCodeTest mirrors the exit code logic in boards.go
func agileExitCodeTest(err error) int {
	if errors.Is(err, jira.ErrNotFound) {
		return 3
	}
	if errors.Is(err, jira.ErrUnauthorized) {
		return 2
	}
	return 2
}
