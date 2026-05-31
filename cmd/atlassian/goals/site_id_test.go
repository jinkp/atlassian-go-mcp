package goals_test

import (
	"context"
	"errors"
	"testing"

	goalssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
)

// mockGoalsService implements goalssvc.GoalsService for testing.
type mockGoalsService struct {
	getSiteIDFunc        func(ctx context.Context, subdomain string) (string, error)
	getGoalFunc          func(ctx context.Context, goalID string) (*goalssvc.Goal, error)
	searchGoalsFunc      func(ctx context.Context, req goalssvc.SearchGoalsRequest) (*goalssvc.GoalSearchResult, error)
	updateGoalStatusFunc func(ctx context.Context, req goalssvc.UpdateGoalStatusRequest) error
	createGoalFunc       func(ctx context.Context, req goalssvc.CreateGoalRequest) (*goalssvc.CreateGoalResult, error)
	editGoalFunc         func(ctx context.Context, req goalssvc.EditGoalRequest) (*goalssvc.Goal, error)
}

func (m *mockGoalsService) GetSiteID(ctx context.Context, subdomain string) (string, error) {
	if m.getSiteIDFunc != nil {
		return m.getSiteIDFunc(ctx, subdomain)
	}
	return "", errors.New("not implemented")
}
func (m *mockGoalsService) GetGoal(ctx context.Context, goalID string) (*goalssvc.Goal, error) {
	if m.getGoalFunc != nil {
		return m.getGoalFunc(ctx, goalID)
	}
	return nil, errors.New("not implemented")
}
func (m *mockGoalsService) SearchGoals(ctx context.Context, req goalssvc.SearchGoalsRequest) (*goalssvc.GoalSearchResult, error) {
	if m.searchGoalsFunc != nil {
		return m.searchGoalsFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}
func (m *mockGoalsService) UpdateGoalStatus(ctx context.Context, req goalssvc.UpdateGoalStatusRequest) error {
	if m.updateGoalStatusFunc != nil {
		return m.updateGoalStatusFunc(ctx, req)
	}
	return errors.New("not implemented")
}
func (m *mockGoalsService) CreateGoal(ctx context.Context, req goalssvc.CreateGoalRequest) (*goalssvc.CreateGoalResult, error) {
	if m.createGoalFunc != nil {
		return m.createGoalFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockGoalsService) EditGoal(ctx context.Context, req goalssvc.EditGoalRequest) (*goalssvc.Goal, error) {
	if m.editGoalFunc != nil {
		return m.editGoalFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

// --- SC-G1: site-id — success ---

func TestSiteID_Success(t *testing.T) {
	svc := &mockGoalsService{
		getSiteIDFunc: func(_ context.Context, subdomain string) (string, error) {
			if subdomain != "myorg" {
				t.Errorf("expected subdomain 'myorg', got %q", subdomain)
			}
			return "abc-123", nil
		},
	}

	siteID, err := svc.GetSiteID(context.Background(), "myorg")
	if err != nil {
		t.Fatalf("GetSiteID() unexpected error: %v", err)
	}
	if siteID != "abc-123" {
		t.Errorf("expected 'abc-123', got %q", siteID)
	}
}
