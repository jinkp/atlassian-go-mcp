package goals_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	atlcligoals "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/goals"
	goalssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
)

// runSearchCmd executes the search command against svc and returns stdout, stderr.
func runSearchCmd(t *testing.T, svc goalssvc.GoalsService, args ...string) (string, string) {
	t.Helper()

	cmd := atlcligoals.NewSearchCmd(svc)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}
	return stdout.String(), stderr.String()
}

// --- A bounded scan that found nothing must not be reported as "nothing exists". ---
//
// The whole point of fixing --query was to stop reporting a result the data does
// not support. Printing the usual "No goals found" after a scan that ran out of
// page budget would reintroduce exactly that defect at the presentation layer.

func TestSearchCmd_IncompleteScanWithNoMatchesDoesNotClaimNothingFound(t *testing.T) {
	svc := &mockGoalsService{
		searchGoalsFunc: func(_ context.Context, _ goalssvc.SearchGoalsRequest) (*goalssvc.GoalSearchResult, error) {
			return &goalssvc.GoalSearchResult{
				Goals:          []goalssvc.Goal{},
				HasMore:        true,
				NextCursor:     "cursor-20",
				ScanIncomplete: true,
			}, nil
		},
	}

	stdout, stderr := runSearchCmd(t, svc, "--site-id", "abc-123", "--query", "analytics")
	combined := stdout + stderr

	if strings.Contains(stdout, "No goals found") {
		t.Errorf("an incomplete scan must not report the definitive %q\nstdout: %s", "No goals found", stdout)
	}
	if !strings.Contains(strings.ToLower(combined), "incomplete") {
		t.Errorf("output must tell the user the scan was incomplete\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	if !strings.Contains(combined, "cursor-20") {
		t.Errorf("output must surface the cursor so the scan can be resumed\nstdout: %s\nstderr: %s", stdout, stderr)
	}
}

// --- An incomplete scan that did find matches must still warn. ---

func TestSearchCmd_IncompleteScanWithMatchesStillWarns(t *testing.T) {
	svc := &mockGoalsService{
		searchGoalsFunc: func(_ context.Context, _ goalssvc.SearchGoalsRequest) (*goalssvc.GoalSearchResult, error) {
			return &goalssvc.GoalSearchResult{
				Goals:          []goalssvc.Goal{{ID: "g1", Key: "K-1", Name: "Analytics rollout", Status: "on_track"}},
				HasMore:        true,
				NextCursor:     "cursor-20",
				ScanIncomplete: true,
			}, nil
		},
	}

	stdout, stderr := runSearchCmd(t, svc, "--site-id", "abc-123", "--query", "analytics")

	if !strings.Contains(stdout, "Analytics rollout") {
		t.Errorf("matches must still be rendered\nstdout: %s", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "incomplete") {
		t.Errorf("an incomplete scan must warn on stderr so -o json stays parseable\nstderr: %s", stderr)
	}
	if strings.Contains(stdout, "incomplete") {
		t.Errorf("the warning must not pollute stdout, which may carry JSON\nstdout: %s", stdout)
	}
}

// --- A complete scan that found nothing is allowed to say so. ---

func TestSearchCmd_CompleteScanWithNoMatchesReportsNoGoals(t *testing.T) {
	svc := &mockGoalsService{
		searchGoalsFunc: func(_ context.Context, _ goalssvc.SearchGoalsRequest) (*goalssvc.GoalSearchResult, error) {
			return &goalssvc.GoalSearchResult{Goals: []goalssvc.Goal{}}, nil
		},
	}

	stdout, stderr := runSearchCmd(t, svc, "--site-id", "abc-123", "--query", "zzzznotexist")

	if !strings.Contains(stdout, "No goals found") {
		t.Errorf("a complete scan with no matches should say so plainly\nstdout: %s", stdout)
	}
	if strings.Contains(strings.ToLower(stderr), "incomplete") {
		t.Errorf("a complete scan must not warn about incompleteness\nstderr: %s", stderr)
	}
}

// --- Reporting a resume cursor is useless if the cursor cannot be passed back. ---

func TestSearchCmd_ForwardsCursorFlagToService(t *testing.T) {
	var got goalssvc.SearchGoalsRequest
	svc := &mockGoalsService{
		searchGoalsFunc: func(_ context.Context, req goalssvc.SearchGoalsRequest) (*goalssvc.GoalSearchResult, error) {
			got = req
			return &goalssvc.GoalSearchResult{Goals: []goalssvc.Goal{}}, nil
		},
	}

	runSearchCmd(t, svc, "--site-id", "abc-123", "--query", "analytics", "--cursor", "cursor-20")

	if got.Cursor != "cursor-20" {
		t.Errorf("Cursor: got %q, want %q", got.Cursor, "cursor-20")
	}
}
