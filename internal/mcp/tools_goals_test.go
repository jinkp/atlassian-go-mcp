package mcpserver_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

// mockGoalsService implements goals.GoalsService for testing.
// Each method delegates to a stored func field; guards against nil with safe defaults.
type mockGoalsService struct {
	getSiteIDFunc        func(ctx context.Context, subdomain string) (string, error)
	getGoalFunc          func(ctx context.Context, goalID string) (*goals.Goal, error)
	searchGoalsFunc      func(ctx context.Context, req goals.SearchGoalsRequest) (*goals.GoalSearchResult, error)
	updateGoalStatusFunc func(ctx context.Context, req goals.UpdateGoalStatusRequest) error
	createGoalFunc       func(ctx context.Context, req goals.CreateGoalRequest) (*goals.CreateGoalResult, error)
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
			handler := mcpserver.ToolCreateGoal(svc)
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
			handler := mcpserver.ToolUpdateGoalStatus(svc)
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
