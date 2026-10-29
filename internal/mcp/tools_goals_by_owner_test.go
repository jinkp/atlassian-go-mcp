package mcpserver_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

func TestToolGetGoalsByOwner(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, req goals.GoalsByOwnerRequest) (*goals.GoalSearchResult, error)
		wantIsError bool
		wantContain string
		checkReqFn  func(t *testing.T, req goals.GoalsByOwnerRequest)
	}{
		{
			name: "success - returns the owner's goals",
			args: map[string]any{
				"site_id":  "abc-123",
				"owner_id": "712020:aaa",
			},
			mockFn: func(_ context.Context, _ goals.GoalsByOwnerRequest) (*goals.GoalSearchResult, error) {
				return &goals.GoalSearchResult{
					Goals: []goals.Goal{
						{ID: "g1", Key: "ZQEAYBZU-104", Name: "Bugs y severidad", OwnerName: "Roberto Dzul"},
					},
				}, nil
			},
			wantContain: "ZQEAYBZU-104",
		},
		{
			name: "forwards every argument to the service",
			args: map[string]any{
				"site_id":     "abc-123",
				"owner_id":    "ari:cloud:identity::user/712020:aaa",
				"max_results": float64(7),
				"cursor":      "cursor-3",
			},
			mockFn: func(_ context.Context, _ goals.GoalsByOwnerRequest) (*goals.GoalSearchResult, error) {
				return &goals.GoalSearchResult{Goals: []goals.Goal{}}, nil
			},
			checkReqFn: func(t *testing.T, req goals.GoalsByOwnerRequest) {
				if req.SiteID != "abc-123" {
					t.Errorf("SiteID: got %q", req.SiteID)
				}
				if req.OwnerID != "ari:cloud:identity::user/712020:aaa" {
					t.Errorf("OwnerID: got %q", req.OwnerID)
				}
				if req.MaxResults != 7 {
					t.Errorf("MaxResults: got %d, want 7", req.MaxResults)
				}
				if req.Cursor != "cursor-3" {
					t.Errorf("Cursor: got %q, want cursor-3", req.Cursor)
				}
			},
		},
		{
			name:        "missing site_id - error, no service call",
			args:        map[string]any{"owner_id": "712020:aaa"},
			wantIsError: true,
			wantContain: "site_id",
		},
		{
			name:        "missing owner_id - error, no service call",
			args:        map[string]any{"site_id": "abc-123"},
			wantIsError: true,
			wantContain: "owner_id",
		},
		{
			name: "an owner with no goals returns an empty list, not null",
			args: map[string]any{"site_id": "abc-123", "owner_id": "712020:nobody"},
			mockFn: func(_ context.Context, _ goals.GoalsByOwnerRequest) (*goals.GoalSearchResult, error) {
				return &goals.GoalSearchResult{}, nil
			},
			wantContain: `"goals":[]`,
		},
		{
			name: "service error is surfaced",
			args: map[string]any{"site_id": "abc-123", "owner_id": "712020:aaa"},
			mockFn: func(_ context.Context, _ goals.GoalsByOwnerRequest) (*goals.GoalSearchResult, error) {
				return nil, errors.New("some service error")
			},
			wantIsError: true,
			wantContain: "some service error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotReq goals.GoalsByOwnerRequest
			called := false
			svc := &mockGoalsService{}
			if tc.mockFn != nil {
				svc.getGoalsByOwnerFunc = func(ctx context.Context, req goals.GoalsByOwnerRequest) (*goals.GoalSearchResult, error) {
					called = true
					gotReq = req
					return tc.mockFn(ctx, req)
				}
			} else {
				svc.getGoalsByOwnerFunc = func(_ context.Context, _ goals.GoalsByOwnerRequest) (*goals.GoalSearchResult, error) {
					called = true
					t.Error("service must not be called when required arguments are missing")
					return nil, errors.New("should not be called")
				}
			}

			handler := mcpserver.ToolGetGoalsByOwner(svc)
			res, err := handler(context.Background(), makeCallToolRequest(tc.args))
			if err != nil {
				t.Fatalf("handler returned a transport error: %v", err)
			}

			text := getResultText(t, res)
			if res.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", res.IsError, tc.wantIsError, text)
			}
			if tc.wantContain != "" && !strings.Contains(text, tc.wantContain) {
				t.Errorf("result %q does not contain %q", text, tc.wantContain)
			}
			// The missing-argument cases have no mockFn; the stub installed for
			// them already fails if it is reached. A surfaced service error, by
			// contrast, requires the service to have been called.
			if tc.mockFn != nil && !called {
				t.Error("service was never called")
			}
			if tc.checkReqFn != nil {
				tc.checkReqFn(t, gotReq)
			}
		})
	}
}
