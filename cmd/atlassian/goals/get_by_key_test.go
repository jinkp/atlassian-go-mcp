package goals_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	atlcligoals "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/goals"
	goalssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
)

// TestGetCmdRoutesByIdentifierShape covers how "goals get" decides between the
// ARI lookup and the key lookup. The ARI form must keep working untouched.
func TestGetCmdRoutesByIdentifierShape(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantRoute     string
		wantSiteID    string
		wantIdentifer string
	}{
		{
			name:          "a full ARI goes to the ARI lookup and needs no site",
			args:          []string{"ari:cloud:townsquare:s:goal/abc"},
			wantRoute:     "ari",
			wantIdentifer: "ari:cloud:townsquare:s:goal/abc",
		},
		{
			name:          "a short key goes to the key lookup with the site",
			args:          []string{"ZQEAYBZU-633", "--site-id", "site-abc"},
			wantRoute:     "key",
			wantSiteID:    "site-abc",
			wantIdentifer: "ZQEAYBZU-633",
		},
		{
			name:          "surrounding whitespace does not change the routing",
			args:          []string{"  ZQEAYBZU-633  ", "--site-id", "site-abc"},
			wantRoute:     "key",
			wantSiteID:    "site-abc",
			wantIdentifer: "ZQEAYBZU-633",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var route, gotSiteID, gotIdentifier string

			svc := &mockGoalsService{
				getGoalFunc: func(_ context.Context, goalID string) (*goalssvc.Goal, error) {
					route, gotIdentifier = "ari", goalID
					return &goalssvc.Goal{ID: goalID, Name: "By ARI", Status: "on_track"}, nil
				},
				getGoalByKeyFunc: func(_ context.Context, siteID, goalKey string) (*goalssvc.Goal, error) {
					route, gotSiteID, gotIdentifier = "key", siteID, goalKey
					return &goalssvc.Goal{ID: "ari:x", Key: goalKey, Name: "By key", Status: "on_track"}, nil
				},
			}

			cmd := atlcligoals.NewGetCmd(svc)
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(tc.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("command returned error: %v (stderr: %s)", err, stderr.String())
			}

			if route != tc.wantRoute {
				t.Errorf("route: got %q, want %q", route, tc.wantRoute)
			}
			if gotIdentifier != tc.wantIdentifer {
				t.Errorf("identifier passed to service: got %q, want %q", gotIdentifier, tc.wantIdentifer)
			}
			if tc.wantSiteID != "" && gotSiteID != tc.wantSiteID {
				t.Errorf("siteID passed to service: got %q, want %q", gotSiteID, tc.wantSiteID)
			}
			if strings.TrimSpace(stdout.String()) == "" {
				t.Error("expected the goal to be rendered on stdout")
			}
		})
	}
}
