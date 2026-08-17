package jira_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	jirasvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	jiracli "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/jira"
)

// --- users search — success ---

func TestUsersSearchCommand_Success(t *testing.T) {
	svc := &mockJiraService{
		lookupAccountIDFunc: func(_ context.Context, query string, maxResults int) ([]jirasvc.User, error) {
			if query != "alice" {
				t.Errorf("expected query 'alice', got %q", query)
			}
			return []jirasvc.User{
				{AccountID: "acc-1", DisplayName: "Alice Smith", Email: "alice@example.com", Active: true},
			}, nil
		},
	}

	cmd := jiracli.NewUsersCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"search", "--query", "alice"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "acc-1") {
		t.Errorf("expected account ID 'acc-1' in output, got: %q", out)
	}
	if !strings.Contains(out, "Alice Smith") {
		t.Errorf("expected display name 'Alice Smith' in output, got: %q", out)
	}
}

// --- users search — unauthorized ---

func TestUsersSearchCommand_Unauthorized(t *testing.T) {
	svc := &mockJiraService{
		lookupAccountIDFunc: func(_ context.Context, _ string, _ int) ([]jirasvc.User, error) {
			return nil, jirasvc.ErrUnauthorized
		},
	}

	_, err := svc.LookupAccountID(context.Background(), "alice", 0)
	if err != jirasvc.ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
	if code := mapErrorToExitCode(err); code != 2 {
		t.Errorf("expected exit code 2 for ErrUnauthorized, got %d", code)
	}
}
