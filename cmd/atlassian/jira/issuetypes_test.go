package jira_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	jiracli "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/jira"
	jirasvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// --- issue-types — success ---

func TestIssueTypesCommand_Success(t *testing.T) {
	svc := &mockJiraService{
		getIssueTypeMetaFunc: func(_ context.Context, projectKey string) ([]jirasvc.IssueTypeMeta, error) {
			if projectKey != "PROJ" {
				t.Errorf("expected projectKey 'PROJ', got %q", projectKey)
			}
			return []jirasvc.IssueTypeMeta{
				{ID: "10001", Name: "Bug", Desc: "A software bug", Subtask: false},
				{ID: "10002", Name: "Task", Desc: "A task", Subtask: false},
			}, nil
		},
	}

	cmd := jiracli.NewIssueTypesCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"PROJ"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Bug") {
		t.Errorf("expected 'Bug' in output, got: %q", out)
	}
	if !strings.Contains(out, "Task") {
		t.Errorf("expected 'Task' in output, got: %q", out)
	}
}

// --- issue-types — not found ---

func TestIssueTypesCommand_NotFound(t *testing.T) {
	svc := &mockJiraService{
		getIssueTypeMetaFunc: func(_ context.Context, _ string) ([]jirasvc.IssueTypeMeta, error) {
			return nil, jirasvc.ErrNotFound
		},
	}

	_, err := svc.GetIssueTypeMetadata(context.Background(), "NOPROJECT")
	if !errors.Is(err, jirasvc.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
	if code := mapErrorToExitCode(err); code != 3 {
		t.Errorf("expected exit code 3 for ErrNotFound, got %d", code)
	}
}

// --- issue-types — unauthorized ---

func TestIssueTypesCommand_Unauthorized(t *testing.T) {
	svc := &mockJiraService{
		getIssueTypeMetaFunc: func(_ context.Context, _ string) ([]jirasvc.IssueTypeMeta, error) {
			return nil, jirasvc.ErrUnauthorized
		},
	}

	_, err := svc.GetIssueTypeMetadata(context.Background(), "PROJ")
	if !errors.Is(err, jirasvc.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
	if code := mapErrorToExitCode(err); code != 2 {
		t.Errorf("expected exit code 2 for ErrUnauthorized, got %d", code)
	}
}
