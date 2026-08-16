package mcpserver_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

// captureLogger records all audit entries for test assertions.
type captureLogger struct {
	entries []audit.Entry
}

func (c *captureLogger) Log(e audit.Entry) {
	c.entries = append(c.entries, e)
}

// mockJiraService implements jira.Service for testing.
type mockJiraService struct {
	getIssueFunc        func(ctx context.Context, key string) (*jira.Issue, error)
	searchIssuesFunc    func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error)
	createIssueFunc     func(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error)
	updateIssueFunc     func(ctx context.Context, key string, req jira.UpdateIssueRequest) error
	getTransitionsFunc  func(ctx context.Context, key string) ([]jira.Transition, error)
	transitionIssueFunc func(ctx context.Context, key string, transitionID string) error
}

func (m *mockJiraService) GetIssue(ctx context.Context, key string) (*jira.Issue, error) {
	return m.getIssueFunc(ctx, key)
}

func (m *mockJiraService) SearchIssues(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
	return m.searchIssuesFunc(ctx, jql, maxResults)
}

func (m *mockJiraService) CreateIssue(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error) {
	if m.createIssueFunc != nil {
		return m.createIssueFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockJiraService) UpdateIssue(ctx context.Context, key string, req jira.UpdateIssueRequest) error {
	if m.updateIssueFunc != nil {
		return m.updateIssueFunc(ctx, key, req)
	}
	return nil
}

func (m *mockJiraService) GetTransitions(ctx context.Context, key string) ([]jira.Transition, error) {
	if m.getTransitionsFunc != nil {
		return m.getTransitionsFunc(ctx, key)
	}
	return nil, nil
}

func (m *mockJiraService) TransitionIssue(ctx context.Context, key string, transitionID string) error {
	if m.transitionIssueFunc != nil {
		return m.transitionIssueFunc(ctx, key, transitionID)
	}
	return nil
}

// makeCallToolRequest builds a mcp.CallToolRequest with the given arguments map.
// Passes arguments as map[string]any so GetArguments() type-asserts correctly.
func makeCallToolRequest(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

// getResultText extracts text from the first content item of a CallToolResult.
func getResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("CallToolResult is nil")
	}
	if len(result.Content) == 0 {
		t.Fatal("CallToolResult has no content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("Content[0] is not TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

// --- TestToolGetJiraIssue ---

func TestToolGetJiraIssue(t *testing.T) {
	fixedTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, key string) (*jira.Issue, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "valid issue returned as JSON",
			args: map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string) (*jira.Issue, error) {
				return &jira.Issue{
					Key:      "PROJ-1",
					Summary:  "Fix login",
					Status:   "In Progress",
					Assignee: "John",
					Priority: "High",
					Labels:   []string{"backend"},
					Created:  fixedTime,
					Updated:  fixedTime,
				}, nil
			},
			wantIsError: false,
			wantContain: `"key":"PROJ-1"`,
		},
		{
			name: "valid issue JSON contains summary",
			args: map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string) (*jira.Issue, error) {
				return &jira.Issue{
					Key:     "PROJ-1",
					Summary: "Fix login",
					Labels:  []string{},
				}, nil
			},
			wantIsError: false,
			wantContain: `"summary":"Fix login"`,
		},
		{
			name:        "missing issue_key argument",
			args:        map[string]any{},
			mockFn:      nil, // should never be called
			wantIsError: true,
			wantContain: "issue_key",
		},
		{
			name: "ErrNotFound maps to error result",
			args: map[string]any{"issue_key": "PROJ-999"},
			mockFn: func(ctx context.Context, key string) (*jira.Issue, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name: "ErrUnauthorized maps to error result",
			args: map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string) (*jira.Issue, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name: "ErrRateLimit maps to error result",
			args: map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string) (*jira.Issue, error) {
				return nil, jira.ErrRateLimit
			},
			wantIsError: true,
			wantContain: "rate limit",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockJiraService{
				getIssueFunc: tc.mockFn,
			}
			handler := mcpserver.ToolGetJiraIssue(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolSearchJiraIssues ---

func TestToolSearchJiraIssues(t *testing.T) {
	tests := []struct {
		name          string
		args          map[string]any
		mockFn        func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error)
		wantIsError   bool
		wantContain   string
		wantMaxResult int // 0 means not checked; non-zero verifies the value passed to mock
	}{
		{
			name: "valid search returns results with pagination metadata",
			args: map[string]any{"jql": "project=PROJ", "max_results": float64(10)},
			mockFn: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
				return &jira.SearchResult{
					Issues:     []jira.Issue{{Key: "PROJ-1", Summary: "Issue 1", Labels: []string{}}, {Key: "PROJ-2", Summary: "Issue 2", Labels: []string{}}},
					Total:      2,
					StartAt:    0,
					MaxResults: 10,
				}, nil
			},
			wantIsError: false,
			wantContain: `"total":2`,
		},
		{
			name: "valid search contains issues array",
			args: map[string]any{"jql": "project=PROJ", "max_results": float64(10)},
			mockFn: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
				return &jira.SearchResult{
					Issues:     []jira.Issue{{Key: "PROJ-1", Labels: []string{}}},
					Total:      1,
					StartAt:    0,
					MaxResults: 10,
				}, nil
			},
			wantIsError: false,
			wantContain: "PROJ-1",
		},
		{
			name:        "missing jql argument",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "jql",
		},
		{
			name: "max_results defaults to 50 when not provided",
			args: map[string]any{"jql": "project=PROJ"},
			mockFn: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
				if maxResults != 50 {
					return nil, nil // will cause test to note wrong value
				}
				return &jira.SearchResult{Issues: []jira.Issue{}, Total: 0, MaxResults: 50}, nil
			},
			wantIsError:   false,
			wantContain:   `"total":0`,
			wantMaxResult: 50,
		},
		{
			name: "ErrInvalidJQL maps to error result",
			args: map[string]any{"jql": "INVALID ???"},
			mockFn: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
				return nil, jira.ErrInvalidJQL
			},
			wantIsError: true,
			wantContain: "invalid JQL",
		},
		{
			name: "empty result set returns empty array and total 0",
			args: map[string]any{"jql": "project=EMPTY"},
			mockFn: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
				return &jira.SearchResult{Issues: []jira.Issue{}, Total: 0, MaxResults: 50}, nil
			},
			wantIsError: false,
			wantContain: `"total":0`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotMaxResults int
			svc := &mockJiraService{
				searchIssuesFunc: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
					gotMaxResults = maxResults
					if tc.mockFn == nil {
						t.Error("mockFn was called but should not have been")
						return nil, nil
					}
					return tc.mockFn(ctx, jql, maxResults)
				},
			}
			handler := mcpserver.ToolSearchJiraIssues(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
			if tc.wantMaxResult != 0 && gotMaxResults != tc.wantMaxResult {
				t.Errorf("maxResults passed to mock: got %d, want %d", gotMaxResults, tc.wantMaxResult)
			}
		})
	}
}

// --- Phase 3: Write tool tests ---

// enableWrite sets ENABLE_WRITE=true for the duration of the test.
func enableWrite(t *testing.T) {
	t.Helper()
	t.Setenv("ENABLE_WRITE", "true")
}

// disableWrite ensures ENABLE_WRITE is unset for the duration of the test.
func disableWrite(t *testing.T) {
	t.Helper()
	os.Unsetenv("ENABLE_WRITE") //nolint:errcheck
}

// --- TestToolCreateJiraIssue ---

func TestToolCreateJiraIssue(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:  "success with required fields only",
			setup: enableWrite,
			args:  map[string]any{"project_key": "PROJ", "issue_type": "Task", "summary": "New task"},
			mockFn: func(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error) {
				return &jira.CreateIssueResponse{Key: "PROJ-42", ID: "10042"}, nil
			},
			wantIsError: false,
			wantContain: `"key":"PROJ-42"`,
		},
		{
			name:  "success returns id in JSON",
			setup: enableWrite,
			args:  map[string]any{"project_key": "PROJ", "issue_type": "Bug", "summary": "Fix bug"},
			mockFn: func(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error) {
				return &jira.CreateIssueResponse{Key: "PROJ-1", ID: "10001"}, nil
			},
			wantIsError: false,
			wantContain: `"id":"10001"`,
		},
		{
			name:        "write guard blocks when ENABLE_WRITE unset",
			setup:       disableWrite,
			args:        map[string]any{"project_key": "PROJ", "issue_type": "Task", "summary": "Test"},
			mockFn:      nil, // must never be called
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing project_key returns error",
			setup:       enableWrite,
			args:        map[string]any{"issue_type": "Task", "summary": "Test"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "project_key",
		},
		{
			name:        "missing summary returns error",
			setup:       enableWrite,
			args:        map[string]any{"project_key": "PROJ", "issue_type": "Task"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "summary",
		},
		{
			name:  "service error ErrUnauthorized forwarded",
			setup: enableWrite,
			args:  map[string]any{"project_key": "PROJ", "issue_type": "Task", "summary": "Test"},
			mockFn: func(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name:  "service error ErrConflict forwarded",
			setup: enableWrite,
			args:  map[string]any{"project_key": "PROJ", "issue_type": "Task", "summary": "Test"},
			mockFn: func(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreateIssueResponse, error) {
				return nil, jira.ErrConflict
			},
			wantIsError: true,
			wantContain: "conflict",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			svc := &mockJiraService{
				createIssueFunc: tc.mockFn,
			}
			handler := mcpserver.ToolCreateJiraIssue(svc, audit.NewNoopLogger())
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolUpdateJiraIssue ---

func TestToolUpdateJiraIssue(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, key string, req jira.UpdateIssueRequest) error
		wantIsError bool
		wantContain string
	}{
		{
			name:  "success returns updated message",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-1", "summary": "Updated"},
			mockFn: func(ctx context.Context, key string, req jira.UpdateIssueRequest) error {
				return nil
			},
			wantIsError: false,
			wantContain: "issue updated successfully",
		},
		{
			name:        "write guard blocks when ENABLE_WRITE unset",
			setup:       disableWrite,
			args:        map[string]any{"issue_key": "PROJ-1"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing issue_key returns error",
			setup:       enableWrite,
			args:        map[string]any{"summary": "Updated"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "issue_key",
		},
		{
			name:  "ErrNotFound forwarded",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-999", "summary": "X"},
			mockFn: func(ctx context.Context, key string, req jira.UpdateIssueRequest) error {
				return jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name:  "ErrUnauthorized forwarded",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-1", "summary": "X"},
			mockFn: func(ctx context.Context, key string, req jira.UpdateIssueRequest) error {
				return jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			svc := &mockJiraService{
				updateIssueFunc: tc.mockFn,
			}
			handler := mcpserver.ToolUpdateJiraIssue(svc, audit.NewNoopLogger())
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolGetJiraTransitions ---

func TestToolGetJiraTransitions(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, key string) ([]jira.Transition, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "success returns JSON array with transitions",
			args: map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string) ([]jira.Transition, error) {
				return []jira.Transition{
					{ID: "11", Name: "In Progress", StatusCategory: "indeterminate"},
					{ID: "21", Name: "Done", StatusCategory: "done"},
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"11"`,
		},
		{
			name: "success returns status_category in snake_case",
			args: map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string) ([]jira.Transition, error) {
				return []jira.Transition{
					{ID: "21", Name: "Done", StatusCategory: "done"},
				}, nil
			},
			wantIsError: false,
			wantContain: `"status_category"`,
		},
		{
			name: "empty transitions returns [] not null",
			args: map[string]any{"issue_key": "PROJ-2"},
			mockFn: func(ctx context.Context, key string) ([]jira.Transition, error) {
				return []jira.Transition{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
		{
			name: "works without ENABLE_WRITE (read-only operation)",
			setup: disableWrite,
			args: map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string) ([]jira.Transition, error) {
				return []jira.Transition{
					{ID: "11", Name: "In Progress", StatusCategory: "indeterminate"},
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"11"`,
		},
		{
			name:        "missing issue_key returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "issue_key",
		},
		{
			name: "ErrNotFound forwarded",
			args: map[string]any{"issue_key": "PROJ-999"},
			mockFn: func(ctx context.Context, key string) ([]jira.Transition, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			svc := &mockJiraService{
				getTransitionsFunc: tc.mockFn,
			}
			handler := mcpserver.ToolGetJiraTransitions(svc)
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}

// --- TestToolTransitionJiraIssue ---

func TestToolTransitionJiraIssue(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, key string, transitionID string) error
		wantIsError bool
		wantContain string
	}{
		{
			name:  "success returns transitioned message",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-1", "transition_id": "21"},
			mockFn: func(ctx context.Context, key string, transitionID string) error {
				return nil
			},
			wantIsError: false,
			wantContain: "issue transitioned successfully",
		},
		{
			name:        "write guard blocks when ENABLE_WRITE unset",
			setup:       disableWrite,
			args:        map[string]any{"issue_key": "PROJ-1", "transition_id": "21"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing issue_key returns error",
			setup:       enableWrite,
			args:        map[string]any{"transition_id": "21"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "issue_key",
		},
		{
			name:        "missing transition_id returns error",
			setup:       enableWrite,
			args:        map[string]any{"issue_key": "PROJ-1"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "transition_id",
		},
		{
			name:  "ErrNotFound forwarded",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-999", "transition_id": "21"},
			mockFn: func(ctx context.Context, key string, transitionID string) error {
				return jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name:  "descriptive error forwarded",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-1", "transition_id": "invalid"},
			mockFn: func(ctx context.Context, key string, transitionID string) error {
				return jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			svc := &mockJiraService{
				transitionIssueFunc: tc.mockFn,
			}
			handler := mcpserver.ToolTransitionJiraIssue(svc, audit.NewNoopLogger())
			req := makeCallToolRequest(tc.args)
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned Go error (must never happen): %v", err)
			}
			if result.IsError != tc.wantIsError {
				t.Errorf("IsError: got %v, want %v (text: %s)", result.IsError, tc.wantIsError, getResultText(t, result))
			}
			text := getResultText(t, result)
			if !strings.Contains(strings.ToLower(text), strings.ToLower(tc.wantContain)) {
				t.Errorf("result text %q does not contain %q", text, tc.wantContain)
			}
		})
	}
}
