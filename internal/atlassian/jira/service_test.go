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
		if r.URL.Path != "/rest/api/3/search/jql" {
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
	var gotMaxResults string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	// Note: startAt is no longer sent — the new /search/jql endpoint uses nextPageToken
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

// --- Phase 2 expansion: LookupAccountID tests ---

func TestJiraService_LookupAccountID_Success(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		maxResults   int
		responseJSON string
		wantCount    int
		wantEmail    string // email of first user
	}{
		{
			name:         "finds users",
			query:        "Jane",
			maxResults:   5,
			responseJSON: `[{"accountId":"acc1","displayName":"Jane Doe","emailAddress":"jane@example.com","active":true}]`,
			wantCount:    1,
			wantEmail:    "jane@example.com",
		},
		{
			name:         "empty result",
			query:        "NoMatchAtAll",
			maxResults:   10,
			responseJSON: `[]`,
			wantCount:    0,
		},
		{
			name:         "privacy-hidden email returns empty string",
			query:        "Hidden",
			maxResults:   5,
			responseJSON: `[{"accountId":"acc2","displayName":"Hidden User","emailAddress":"","active":true}]`,
			wantCount:    1,
			wantEmail:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMaxResults string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/rest/api/3/user/search" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				gotMaxResults = r.URL.Query().Get("maxResults")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.responseJSON)) //nolint:errcheck
			}))
			defer srv.Close()

			svc := jira.NewService(srv.Client(), srv.URL)
			users, err := svc.LookupAccountID(context.Background(), tt.query, tt.maxResults)
			if err != nil {
				t.Fatalf("LookupAccountID() unexpected error: %v", err)
			}
			if len(users) != tt.wantCount {
				t.Errorf("expected %d users, got %d", tt.wantCount, len(users))
			}
			if tt.wantCount > 0 && users[0].Email != tt.wantEmail {
				t.Errorf("email: expected %q, got %q", tt.wantEmail, users[0].Email)
			}
			_ = gotMaxResults
		})
	}
}

func TestJiraService_LookupAccountID_DefaultMaxResults(t *testing.T) {
	var gotMaxResults string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMaxResults = r.URL.Query().Get("maxResults")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	// maxResults=0 should default to 10 (token-saving default for account lookup)
	_, err := svc.LookupAccountID(context.Background(), "anyone", 0)
	if err != nil {
		t.Fatalf("LookupAccountID() unexpected error: %v", err)
	}
	if gotMaxResults != "10" {
		t.Errorf("expected default maxResults=10, got %q", gotMaxResults)
	}
}

func TestJiraService_LookupAccountID_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.LookupAccountID(context.Background(), "Jane", 5)
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestJiraService_LookupAccountID_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.LookupAccountID(context.Background(), "Jane", 5)
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for 403, got: %v", err)
	}
}

func TestJiraService_LookupAccountID_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.LookupAccountID(context.Background(), "Jane", 5)
	if !errors.Is(err, jira.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- Phase 2 expansion: AddComment tests ---

func TestJiraService_AddComment_Success(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1/comment" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"101","author":{"displayName":"Alice"},"body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Looks good"}]}]},"created":"2026-08-16T10:00:00.000+0000","updated":"2026-08-16T10:00:00.000+0000"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	comment, err := svc.AddComment(context.Background(), "PROJ-1", "Looks good")
	if err != nil {
		t.Fatalf("AddComment() unexpected error: %v", err)
	}
	if comment.ID != "101" {
		t.Errorf("ID: expected 101, got %s", comment.ID)
	}
	if comment.Author != "Alice" {
		t.Errorf("Author: expected Alice, got %s", comment.Author)
	}
	if comment.Body != "Looks good" {
		t.Errorf("Body: expected 'Looks good', got %q", comment.Body)
	}

	// Verify the request body contains ADF structure (not raw text).
	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	adfBody, ok := body["body"].(map[string]interface{})
	if !ok {
		t.Fatalf("request body.body is not ADF object, got %T", body["body"])
	}
	if adfBody["type"] != "doc" {
		t.Errorf("ADF body type: expected 'doc', got %v", adfBody["type"])
	}
}

func TestJiraService_AddComment_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.AddComment(context.Background(), "PROJ-999", "hello")
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestJiraService_AddComment_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.AddComment(context.Background(), "PROJ-1", "hello")
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestJiraService_AddComment_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.AddComment(context.Background(), "PROJ-1", "hello")
	if !errors.Is(err, jira.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

func TestJiraService_AddComment_BadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Invalid body"]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.AddComment(context.Background(), "PROJ-1", "hello")
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if errors.Is(err, jira.ErrNotFound) || errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("400 should produce descriptive error, not sentinel: %v", err)
	}
}

// --- Phase 2 expansion: GetComments tests ---

func TestJiraService_GetComments_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1/comment" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Two comments; bodies are ADF — adfToPlainText should extract "First comment" and "Second comment".
		w.Write([]byte(`{"comments":[` +
			`{"id":"1","author":{"displayName":"Bob"},"body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"First comment"}]}]},"created":"2026-01-01T00:00:00.000+0000","updated":"2026-01-01T00:00:00.000+0000"},` +
			`{"id":"2","author":{"displayName":"Alice"},"body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Second comment"}]}]},"created":"2026-01-02T00:00:00.000+0000","updated":"2026-01-02T00:00:00.000+0000"}` +
			`]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	comments, err := svc.GetComments(context.Background(), "PROJ-1", 10)
	if err != nil {
		t.Fatalf("GetComments() unexpected error: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].ID != "1" {
		t.Errorf("comments[0].ID: expected '1', got %s", comments[0].ID)
	}
	if comments[0].Body != "First comment" {
		t.Errorf("comments[0].Body: expected 'First comment', got %q", comments[0].Body)
	}
	if comments[1].Author != "Alice" {
		t.Errorf("comments[1].Author: expected 'Alice', got %s", comments[1].Author)
	}
}

func TestJiraService_GetComments_Empty(t *testing.T) {
	// Verify empty comment list returns [] not nil (spec requirement).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"comments":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	comments, err := svc.GetComments(context.Background(), "PROJ-1", 10)
	if err != nil {
		t.Fatalf("GetComments() unexpected error: %v", err)
	}
	if comments == nil {
		t.Error("expected empty []Comment slice, got nil")
	}
	if len(comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(comments))
	}
}

func TestJiraService_GetComments_AdfExtraction(t *testing.T) {
	// Verify adfToPlainText correctly extracts nested text nodes.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Nested ADF: doc → paragraph → text
		w.Write([]byte(`{"comments":[{"id":"99","author":{"displayName":"Dev"},"body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Hello "},{"type":"text","text":"World"}]}]},"created":"2026-01-01T00:00:00.000+0000","updated":"2026-01-01T00:00:00.000+0000"}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	comments, err := svc.GetComments(context.Background(), "PROJ-1", 10)
	if err != nil {
		t.Fatalf("GetComments() unexpected error: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Body != "Hello World" {
		t.Errorf("adfToPlainText: expected 'Hello World', got %q", comments[0].Body)
	}
}

func TestJiraService_GetComments_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetComments(context.Background(), "PROJ-999", 10)
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestJiraService_GetComments_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetComments(context.Background(), "PROJ-1", 10)
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestJiraService_GetComments_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetComments(context.Background(), "PROJ-1", 10)
	if !errors.Is(err, jira.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- Phase 2 expansion: LinkIssues tests ---

func TestJiraService_LinkIssues_Success(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issueLink" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.LinkIssues(context.Background(), "PROJ-1", "PROJ-2", "Blocks")
	if err != nil {
		t.Fatalf("LinkIssues() unexpected error: %v", err)
	}

	// Verify request body structure.
	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	linkType, _ := body["type"].(map[string]interface{})
	if linkType["name"] != "Blocks" {
		t.Errorf("type.name: expected 'Blocks', got %v", linkType["name"])
	}
	inward, _ := body["inwardIssue"].(map[string]interface{})
	if inward["key"] != "PROJ-1" {
		t.Errorf("inwardIssue.key: expected 'PROJ-1', got %v", inward["key"])
	}
	outward, _ := body["outwardIssue"].(map[string]interface{})
	if outward["key"] != "PROJ-2" {
		t.Errorf("outwardIssue.key: expected 'PROJ-2', got %v", outward["key"])
	}
}

func TestJiraService_LinkIssues_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.LinkIssues(context.Background(), "PROJ-999", "PROJ-2", "Blocks")
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestJiraService_LinkIssues_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.LinkIssues(context.Background(), "PROJ-1", "PROJ-2", "Blocks")
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestJiraService_LinkIssues_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.LinkIssues(context.Background(), "PROJ-1", "PROJ-2", "Blocks")
	if !errors.Is(err, jira.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

func TestJiraService_LinkIssues_BadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Invalid link type"]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	err := svc.LinkIssues(context.Background(), "PROJ-1", "PROJ-2", "NonExistent")
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if errors.Is(err, jira.ErrNotFound) || errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("400 should produce descriptive error, not sentinel: %v", err)
	}
}

// --- Phase 2 expansion: GetIssueLinkTypes tests ---

func TestJiraService_GetIssueLinkTypes_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issueLinkType" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"issueLinkTypes":[{"id":"10000","name":"Blocks","inward":"is blocked by","outward":"blocks"},{"id":"10001","name":"Cloners","inward":"is cloned by","outward":"clones"}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	types, err := svc.GetIssueLinkTypes(context.Background())
	if err != nil {
		t.Fatalf("GetIssueLinkTypes() unexpected error: %v", err)
	}
	if len(types) != 2 {
		t.Fatalf("expected 2 link types, got %d", len(types))
	}
	if types[0].Name != "Blocks" {
		t.Errorf("types[0].Name: expected 'Blocks', got %s", types[0].Name)
	}
	if types[0].Inward != "is blocked by" {
		t.Errorf("types[0].Inward: expected 'is blocked by', got %s", types[0].Inward)
	}
	if types[0].Outward != "blocks" {
		t.Errorf("types[0].Outward: expected 'blocks', got %s", types[0].Outward)
	}
}

func TestJiraService_GetIssueLinkTypes_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetIssueLinkTypes(context.Background())
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestJiraService_GetIssueLinkTypes_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetIssueLinkTypes(context.Background())
	if !errors.Is(err, jira.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- Phase 2 expansion: AddWorklog tests ---

func TestJiraService_AddWorklog_Success(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1/worklog" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"wl1","timeSpentSeconds":7200,"started":"2026-08-16T10:00:00.000+0000","author":{"displayName":"Dev"}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	wl, err := svc.AddWorklog(context.Background(), "PROJ-1", jira.AddWorklogRequest{TimeSpent: "2h"})
	if err != nil {
		t.Fatalf("AddWorklog() unexpected error: %v", err)
	}
	if wl.ID != "wl1" {
		t.Errorf("ID: expected 'wl1', got %s", wl.ID)
	}
	if wl.TimeSpentSeconds != 7200 {
		t.Errorf("TimeSpentSeconds: expected 7200, got %d", wl.TimeSpentSeconds)
	}
	if wl.Author != "Dev" {
		t.Errorf("Author: expected 'Dev', got %s", wl.Author)
	}

	// timeSpent should be sent as-is in the request body.
	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["timeSpent"] != "2h" {
		t.Errorf("timeSpent: expected '2h', got %v", body["timeSpent"])
	}
	// No comment or started should appear when not set.
	if _, ok := body["comment"]; ok {
		t.Error("comment should not be in body when not set")
	}
	if _, ok := body["started"]; ok {
		t.Error("started should not be in body when not set")
	}
}

func TestJiraService_AddWorklog_WithCommentAndStarted(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"wl2","timeSpentSeconds":5400,"started":"2026-08-16T10:00:00.000+0000","author":{"displayName":"Dev"}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.AddWorklog(context.Background(), "PROJ-1", jira.AddWorklogRequest{
		TimeSpent: "1h 30m",
		Comment:   "Sprint review",
		Started:   "2026-08-16T10:00:00.000+0000",
	})
	if err != nil {
		t.Fatalf("AddWorklog() unexpected error: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	// Verify comment is ADF.
	commentField, ok := body["comment"].(map[string]interface{})
	if !ok {
		t.Fatalf("comment should be ADF object, got %T", body["comment"])
	}
	if commentField["type"] != "doc" {
		t.Errorf("comment ADF type: expected 'doc', got %v", commentField["type"])
	}

	// Verify started is forwarded as-is.
	if body["started"] != "2026-08-16T10:00:00.000+0000" {
		t.Errorf("started: expected pass-through, got %v", body["started"])
	}
}

func TestJiraService_AddWorklog_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.AddWorklog(context.Background(), "PROJ-999", jira.AddWorklogRequest{TimeSpent: "1h"})
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestJiraService_AddWorklog_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.AddWorklog(context.Background(), "PROJ-1", jira.AddWorklogRequest{TimeSpent: "1h"})
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestJiraService_AddWorklog_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.AddWorklog(context.Background(), "PROJ-1", jira.AddWorklogRequest{TimeSpent: "1h"})
	if !errors.Is(err, jira.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

func TestJiraService_AddWorklog_BadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Invalid timeSpent"]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.AddWorklog(context.Background(), "PROJ-1", jira.AddWorklogRequest{TimeSpent: "bad"})
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if errors.Is(err, jira.ErrNotFound) || errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("400 should produce descriptive error, not sentinel: %v", err)
	}
}

// --- Phase 2 expansion: GetIssueTypeMetadata tests ---

func TestJiraService_GetIssueTypeMetadata_SuccessCloud(t *testing.T) {
	// Cloud shape: "values" key.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/createmeta/PROJ/issuetypes" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"id":"1","name":"Story","description":"A user story","subtask":false},{"id":"2","name":"Sub-task","description":"","subtask":true}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	types, err := svc.GetIssueTypeMetadata(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("GetIssueTypeMetadata() unexpected error: %v", err)
	}
	if len(types) != 2 {
		t.Fatalf("expected 2 issue types, got %d", len(types))
	}
	if types[0].Name != "Story" {
		t.Errorf("types[0].Name: expected 'Story', got %s", types[0].Name)
	}
	if types[1].Subtask != true {
		t.Errorf("types[1].Subtask: expected true, got %v", types[1].Subtask)
	}
}

func TestJiraService_GetIssueTypeMetadata_SuccessServerDC(t *testing.T) {
	// Server/DC shape: "issueTypes" key (fallback).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"issueTypes":[{"id":"10","name":"Bug","description":"A bug","subtask":false}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	types, err := svc.GetIssueTypeMetadata(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("GetIssueTypeMetadata() unexpected error: %v", err)
	}
	if len(types) != 1 {
		t.Fatalf("expected 1 issue type, got %d", len(types))
	}
	if types[0].Name != "Bug" {
		t.Errorf("types[0].Name: expected 'Bug', got %s", types[0].Name)
	}
}

func TestJiraService_GetIssueTypeMetadata_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetIssueTypeMetadata(context.Background(), "NOTEXIST")
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestJiraService_GetIssueTypeMetadata_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetIssueTypeMetadata(context.Background(), "PROJ")
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestJiraService_GetIssueTypeMetadata_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := jira.NewService(srv.Client(), srv.URL)
	_, err := svc.GetIssueTypeMetadata(context.Background(), "PROJ")
	if !errors.Is(err, jira.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}
