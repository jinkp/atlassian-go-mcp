package jira_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	jiracli "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/jira"
	jirasvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
)

// --- comment add — dry run ---

func TestCommentAddCommand_DryRun(t *testing.T) {
	svc := &mockJiraService{
		addCommentFunc: func(_ context.Context, _ string, _ string) (*jirasvc.Comment, error) {
			t.Error("service should NOT be called in dry-run mode")
			return nil, nil
		},
	}

	cmd := jiracli.NewCommentCmd(svc, audit.NewNoopLogger(), true)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"add", "PROJ-1", "This is a comment"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] in output, got: %q", out)
	}
}

// --- comment add — success (service level) ---

func TestCommentAddCommand_CallsService(t *testing.T) {
	var called bool
	svc := &mockJiraService{
		addCommentFunc: func(_ context.Context, key string, body string) (*jirasvc.Comment, error) {
			called = true
			if key != "PROJ-1" {
				t.Errorf("expected key 'PROJ-1', got %q", key)
			}
			if body != "hello world" {
				t.Errorf("expected body 'hello world', got %q", body)
			}
			return &jirasvc.Comment{ID: "10001", Author: "Alice"}, nil
		},
	}

	cmd := jiracli.NewCommentCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"add", "PROJ-1", "hello world"})
	_ = cmd.Execute()

	if !called {
		t.Error("expected service.AddComment to be called")
	}
	if !strings.Contains(buf.String(), "10001") {
		t.Errorf("expected comment ID '10001' in output, got: %q", buf.String())
	}
}

// --- comment list — success ---

func TestCommentListCommand_Success(t *testing.T) {
	svc := &mockJiraService{
		getCommentsFunc: func(_ context.Context, key string, _ int) ([]jirasvc.Comment, error) {
			if key != "PROJ-1" {
				t.Errorf("expected key 'PROJ-1', got %q", key)
			}
			return []jirasvc.Comment{
				{ID: "200", Author: "Bob", Body: "Looks good"},
			}, nil
		},
	}

	cmd := jiracli.NewCommentCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"list", "PROJ-1"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "200") {
		t.Errorf("expected comment ID '200' in output, got: %q", out)
	}
	if !strings.Contains(out, "Looks good") {
		t.Errorf("expected comment body in output, got: %q", out)
	}
}

// --- comment list — not found ---

func TestCommentListCommand_NotFound(t *testing.T) {
	svc := &mockJiraService{
		getCommentsFunc: func(_ context.Context, _ string, _ int) ([]jirasvc.Comment, error) {
			return nil, jirasvc.ErrNotFound
		},
	}

	_, err := svc.GetComments(context.Background(), "PROJ-999", 0)
	if !errors.Is(err, jirasvc.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
	if code := mapErrorToExitCode(err); code != 3 {
		t.Errorf("expected exit code 3 for ErrNotFound, got %d", code)
	}
}
