package goals_test

import (
	"context"
	"errors"
	"testing"

	goalssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
)

// mockGoalsService implements goalssvc.GoalsService for testing.
type mockGoalsService struct {
	getSiteIDFunc          func(ctx context.Context, subdomain string) (string, error)
	getGoalFunc            func(ctx context.Context, goalID string) (*goalssvc.Goal, error)
	searchGoalsFunc        func(ctx context.Context, req goalssvc.SearchGoalsRequest) (*goalssvc.GoalSearchResult, error)
	updateGoalStatusFunc   func(ctx context.Context, req goalssvc.UpdateGoalStatusRequest) error
	createGoalFunc         func(ctx context.Context, req goalssvc.CreateGoalRequest) (*goalssvc.CreateGoalResult, error)
	editGoalFunc           func(ctx context.Context, req goalssvc.EditGoalRequest) (*goalssvc.Goal, error)
	getGoalMetricsFunc     func(ctx context.Context, goalID string) ([]goalssvc.MetricTarget, error)
	createMetricFunc       func(ctx context.Context, req goalssvc.CreateMetricRequest) (*goalssvc.MetricTarget, error)
	updateMetricValueFunc  func(ctx context.Context, req goalssvc.UpdateMetricValueRequest) (*goalssvc.MetricValue, error)
	updateMetricTargetFunc func(ctx context.Context, req goalssvc.UpdateMetricTargetRequest) error
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

func (m *mockGoalsService) GetGoalMetrics(ctx context.Context, goalID string) ([]goalssvc.MetricTarget, error) {
	if m.getGoalMetricsFunc != nil {
		return m.getGoalMetricsFunc(ctx, goalID)
	}
	return []goalssvc.MetricTarget{}, nil
}

func (m *mockGoalsService) CreateMetric(ctx context.Context, req goalssvc.CreateMetricRequest) (*goalssvc.MetricTarget, error) {
	if m.createMetricFunc != nil {
		return m.createMetricFunc(ctx, req)
	}
	return &goalssvc.MetricTarget{}, nil
}

func (m *mockGoalsService) UpdateMetricValue(ctx context.Context, req goalssvc.UpdateMetricValueRequest) (*goalssvc.MetricValue, error) {
	if m.updateMetricValueFunc != nil {
		return m.updateMetricValueFunc(ctx, req)
	}
	return &goalssvc.MetricValue{}, nil
}

func (m *mockGoalsService) UpdateMetricTarget(ctx context.Context, req goalssvc.UpdateMetricTargetRequest) error {
	if m.updateMetricTargetFunc != nil {
		return m.updateMetricTargetFunc(ctx, req)
	}
	return nil
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
