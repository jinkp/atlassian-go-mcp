package mcpserver_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

// mockAgileService implements agile.AgileService for testing.
// Each method delegates to a stored func field; guards against nil with safe defaults.
type mockAgileService struct {
	getBoardsFunc          func(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error)
	getSprintsFunc         func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error)
	getSprintIssuesFunc    func(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error)
	updateSprintFunc       func(ctx context.Context, sprintID int, req agile.UpdateSprintRequest) (*agile.Sprint, error)
	moveIssuesToSprintFunc func(ctx context.Context, sprintID int, issueKeys []string) error
	moveIssuesToEpicFunc   func(ctx context.Context, epicKey string, issueKeys []string) error
	createSprintFunc       func(ctx context.Context, req agile.CreateSprintRequest) (*agile.Sprint, error)
}

func (m *mockAgileService) GetBoards(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error) {
	if m.getBoardsFunc != nil {
		return m.getBoardsFunc(ctx, projectKey, maxResults)
	}
	return []agile.Board{}, nil
}

func (m *mockAgileService) GetSprints(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
	if m.getSprintsFunc != nil {
		return m.getSprintsFunc(ctx, boardID, state, maxResults)
	}
	return []agile.Sprint{}, nil
}

func (m *mockAgileService) GetSprintIssues(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error) {
	if m.getSprintIssuesFunc != nil {
		return m.getSprintIssuesFunc(ctx, sprintID, maxResults)
	}
	return &agile.SprintIssueResult{Issues: []agile.SprintIssue{}, Total: 0}, nil
}

func (m *mockAgileService) UpdateSprint(ctx context.Context, sprintID int, req agile.UpdateSprintRequest) (*agile.Sprint, error) {
	if m.updateSprintFunc != nil {
		return m.updateSprintFunc(ctx, sprintID, req)
	}
	return &agile.Sprint{}, nil
}

func (m *mockAgileService) MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error {
	if m.moveIssuesToSprintFunc != nil {
		return m.moveIssuesToSprintFunc(ctx, sprintID, issueKeys)
	}
	return nil
}

func (m *mockAgileService) MoveIssuesToEpic(ctx context.Context, epicKey string, issueKeys []string) error {
	if m.moveIssuesToEpicFunc != nil {
		return m.moveIssuesToEpicFunc(ctx, epicKey, issueKeys)
	}
	return nil
}

func (m *mockAgileService) CreateSprint(ctx context.Context, req agile.CreateSprintRequest) (*agile.Sprint, error) {
	if m.createSprintFunc != nil {
		return m.createSprintFunc(ctx, req)
	}
	return &agile.Sprint{}, nil
}

// --- TestToolGetJiraBoards ---

func TestToolGetJiraBoards(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns boards JSON with id and type",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error) {
				return []agile.Board{
					{ID: 1, Name: "PROJ Scrum Board", Type: "scrum"},
					{ID: 2, Name: "PROJ Kanban Board", Type: "kanban"},
				}, nil
			},
			wantIsError: false,
			wantContain: `"id"`,
		},
		{
			name: "boards JSON contains type field",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error) {
				return []agile.Board{
					{ID: 1, Name: "PROJ Scrum Board", Type: "scrum"},
				}, nil
			},
			wantIsError: false,
			wantContain: `"type"`,
		},
		{
			name:        "missing project_key returns IsError true with project_key in text",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "project_key",
		},
		{
			name: "service ErrUnauthorized returns IsError true",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name: "max_results defaults to 50 when not provided",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error) {
				if maxResults != 50 {
					t.Errorf("expected maxResults=50, got %d", maxResults)
				}
				return []agile.Board{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockAgileService{getBoardsFunc: tc.mockFn}
			handler := mcpserver.ToolGetJiraBoards(svc)
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

// --- TestToolGetJiraSprints ---

func TestToolGetJiraSprints(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns sprints JSON with state and board_id fields",
			args: map[string]any{"board_id": float64(10)},
			mockFn: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
				return []agile.Sprint{
					{ID: 1, Name: "Sprint 1", State: "active", BoardID: 10},
					{ID: 2, Name: "Sprint 2", State: "future", BoardID: 10},
				}, nil
			},
			wantIsError: false,
			wantContain: `"state"`,
		},
		{
			name: "returns sprints JSON with board_id field",
			args: map[string]any{"board_id": float64(10)},
			mockFn: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
				return []agile.Sprint{
					{ID: 1, Name: "Sprint 1", State: "active", BoardID: 10},
				}, nil
			},
			wantIsError: false,
			wantContain: `"board_id"`,
		},
		{
			name:        "missing board_id returns IsError true",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "board_id",
		},
		{
			name: "state param passes through to service",
			args: map[string]any{"board_id": float64(10), "state": "active"},
			mockFn: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
				if state != "active" {
					t.Errorf("expected state=active, got %q", state)
				}
				return []agile.Sprint{{ID: 1, Name: "Sprint 1", State: "active", BoardID: boardID}}, nil
			},
			wantIsError: false,
			wantContain: "active",
		},
		{
			name: "service ErrNotFound returns IsError true",
			args: map[string]any{"board_id": float64(999)},
			mockFn: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockAgileService{getSprintsFunc: tc.mockFn}
			handler := mcpserver.ToolGetJiraSprints(svc)
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

// --- TestToolGetJiraActiveSprint ---

func TestToolGetJiraActiveSprint(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "active sprint found returns sprint JSON",
			args: map[string]any{"board_id": float64(10)},
			mockFn: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
				if state != "active" {
					t.Errorf("expected state=active, got %q", state)
				}
				if maxResults != 1 {
					t.Errorf("expected maxResults=1, got %d", maxResults)
				}
				return []agile.Sprint{{ID: 42, Name: "Sprint 5", State: "active", BoardID: 10}}, nil
			},
			wantIsError: false,
			wantContain: `"state":"active"`,
		},
		{
			name: "no active sprint returns IsError true with no active sprint message",
			args: map[string]any{"board_id": float64(20)},
			mockFn: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
				return []agile.Sprint{}, nil
			},
			wantIsError: true,
			wantContain: "no active sprint",
		},
		{
			name:        "missing board_id returns IsError true",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "board_id",
		},
		{
			name: "service ErrNotFound returns IsError true",
			args: map[string]any{"board_id": float64(999)},
			mockFn: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockAgileService{getSprintsFunc: tc.mockFn}
			handler := mcpserver.ToolGetJiraActiveSprint(svc)
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

// --- TestToolGetJiraSprintIssues ---

func TestToolGetJiraSprintIssues(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns sprint issues with total count",
			args: map[string]any{"sprint_id": float64(100)},
			mockFn: func(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error) {
				return &agile.SprintIssueResult{
					Issues: []agile.SprintIssue{
						{Key: "PROJ-1", Summary: "Issue one", Status: "To Do", Assignee: "Alice"},
						{Key: "PROJ-2", Summary: "Issue two", Status: "In Progress", Assignee: ""},
					},
					Total:      2,
					StartAt:    0,
					MaxResults: 50,
				}, nil
			},
			wantIsError: false,
			wantContain: `"total":2`,
		},
		{
			name: "empty sprint returns valid JSON with total 0",
			args: map[string]any{"sprint_id": float64(200)},
			mockFn: func(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error) {
				return &agile.SprintIssueResult{Issues: []agile.SprintIssue{}, Total: 0, MaxResults: 50}, nil
			},
			wantIsError: false,
			wantContain: `"total":0`,
		},
		{
			name:        "missing sprint_id returns IsError true",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "sprint_id",
		},
		{
			name: "service ErrUnauthorized returns IsError true",
			args: map[string]any{"sprint_id": float64(100)},
			mockFn: func(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name: "JSON contains issues array",
			args: map[string]any{"sprint_id": float64(100)},
			mockFn: func(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error) {
				return &agile.SprintIssueResult{
					Issues: []agile.SprintIssue{{Key: "PROJ-1", Summary: "X", Status: "To Do"}},
					Total:  1,
				}, nil
			},
			wantIsError: false,
			wantContain: `"issues"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockAgileService{getSprintIssuesFunc: tc.mockFn}
			handler := mcpserver.ToolGetJiraSprintIssues(svc)
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

// --- TestToolGetJiraEpics ---

func TestToolGetJiraEpics(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockFn      func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error)
		wantIsError bool
		wantContain string
		wantJQL     string
	}{
		{
			name: "returns epic search results with total",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
				return &jira.SearchResult{
					Issues:     []jira.Issue{{Key: "PROJ-10", Summary: "Epic One"}, {Key: "PROJ-20", Summary: "Epic Two"}},
					Total:      2,
					MaxResults: 50,
				}, nil
			},
			wantIsError: false,
			wantContain: `"total":2`,
			wantJQL:     "project=PROJ AND issuetype=Epic ORDER BY created DESC",
		},
		{
			name: "no epics returns empty result with total 0",
			args: map[string]any{"project_key": "EMPTY"},
			mockFn: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
				return &jira.SearchResult{Issues: []jira.Issue{}, Total: 0, MaxResults: 50}, nil
			},
			wantIsError: false,
			wantContain: `"total":0`,
		},
		{
			name:        "missing project_key returns IsError true",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "project_key",
		},
		{
			name: "service ErrUnauthorized returns IsError true",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotJQL string
			svc := &mockJiraService{
				searchIssuesFunc: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
					gotJQL = jql
					if tc.mockFn == nil {
						return nil, nil
					}
					return tc.mockFn(ctx, jql, maxResults)
				},
			}
			handler := mcpserver.ToolGetJiraEpics(svc)
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
			// Verify the JQL contains required parts when a JQL expectation is set
			if tc.wantJQL != "" && !strings.Contains(gotJQL, "issuetype=Epic") {
				t.Errorf("JQL %q does not contain 'issuetype=Epic'", gotJQL)
			}
			if tc.wantJQL != "" && !strings.Contains(gotJQL, "ORDER BY created DESC") {
				t.Errorf("JQL %q does not contain 'ORDER BY created DESC'", gotJQL)
			}
		})
	}
}

// TestToolGetJiraBoards_MaxResultsPassthrough verifies max_results from args passes to service.
func TestToolGetJiraBoards_MaxResultsPassthrough(t *testing.T) {
	var gotMaxResults int
	svc := &mockAgileService{
		getBoardsFunc: func(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error) {
			gotMaxResults = maxResults
			return []agile.Board{}, nil
		},
	}
	handler := mcpserver.ToolGetJiraBoards(svc)
	req := makeCallToolRequest(map[string]any{"project_key": "PROJ", "max_results": float64(25)})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if gotMaxResults != 25 {
		t.Errorf("maxResults passed to service: got %d, want 25", gotMaxResults)
	}
}

// TestToolGetJiraSprints_DefaultState verifies state defaults to "" when not provided.
func TestToolGetJiraSprints_DefaultState(t *testing.T) {
	var gotState string
	svc := &mockAgileService{
		getSprintsFunc: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
			gotState = state
			return []agile.Sprint{}, nil
		},
	}
	handler := mcpserver.ToolGetJiraSprints(svc)
	req := makeCallToolRequest(map[string]any{"board_id": float64(10)})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if gotState != "" {
		t.Errorf("default state: got %q, want empty string", gotState)
	}
}

// TestToolGetJiraActiveSprint_CallsGetSprintsWithActive verifies the internal call parameters.
func TestToolGetJiraActiveSprint_CallsGetSprintsWithActive(t *testing.T) {
	var gotState string
	var gotMaxResults int
	svc := &mockAgileService{
		getSprintsFunc: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
			gotState = state
			gotMaxResults = maxResults
			return []agile.Sprint{{ID: 1, Name: "S1", State: "active", BoardID: boardID}}, nil
		},
	}
	handler := mcpserver.ToolGetJiraActiveSprint(svc)
	req := makeCallToolRequest(map[string]any{"board_id": float64(5)})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if gotState != "active" {
		t.Errorf("state passed to service: got %q, want 'active'", gotState)
	}
	if gotMaxResults != 1 {
		t.Errorf("maxResults passed to service: got %d, want 1", gotMaxResults)
	}
}

// TestToolGetJiraSprintIssues_MaxResultsPassthrough verifies max_results from args passes to service.
func TestToolGetJiraSprintIssues_MaxResultsPassthrough(t *testing.T) {
	var gotMaxResults int
	svc := &mockAgileService{
		getSprintIssuesFunc: func(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error) {
			gotMaxResults = maxResults
			return &agile.SprintIssueResult{Issues: []agile.SprintIssue{}, Total: 0}, nil
		},
	}
	handler := mcpserver.ToolGetJiraSprintIssues(svc)
	req := makeCallToolRequest(map[string]any{"sprint_id": float64(100), "max_results": float64(10)})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if gotMaxResults != 10 {
		t.Errorf("maxResults passed to service: got %d, want 10", gotMaxResults)
	}
}

// TestToolGetJiraEpics_JQLContainsProjectKey verifies the JQL is built with the project key.
func TestToolGetJiraEpics_JQLContainsProjectKey(t *testing.T) {
	var gotJQL string
	svc := &mockJiraService{
		searchIssuesFunc: func(ctx context.Context, jql string, maxResults int) (*jira.SearchResult, error) {
			gotJQL = jql
			return &jira.SearchResult{Issues: []jira.Issue{}, Total: 0}, nil
		},
	}
	handler := mcpserver.ToolGetJiraEpics(svc)
	req := makeCallToolRequest(map[string]any{"project_key": "MYPROJ"})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if !strings.Contains(gotJQL, "MYPROJ") {
		t.Errorf("JQL %q does not contain project key 'MYPROJ'", gotJQL)
	}
	if !strings.Contains(gotJQL, "issuetype=Epic") {
		t.Errorf("JQL %q does not contain 'issuetype=Epic'", gotJQL)
	}
}

// --- TestToolUpdateSprint ---

func TestToolUpdateSprint(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, sprintID int, req agile.UpdateSprintRequest) (*agile.Sprint, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:        "WriteGuard blocked when ENABLE_WRITE unset",
			args:        map[string]any{"sprint_id": float64(42)},
			envWrite:    "",
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing sprint_id returns IsError true",
			args:        map[string]any{},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "sprint_id",
		},
		{
			name:     "success with state=closed returns Sprint JSON",
			args:     map[string]any{"sprint_id": float64(42), "state": "closed"},
			envWrite: "true",
			mockFn: func(ctx context.Context, sprintID int, req agile.UpdateSprintRequest) (*agile.Sprint, error) {
				if sprintID != 42 {
					t.Errorf("sprintID: got %d, want 42", sprintID)
				}
				if req.State == nil || *req.State != "closed" {
					t.Errorf("req.State: got %v, want closed", req.State)
				}
				return &agile.Sprint{ID: 42, Name: "Sprint 5", State: "closed"}, nil
			},
			wantIsError: false,
			wantContain: `"id":42`,
		},
		{
			name:     "success with name update returns Sprint JSON",
			args:     map[string]any{"sprint_id": float64(42), "name": "Sprint 5 Renamed"},
			envWrite: "true",
			mockFn: func(ctx context.Context, sprintID int, req agile.UpdateSprintRequest) (*agile.Sprint, error) {
				if req.Name == nil || *req.Name != "Sprint 5 Renamed" {
					t.Errorf("req.Name: got %v, want Sprint 5 Renamed", req.Name)
				}
				return &agile.Sprint{ID: 42, Name: "Sprint 5 Renamed", State: "active"}, nil
			},
			wantIsError: false,
			wantContain: "Sprint 5 Renamed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}
			svc := &mockAgileService{updateSprintFunc: tc.mockFn}
			handler := mcpserver.ToolUpdateSprint(svc)
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

// --- TestToolMoveIssuesToSprint ---

func TestToolMoveIssuesToSprint(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, sprintID int, issueKeys []string) error
		wantIsError bool
		wantContain string
	}{
		{
			name:        "WriteGuard blocked when ENABLE_WRITE unset",
			args:        map[string]any{"sprint_id": float64(42), "issue_keys": "PROJ-1"},
			envWrite:    "",
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing sprint_id returns IsError true",
			args:        map[string]any{"issue_keys": "PROJ-1"},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "sprint_id",
		},
		{
			name:        "missing issue_keys returns IsError true",
			args:        map[string]any{"sprint_id": float64(42)},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "issue_keys",
		},
		{
			name:     "success with CSV issue_keys calls service and returns ok",
			args:     map[string]any{"sprint_id": float64(42), "issue_keys": "PROJ-1, PROJ-2"},
			envWrite: "true",
			mockFn: func(ctx context.Context, sprintID int, issueKeys []string) error {
				if sprintID != 42 {
					t.Errorf("sprintID: got %d, want 42", sprintID)
				}
				if len(issueKeys) != 2 || issueKeys[0] != "PROJ-1" || issueKeys[1] != "PROJ-2" {
					t.Errorf("issueKeys: got %v, want [PROJ-1 PROJ-2]", issueKeys)
				}
				return nil
			},
			wantIsError: false,
			wantContain: "ok",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}
			svc := &mockAgileService{moveIssuesToSprintFunc: tc.mockFn}
			handler := mcpserver.ToolMoveIssuesToSprint(svc)
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

// --- TestToolMoveIssuesToEpic ---

func TestToolMoveIssuesToEpic(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, epicKey string, issueKeys []string) error
		wantIsError bool
		wantContain string
	}{
		{
			name:        "WriteGuard blocked when ENABLE_WRITE unset",
			args:        map[string]any{"epic_key": "PROJ-100", "issue_keys": "PROJ-1"},
			envWrite:    "",
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing epic_key returns IsError true",
			args:        map[string]any{"issue_keys": "PROJ-1"},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "epic_key",
		},
		{
			name:        "missing issue_keys returns IsError true",
			args:        map[string]any{"epic_key": "PROJ-100"},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "issue_keys",
		},
		{
			name:     "success calls service with correct args and returns ok",
			args:     map[string]any{"epic_key": "PROJ-100", "issue_keys": "PROJ-1, PROJ-2"},
			envWrite: "true",
			mockFn: func(ctx context.Context, epicKey string, issueKeys []string) error {
				if epicKey != "PROJ-100" {
					t.Errorf("epicKey: got %q, want PROJ-100", epicKey)
				}
				if len(issueKeys) != 2 || issueKeys[0] != "PROJ-1" || issueKeys[1] != "PROJ-2" {
					t.Errorf("issueKeys: got %v, want [PROJ-1 PROJ-2]", issueKeys)
				}
				return nil
			},
			wantIsError: false,
			wantContain: "ok",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}
			svc := &mockAgileService{moveIssuesToEpicFunc: tc.mockFn}
			handler := mcpserver.ToolMoveIssuesToEpic(svc)
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

// --- TestToolCreateSprint ---

func TestToolCreateSprint(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		envWrite    string
		mockFn      func(ctx context.Context, req agile.CreateSprintRequest) (*agile.Sprint, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:        "WriteGuard blocked when ENABLE_WRITE unset",
			args:        map[string]any{"name": "Sprint 8", "board_id": float64(10)},
			envWrite:    "",
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing name returns IsError true",
			args:        map[string]any{"board_id": float64(10)},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "name",
		},
		{
			name:        "missing board_id returns IsError true",
			args:        map[string]any{"name": "Sprint 8"},
			envWrite:    "true",
			wantIsError: true,
			wantContain: "board_id",
		},
		{
			name:     "success name+boardID only — IsError false, result contains id",
			args:     map[string]any{"name": "Sprint 8", "board_id": float64(10)},
			envWrite: "true",
			mockFn: func(ctx context.Context, req agile.CreateSprintRequest) (*agile.Sprint, error) {
				if req.Name != "Sprint 8" {
					t.Errorf("req.Name: got %q, want Sprint 8", req.Name)
				}
				if req.BoardID != 10 {
					t.Errorf("req.BoardID: got %d, want 10", req.BoardID)
				}
				if req.StartDate != "" {
					t.Errorf("req.StartDate: got %q, want empty", req.StartDate)
				}
				if req.EndDate != "" {
					t.Errorf("req.EndDate: got %q, want empty", req.EndDate)
				}
				return &agile.Sprint{ID: 55, Name: "Sprint 8", State: "future"}, nil
			},
			wantIsError: false,
			wantContain: `"id":55`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envWrite == "" {
				t.Setenv("ENABLE_WRITE", "")
			} else {
				t.Setenv("ENABLE_WRITE", tc.envWrite)
			}
			svc := &mockAgileService{createSprintFunc: tc.mockFn}
			handler := mcpserver.ToolCreateSprint(svc)
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

// TestToolCreateSprint_WithDates verifies start_date and end_date args pass through to service req.
func TestToolCreateSprint_WithDates(t *testing.T) {
	t.Setenv("ENABLE_WRITE", "true")
	var gotReq agile.CreateSprintRequest
	svc := &mockAgileService{
		createSprintFunc: func(ctx context.Context, req agile.CreateSprintRequest) (*agile.Sprint, error) {
			gotReq = req
			return &agile.Sprint{ID: 55, Name: "Sprint 8", State: "future"}, nil
		},
	}
	handler := mcpserver.ToolCreateSprint(svc)
	req := makeCallToolRequest(map[string]any{
		"name":       "Sprint 8",
		"board_id":   float64(10),
		"start_date": "2024-01-15T00:00:00.000Z",
		"end_date":   "2024-01-29T00:00:00.000Z",
	})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if gotReq.StartDate != "2024-01-15T00:00:00.000Z" {
		t.Errorf("StartDate: got %q, want 2024-01-15T00:00:00.000Z", gotReq.StartDate)
	}
	if gotReq.EndDate != "2024-01-29T00:00:00.000Z" {
		t.Errorf("EndDate: got %q, want 2024-01-29T00:00:00.000Z", gotReq.EndDate)
	}
}

// TestToolCreateSprint_ServiceErrorPropagates verifies service errors surface as tool errors.
func TestToolCreateSprint_ServiceErrorPropagates(t *testing.T) {
	t.Setenv("ENABLE_WRITE", "true")
	svc := &mockAgileService{
		createSprintFunc: func(ctx context.Context, req agile.CreateSprintRequest) (*agile.Sprint, error) {
			return nil, fmt.Errorf("Board does not support sprints")
		},
	}
	handler := mcpserver.ToolCreateSprint(svc)
	req := makeCallToolRequest(map[string]any{"name": "Sprint 8", "board_id": float64(20)})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true on service error")
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "Board does not support sprints") {
		t.Errorf("result text %q does not contain expected error message", text)
	}
}

// Ensure the mcp package's ParseInt helper is being used — boards tool with explicit max_results as integer
func TestToolGetJiraSprints_ExplicitMaxResults(t *testing.T) {
	var gotMaxResults int
	svc := &mockAgileService{
		getSprintsFunc: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
			gotMaxResults = maxResults
			return []agile.Sprint{}, nil
		},
	}
	handler := mcpserver.ToolGetJiraSprints(svc)
	req := makeCallToolRequest(map[string]any{"board_id": float64(10), "max_results": float64(20)})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected tool error: %s", getResultText(t, result))
	}
	if gotMaxResults != 20 {
		t.Errorf("maxResults: got %d, want 20", gotMaxResults)
	}
}
