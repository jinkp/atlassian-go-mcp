package goals_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	goalssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
)

// --- SC-G2: goals get — table ---

func TestGetGoal_RendersTable(t *testing.T) {
	svc := &mockGoalsService{
		getGoalFunc: func(_ context.Context, goalID string) (*goalssvc.Goal, error) {
			return &goalssvc.Goal{
				ID:     goalID,
				Name:   "Q1 Revenue",
				Status: "on_track",
				Phase:  "in_progress",
				Score:  75,
			}, nil
		},
	}

	goal, err := svc.GetGoal(context.Background(), "ari:cloud:townsquare::goal/abc")
	if err != nil {
		t.Fatalf("GetGoal() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(goal)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "Q1 Revenue") {
		t.Errorf("table missing goal name\nGot: %s", out)
	}
	if !strings.Contains(out, "on_track") {
		t.Errorf("table missing status\nGot: %s", out)
	}
}

// --- SC-G3: goals get — not found ---

func TestGetGoal_NotFound(t *testing.T) {
	svc := &mockGoalsService{
		getGoalFunc: func(_ context.Context, _ string) (*goalssvc.Goal, error) {
			return nil, jira.ErrNotFound
		},
	}

	_, err := svc.GetGoal(context.Background(), "ari:cloud:townsquare::goal/missing")
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
	if code := goalsExitCodeTest(err); code != 3 {
		t.Errorf("expected exit code 3 for ErrNotFound, got %d", code)
	}
}

// --- SC-G4: goals search — success ---

func TestSearchGoals_RendersResults(t *testing.T) {
	svc := &mockGoalsService{
		searchGoalsFunc: func(_ context.Context, req goalssvc.SearchGoalsRequest) (*goalssvc.GoalSearchResult, error) {
			if req.SiteID != "abc-123" {
				t.Errorf("expected siteID 'abc-123', got %q", req.SiteID)
			}
			return &goalssvc.GoalSearchResult{
				Goals: []goalssvc.Goal{
					{ID: "g1", Name: "Grow revenue", Status: "on_track"},
				},
			}, nil
		},
	}

	result, err := svc.SearchGoals(context.Background(), goalssvc.SearchGoalsRequest{SiteID: "abc-123"})
	if err != nil {
		t.Fatalf("SearchGoals() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(result)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "Grow revenue") {
		t.Errorf("table missing goal name\nGot: %s", out)
	}
}

// --- SC-G5: goals search — empty ---

func TestSearchGoals_EmptyResult(t *testing.T) {
	svc := &mockGoalsService{
		searchGoalsFunc: func(_ context.Context, _ goalssvc.SearchGoalsRequest) (*goalssvc.GoalSearchResult, error) {
			return &goalssvc.GoalSearchResult{Goals: []goalssvc.Goal{}}, nil
		},
	}

	result, err := svc.SearchGoals(context.Background(), goalssvc.SearchGoalsRequest{SiteID: "abc-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Goals) != 0 {
		t.Errorf("expected 0 goals, got %d", len(result.Goals))
	}
}

// goalsExitCodeTest mirrors the exit code logic in errors.go
func goalsExitCodeTest(err error) int {
	if errors.Is(err, jira.ErrNotFound) {
		return 3
	}
	if errors.Is(err, jira.ErrUnauthorized) {
		return 2
	}
	return 2
}
