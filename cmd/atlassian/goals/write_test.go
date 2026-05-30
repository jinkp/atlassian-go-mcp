package goals_test

import (
	"context"
	"strings"
	"testing"

	goalssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
)

// --- SC-G6: goals create — success ---

func TestCreateGoal_Success(t *testing.T) {
	var gotReq goalssvc.CreateGoalRequest
	svc := &mockGoalsService{
		createGoalFunc: func(_ context.Context, req goalssvc.CreateGoalRequest) (*goalssvc.CreateGoalResult, error) {
			gotReq = req
			return &goalssvc.CreateGoalResult{
				ID:   "ari:cloud:townsquare::goal/xyz",
				Name: "Q2 Goal",
			}, nil
		},
	}

	result, err := svc.CreateGoal(context.Background(), goalssvc.CreateGoalRequest{
		SiteID:     "abc-123",
		Name:       "Q2 Goal",
		GoalTypeID: "ari:cloud:townsquare::goalType/tid",
		TargetDate: "2026-06-30",
	})
	if err != nil {
		t.Fatalf("CreateGoal() unexpected error: %v", err)
	}
	if result.Name != "Q2 Goal" {
		t.Errorf("expected 'Q2 Goal', got %q", result.Name)
	}
	if gotReq.SiteID != "abc-123" {
		t.Errorf("expected siteID 'abc-123', got %q", gotReq.SiteID)
	}

	out := "Created goal: ari:cloud:townsquare::goal/xyz Q2 Goal"
	if !strings.Contains(out, "Q2 Goal") {
		t.Errorf("output missing goal name\nGot: %s", out)
	}
}

// --- SC-G7: goals update — success ---

func TestUpdateGoalStatus_Success(t *testing.T) {
	svc := &mockGoalsService{
		updateGoalStatusFunc: func(_ context.Context, req goalssvc.UpdateGoalStatusRequest) error {
			if req.GoalID != "ari:cloud:townsquare::goal/xyz" {
				t.Errorf("expected goalID, got %q", req.GoalID)
			}
			if req.Status != "on_track" {
				t.Errorf("expected status 'on_track', got %q", req.Status)
			}
			return nil
		},
	}

	err := svc.UpdateGoalStatus(context.Background(), goalssvc.UpdateGoalStatusRequest{
		GoalID: "ari:cloud:townsquare::goal/xyz",
		Status: "on_track",
	})
	if err != nil {
		t.Fatalf("UpdateGoalStatus() unexpected error: %v", err)
	}

	out := "Updated goal: ari:cloud:townsquare::goal/xyz"
	if !strings.Contains(out, "Updated goal:") {
		t.Errorf("output missing prefix\nGot: %s", out)
	}
}

// --- goals create with optional fields ---

func TestCreateGoal_WithOptionalFields(t *testing.T) {
	svc := &mockGoalsService{
		createGoalFunc: func(_ context.Context, req goalssvc.CreateGoalRequest) (*goalssvc.CreateGoalResult, error) {
			if req.Confidence != "MONTH" {
				t.Errorf("expected confidence 'MONTH', got %q", req.Confidence)
			}
			if req.Description != "A monthly goal" {
				t.Errorf("expected description, got %q", req.Description)
			}
			return &goalssvc.CreateGoalResult{ID: "g1", Name: "Monthly Goal"}, nil
		},
	}

	_, err := svc.CreateGoal(context.Background(), goalssvc.CreateGoalRequest{
		SiteID:      "abc-123",
		Name:        "Monthly Goal",
		GoalTypeID:  "tid",
		TargetDate:  "2026-01-31",
		Confidence:  "MONTH",
		Description: "A monthly goal",
	})
	if err != nil {
		t.Fatalf("CreateGoal() unexpected error: %v", err)
	}
}
