package jira_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// issueFixture builds a minimal Jira issue API JSON response for test servers.
func issueFixture(key, summary, status string) string {
	return `{
		"key": "` + key + `",
		"fields": {
			"summary": "` + summary + `",
			"status":  {"name": "` + status + `"},
			"assignee": {"displayName": "Test User"},
			"priority": {"name": "Medium"},
			"labels":   ["test"],
			"created":  "2024-01-01T00:00:00.000+0000",
			"updated":  "2024-01-02T00:00:00.000+0000"
		}
	}`
}

func searchFixture(total, maxResults int, keys ...string) string {
	issues := make([]map[string]interface{}, len(keys))
	for i, k := range keys {
		issues[i] = map[string]interface{}{
			"key": k,
			"fields": map[string]interface{}{
				"summary":  "Issue " + k,
				"status":   map[string]string{"name": "Open"},
				"assignee": nil,
				"priority": map[string]string{"name": "Low"},
				"labels":   []string{},
				"created":  "2024-01-01T00:00:00.000+0000",
				"updated":  "2024-01-01T00:00:00.000+0000",
			},
		}
	}
	data, _ := json.Marshal(map[string]interface{}{
		"total":      total,
		"startAt":    0,
		"maxResults": maxResults,
		"issues":     issues,
	})
	return string(data)
}

// --- GetIssue tests ---

func TestJiraService_GetIssue_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(issueFixture("PROJ-1", "Fix login bug", "In Progress"))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	issue, err := svc.GetIssue(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("GetIssue() unexpected error: %v", err)
	}
	if issue.Key != "PROJ-1" {
		t.Errorf("Key: expected PROJ-1, got %s", issue.Key)
	}
	if issue.Summary != "Fix login bug" {
		t.Errorf("Summary: expected 'Fix login bug', got %s", issue.Summary)
	}
	if issue.Status != "In Progress" {
		t.Errorf("Status: expected 'In Progress', got %s", issue.Status)
	}
}

func TestJiraService_GetIssue_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetIssue(context.Background(), "PROJ-999")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("expected errors.Is(err, ErrNotFound), got: %v", err)
	}
}

func TestJiraService_GetIssue_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetIssue(context.Background(), "PROJ-1")
	if err == nil {
		t.Fatal("expected ErrUnauthorized, got nil")
	}
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected errors.Is(err, ErrUnauthorized), got: %v", err)
	}
}

func TestJiraService_GetIssue_Forbidden(t *testing.T) {
	// Triangulate: 403 also maps to ErrUnauthorized
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetIssue(context.Background(), "PROJ-1")
	if err == nil {
		t.Fatal("expected ErrUnauthorized for 403, got nil")
	}
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected errors.Is(err, ErrUnauthorized) for 403, got: %v", err)
	}
}

// --- SearchIssues tests ---

func TestJiraService_SearchIssues_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("jql") == "" {
			t.Error("expected jql query parameter")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(searchFixture(3, 10, "PROJ-1", "PROJ-2", "PROJ-3"))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	result, err := svc.SearchIssues(context.Background(), "project = PROJ ORDER BY updated DESC", 10)
	if err != nil {
		t.Fatalf("SearchIssues() unexpected error: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("Total: expected 3, got %d", result.Total)
	}
	if len(result.Issues) != 3 {
		t.Errorf("len(Issues): expected 3, got %d", len(result.Issues))
	}
	if result.Issues[0].Key != "PROJ-1" {
		t.Errorf("Issues[0].Key: expected PROJ-1, got %s", result.Issues[0].Key)
	}
}

func TestJiraService_SearchIssues_InvalidJQL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Error in the JQL query: Unknown operator '???'"]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.SearchIssues(context.Background(), "ORDER BY ???", 50)
	if err == nil {
		t.Fatal("expected ErrInvalidJQL, got nil")
	}
	if !errors.Is(err, jira.ErrInvalidJQL) {
		t.Errorf("expected errors.Is(err, ErrInvalidJQL), got: %v", err)
	}
}

func TestJiraService_SearchIssues_DefaultMaxResults(t *testing.T) {
	var gotMaxResults string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMaxResults = r.URL.Query().Get("maxResults")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(searchFixture(0, 50))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	// maxResults=0 should default to 50
	_, err := svc.SearchIssues(context.Background(), "project = PROJ", 0)
	if err != nil {
		t.Fatalf("SearchIssues() unexpected error: %v", err)
	}
	if gotMaxResults != "50" {
		t.Errorf("expected maxResults=50 as default, got %q", gotMaxResults)
	}
}

func TestJiraService_SearchIssues_PaginationParams(t *testing.T) {
	var gotStartAt, gotMaxResults string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotStartAt = r.URL.Query().Get("startAt")
		gotMaxResults = r.URL.Query().Get("maxResults")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(searchFixture(120, 50, "PROJ-51"))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	result, err := svc.SearchIssues(context.Background(), "project = PROJ", 50)
	if err != nil {
		t.Fatalf("SearchIssues() unexpected error: %v", err)
	}
	if gotMaxResults != "50" {
		t.Errorf("maxResults: expected 50, got %q", gotMaxResults)
	}
	if gotStartAt != "0" {
		t.Errorf("startAt: expected 0, got %q", gotStartAt)
	}
	if result.Total != 120 {
		t.Errorf("Total: expected 120, got %d", result.Total)
	}
}

// --- Phase 3: CreateIssue tests ---

func TestJiraService_CreateIssue_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"key":"PROJ-42","id":"10042"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	resp, err := svc.CreateIssue(context.Background(), jira.CreateIssueRequest{
		ProjectKey: "PROJ",
		IssueType:  "Task",
		Summary:    "New task",
	})
	if err != nil {
		t.Fatalf("CreateIssue() unexpected error: %v", err)
	}
	if resp.Key != "PROJ-42" {
		t.Errorf("Key: expected PROJ-42, got %s", resp.Key)
	}
	if resp.ID != "10042" {
		t.Errorf("ID: expected 10042, got %s", resp.ID)
	}
}

func TestJiraService_CreateIssue_WithOptionalFields(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"key":"PROJ-1","id":"10001"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateIssue(context.Background(), jira.CreateIssueRequest{
		ProjectKey:   "PROJ",
		IssueType:    "Bug",
		Summary:      "Fix the bug",
		Description:  "Fix the login page",
		AssigneeID:   "abc123",
		Labels:       []string{"backend", "urgent"},
		PriorityName: "High",
	})
	if err != nil {
		t.Fatalf("CreateIssue() unexpected error: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	fields, ok := body["fields"].(map[string]interface{})
	if !ok {
		t.Fatalf("fields not found in request body")
	}

	// Verify project key
	project, _ := fields["project"].(map[string]interface{})
	if project["key"] != "PROJ" {
		t.Errorf("project.key: expected PROJ, got %v", project["key"])
	}

	// Verify issuetype name
	issuetype, _ := fields["issuetype"].(map[string]interface{})
	if issuetype["name"] != "Bug" {
		t.Errorf("issuetype.name: expected Bug, got %v", issuetype["name"])
	}

	// Verify summary
	if fields["summary"] != "Fix the bug" {
		t.Errorf("summary: expected 'Fix the bug', got %v", fields["summary"])
	}

	// Verify description is ADF with the text
	desc, ok := fields["description"].(map[string]interface{})
	if !ok {
		t.Fatalf("description not ADF object, got %T", fields["description"])
	}
	if desc["type"] != "doc" {
		t.Errorf("description.type: expected 'doc', got %v", desc["type"])
	}

	// Verify assignee
	assignee, _ := fields["assignee"].(map[string]interface{})
	if assignee["accountId"] != "abc123" {
		t.Errorf("assignee.accountId: expected 'abc123', got %v", assignee["accountId"])
	}

	// Verify priority
	priority, _ := fields["priority"].(map[string]interface{})
	if priority["name"] != "High" {
		t.Errorf("priority.name: expected 'High', got %v", priority["name"])
	}

	// Verify labels
	labels, _ := fields["labels"].([]interface{})
	if len(labels) != 2 || labels[0] != "backend" || labels[1] != "urgent" {
		t.Errorf("labels: expected [backend urgent], got %v", labels)
	}
}

func TestJiraService_CreateIssue_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateIssue(context.Background(), jira.CreateIssueRequest{ProjectKey: "PROJ", IssueType: "Task", Summary: "Test"})
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestJiraService_CreateIssue_Forbidden(t *testing.T) {
	// Triangulate: 403 also maps to ErrUnauthorized
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateIssue(context.Background(), jira.CreateIssueRequest{ProjectKey: "PROJ", IssueType: "Task", Summary: "Test"})
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for 403, got: %v", err)
	}
}

func TestJiraService_CreateIssue_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateIssue(context.Background(), jira.CreateIssueRequest{ProjectKey: "PROJ", IssueType: "Task", Summary: "Test"})
	if !errors.Is(err, jira.ErrConflict) {
		t.Errorf("expected ErrConflict, got: %v", err)
	}
}

func TestJiraService_CreateIssue_BadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Field 'summary' is required"]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateIssue(context.Background(), jira.CreateIssueRequest{ProjectKey: "PROJ", IssueType: "Task", Summary: "Test"})
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if errors.Is(err, jira.ErrUnauthorized) || errors.Is(err, jira.ErrConflict) {
		t.Errorf("400 should produce descriptive error, not sentinel: %v", err)
	}
}

func TestJiraService_CreateIssue_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateIssue(context.Background(), jira.CreateIssueRequest{ProjectKey: "PROJ", IssueType: "Task", Summary: "Test"})
	if !errors.Is(err, jira.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- Phase 3: UpdateIssue tests ---

func TestJiraService_UpdateIssue_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	summary := "Updated title"
	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.UpdateIssue(context.Background(), "PROJ-1", jira.UpdateIssueRequest{
		Summary: &summary,
	})
	if err != nil {
		t.Fatalf("UpdateIssue() unexpected error: %v", err)
	}
}

func TestJiraService_UpdateIssue_PartialUpdate(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	summary := "Updated title"
	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.UpdateIssue(context.Background(), "PROJ-1", jira.UpdateIssueRequest{
		Summary: &summary,
		// Description, AssigneeID, PriorityName are nil — must NOT appear in body
	})
	if err != nil {
		t.Fatalf("UpdateIssue() unexpected error: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	fields, ok := body["fields"].(map[string]interface{})
	if !ok {
		t.Fatalf("fields not found in request body")
	}

	if fields["summary"] != "Updated title" {
		t.Errorf("summary: expected 'Updated title', got %v", fields["summary"])
	}
	if _, found := fields["assignee"]; found {
		t.Error("assignee should NOT be in partial update body")
	}
	if _, found := fields["priority"]; found {
		t.Error("priority should NOT be in partial update body")
	}
	if _, found := fields["description"]; found {
		t.Error("description should NOT be in partial update body when nil")
	}
}

func TestJiraService_UpdateIssue_WithDescription(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	desc := "New desc"
	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.UpdateIssue(context.Background(), "PROJ-1", jira.UpdateIssueRequest{
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("UpdateIssue() unexpected error: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	fields, _ := body["fields"].(map[string]interface{})
	descField, ok := fields["description"].(map[string]interface{})
	if !ok {
		t.Fatalf("description field is not ADF object, got %T", fields["description"])
	}
	if descField["type"] != "doc" {
		t.Errorf("description ADF type: expected 'doc', got %v", descField["type"])
	}
}

func TestJiraService_UpdateIssue_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	summary := "x"
	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.UpdateIssue(context.Background(), "PROJ-999", jira.UpdateIssueRequest{Summary: &summary})
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestJiraService_UpdateIssue_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	summary := "x"
	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.UpdateIssue(context.Background(), "PROJ-1", jira.UpdateIssueRequest{Summary: &summary})
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestJiraService_UpdateIssue_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	summary := "x"
	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.UpdateIssue(context.Background(), "PROJ-1", jira.UpdateIssueRequest{Summary: &summary})
	if !errors.Is(err, jira.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- Phase 3: GetTransitions tests ---

func TestJiraService_GetTransitions_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1/transitions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"transitions":[{"id":"11","name":"In Progress","to":{"statusCategory":{"key":"indeterminate"}}},{"id":"21","name":"Done","to":{"statusCategory":{"key":"done"}}}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	transitions, err := svc.GetTransitions(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("GetTransitions() unexpected error: %v", err)
	}
	if len(transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(transitions))
	}
	if transitions[0].ID != "11" {
		t.Errorf("transitions[0].ID: expected '11', got %s", transitions[0].ID)
	}
	if transitions[0].Name != "In Progress" {
		t.Errorf("transitions[0].Name: expected 'In Progress', got %s", transitions[0].Name)
	}
	if transitions[0].StatusCategory != "indeterminate" {
		t.Errorf("transitions[0].StatusCategory: expected 'indeterminate', got %s", transitions[0].StatusCategory)
	}
	if transitions[1].ID != "21" {
		t.Errorf("transitions[1].ID: expected '21', got %s", transitions[1].ID)
	}
	if transitions[1].Name != "Done" {
		t.Errorf("transitions[1].Name: expected 'Done', got %s", transitions[1].Name)
	}
}

func TestJiraService_GetTransitions_Empty(t *testing.T) {
	// Triangulate: empty transitions list returns []Transition (not nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"transitions":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	transitions, err := svc.GetTransitions(context.Background(), "PROJ-2")
	if err != nil {
		t.Fatalf("GetTransitions() unexpected error: %v", err)
	}
	if transitions == nil {
		t.Error("expected empty []Transition slice, got nil")
	}
	if len(transitions) != 0 {
		t.Errorf("expected 0 transitions, got %d", len(transitions))
	}
}

func TestJiraService_GetTransitions_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetTransitions(context.Background(), "PROJ-999")
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestJiraService_GetTransitions_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetTransitions(context.Background(), "PROJ-1")
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestJiraService_GetTransitions_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetTransitions(context.Background(), "PROJ-1")
	if !errors.Is(err, jira.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- Phase 3: TransitionIssue tests ---

func TestJiraService_TransitionIssue_Success(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1/transitions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.TransitionIssue(context.Background(), "PROJ-1", "21")
	if err != nil {
		t.Fatalf("TransitionIssue() unexpected error: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	transition, _ := body["transition"].(map[string]interface{})
	if transition["id"] != "21" {
		t.Errorf("transition.id: expected '21', got %v", transition["id"])
	}
}

func TestJiraService_TransitionIssue_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.TransitionIssue(context.Background(), "PROJ-999", "21")
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestJiraService_TransitionIssue_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.TransitionIssue(context.Background(), "PROJ-1", "21")
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestJiraService_TransitionIssue_BadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Invalid transition ID"]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.TransitionIssue(context.Background(), "PROJ-1", "invalid")
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if errors.Is(err, jira.ErrUnauthorized) || errors.Is(err, jira.ErrNotFound) {
		t.Errorf("400 should produce descriptive error, not sentinel: %v", err)
	}
}

func TestJiraService_TransitionIssue_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.TransitionIssue(context.Background(), "PROJ-1", "21")
	if !errors.Is(err, jira.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}
