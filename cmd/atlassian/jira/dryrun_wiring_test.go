package jira_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	jiracli "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/jira"
	jirasvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/spf13/cobra"
)

// newDryRunTestRoot mimics main.go: a root command that owns the persistent
// --dry-run flag, with the jira subgroup registered while capturing
// dryRun=false at CONSTRUCTION time (before cobra parses flags). This is the
// exact wiring that hid the bug where --dry-run was a no-op.
func newDryRunTestRoot(svc jirasvc.Service) *cobra.Command {
	var dryRun bool
	root := &cobra.Command{Use: "atlassian"}
	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Print what would happen without executing write operations")

	jiraRoot := jiracli.NewJiraCmd()
	// Capture dryRun BY VALUE (false) at construction, exactly like main.go does.
	jiracli.RegisterCommands(jiraRoot, svc, audit.NewNoopLogger(), dryRun)
	root.AddCommand(jiraRoot)
	return root
}

// TestDryRunWiring_FlagBlocksWrite is a WIRING/integration test that would have
// caught the construction-time capture bug. With --dry-run passed at execution
// time, the write service method must NOT be called and stdout must contain the
// [DRY RUN] marker. Against the buggy code (plain `if dryRun {`), the captured
// false value wins and CreateIssue IS called, failing this test.
func TestDryRunWiring_FlagBlocksWrite(t *testing.T) {
	svc := &mockJiraService{
		createIssueFunc: func(_ context.Context, _ jirasvc.CreateIssueRequest) (*jirasvc.CreateIssueResponse, error) {
			t.Fatal("CreateIssue must NOT be called when --dry-run is passed")
			return nil, nil
		},
	}

	root := newDryRunTestRoot(svc)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"jira", "create", "--dry-run", "--project", "X", "--type", "Task", "--summary", "s"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out := buf.String(); !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] in output, got: %q", out)
	}
}

// TestDryRunWiring_NoFlagCallsWrite guards against over-blocking: WITHOUT
// --dry-run, the write service method MUST be called.
func TestDryRunWiring_NoFlagCallsWrite(t *testing.T) {
	var called bool
	svc := &mockJiraService{
		createIssueFunc: func(_ context.Context, _ jirasvc.CreateIssueRequest) (*jirasvc.CreateIssueResponse, error) {
			called = true
			return &jirasvc.CreateIssueResponse{Key: "X-1", ID: "1"}, nil
		},
	}

	root := newDryRunTestRoot(svc)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"jira", "create", "--project", "X", "--type", "Task", "--summary", "s"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected CreateIssue to be called when --dry-run is not passed")
	}
}
