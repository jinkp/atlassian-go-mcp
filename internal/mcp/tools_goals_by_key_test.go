package mcpserver_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

// TestToolGetGoalByKeyRouting covers how get_goal picks between the ARI lookup
// and the key lookup. Getting this wrong would silently send a key to the ARI
// endpoint, or an ARI to the key endpoint, and report a confusing not-found.
func TestToolGetGoalByKeyRouting(t *testing.T) {
	tests := []struct {
		name          string
		args          map[string]any
		wantRoute     string // "ari", "key" or "none"
		wantIsError   bool
		wantContain   string
		wantSiteID    string
		wantGoalKeyIn string
	}{
		{
			name:        "goal_id routes to the ARI lookup, unchanged",
			args:        map[string]any{"goal_id": "ari:cloud:townsquare:s:goal/abc"},
			wantRoute:   "ari",
			wantContain: "ari:cloud:townsquare:s:goal/abc",
		},
		{
			name:          "goal_key plus site_id routes to the key lookup",
			args:          map[string]any{"goal_key": "ZQEAYBZU-633", "site_id": "site-abc"},
			wantRoute:     "key",
			wantSiteID:    "site-abc",
			wantGoalKeyIn: "ZQEAYBZU-633",
			wantContain:   "ZQEAYBZU-633",
		},
		{
			name:        "goal_key without site_id is refused, not guessed",
			args:        map[string]any{"goal_key": "ZQEAYBZU-633"},
			wantRoute:   "none",
			wantIsError: true,
			wantContain: "site_id",
		},
		{
			name:        "neither identifier is refused",
			args:        map[string]any{},
			wantRoute:   "none",
			wantIsError: true,
			wantContain: "goal_id",
		},
		{
			name:        "goal_id wins when both are supplied",
			args:        map[string]any{"goal_id": "ari:cloud:townsquare:s:goal/abc", "goal_key": "ZQEAYBZU-633", "site_id": "site-abc"},
			wantRoute:   "ari",
			wantContain: "ari:cloud:townsquare:s:goal/abc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var route string
			var gotSiteID, gotGoalKey string

			svc := &mockGoalsService{
				getGoalFunc: func(_ context.Context, goalID string) (*goals.Goal, error) {
					route = "ari"
					return &goals.Goal{ID: goalID, Name: "By ARI"}, nil
				},
				getGoalByKeyFunc: func(_ context.Context, siteID, goalKey string) (*goals.Goal, error) {
					route = "key"
					gotSiteID, gotGoalKey = siteID, goalKey
					return &goals.Goal{ID: "ari:cloud:townsquare:s:goal/xyz", Key: goalKey, Name: "By key"}, nil
				},
			}

			handler := mcpserver.ToolGetGoal(svc)
			res, err := handler(context.Background(), makeCallToolRequest(tc.args))
			if err != nil {
				t.Fatalf("handler returned a transport error: %v", err)
			}

			text := getResultText(t, res)
			if res.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", res.IsError, tc.wantIsError, text)
			}
			if route != tc.wantRoute && !(tc.wantRoute == "none" && route == "") {
				t.Errorf("route: got %q, want %q", route, tc.wantRoute)
			}
			if tc.wantContain != "" && !strings.Contains(text, tc.wantContain) {
				t.Errorf("result %q does not contain %q", text, tc.wantContain)
			}
			if tc.wantSiteID != "" && gotSiteID != tc.wantSiteID {
				t.Errorf("siteID passed to service: got %q, want %q", gotSiteID, tc.wantSiteID)
			}
			if tc.wantGoalKeyIn != "" && gotGoalKey != tc.wantGoalKeyIn {
				t.Errorf("goalKey passed to service: got %q, want %q", gotGoalKey, tc.wantGoalKeyIn)
			}
		})
	}
}
