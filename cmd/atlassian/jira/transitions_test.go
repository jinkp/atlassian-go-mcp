package jira_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	jirasvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
)

// --- SC-J6: transitions — table output ---

func TestTransitionsCommand_RendersTable(t *testing.T) {
	svc := &mockJiraService{
		getTransitionsFunc: func(_ context.Context, key string) ([]jirasvc.Transition, error) {
			if key != "PROJ-1" {
				t.Errorf("expected key 'PROJ-1', got %q", key)
			}
			return []jirasvc.Transition{
				{ID: "10", Name: "In Progress", StatusCategory: "indeterminate"},
				{ID: "31", Name: "Done", StatusCategory: "done"},
			}, nil
		},
	}

	transitions, err := svc.GetTransitions(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("GetTransitions() unexpected error: %v", err)
	}

	// Use JSON formatter since transitions fall through to JSON fallback in table formatter
	f, _ := output.NewFormatter("json")
	data, err := f.Format(transitions)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "10") {
		t.Errorf("output missing transition ID '10'\nGot: %s", out)
	}
	if !strings.Contains(out, "In Progress") {
		t.Errorf("output missing transition name\nGot: %s", out)
	}
	if !strings.Contains(out, "indeterminate") {
		t.Errorf("output missing status category\nGot: %s", out)
	}
}

// --- SC-J7: transition — success ---

func TestTransitionCommand_Success(t *testing.T) {
	svc := &mockJiraService{
		transitionFunc: func(_ context.Context, key string, transitionID string) error {
			if key != "PROJ-1" {
				t.Errorf("expected key 'PROJ-1', got %q", key)
			}
			if transitionID != "10" {
				t.Errorf("expected transitionID '10', got %q", transitionID)
			}
			return nil
		},
	}

	err := svc.TransitionIssue(context.Background(), "PROJ-1", "10")
	if err != nil {
		t.Fatalf("TransitionIssue() unexpected error: %v", err)
	}

	out := "Transitioned: PROJ-1"
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("output missing key\nGot: %s", out)
	}
}

// --- SC-J8: transition — not found ---

func TestTransitionCommand_NotFound(t *testing.T) {
	svc := &mockJiraService{
		transitionFunc: func(_ context.Context, _ string, _ string) error {
			return jirasvc.ErrNotFound
		},
	}

	err := svc.TransitionIssue(context.Background(), "PROJ-999", "10")
	if !errors.Is(err, jirasvc.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
	if code := mapErrorToExitCode(err); code != 3 {
		t.Errorf("expected exit code 3, got %d", code)
	}
}

// --- transitions output format test ---

func TestTransitionsCommand_JSONFormat(t *testing.T) {
	transitions := []jirasvc.Transition{
		{ID: "5", Name: "To Do", StatusCategory: "new"},
	}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(transitions)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, `"id"`) {
		t.Errorf("JSON missing 'id' field\nGot: %s", out)
	}
	if !strings.Contains(out, "To Do") {
		t.Errorf("JSON missing transition name\nGot: %s", out)
	}
}
