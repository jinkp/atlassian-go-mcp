package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// mockGoalsService implements goals.GoalsService for testing.
type mockGoalsService struct {
	getSiteIDFunc         func(ctx context.Context, subdomain string) (string, error)
	getGoalFunc           func(ctx context.Context, goalID string) (*goals.Goal, error)
	searchGoalsFunc       func(ctx context.Context, req goals.SearchGoalsRequest) (*goals.GoalSearchResult, error)
	updateGoalStatusFunc  func(ctx context.Context, req goals.UpdateGoalStatusRequest) error
	createGoalFunc        func(ctx context.Context, req goals.CreateGoalRequest) (*goals.CreateGoalResult, error)
	editGoalFunc          func(ctx context.Context, req goals.EditGoalRequest) (*goals.Goal, error)
}

func (m *mockGoalsService) GetSiteID(ctx context.Context, subdomain string) (string, error) {
	if m.getSiteIDFunc != nil {
		return m.getSiteIDFunc(ctx, subdomain)
	}
	return "site-123", nil
}
func (m *mockGoalsService) GetGoal(ctx context.Context, goalID string) (*goals.Goal, error) {
	if m.getGoalFunc != nil {
		return m.getGoalFunc(ctx, goalID)
	}
	return &goals.Goal{ID: goalID, Name: "Test Goal"}, nil
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
	return &goals.CreateGoalResult{ID: "goal-1", Name: req.Name}, nil
}
func (m *mockGoalsService) EditGoal(ctx context.Context, req goals.EditGoalRequest) (*goals.Goal, error) {
	if m.editGoalFunc != nil {
		return m.editGoalFunc(ctx, req)
	}
	return &goals.Goal{ID: req.GoalID, Name: "Updated"}, nil
}

func TestGoalsGetSiteID(t *testing.T) {
	t.Run("success returns site id", func(t *testing.T) {
		h := NewGoalsHandler(&mockGoalsService{
			getSiteIDFunc: func(ctx context.Context, subdomain string) (string, error) {
				return "abc-123", nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("GET /goals/site-id", h.GetSiteID)

		req := httptest.NewRequest(http.MethodGet, "/goals/site-id?subdomain=myorg", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "abc-123") {
			t.Errorf("body %q does not contain 'abc-123'", w.Body.String())
		}
	})
}

func TestGoalsSearchGoals(t *testing.T) {
	t.Run("success returns goals list", func(t *testing.T) {
		h := NewGoalsHandler(&mockGoalsService{
			searchGoalsFunc: func(ctx context.Context, req goals.SearchGoalsRequest) (*goals.GoalSearchResult, error) {
				return &goals.GoalSearchResult{
					Goals: []goals.Goal{{ID: "goal-1", Name: "Q1 Goal"}},
				}, nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("GET /goals", h.SearchGoals)

		req := httptest.NewRequest(http.MethodGet, "/goals?siteId=abc&query=Q1", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "Q1 Goal") {
			t.Errorf("body %q does not contain 'Q1 Goal'", w.Body.String())
		}
	})
}

func TestGoalsGetGoal(t *testing.T) {
	tests := []struct {
		name        string
		goalID      string
		mockFn      func(ctx context.Context, goalID string) (*goals.Goal, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:   "success returns goal",
			goalID: "goal-123",
			mockFn: func(ctx context.Context, goalID string) (*goals.Goal, error) {
				return &goals.Goal{ID: goalID, Name: "My Goal"}, nil
			},
			wantStatus:  200,
			wantContain: "My Goal",
		},
		{
			name:   "not found returns 404",
			goalID: "goal-999",
			mockFn: func(ctx context.Context, goalID string) (*goals.Goal, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewGoalsHandler(&mockGoalsService{getGoalFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /goals/{goalId}", h.GetGoal)

			req := httptest.NewRequest(http.MethodGet, "/goals/"+tc.goalID, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

func TestGoalsCreateGoal(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]any
		mockFn      func(ctx context.Context, req goals.CreateGoalRequest) (*goals.CreateGoalResult, error)
		wantStatus  int
		wantContain string
	}{
		{
			name: "success returns 201",
			body: map[string]any{"site_id": "abc", "name": "New Goal", "goal_type_id": "gtype-1", "target_date": "2026-12-31"},
			mockFn: func(ctx context.Context, req goals.CreateGoalRequest) (*goals.CreateGoalResult, error) {
				return &goals.CreateGoalResult{ID: "goal-new", Name: req.Name}, nil
			},
			wantStatus:  201,
			wantContain: "New Goal",
		},
		{
			name:        "missing name returns 400",
			body:        map[string]any{"site_id": "abc"},
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewGoalsHandler(&mockGoalsService{createGoalFunc: tc.mockFn}, audit.NewNoopLogger())
			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/goals", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.CreateGoal(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

func TestGoalsUpdateGoalStatus(t *testing.T) {
	t.Run("success returns ok", func(t *testing.T) {
		h := NewGoalsHandler(&mockGoalsService{
			updateGoalStatusFunc: func(ctx context.Context, req goals.UpdateGoalStatusRequest) error {
				return nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("PUT /goals/{goalId}/status", h.UpdateGoalStatus)

		body, _ := json.Marshal(map[string]any{"status": "on_track"})
		req := httptest.NewRequest(http.MethodPut, "/goals/goal-1/status", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
	})
}

func TestGoalsEditGoal(t *testing.T) {
	t.Run("success returns updated goal", func(t *testing.T) {
		h := NewGoalsHandler(&mockGoalsService{
			editGoalFunc: func(ctx context.Context, req goals.EditGoalRequest) (*goals.Goal, error) {
				return &goals.Goal{ID: req.GoalID, Name: "Updated Name"}, nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("PUT /goals/{goalId}", h.EditGoal)

		body, _ := json.Marshal(map[string]any{"name": "Updated Name"})
		req := httptest.NewRequest(http.MethodPut, "/goals/goal-1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "Updated Name") {
			t.Errorf("body %q does not contain 'Updated Name'", w.Body.String())
		}
	})
}
