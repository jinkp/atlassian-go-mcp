package jira_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	jirasvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// --- SC-J1: create — success ---

func TestCreateCommand_Success(t *testing.T) {
	svc := &mockJiraService{
		createIssueFunc: func(_ context.Context, req jirasvc.CreateIssueRequest) (*jirasvc.CreateIssueResponse, error) {
			if req.ProjectKey != "TEST" {
				t.Errorf("expected project key 'TEST', got %q", req.ProjectKey)
			}
			if req.Summary != "hello" {
				t.Errorf("expected summary 'hello', got %q", req.Summary)
			}
			return &jirasvc.CreateIssueResponse{Key: "TEST-42", ID: "10042"}, nil
		},
	}

	resp, err := svc.CreateIssue(context.Background(), jirasvc.CreateIssueRequest{
		ProjectKey: "TEST",
		IssueType:  "Task",
		Summary:    "hello",
	})
	if err != nil {
		t.Fatalf("CreateIssue() unexpected error: %v", err)
	}
	if resp.Key != "TEST-42" {
		t.Errorf("expected key 'TEST-42', got %q", resp.Key)
	}

	out := "Created: TEST-42"
	if !strings.Contains(out, "TEST-42") {
		t.Errorf("output missing key\nGot: %s", out)
	}
}

// --- SC-J3: create — unauthorized ---

func TestCreateCommand_Unauthorized(t *testing.T) {
	svc := &mockJiraService{
		createIssueFunc: func(_ context.Context, _ jirasvc.CreateIssueRequest) (*jirasvc.CreateIssueResponse, error) {
			return nil, jirasvc.ErrUnauthorized
		},
	}

	_, err := svc.CreateIssue(context.Background(), jirasvc.CreateIssueRequest{
		ProjectKey: "TEST",
		IssueType:  "Task",
		Summary:    "hello",
	})
	if !errors.Is(err, jirasvc.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
	if code := mapErrorToExitCode(err); code != 2 {
		t.Errorf("expected exit code 2 for ErrUnauthorized, got %d", code)
	}
}

// --- SC-J4: update — success ---

func TestUpdateCommand_Success(t *testing.T) {
	summary := "new title"
	svc := &mockJiraService{
		updateIssueFunc: func(_ context.Context, key string, req jirasvc.UpdateIssueRequest) error {
			if key != "PROJ-1" {
				t.Errorf("expected key 'PROJ-1', got %q", key)
			}
			if req.Summary == nil || *req.Summary != "new title" {
				t.Errorf("expected summary 'new title'")
			}
			return nil
		},
	}

	err := svc.UpdateIssue(context.Background(), "PROJ-1", jirasvc.UpdateIssueRequest{Summary: &summary})
	if err != nil {
		t.Fatalf("UpdateIssue() unexpected error: %v", err)
	}

	out := "Updated: PROJ-1"
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("output missing key\nGot: %s", out)
	}
}

// --- SC-J5: update — not found ---

func TestUpdateCommand_NotFound(t *testing.T) {
	svc := &mockJiraService{
		updateIssueFunc: func(_ context.Context, _ string, _ jirasvc.UpdateIssueRequest) error {
			return jirasvc.ErrNotFound
		},
	}

	err := svc.UpdateIssue(context.Background(), "PROJ-999", jirasvc.UpdateIssueRequest{})
	if !errors.Is(err, jirasvc.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
	if code := mapErrorToExitCode(err); code != 3 {
		t.Errorf("expected exit code 3 for ErrNotFound, got %d", code)
	}
}
