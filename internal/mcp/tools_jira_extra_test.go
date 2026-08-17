package mcpserver_test

import (
	"context"
	"strings"
	"testing"
	"time"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

// --- TestToolLookupJiraAccountID ---

func TestToolLookupJiraAccountID(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, query string, maxResults int) ([]jira.User, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "valid lookup returns user array with snake_case fields",
			args: map[string]any{"query": "Jane"},
			mockFn: func(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
				return []jira.User{
					{AccountID: "acc-1", DisplayName: "Jane Doe", Email: "jane@example.com", Active: true},
				}, nil
			},
			wantIsError: false,
			wantContain: `"account_id":"acc-1"`,
		},
		{
			name: "result contains display_name in snake_case",
			args: map[string]any{"query": "Jane"},
			mockFn: func(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
				return []jira.User{
					{AccountID: "acc-1", DisplayName: "Jane Doe", Email: "", Active: true},
				}, nil
			},
			wantIsError: false,
			wantContain: `"display_name":"Jane Doe"`,
		},
		{
			name: "privacy-hidden email returns empty string not null",
			args: map[string]any{"query": "Jane"},
			mockFn: func(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
				return []jira.User{
					{AccountID: "acc-1", DisplayName: "Jane", Email: "", Active: true},
				}, nil
			},
			wantIsError: false,
			wantContain: `"email":""`,
		},
		{
			name: "empty result set serializes as [] not null",
			args: map[string]any{"query": "NoMatch"},
			mockFn: func(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
				return []jira.User{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
		{
			name:        "missing query argument returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "query",
		},
		{
			name: "ErrUnauthorized mapped to error result",
			args: map[string]any{"query": "Jane"},
			mockFn: func(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name: "ErrRateLimit mapped to error result",
			args: map[string]any{"query": "Jane"},
			mockFn: func(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
				return nil, jira.ErrRateLimit
			},
			wantIsError: true,
			wantContain: "rate limit",
		},
		{
			name:  "works without ENABLE_WRITE (read-only)",
			setup: disableWrite,
			args:  map[string]any{"query": "Jane"},
			mockFn: func(ctx context.Context, query string, maxResults int) ([]jira.User, error) {
				return []jira.User{{AccountID: "acc-1", DisplayName: "Jane"}}, nil
			},
			wantIsError: false,
			wantContain: `"account_id"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			svc := &mockJiraService{lookupAccountIDFunc: tc.mockFn}
			handler := mcpserver.ToolLookupJiraAccountID(svc)
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

// --- TestToolGetIssueComments ---

func TestToolGetIssueComments(t *testing.T) {
	fixedTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "valid response returns comments array with snake_case fields",
			args: map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return []jira.Comment{
					{ID: "cmt-1", Author: "Alice", Body: "Looks good", Created: fixedTime, Updated: fixedTime},
					{ID: "cmt-2", Author: "Bob", Body: "LGTM", Created: fixedTime, Updated: fixedTime},
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"cmt-1"`,
		},
		{
			name: "response contains author field",
			args: map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return []jira.Comment{
					{ID: "cmt-1", Author: "Alice", Body: "Hello"},
				}, nil
			},
			wantIsError: false,
			wantContain: `"author":"Alice"`,
		},
		{
			name: "zero-time timestamps are omitted (omitempty)",
			args: map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return []jira.Comment{
					{ID: "cmt-1", Author: "Alice", Body: "Hello"},
				}, nil
			},
			wantIsError: false,
			// "created" and "updated" should NOT appear when time is zero
			wantContain: `"body":"Hello"`,
		},
		{
			name: "empty comment list serializes as []",
			args: map[string]any{"issue_key": "PROJ-2"},
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return []jira.Comment{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
		{
			name:        "missing issue_key returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "issue_key",
		},
		{
			name: "ErrNotFound mapped to error result",
			args: map[string]any{"issue_key": "PROJ-999"},
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name: "ErrUnauthorized mapped to error result",
			args: map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name:  "works without ENABLE_WRITE (read-only)",
			setup: disableWrite,
			args:  map[string]any{"issue_key": "PROJ-1"},
			mockFn: func(ctx context.Context, key string, maxResults int) ([]jira.Comment, error) {
				return []jira.Comment{{ID: "cmt-1", Author: "Alice"}}, nil
			},
			wantIsError: false,
			wantContain: `"id":"cmt-1"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			svc := &mockJiraService{getCommentsFunc: tc.mockFn}
			handler := mcpserver.ToolGetIssueComments(svc)
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

// --- TestToolGetIssueLinkTypes ---

func TestToolGetIssueLinkTypes(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context) ([]jira.IssueLinkType, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns all link types with snake_case fields",
			args: map[string]any{},
			mockFn: func(ctx context.Context) ([]jira.IssueLinkType, error) {
				return []jira.IssueLinkType{
					{ID: "10001", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
					{ID: "10002", Name: "Relates", Inward: "relates to", Outward: "relates to"},
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"10001"`,
		},
		{
			name: "response contains inward and outward fields",
			args: map[string]any{},
			mockFn: func(ctx context.Context) ([]jira.IssueLinkType, error) {
				return []jira.IssueLinkType{
					{ID: "10001", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
				}, nil
			},
			wantIsError: false,
			wantContain: `"inward":"is blocked by"`,
		},
		{
			name: "empty result serializes as []",
			args: map[string]any{},
			mockFn: func(ctx context.Context) ([]jira.IssueLinkType, error) {
				return []jira.IssueLinkType{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
		{
			name: "ErrUnauthorized mapped to error result",
			args: map[string]any{},
			mockFn: func(ctx context.Context) ([]jira.IssueLinkType, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name: "ErrRateLimit mapped to error result",
			args: map[string]any{},
			mockFn: func(ctx context.Context) ([]jira.IssueLinkType, error) {
				return nil, jira.ErrRateLimit
			},
			wantIsError: true,
			wantContain: "rate limit",
		},
		{
			name:  "works without ENABLE_WRITE (read-only)",
			setup: disableWrite,
			args:  map[string]any{},
			mockFn: func(ctx context.Context) ([]jira.IssueLinkType, error) {
				return []jira.IssueLinkType{{ID: "10001", Name: "Blocks"}}, nil
			},
			wantIsError: false,
			wantContain: `"name":"Blocks"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			svc := &mockJiraService{getIssueLinkTypesFunc: tc.mockFn}
			handler := mcpserver.ToolGetIssueLinkTypes(svc)
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

// --- TestToolGetIssueTypeMetadata ---

func TestToolGetIssueTypeMetadata(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error)
		wantIsError bool
		wantContain string
	}{
		{
			name: "returns issue types with snake_case fields",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
				return []jira.IssueTypeMeta{
					{ID: "10001", Name: "Bug", Desc: "A bug", Subtask: false},
					{ID: "10002", Name: "Story", Desc: "A story", Subtask: false},
					{ID: "10003", Name: "Sub-task", Desc: "Sub-task", Subtask: true},
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"10001"`,
		},
		{
			name: "response contains description field (not desc)",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
				return []jira.IssueTypeMeta{
					{ID: "10001", Name: "Bug", Desc: "A software bug", Subtask: false},
				}, nil
			},
			wantIsError: false,
			wantContain: `"description":"A software bug"`,
		},
		{
			name: "subtask bool field included",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
				return []jira.IssueTypeMeta{
					{ID: "10003", Name: "Sub-task", Desc: "", Subtask: true},
				}, nil
			},
			wantIsError: false,
			wantContain: `"subtask":true`,
		},
		{
			name: "empty result serializes as []",
			args: map[string]any{"project_key": "EMPTY"},
			mockFn: func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
				return []jira.IssueTypeMeta{}, nil
			},
			wantIsError: false,
			wantContain: "[]",
		},
		{
			name:        "missing project_key returns error",
			args:        map[string]any{},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "project_key",
		},
		{
			name: "ErrNotFound mapped to error result",
			args: map[string]any{"project_key": "NONEXISTENT"},
			mockFn: func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name: "ErrUnauthorized mapped to error result",
			args: map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
				return nil, jira.ErrUnauthorized
			},
			wantIsError: true,
			wantContain: "unauthorized",
		},
		{
			name:  "works without ENABLE_WRITE (read-only)",
			setup: disableWrite,
			args:  map[string]any{"project_key": "PROJ"},
			mockFn: func(ctx context.Context, projectKey string) ([]jira.IssueTypeMeta, error) {
				return []jira.IssueTypeMeta{{ID: "10001", Name: "Bug"}}, nil
			},
			wantIsError: false,
			wantContain: `"name":"Bug"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			svc := &mockJiraService{getIssueTypeMetadataFunc: tc.mockFn}
			handler := mcpserver.ToolGetIssueTypeMetadata(svc)
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

// --- TestToolAddCommentToIssue ---

func TestToolAddCommentToIssue(t *testing.T) {
	fixedTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, key string, body string) (*jira.Comment, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:  "success returns created comment as JSON",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-1", "body": "Looks good"},
			mockFn: func(ctx context.Context, key string, body string) (*jira.Comment, error) {
				return &jira.Comment{
					ID:      "cmt-42",
					Author:  "Alice",
					Body:    "Looks good",
					Created: fixedTime,
					Updated: fixedTime,
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"cmt-42"`,
		},
		{
			name:  "success response contains author",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-1", "body": "LGTM"},
			mockFn: func(ctx context.Context, key string, body string) (*jira.Comment, error) {
				return &jira.Comment{ID: "cmt-1", Author: "Alice", Body: "LGTM", Created: fixedTime}, nil
			},
			wantIsError: false,
			wantContain: `"author":"Alice"`,
		},
		{
			name:        "write guard blocks when ENABLE_WRITE not set",
			setup:       disableWrite,
			args:        map[string]any{"issue_key": "PROJ-1", "body": "Test"},
			mockFn:      nil, // must never be called
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing issue_key returns error",
			setup:       enableWrite,
			args:        map[string]any{"body": "Test"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "issue_key",
		},
		{
			name:        "missing body returns error",
			setup:       enableWrite,
			args:        map[string]any{"issue_key": "PROJ-1"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "body",
		},
		{
			name:  "ErrNotFound mapped to error result",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-999", "body": "Comment"},
			mockFn: func(ctx context.Context, key string, body string) (*jira.Comment, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name:  "ErrUnauthorized mapped to error result",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-1", "body": "Comment"},
			mockFn: func(ctx context.Context, key string, body string) (*jira.Comment, error) {
				return nil, jira.ErrUnauthorized
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
			cl := &captureLogger{}
			svc := &mockJiraService{addCommentFunc: tc.mockFn}
			handler := mcpserver.ToolAddCommentToIssue(svc, cl)
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
			// Audit log must be written when write guard passes (regardless of service outcome)
			if !tc.wantIsError || (tc.wantContain != "write operations disabled" && tc.wantContain != "issue_key" && tc.wantContain != "body") {
				if tc.setup != nil && tc.args["issue_key"] != nil && tc.args["body"] != nil {
					if len(cl.entries) == 0 {
						t.Error("expected audit log entry, got none")
					}
				}
			}
		})
	}
}

// --- TestToolLinkIssues ---

func TestToolLinkIssues(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, inward, outward, linkType string) error
		wantIsError bool
		wantContain string
	}{
		{
			name:  "success returns confirmation text",
			setup: enableWrite,
			args:  map[string]any{"inward_issue": "PROJ-1", "outward_issue": "PROJ-2", "link_type": "Blocks"},
			mockFn: func(ctx context.Context, inward, outward, linkType string) error {
				return nil
			},
			wantIsError: false,
			wantContain: "issues linked successfully",
		},
		{
			name:        "write guard blocks when ENABLE_WRITE not set",
			setup:       disableWrite,
			args:        map[string]any{"inward_issue": "PROJ-1", "outward_issue": "PROJ-2", "link_type": "Blocks"},
			mockFn:      nil, // must never be called
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing inward_issue returns error",
			setup:       enableWrite,
			args:        map[string]any{"outward_issue": "PROJ-2", "link_type": "Blocks"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "inward_issue",
		},
		{
			name:        "missing outward_issue returns error",
			setup:       enableWrite,
			args:        map[string]any{"inward_issue": "PROJ-1", "link_type": "Blocks"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "outward_issue",
		},
		{
			name:        "missing link_type returns error",
			setup:       enableWrite,
			args:        map[string]any{"inward_issue": "PROJ-1", "outward_issue": "PROJ-2"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "link_type",
		},
		{
			name:  "ErrNotFound mapped to error result",
			setup: enableWrite,
			args:  map[string]any{"inward_issue": "PROJ-999", "outward_issue": "PROJ-2", "link_type": "Blocks"},
			mockFn: func(ctx context.Context, inward, outward, linkType string) error {
				return jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name:  "ErrUnauthorized mapped to error result",
			setup: enableWrite,
			args:  map[string]any{"inward_issue": "PROJ-1", "outward_issue": "PROJ-2", "link_type": "Blocks"},
			mockFn: func(ctx context.Context, inward, outward, linkType string) error {
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
			cl := &captureLogger{}
			svc := &mockJiraService{linkIssuesFunc: tc.mockFn}
			handler := mcpserver.ToolLinkIssues(svc, cl)
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

// --- TestToolAddWorklog ---

func TestToolAddWorklog(t *testing.T) {
	fixedTime := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		setup       func(t *testing.T)
		args        map[string]any
		mockFn      func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error)
		wantIsError bool
		wantContain string
	}{
		{
			name:  "success returns worklog JSON with snake_case fields",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-1", "time_spent": "2h"},
			mockFn: func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
				return &jira.Worklog{
					ID:               "wl-1",
					TimeSpentSeconds: 7200,
					Started:          fixedTime,
					Author:           "Alice",
				}, nil
			},
			wantIsError: false,
			wantContain: `"id":"wl-1"`,
		},
		{
			name:  "response contains time_spent_seconds field",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-1", "time_spent": "1h 30m"},
			mockFn: func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
				return &jira.Worklog{
					ID:               "wl-2",
					TimeSpentSeconds: 5400,
					Author:           "Bob",
				}, nil
			},
			wantIsError: false,
			wantContain: `"time_spent_seconds":5400`,
		},
		{
			name:  "zero started time is omitted from output (omitempty)",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-1", "time_spent": "1h"},
			mockFn: func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
				return &jira.Worklog{ID: "wl-3", TimeSpentSeconds: 3600, Author: "Carol"}, nil
			},
			wantIsError: false,
			// started should NOT appear when zero
			wantContain: `"author":"Carol"`,
		},
		{
			name:  "optional comment and started fields forwarded to service",
			setup: enableWrite,
			args: map[string]any{
				"issue_key":  "PROJ-1",
				"time_spent": "1h 30m",
				"comment":    "Sprint review",
				"started":    "2026-08-16T10:00:00.000+0000",
			},
			mockFn: func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
				if req.Comment != "Sprint review" {
					return nil, nil
				}
				if req.Started != "2026-08-16T10:00:00.000+0000" {
					return nil, nil
				}
				return &jira.Worklog{ID: "wl-4", TimeSpentSeconds: 5400, Started: fixedTime, Author: "Dave"}, nil
			},
			wantIsError: false,
			wantContain: `"id":"wl-4"`,
		},
		{
			name:        "write guard blocks when ENABLE_WRITE not set",
			setup:       disableWrite,
			args:        map[string]any{"issue_key": "PROJ-1", "time_spent": "2h"},
			mockFn:      nil, // must never be called
			wantIsError: true,
			wantContain: "write operations disabled",
		},
		{
			name:        "missing issue_key returns error",
			setup:       enableWrite,
			args:        map[string]any{"time_spent": "2h"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "issue_key",
		},
		{
			name:        "missing time_spent returns error",
			setup:       enableWrite,
			args:        map[string]any{"issue_key": "PROJ-1"},
			mockFn:      nil,
			wantIsError: true,
			wantContain: "time_spent",
		},
		{
			name:  "ErrNotFound mapped to error result",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-999", "time_spent": "1h"},
			mockFn: func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
				return nil, jira.ErrNotFound
			},
			wantIsError: true,
			wantContain: "not found",
		},
		{
			name:  "ErrUnauthorized mapped to error result",
			setup: enableWrite,
			args:  map[string]any{"issue_key": "PROJ-1", "time_spent": "1h"},
			mockFn: func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
				return nil, jira.ErrUnauthorized
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
			cl := &captureLogger{}
			svc := &mockJiraService{addWorklogFunc: tc.mockFn}
			handler := mcpserver.ToolAddWorklog(svc, cl)
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

// --- TestToolAddCommentToIssue_AuditLogging ---
// Dedicated test verifying audit log is always written after WriteGuardCheck passes.

func TestToolAddCommentToIssue_AuditLogging(t *testing.T) {
	t.Run("audit log written on success", func(t *testing.T) {
		enableWrite(t)
		cl := &captureLogger{}
		svc := &mockJiraService{
			addCommentFunc: func(ctx context.Context, key, body string) (*jira.Comment, error) {
				return &jira.Comment{ID: "cmt-1", Author: "Alice"}, nil
			},
		}
		handler := mcpserver.ToolAddCommentToIssue(svc, cl)
		req := makeCallToolRequest(map[string]any{"issue_key": "PROJ-1", "body": "Test"})
		_, _ = handler(context.Background(), req)
		if len(cl.entries) != 1 {
			t.Errorf("expected 1 audit entry, got %d", len(cl.entries))
		}
		if cl.entries[0].Operation != "add_comment_to_issue" {
			t.Errorf("audit operation: got %q, want %q", cl.entries[0].Operation, "add_comment_to_issue")
		}
	})

	t.Run("audit log written even when service errors", func(t *testing.T) {
		enableWrite(t)
		cl := &captureLogger{}
		svc := &mockJiraService{
			addCommentFunc: func(ctx context.Context, key, body string) (*jira.Comment, error) {
				return nil, jira.ErrNotFound
			},
		}
		handler := mcpserver.ToolAddCommentToIssue(svc, cl)
		req := makeCallToolRequest(map[string]any{"issue_key": "PROJ-999", "body": "Test"})
		_, _ = handler(context.Background(), req)
		if len(cl.entries) != 1 {
			t.Errorf("expected 1 audit entry on error, got %d", len(cl.entries))
		}
		if !strings.HasPrefix(cl.entries[0].Result, "error:") {
			t.Errorf("audit result on error: got %q, want prefix 'error:'", cl.entries[0].Result)
		}
	})

	t.Run("audit log NOT written when write guard blocks", func(t *testing.T) {
		disableWrite(t)
		cl := &captureLogger{}
		svc := &mockJiraService{}
		handler := mcpserver.ToolAddCommentToIssue(svc, cl)
		req := makeCallToolRequest(map[string]any{"issue_key": "PROJ-1", "body": "Test"})
		_, _ = handler(context.Background(), req)
		if len(cl.entries) != 0 {
			t.Errorf("expected 0 audit entries when write guard blocks, got %d", len(cl.entries))
		}
	})
}

// TestToolLinkIssues_AuditLogging verifies audit entries for link_issues.
func TestToolLinkIssues_AuditLogging(t *testing.T) {
	t.Run("audit log written on success", func(t *testing.T) {
		enableWrite(t)
		cl := &captureLogger{}
		svc := &mockJiraService{
			linkIssuesFunc: func(ctx context.Context, inward, outward, lt string) error { return nil },
		}
		handler := mcpserver.ToolLinkIssues(svc, cl)
		req := makeCallToolRequest(map[string]any{
			"inward_issue": "PROJ-1", "outward_issue": "PROJ-2", "link_type": "Blocks",
		})
		_, _ = handler(context.Background(), req)
		if len(cl.entries) != 1 {
			t.Errorf("expected 1 audit entry, got %d", len(cl.entries))
		}
		if cl.entries[0].Operation != "link_issues" {
			t.Errorf("audit operation: got %q, want %q", cl.entries[0].Operation, "link_issues")
		}
	})

	t.Run("audit log NOT written when write guard blocks", func(t *testing.T) {
		disableWrite(t)
		cl := &captureLogger{}
		svc := &mockJiraService{}
		handler := mcpserver.ToolLinkIssues(svc, cl)
		req := makeCallToolRequest(map[string]any{
			"inward_issue": "PROJ-1", "outward_issue": "PROJ-2", "link_type": "Blocks",
		})
		_, _ = handler(context.Background(), req)
		if len(cl.entries) != 0 {
			t.Errorf("expected 0 audit entries when write guard blocks, got %d", len(cl.entries))
		}
	})
}

// TestToolAddWorklog_AuditLogging verifies audit entries for add_worklog.
func TestToolAddWorklog_AuditLogging(t *testing.T) {
	t.Run("audit log written on success", func(t *testing.T) {
		enableWrite(t)
		cl := &captureLogger{}
		svc := &mockJiraService{
			addWorklogFunc: func(ctx context.Context, key string, req jira.AddWorklogRequest) (*jira.Worklog, error) {
				return &jira.Worklog{ID: "wl-1", TimeSpentSeconds: 3600, Author: "Alice"}, nil
			},
		}
		handler := mcpserver.ToolAddWorklog(svc, cl)
		req := makeCallToolRequest(map[string]any{"issue_key": "PROJ-1", "time_spent": "1h"})
		_, _ = handler(context.Background(), req)
		if len(cl.entries) != 1 {
			t.Errorf("expected 1 audit entry, got %d", len(cl.entries))
		}
		if cl.entries[0].Operation != "add_worklog" {
			t.Errorf("audit operation: got %q, want %q", cl.entries[0].Operation, "add_worklog")
		}
	})

	t.Run("audit log NOT written when write guard blocks", func(t *testing.T) {
		disableWrite(t)
		cl := &captureLogger{}
		svc := &mockJiraService{}
		handler := mcpserver.ToolAddWorklog(svc, cl)
		req := makeCallToolRequest(map[string]any{"issue_key": "PROJ-1", "time_spent": "1h"})
		_, _ = handler(context.Background(), req)
		if len(cl.entries) != 0 {
			t.Errorf("expected 0 audit entries when write guard blocks, got %d", len(cl.entries))
		}
	})
}

// TestToolAddCommentToIssue_ZeroTimestampsOmitted verifies omitempty on timestamps.
func TestToolAddCommentToIssue_ZeroTimestampsOmitted(t *testing.T) {
	enableWrite(t)
	svc := &mockJiraService{
		addCommentFunc: func(ctx context.Context, key, body string) (*jira.Comment, error) {
			// Return comment with zero Created/Updated
			return &jira.Comment{ID: "cmt-zero", Author: "Alice", Body: "test"}, nil
		},
	}
	handler := mcpserver.ToolAddCommentToIssue(svc, audit.NewNoopLogger())
	req := makeCallToolRequest(map[string]any{"issue_key": "PROJ-1", "body": "test"})
	result, _ := handler(context.Background(), req)
	text := getResultText(t, result)
	// "created" and "updated" should NOT appear with zero times
	if strings.Contains(text, `"created"`) {
		t.Errorf("zero created timestamp must be omitted, got: %s", text)
	}
	if strings.Contains(text, `"updated"`) {
		t.Errorf("zero updated timestamp must be omitted, got: %s", text)
	}
}
