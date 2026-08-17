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

// --- worklog add — dry run ---

func TestWorklogAddCommand_DryRun(t *testing.T) {
	svc := &mockJiraService{
		addWorklogFunc: func(_ context.Context, _ string, _ jirasvc.AddWorklogRequest) (*jirasvc.Worklog, error) {
			t.Error("service should NOT be called in dry-run mode")
			return nil, nil
		},
	}

	cmd := jiracli.NewWorklogCmd(svc, audit.NewNoopLogger(), true)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"add", "PROJ-1", "--time-spent", "2h"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] in output, got: %q", out)
	}
}

// --- worklog add — success ---

func TestWorklogAddCommand_CallsService(t *testing.T) {
	var called bool
	svc := &mockJiraService{
		addWorklogFunc: func(_ context.Context, key string, req jirasvc.AddWorklogRequest) (*jirasvc.Worklog, error) {
			called = true
			if key != "PROJ-1" {
				t.Errorf("expected key 'PROJ-1', got %q", key)
			}
			if req.TimeSpent != "2h" {
				t.Errorf("expected time-spent '2h', got %q", req.TimeSpent)
			}
			return &jirasvc.Worklog{ID: "wl-1", TimeSpentSeconds: 7200}, nil
		},
	}

	cmd := jiracli.NewWorklogCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"add", "PROJ-1", "--time-spent", "2h"})
	_ = cmd.Execute()

	if !called {
		t.Error("expected service.AddWorklog to be called")
	}
	if !strings.Contains(buf.String(), "wl-1") {
		t.Errorf("expected worklog ID 'wl-1' in output, got: %q", buf.String())
	}
}

// --- worklog add — not found ---

func TestWorklogAddCommand_NotFound(t *testing.T) {
	svc := &mockJiraService{
		addWorklogFunc: func(_ context.Context, _ string, _ jirasvc.AddWorklogRequest) (*jirasvc.Worklog, error) {
			return nil, jirasvc.ErrNotFound
		},
	}

	_, err := svc.AddWorklog(context.Background(), "PROJ-999", jirasvc.AddWorklogRequest{TimeSpent: "1h"})
	if !errors.Is(err, jirasvc.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
	if code := mapErrorToExitCode(err); code != 3 {
		t.Errorf("expected exit code 3 for ErrNotFound, got %d", code)
	}
}
