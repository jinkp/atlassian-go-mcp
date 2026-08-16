package releases_test

import (
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
)

func doneStatuses() []string   { return []string{"Done", "Closed"} }
func criticalLabels() []string { return []string{"critical"} }

func TestEvaluate_AllIssuesDone_Ready(t *testing.T) {
	issues := []jira.Issue{
		{Key: "P-1", Status: "Done", IssueType: "Story"},
		{Key: "P-2", Status: "Closed", IssueType: "Task"},
	}
	e := releases.NewValidationEngine(doneStatuses(), criticalLabels())
	res := e.Evaluate(issues, []string{"all_issues_done"})
	if !res.Ready {
		t.Fatalf("expected ready, got errors: %v", res.Errors)
	}
	if len(res.Errors) != 0 {
		t.Errorf("expected no errors, got %v", res.Errors)
	}
}

func TestEvaluate_AllIssuesDone_NotReady(t *testing.T) {
	issues := []jira.Issue{
		{Key: "P-1", Status: "In Progress", IssueType: "Story"},
		{Key: "P-2", Status: "Done", IssueType: "Task"},
	}
	e := releases.NewValidationEngine(doneStatuses(), criticalLabels())
	res := e.Evaluate(issues, []string{"all_issues_done"})
	if res.Ready {
		t.Fatal("expected not ready")
	}
	if len(res.Errors) != 1 {
		t.Errorf("expected 1 error, got %v", res.Errors)
	}
}

func TestEvaluate_NoCriticalBugsOpen(t *testing.T) {
	issues := []jira.Issue{
		{Key: "P-1", Status: "Open", IssueType: "Bug", Labels: []string{"critical"}},
	}
	e := releases.NewValidationEngine(doneStatuses(), criticalLabels())
	res := e.Evaluate(issues, []string{"no_critical_bugs_open"})
	if res.Ready {
		t.Fatal("expected not ready due to open critical bug")
	}
}

func TestEvaluate_NoCriticalBugsOpen_DoneBugIgnored(t *testing.T) {
	issues := []jira.Issue{
		{Key: "P-1", Status: "Done", IssueType: "Bug", Labels: []string{"critical"}},
	}
	e := releases.NewValidationEngine(doneStatuses(), criticalLabels())
	res := e.Evaluate(issues, []string{"no_critical_bugs_open"})
	if !res.Ready {
		t.Fatalf("done critical bug should not block: %v", res.Errors)
	}
}

func TestEvaluate_NoBlockingIssues(t *testing.T) {
	issues := []jira.Issue{
		{Key: "P-1", Status: "Open", IssueType: "Task", Labels: []string{"blocker"}},
	}
	e := releases.NewValidationEngine(doneStatuses(), criticalLabels())
	res := e.Evaluate(issues, []string{"no_blocking_issues"})
	if res.Ready {
		t.Fatal("expected not ready due to blocker label")
	}
}

func TestEvaluate_MinIssuesCount_Empty(t *testing.T) {
	e := releases.NewValidationEngine(doneStatuses(), criticalLabels())
	res := e.Evaluate([]jira.Issue{}, []string{"min_issues_count"})
	if res.Ready {
		t.Fatal("expected not ready for empty release")
	}
}

func TestEvaluate_UnknownRuleWarns(t *testing.T) {
	e := releases.NewValidationEngine(doneStatuses(), criticalLabels())
	res := e.Evaluate([]jira.Issue{{Key: "P-1", Status: "Done"}}, []string{"nope"})
	if len(res.Warnings) != 1 {
		t.Errorf("expected 1 warning for unknown rule, got %v", res.Warnings)
	}
	if !res.Ready {
		t.Error("unknown rule should not flip Ready to false")
	}
}

func TestEvaluate_DefaultRulesWhenEmpty(t *testing.T) {
	issues := []jira.Issue{
		{Key: "P-1", Status: "In Progress", IssueType: "Bug", Labels: []string{"critical"}},
	}
	e := releases.NewValidationEngine(doneStatuses(), criticalLabels())
	res := e.Evaluate(issues, nil)
	if res.Ready {
		t.Fatal("expected default rules to run and fail")
	}
}

func TestNewValidationEngine_DefaultsFallback(t *testing.T) {
	// Empty config should fall back to package defaults (Done is a default done status).
	e := releases.NewValidationEngine(nil, nil)
	issues := []jira.Issue{{Key: "P-1", Status: "Done", IssueType: "Task"}}
	res := e.Evaluate(issues, []string{"all_issues_done"})
	if !res.Ready {
		t.Fatalf("expected ready with default done statuses, got %v", res.Errors)
	}
}
