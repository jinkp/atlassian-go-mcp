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

// --- link — dry run ---

func TestLinkCommand_DryRun(t *testing.T) {
	svc := &mockJiraService{
		linkIssuesFunc: func(_ context.Context, _, _, _ string) error {
			t.Error("service should NOT be called in dry-run mode")
			return nil
		},
	}

	cmd := jiracli.NewLinkCmd(svc, audit.NewNoopLogger(), true)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"PROJ-1", "PROJ-2", "--type", "Blocks"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] in output, got: %q", out)
	}
}

// --- link — success ---

func TestLinkCommand_CallsService(t *testing.T) {
	var called bool
	svc := &mockJiraService{
		linkIssuesFunc: func(_ context.Context, inward, outward, linkTypeName string) error {
			called = true
			if inward != "PROJ-1" {
				t.Errorf("expected inward 'PROJ-1', got %q", inward)
			}
			if outward != "PROJ-2" {
				t.Errorf("expected outward 'PROJ-2', got %q", outward)
			}
			if linkTypeName != "Blocks" {
				t.Errorf("expected linkType 'Blocks', got %q", linkTypeName)
			}
			return nil
		},
	}

	cmd := jiracli.NewLinkCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"PROJ-1", "PROJ-2", "--type", "Blocks"})
	_ = cmd.Execute()

	if !called {
		t.Error("expected service.LinkIssues to be called")
	}
	if !strings.Contains(buf.String(), "PROJ-1") {
		t.Errorf("expected 'PROJ-1' in output, got: %q", buf.String())
	}
}

// --- link — not found ---

func TestLinkCommand_NotFound(t *testing.T) {
	svc := &mockJiraService{
		linkIssuesFunc: func(_ context.Context, _, _, _ string) error {
			return jirasvc.ErrNotFound
		},
	}

	err := svc.LinkIssues(context.Background(), "PROJ-999", "PROJ-1", "Blocks")
	if !errors.Is(err, jirasvc.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
	if code := mapErrorToExitCode(err); code != 3 {
		t.Errorf("expected exit code 3 for ErrNotFound, got %d", code)
	}
}

// --- link-types — success ---

func TestLinkTypesCommand_Success(t *testing.T) {
	svc := &mockJiraService{
		getIssueLinkTypesFunc: func(_ context.Context) ([]jirasvc.IssueLinkType, error) {
			return []jirasvc.IssueLinkType{
				{ID: "1", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
			}, nil
		},
	}

	cmd := jiracli.NewLinkTypesCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Blocks") {
		t.Errorf("expected 'Blocks' in output, got: %q", out)
	}
}
