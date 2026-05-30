package jira_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

func TestIssue_JSONUnmarshal(t *testing.T) {
	raw := `{
		"key": "PROJ-1",
		"fields": {
			"summary": "Fix login bug",
			"status":  {"name": "In Progress"},
			"assignee": {"displayName": "Alice"},
			"priority": {"name": "High"},
			"labels":   ["backend", "auth"],
			"created":  "2024-01-15T10:00:00.000+0000",
			"updated":  "2024-01-16T12:00:00.000+0000"
		}
	}`

	var raw2 jira.IssueAPIResponse
	if err := json.Unmarshal([]byte(raw), &raw2); err != nil {
		t.Fatalf("failed to unmarshal IssueAPIResponse: %v", err)
	}

	issue := raw2.ToIssue()
	if issue.Key != "PROJ-1" {
		t.Errorf("Key: expected PROJ-1, got %s", issue.Key)
	}
	if issue.Summary != "Fix login bug" {
		t.Errorf("Summary: expected 'Fix login bug', got %s", issue.Summary)
	}
	if issue.Status != "In Progress" {
		t.Errorf("Status: expected 'In Progress', got %s", issue.Status)
	}
	if issue.Assignee != "Alice" {
		t.Errorf("Assignee: expected 'Alice', got %s", issue.Assignee)
	}
	if issue.Priority != "High" {
		t.Errorf("Priority: expected 'High', got %s", issue.Priority)
	}
	if len(issue.Labels) != 2 || issue.Labels[0] != "backend" || issue.Labels[1] != "auth" {
		t.Errorf("Labels: expected [backend auth], got %v", issue.Labels)
	}
	if issue.Created.IsZero() {
		t.Error("Created should not be zero")
	}
	if issue.Updated.IsZero() {
		t.Error("Updated should not be zero")
	}
}

func TestIssue_JSONUnmarshal_NullAssignee(t *testing.T) {
	// Triangulate: unassigned issue has null assignee
	raw := `{
		"key": "PROJ-2",
		"fields": {
			"summary": "Unassigned task",
			"status":  {"name": "Open"},
			"assignee": null,
			"priority": {"name": "Low"},
			"labels":   [],
			"created":  "2024-01-15T10:00:00.000+0000",
			"updated":  "2024-01-15T10:00:00.000+0000"
		}
	}`

	var raw2 jira.IssueAPIResponse
	if err := json.Unmarshal([]byte(raw), &raw2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	issue := raw2.ToIssue()
	if issue.Assignee != "" {
		t.Errorf("expected empty assignee for null, got %q", issue.Assignee)
	}
}

func TestSearchResult_JSONUnmarshal(t *testing.T) {
	raw := `{
		"total": 120,
		"startAt": 0,
		"maxResults": 50,
		"issues": [
			{
				"key": "PROJ-1",
				"fields": {
					"summary": "Issue one",
					"status":  {"name": "Open"},
					"assignee": null,
					"priority": {"name": "Medium"},
					"labels":   [],
					"created":  "2024-01-01T00:00:00.000+0000",
					"updated":  "2024-01-01T00:00:00.000+0000"
				}
			},
			{
				"key": "PROJ-2",
				"fields": {
					"summary": "Issue two",
					"status":  {"name": "Done"},
					"assignee": {"displayName": "Bob"},
					"priority": {"name": "Low"},
					"labels":   ["frontend"],
					"created":  "2024-01-02T00:00:00.000+0000",
					"updated":  "2024-01-02T00:00:00.000+0000"
				}
			}
		]
	}`

	var result jira.SearchAPIResponse
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("failed to unmarshal SearchAPIResponse: %v", err)
	}

	sr := result.ToSearchResult()
	if sr.Total != 120 {
		t.Errorf("Total: expected 120, got %d", sr.Total)
	}
	if sr.MaxResults != 50 {
		t.Errorf("MaxResults: expected 50, got %d", sr.MaxResults)
	}
	if len(sr.Issues) != 2 {
		t.Fatalf("Issues: expected 2, got %d", len(sr.Issues))
	}
	if sr.Issues[0].Key != "PROJ-1" {
		t.Errorf("Issues[0].Key: expected PROJ-1, got %s", sr.Issues[0].Key)
	}
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	errs := []error{
		jira.ErrNotFound,
		jira.ErrUnauthorized,
		jira.ErrRateLimit,
		jira.ErrInvalidJQL,
	}
	for i := 0; i < len(errs); i++ {
		for j := i + 1; j < len(errs); j++ {
			if errors.Is(errs[i], errs[j]) {
				t.Errorf("sentinel errors[%d] and [%d] should be distinct", i, j)
			}
		}
	}
}

// --- Phase 3: ErrConflict sentinel ---

func TestErrConflict_IsDistinctFromOtherSentinels(t *testing.T) {
	others := []error{
		jira.ErrNotFound,
		jira.ErrUnauthorized,
		jira.ErrRateLimit,
		jira.ErrInvalidJQL,
	}
	for _, other := range others {
		if errors.Is(jira.ErrConflict, other) {
			t.Errorf("ErrConflict should be distinct from %v", other)
		}
		if errors.Is(other, jira.ErrConflict) {
			t.Errorf("%v should be distinct from ErrConflict", other)
		}
	}
}

func TestErrConflict_DetectableViaErrorsIs(t *testing.T) {
	err := jira.ErrConflict
	if !errors.Is(err, jira.ErrConflict) {
		t.Error("errors.Is(ErrConflict, ErrConflict) must be true")
	}
}

func TestIssue_TimeParsing(t *testing.T) {
	// Verify timestamps are parsed into proper UTC time.Time values
	raw := `{
		"key": "PROJ-3",
		"fields": {
			"summary": "Time test",
			"status":  {"name": "Open"},
			"assignee": null,
			"priority": {"name": "Low"},
			"labels":   [],
			"created":  "2024-03-15T08:30:00.000+0000",
			"updated":  "2024-03-16T14:45:00.000+0000"
		}
	}`

	var raw2 jira.IssueAPIResponse
	if err := json.Unmarshal([]byte(raw), &raw2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	issue := raw2.ToIssue()
	expectedCreated := time.Date(2024, 3, 15, 8, 30, 0, 0, time.UTC)
	if !issue.Created.Equal(expectedCreated) {
		t.Errorf("Created: expected %v, got %v", expectedCreated, issue.Created)
	}
}
