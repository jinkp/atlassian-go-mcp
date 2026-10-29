package goals_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	atlcligoals "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/goals"
	goalssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
)

// runByOwnerCmd executes the by-owner command against svc and returns stdout.
func runByOwnerCmd(t *testing.T, svc goalssvc.GoalsService, args ...string) string {
	t.Helper()

	cmd := atlcligoals.NewByOwnerCmd(svc)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command returned error: %v (stderr: %s)", err, stderr.String())
	}
	return stdout.String()
}

func TestByOwnerCmd_RendersTheOwnersGoals(t *testing.T) {
	svc := &mockGoalsService{
		getGoalsByOwnerFunc: func(_ context.Context, _ goalssvc.GoalsByOwnerRequest) (*goalssvc.GoalSearchResult, error) {
			return &goalssvc.GoalSearchResult{
				Goals: []goalssvc.Goal{
					{ID: "g1", Key: "ZQEAYBZU-104", Name: "Bugs y severidad", Status: "off_track", OwnerName: "Roberto Dzul"},
				},
			}, nil
		},
	}

	stdout := runByOwnerCmd(t, svc, "--site-id", "abc-123", "--owner", "712020:aaa", "-o", "json")

	for _, want := range []string{"ZQEAYBZU-104", "Bugs y severidad", "off_track"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output missing %q\nstdout: %s", want, stdout)
		}
	}
}

func TestByOwnerCmd_ForwardsFlagsToService(t *testing.T) {
	var got goalssvc.GoalsByOwnerRequest
	svc := &mockGoalsService{
		getGoalsByOwnerFunc: func(_ context.Context, req goalssvc.GoalsByOwnerRequest) (*goalssvc.GoalSearchResult, error) {
			got = req
			return &goalssvc.GoalSearchResult{Goals: []goalssvc.Goal{}}, nil
		},
	}

	runByOwnerCmd(t, svc,
		"--site-id", "abc-123",
		"--owner", "712020:aaa",
		"--max-results", "7",
		"--cursor", "cursor-3",
	)

	if got.SiteID != "abc-123" {
		t.Errorf("SiteID: got %q", got.SiteID)
	}
	if got.OwnerID != "712020:aaa" {
		t.Errorf("OwnerID: got %q", got.OwnerID)
	}
	if got.MaxResults != 7 {
		t.Errorf("MaxResults: got %d, want 7", got.MaxResults)
	}
	if got.Cursor != "cursor-3" {
		t.Errorf("Cursor: got %q, want cursor-3", got.Cursor)
	}
}

func TestByOwnerCmd_OwnerWithNoGoalsSaysSoWithoutErroring(t *testing.T) {
	svc := &mockGoalsService{
		getGoalsByOwnerFunc: func(_ context.Context, _ goalssvc.GoalsByOwnerRequest) (*goalssvc.GoalSearchResult, error) {
			return &goalssvc.GoalSearchResult{Goals: []goalssvc.Goal{}}, nil
		},
	}

	stdout := runByOwnerCmd(t, svc, "--site-id", "abc-123", "--owner", "712020:nobody")

	if !strings.Contains(stdout, "No goals found for that owner") {
		t.Errorf("output should state the owner has no goals\nstdout: %s", stdout)
	}
}

// --- Both identifiers are required; cobra must enforce it. ---

func TestByOwnerCmd_RequiresSiteIDAndOwner(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErrMsg string
	}{
		{name: "owner missing", args: []string{"--site-id", "abc-123"}, wantErrMsg: "owner"},
		{name: "site-id missing", args: []string{"--owner", "712020:aaa"}, wantErrMsg: "site-id"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockGoalsService{
				getGoalsByOwnerFunc: func(_ context.Context, _ goalssvc.GoalsByOwnerRequest) (*goalssvc.GoalSearchResult, error) {
					t.Error("service must not be called when a required flag is missing")
					return nil, nil
				},
			}

			cmd := atlcligoals.NewByOwnerCmd(svc)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SilenceUsage = true
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected an error for a missing required flag, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErrMsg) {
				t.Errorf("error %q should mention %q", err.Error(), tc.wantErrMsg)
			}
		})
	}
}
