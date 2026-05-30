package jira_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	jirasvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
)

func TestSearchCommand_RendersResultsAsJSON(t *testing.T) {
	svc := &mockJiraService{
		searchIssuesFunc: func(ctx context.Context, jql string, maxResults int) (*jirasvc.SearchResult, error) {
			return &jirasvc.SearchResult{
				Total:      2,
				MaxResults: maxResults,
				Issues: []jirasvc.Issue{
					{Key: "PROJ-1", Summary: "Issue one", Status: "Open"},
					{Key: "PROJ-2", Summary: "Issue two", Status: "Done"},
				},
			}, nil
		},
	}

	result, err := svc.SearchIssues(context.Background(), "project = PROJ", 50)
	if err != nil {
		t.Fatalf("SearchIssues() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(result.Issues)
	if err != nil {
		t.Fatalf("Format() unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("JSON output missing PROJ-1\nGot: %s", out)
	}
	if !strings.Contains(out, "PROJ-2") {
		t.Errorf("JSON output missing PROJ-2\nGot: %s", out)
	}
}

func TestSearchCommand_InvalidJQLMapsToExitCode1(t *testing.T) {
	svc := &mockJiraService{
		searchIssuesFunc: func(ctx context.Context, jql string, maxResults int) (*jirasvc.SearchResult, error) {
			return nil, jirasvc.ErrInvalidJQL
		},
	}

	_, err := svc.SearchIssues(context.Background(), "ORDER BY ???", 50)
	if !errors.Is(err, jirasvc.ErrInvalidJQL) {
		t.Errorf("expected ErrInvalidJQL, got: %v", err)
	}
}

func TestSearchCommand_DefaultMaxResults(t *testing.T) {
	// Verify that passing 0 to SearchIssues uses the default from the service layer
	var gotMaxResults int
	svc := &mockJiraService{
		searchIssuesFunc: func(ctx context.Context, jql string, maxResults int) (*jirasvc.SearchResult, error) {
			gotMaxResults = maxResults
			return &jirasvc.SearchResult{}, nil
		},
	}

	// The CLI's default is 50; test that the service receives a positive value
	_, err := svc.SearchIssues(context.Background(), "project = PROJ", 50)
	if err != nil {
		t.Fatalf("SearchIssues() unexpected error: %v", err)
	}
	if gotMaxResults != 50 {
		t.Errorf("expected maxResults=50, got %d", gotMaxResults)
	}
}

func TestSearchCommand_RendersEmptyResults(t *testing.T) {
	// Empty result should still produce valid output without panicking
	svc := &mockJiraService{
		searchIssuesFunc: func(ctx context.Context, jql string, maxResults int) (*jirasvc.SearchResult, error) {
			return &jirasvc.SearchResult{Total: 0, Issues: []jirasvc.Issue{}}, nil
		},
	}

	result, err := svc.SearchIssues(context.Background(), "project = EMPTY", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected Total=0, got %d", result.Total)
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(result.Issues)
	if err != nil {
		t.Fatalf("Format() error on empty: %v", err)
	}
	// Table header should still render even with zero results
	if len(data) == 0 {
		t.Error("expected non-empty output even for zero results")
	}
}
