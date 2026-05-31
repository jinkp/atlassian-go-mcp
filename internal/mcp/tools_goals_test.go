package mcpserver_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

// mockGoalsService implements goals.GoalsService for testing.
// Each method delegates to a stored func field; guards against nil with safe defaults.
type mockGoalsService struct {
	getSiteIDFunc         func(ctx context.Context, subdomain string) (string, error)
	getGoalFunc           func(ctx context.Context, goalID string) (*goals.Goal, error)
	searchGoalsFunc       func(ctx context.Context, req goals.SearchGoalsRequest) (*goals.GoalSearchResult, error)
	updateGoalStatusFunc  func(ctx context.Context, req goals.UpdateGoalStatusRequest) error
	createGoalFunc        func(ctx context.Context, req goals.CreateGoalRequest) (*goals.CreateGoalResult, error)
	editGoalFunc          func(ctx context.Context, req goals.EditGoalRequest) (*goals.Goal, error)
	getGoalMetricsFunc    func(ctx context.Context, goalID string) ([]goals.MetricTarget, error)
	createMetricFunc      func(ctx context.Context, req goals.CreateMetricRequest) (*goals.MetricTarget, error)
	updateMetricValueFunc func(ctx context.Context, req goals.UpdateMetricValueRequest) (*goals.MetricValue, error)
	updateMetricTargetFunc func(ctx context.Context, req goals.UpdateMetricTargetRequest) error
}

func (m *mockGoalsService) GetSiteID(ctx context.Context, subdomain string) (string, error) {
	if m.getSiteIDFunc != nil {
		return m.getSiteIDFunc(ctx, subdomain)
	}
	return "", nil
}

func (m *mockGoalsService) GetGoal(ctx context.Context, goalID string) (*goals.Goal, error) {
	if m.getGoalFunc != nil {
		return m.getGoalFunc(ctx, goalID)
	}
	return &goals.Goal{}, nil
}

func (m *mockGoalsService) SearchGoals(ctx context.Context, req goals.SearchGoalsRequest) (*goals.GoalSearchResult, error) {
	if m.searchGoalsFunc != nil {
		return m.searchGoalsFunc(ctx, req)
	}
	return &goals.GoalSearchResult{Goals: []goals.Goal{}}, nil
}

func (m *mockGoalsService) UpdateGoalStatus(ctx context.Context, req goals.UpdateGoalStatusRequest) error {
	if m.updateGoalStatusFunc != nil {
		return m.updateGoalStatusFunc(ctx, req)
	}
	return nil
}

func (m *mockGoalsService) CreateGoal(ctx context.Context, req goals.CreateGoalRequest) (*goals.CreateGoalResult, error) {
	if m.createGoalFunc != nil {
		return m.createGoalFunc(ctx, req)
	}
	return &goals.CreateGoalResult{}, nil
}

func (m *mockGoalsService) EditGoal(ctx context.Context, req goals.EditGoalRequest) (*goals.Goal, error) {
	if m.editGoalFunc != nil {
		return m.editGoalFunc(ctx, req)
	}
	return &goals.Goal{}, nil
}

func (m *mockGoalsService) GetGoalMetrics(ctx context.Context, goalID string) ([]goals.MetricTarget, error) {
	if m.getGoalMetricsFunc != nil {
		return m.getGoalMetricsFunc(ctx, goalID)
	}
	return []goals.MetricTarget{}, nil
}

func (m *mockGoalsService) CreateMetric(ctx context.Context, req goals.CreateMetricRequest) (*goals.MetricTarget, error) {
	if m.createMetricFunc != nil {
		return m.createMetricFunc(ctx, req)
	}
	return &goals.MetricTarget{}, nil
}

func (m *mockGoalsService) UpdateMetricValue(ctx context.Context, req goals.UpdateMetricValueRequest) (*goals.MetricValue, error) {
	if m.updateMetricValueFunc != nil {
		return m.updateMetricValueFunc(ctx, req)
	}
	return &goals.MetricValue{}, nil
}

func (m *mockGoalsService) UpdateMetricTarget(ctx context.Context, req goals.UpdateMetricTargetRequest) error {
	if m.updateMetricTargetFunc != nil {
		return m.updateMetricTargetFunc(ctx, req)
	}
	return nil
}

// --- TestToolEditGoal ---

func TestToolEditGoal(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, req goals.EditGoalRequest) (*goals.Goal, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:        "ENABLE_WRITE unset - blocked",
			args:        map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/g1"},
			envWrite:    "",
			mockFn:      nil,
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing goal_id - returns error",
			args:        map[string]any{},
			envWrite:    "true",
			mockFn:      nil,
			wantIsError: true,
			wantContain: "goal_id",
		},
		{
			name:     "success - returns Goal JSON",
			args:     map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/g1", "name": "New Name"},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.EditGoalRequest) (*goals.Goal, error) {
				return &goals.Goal{ID: "ari:cloud:townsquare:abc:goal/g1", Name: "New Name"}, nil
			},
			wantIsError: false,
			wantContain: "New Name",
		},
		{
			name:     "service error - forwarded",
			args:     map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/bad"},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.EditGoalRequest) (*goals.Goal, error) {
				return nil, errors.New("Goal not found")
			},
			wantIsError: true,
			wantContain: "Goal not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}

			svc := &mockGoalsService{editGoalFunc: tc.mockFn}
			handler := mcpserver.ToolEditGoal(svc, audit.NewNoopLogger())
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolCreateGoal ---

func TestToolCreateGoal(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, req goals.CreateGoalRequest) (*goals.CreateGoalResult, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:        "ENABLE_WRITE unset - blocked, no service call",
			args:        map[string]any{"site_id": "abc", "name": "G", "goal_type_id": "ari:x", "target_date": "2026-12-31"},
			envWrite:    "",
			mockFn:      nil,
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing site_id - returns error",
			args:        map[string]any{"name": "G", "goal_type_id": "ari:x", "target_date": "2026-12-31"},
			envWrite:    "true",
			mockFn:      nil,
			wantIsError: true,
			wantContain: "site_id",
		},
		{
			name:        "missing name - returns error",
			args:        map[string]any{"site_id": "abc", "goal_type_id": "ari:x", "target_date": "2026-12-31"},
			envWrite:    "true",
			mockFn:      nil,
			wantIsError: true,
			wantContain: "name",
		},
		{
			name:        "missing goal_type_id - returns error",
			args:        map[string]any{"site_id": "abc", "name": "G", "target_date": "2026-12-31"},
			envWrite:    "true",
			mockFn:      nil,
			wantIsError: true,
			wantContain: "goal_type_id",
		},
		{
			name:        "missing target_date - returns error",
			args:        map[string]any{"site_id": "abc", "name": "G", "goal_type_id": "ari:x"},
			envWrite:    "true",
			mockFn:      nil,
			wantIsError: true,
			wantContain: "target_date",
		},
		{
			name:     "success - returns JSON with id and name",
			args:     map[string]any{"site_id": "abc-123", "name": "Ship Feature X", "goal_type_id": "ari:cloud:goal:abc-123:goal-type/act-1/gt-1", "target_date": "2026-12-31"},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.CreateGoalRequest) (*goals.CreateGoalResult, error) {
				return &goals.CreateGoalResult{ID: "goal-new-1", Name: "Ship Feature X"}, nil
			},
			wantIsError: false,
			wantContain: "goal-new-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}

			svc := &mockGoalsService{createGoalFunc: tc.mockFn}
			handler := mcpserver.ToolCreateGoal(svc, audit.NewNoopLogger())
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolGetSiteID ---

func TestToolGetSiteID(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, subdomain string) (string, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "success - returns JSON with cloud_id",
			args: map[string]any{"subdomain": "myorg"},
			mockFn: func(ctx context.Context, subdomain string) (string, error) {
				return "abc-123", nil
			},
			wantIsError: false,
			wantContain: "abc-123",
		},
		{
			name:        "missing subdomain - returns error, no service call",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "subdomain",
		},
		{
			name: "service error - returns error",
			args: map[string]any{"subdomain": "notexist"},
			mockFn: func(ctx context.Context, subdomain string) (string, error) {
				return "", errors.New("no tenant found")
			},
			wantIsError: true,
			wantContain: "no tenant",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockGoalsService{getSiteIDFunc: tc.mockFn}
			handler := mcpserver.ToolGetSiteID(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolGetGoal ---

func TestToolGetGoal(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, goalID string) (*goals.Goal, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "success - returns Goal JSON",
			args: map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/xyz"},
			mockFn: func(ctx context.Context, goalID string) (*goals.Goal, error) {
				return &goals.Goal{
					ID:     "ari:cloud:townsquare:abc:goal/xyz",
					Name:   "Increase Revenue",
					Status: "on_track",
					Phase:  "in_progress",
					Score:  75,
				}, nil
			},
			wantIsError: false,
			wantContain: "Increase Revenue",
		},
		{
			name:        "missing goal_id - returns error, no service call",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "goal_id",
		},
		{
			name: "service returns ErrNotFound",
			args: map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/missing"},
			mockFn: func(ctx context.Context, goalID string) (*goals.Goal, error) {
				return nil, errors.New("issue not found")
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name: "service returns ErrUnauthorized",
			args: map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/xyz"},
			mockFn: func(ctx context.Context, goalID string) (*goals.Goal, error) {
				return nil, errors.New("unauthorized")
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockGoalsService{getGoalFunc: tc.mockFn}
			handler := mcpserver.ToolGetGoal(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolSearchGoals ---

func TestToolSearchGoals(t *testing.T) {
	tests := []struct {
		name          string
		args          map[string]any
		mockFn        func(ctx context.Context, req goals.SearchGoalsRequest) (*goals.GoalSearchResult, error)
		wantIsError   bool
		wantContain   string
		checkReqFn    func(t *testing.T, req goals.SearchGoalsRequest) // optional check on service call args
	}{
		{
			name: "success with search_string - 3 goals",
			args: map[string]any{
				"site_id":       "abc-123",
				"search_string": "status = on_track",
				"max_results":   float64(10),
			},
			mockFn: func(ctx context.Context, req goals.SearchGoalsRequest) (*goals.GoalSearchResult, error) {
				return &goals.GoalSearchResult{
					Goals: []goals.Goal{
						{ID: "g1", Name: "Alpha"},
						{ID: "g2", Name: "Beta"},
						{ID: "g3", Name: "Gamma"},
					},
					HasMore: false,
				}, nil
			},
			wantIsError: false,
			wantContain: "Alpha",
		},
		{
			name: "success without search_string - empty string passed to service",
			args: map[string]any{"site_id": "abc-123"},
			mockFn: func(ctx context.Context, req goals.SearchGoalsRequest) (*goals.GoalSearchResult, error) {
				if req.SearchString != "" {
					return nil, errors.New("expected empty search string")
				}
				return &goals.GoalSearchResult{Goals: []goals.Goal{
					{ID: "g1"}, {ID: "g2"}, {ID: "g3"}, {ID: "g4"}, {ID: "g5"},
				}}, nil
			},
			wantIsError: false,
			wantContain: `"goals"`,
		},
		{
			name:        "missing site_id - returns error, no service call",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "site_id",
		},
		{
			name: "service error - returns error",
			args: map[string]any{"site_id": "abc-123"},
			mockFn: func(ctx context.Context, req goals.SearchGoalsRequest) (*goals.GoalSearchResult, error) {
				return nil, errors.New("some service error")
			},
			wantIsError: true,
			wantContain: "some service error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockGoalsService{searchGoalsFunc: tc.mockFn}
			handler := mcpserver.ToolSearchGoals(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolGetGoalMetrics ---

func TestToolGetGoalMetrics(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, goalID string) ([]goals.MetricTarget, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:        "missing goal_id - returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "goal_id",
		},
		{
			name: "service error - forwarded",
			args: map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/g1"},
			mockFn: func(ctx context.Context, goalID string) ([]goals.MetricTarget, error) {
				return nil, errors.New("goal not found")
			},
			wantIsError: true,
			wantContain: "goal not found",
		},
		{
			name: "success - returns JSON array with metric",
			args: map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/g1"},
			mockFn: func(ctx context.Context, goalID string) ([]goals.MetricTarget, error) {
				return []goals.MetricTarget{
					{ID: "mt1", Metric: goals.Metric{ID: "m1", Name: "Revenue", Type: "CURRENCY"}, StartValue: 0, TargetValue: 100},
				}, nil
			},
			wantIsError: false,
			wantContain: "mt1",
		},
		{
			name: "empty metrics - returns JSON empty array",
			args: map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/g1"},
			mockFn: func(ctx context.Context, goalID string) ([]goals.MetricTarget, error) {
				return []goals.MetricTarget{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockGoalsService{getGoalMetricsFunc: tc.mockFn}
			handler := mcpserver.ToolGetGoalMetrics(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolCreateMetric ---

func TestToolCreateMetric(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, req goals.CreateMetricRequest) (*goals.MetricTarget, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:        "ENABLE_WRITE unset - blocked",
			args:        map[string]any{"goal_id": "g1", "name": "Rev", "metric_type": "NUMERIC", "start_value": float64(0), "target_value": float64(100), "initial_value": float64(0)},
			envWrite:    "",
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing goal_id - returns error",
			args:        map[string]any{"name": "Rev", "metric_type": "NUMERIC", "start_value": float64(0), "target_value": float64(100), "initial_value": float64(0)},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "goal_id",
		},
		{
			name:        "missing name - returns error",
			args:        map[string]any{"goal_id": "g1", "metric_type": "NUMERIC", "start_value": float64(0), "target_value": float64(100), "initial_value": float64(0)},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "name",
		},
		{
			name:        "missing metric_type - returns error",
			args:        map[string]any{"goal_id": "g1", "name": "Rev", "start_value": float64(0), "target_value": float64(100), "initial_value": float64(0)},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "metric_type",
		},
		{
			name:        "missing start_value - returns error",
			args:        map[string]any{"goal_id": "g1", "name": "Rev", "metric_type": "NUMERIC", "target_value": float64(100), "initial_value": float64(0)},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "start_value",
		},
		{
			name:        "missing target_value - returns error",
			args:        map[string]any{"goal_id": "g1", "name": "Rev", "metric_type": "NUMERIC", "start_value": float64(0), "initial_value": float64(0)},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "target_value",
		},
		{
			name:        "missing initial_value - returns error",
			args:        map[string]any{"goal_id": "g1", "name": "Rev", "metric_type": "NUMERIC", "start_value": float64(0), "target_value": float64(100)},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "initial_value",
		},
		{
			name:     "success - returns JSON with metric target id",
			args:     map[string]any{"goal_id": "g1", "name": "Revenue", "metric_type": "CURRENCY", "start_value": float64(0), "target_value": float64(100), "initial_value": float64(50)},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.CreateMetricRequest) (*goals.MetricTarget, error) {
				return &goals.MetricTarget{ID: "mt-new-1", Metric: goals.Metric{Name: "Revenue"}}, nil
			},
			wantIsError: false,
			wantContain: "mt-new-1",
		},
		{
			name:     "service error - forwarded",
			args:     map[string]any{"goal_id": "g1", "name": "Rev", "metric_type": "NUMERIC", "start_value": float64(0), "target_value": float64(100), "initial_value": float64(0)},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.CreateMetricRequest) (*goals.MetricTarget, error) {
				return nil, errors.New("invalid metric type")
			},
			wantIsError: true,
			wantContain: "invalid metric type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}
			svc := &mockGoalsService{createMetricFunc: tc.mockFn}
			handler := mcpserver.ToolCreateMetric(svc, audit.NewNoopLogger())
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolUpdateMetricValue ---

func TestToolUpdateMetricValue(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, req goals.UpdateMetricValueRequest) (*goals.MetricValue, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:        "ENABLE_WRITE unset - blocked",
			args:        map[string]any{"metric_id": "m1", "value": float64(75)},
			envWrite:    "",
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing metric_id - returns error",
			args:        map[string]any{"value": float64(75)},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "metric_id",
		},
		{
			name:        "missing value - returns error",
			args:        map[string]any{"metric_id": "m1"},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "value",
		},
		{
			name:     "success - returns JSON with metric value id",
			args:     map[string]any{"metric_id": "m1", "value": float64(75)},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.UpdateMetricValueRequest) (*goals.MetricValue, error) {
				return &goals.MetricValue{ID: "mv-new-1", Value: 75}, nil
			},
			wantIsError: false,
			wantContain: "mv-new-1",
		},
		{
			name:     "service error - forwarded",
			args:     map[string]any{"metric_id": "m1", "value": float64(75)},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.UpdateMetricValueRequest) (*goals.MetricValue, error) {
				return nil, errors.New("metric not found")
			},
			wantIsError: true,
			wantContain: "metric not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}
			svc := &mockGoalsService{updateMetricValueFunc: tc.mockFn}
			handler := mcpserver.ToolUpdateMetricValue(svc, audit.NewNoopLogger())
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolUpdateMetricTarget ---

func TestToolUpdateMetricTarget(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, req goals.UpdateMetricTargetRequest) error
		wantIsError bool
		wantContain string
	}{
		{
			name:        "ENABLE_WRITE unset - blocked",
			args:        map[string]any{"metric_target_id": "mt1"},
			envWrite:    "",
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing metric_target_id - returns error",
			args:        map[string]any{},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "metric_target_id",
		},
		{
			name:     "success with no optional fields - returns ok",
			args:     map[string]any{"metric_target_id": "mt1"},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.UpdateMetricTargetRequest) error {
				return nil
			},
			wantIsError: false,
			wantContain: "ok",
		},
		{
			name:     "success with current_value - service called with non-nil CurrentValue",
			args:     map[string]any{"metric_target_id": "mt1", "current_value": "75.5"},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.UpdateMetricTargetRequest) error {
				if req.CurrentValue == nil {
					return errors.New("expected non-nil CurrentValue")
				}
				if *req.CurrentValue != 75.5 {
					return errors.New("expected CurrentValue=75.5")
				}
				return nil
			},
			wantIsError: false,
			wantContain: "ok",
		},
		{
			name:     "service error - forwarded",
			args:     map[string]any{"metric_target_id": "mt-bad"},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.UpdateMetricTargetRequest) error {
				return errors.New("target not found")
			},
			wantIsError: true,
			wantContain: "target not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}
			svc := &mockGoalsService{updateMetricTargetFunc: tc.mockFn}
			handler := mcpserver.ToolUpdateMetricTarget(svc, audit.NewNoopLogger())
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolUpdateGoalStatus ---

func TestToolUpdateGoalStatus(t *testing.T) {
	tests := []struct {
		name           string
		args           map[string]any
		envWrite       string
		mockFn         func(ctx context.Context, req goals.UpdateGoalStatusRequest) error
		capturedReqFn  func(t *testing.T, req goals.UpdateGoalStatusRequest)
		wantIsError    bool
		wantContain    string
	}{
		{
			name:        "ENABLE_WRITE unset - blocked, no service call",
			args:        map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/x", "status": "on_track"},
			envWrite:    "",
			mockFn:      nil, // must NOT be called
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:     "success - on_track",
			args:     map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/x", "status": "on_track"},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.UpdateGoalStatusRequest) error {
				return nil
			},
			wantIsError: false,
			wantContain: "ok",
		},
		{
			name:     "success - with optional score and summary",
			args:     map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/x", "status": "off_track", "score": float64(42), "summary": "Delayed due to scope"},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.UpdateGoalStatusRequest) error {
				if req.Score != 42 {
					return errors.New("expected Score=42")
				}
				if req.Summary != "Delayed due to scope" {
					return errors.New("expected Summary='Delayed due to scope'")
				}
				return nil
			},
			wantIsError: false,
			wantContain: "ok",
		},
		{
			name:        "missing goal_id - returns error, no service call",
			args:        map[string]any{"status": "on_track"},
			envWrite:    "true",
			mockFn:      nil,
			wantIsError: true,
			wantContain: "goal_id",
		},
		{
			name:        "missing status - returns error, no service call",
			args:        map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/x"},
			envWrite:    "true",
			mockFn:      nil,
			wantIsError: true,
			wantContain: "status",
		},
		{
			name:     "service error - goal not found",
			args:     map[string]any{"goal_id": "ari:cloud:townsquare:abc:goal/x", "status": "on_track"},
			envWrite: "true",
			mockFn: func(ctx context.Context, req goals.UpdateGoalStatusRequest) error {
				return errors.New("goal not found")
			},
			wantIsError: true,
			wantContain: "goal not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}

			svc := &mockGoalsService{updateGoalStatusFunc: tc.mockFn}
			handler := mcpserver.ToolUpdateGoalStatus(svc, audit.NewNoopLogger())
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}
