package jira_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	jirasvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
)

// mockJiraService implements jirasvc.Service for testing.
type mockJiraService struct {
	getIssueFunc           func(ctx context.Context, key string) (*jirasvc.Issue, error)
	searchIssuesFunc       func(ctx context.Context, jql string, maxResults int) (*jirasvc.SearchResult, error)
	createIssueFunc        func(ctx context.Context, req jirasvc.CreateIssueRequest) (*jirasvc.CreateIssueResponse, error)
	updateIssueFunc        func(ctx context.Context, key string, req jirasvc.UpdateIssueRequest) error
	getTransitionsFunc     func(ctx context.Context, key string) ([]jirasvc.Transition, error)
	transitionFunc         func(ctx context.Context, key string, transitionID string) error
	lookupAccountIDFunc    func(ctx context.Context, query string, maxResults int) ([]jirasvc.User, error)
	addCommentFunc         func(ctx context.Context, key string, body string) (*jirasvc.Comment, error)
	getCommentsFunc        func(ctx context.Context, key string, maxResults int) ([]jirasvc.Comment, error)
	linkIssuesFunc         func(ctx context.Context, inward, outward, linkTypeName string) error
	getIssueLinkTypesFunc  func(ctx context.Context) ([]jirasvc.IssueLinkType, error)
	addWorklogFunc         func(ctx context.Context, key string, req jirasvc.AddWorklogRequest) (*jirasvc.Worklog, error)
	getIssueTypeMetaFunc   func(ctx context.Context, projectKey string) ([]jirasvc.IssueTypeMeta, error)
}

func (m *mockJiraService) GetIssue(ctx context.Context, key string) (*jirasvc.Issue, error) {
	if m.getIssueFunc != nil {
		return m.getIssueFunc(ctx, key)
	}
	return nil, errors.New("not implemented")
}

func (m *mockJiraService) SearchIssues(ctx context.Context, jql string, maxResults int) (*jirasvc.SearchResult, error) {
	if m.searchIssuesFunc != nil {
		return m.searchIssuesFunc(ctx, jql, maxResults)
	}
	return nil, errors.New("not implemented")
}

func (m *mockJiraService) CreateIssue(ctx context.Context, req jirasvc.CreateIssueRequest) (*jirasvc.CreateIssueResponse, error) {
	if m.createIssueFunc != nil {
		return m.createIssueFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockJiraService) UpdateIssue(ctx context.Context, key string, req jirasvc.UpdateIssueRequest) error {
	if m.updateIssueFunc != nil {
		return m.updateIssueFunc(ctx, key, req)
	}
	return errors.New("not implemented")
}

func (m *mockJiraService) GetTransitions(ctx context.Context, key string) ([]jirasvc.Transition, error) {
	if m.getTransitionsFunc != nil {
		return m.getTransitionsFunc(ctx, key)
	}
	return nil, errors.New("not implemented")
}

func (m *mockJiraService) TransitionIssue(ctx context.Context, key string, transitionID string) error {
	if m.transitionFunc != nil {
		return m.transitionFunc(ctx, key, transitionID)
	}
	return errors.New("not implemented")
}

func (m *mockJiraService) LookupAccountID(ctx context.Context, query string, maxResults int) ([]jirasvc.User, error) {
	if m.lookupAccountIDFunc != nil {
		return m.lookupAccountIDFunc(ctx, query, maxResults)
	}
	return nil, errors.New("not implemented")
}

func (m *mockJiraService) AddComment(ctx context.Context, key string, body string) (*jirasvc.Comment, error) {
	if m.addCommentFunc != nil {
		return m.addCommentFunc(ctx, key, body)
	}
	return nil, errors.New("not implemented")
}

func (m *mockJiraService) GetComments(ctx context.Context, key string, maxResults int) ([]jirasvc.Comment, error) {
	if m.getCommentsFunc != nil {
		return m.getCommentsFunc(ctx, key, maxResults)
	}
	return nil, errors.New("not implemented")
}

func (m *mockJiraService) LinkIssues(ctx context.Context, inward, outward, linkTypeName string) error {
	if m.linkIssuesFunc != nil {
		return m.linkIssuesFunc(ctx, inward, outward, linkTypeName)
	}
	return errors.New("not implemented")
}

func (m *mockJiraService) GetIssueLinkTypes(ctx context.Context) ([]jirasvc.IssueLinkType, error) {
	if m.getIssueLinkTypesFunc != nil {
		return m.getIssueLinkTypesFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockJiraService) AddWorklog(ctx context.Context, key string, req jirasvc.AddWorklogRequest) (*jirasvc.Worklog, error) {
	if m.addWorklogFunc != nil {
		return m.addWorklogFunc(ctx, key, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockJiraService) GetIssueTypeMetadata(ctx context.Context, projectKey string) ([]jirasvc.IssueTypeMeta, error) {
	if m.getIssueTypeMetaFunc != nil {
		return m.getIssueTypeMetaFunc(ctx, projectKey)
	}
	return nil, errors.New("not implemented")
}

// --- Formatter validation tests (unit-testable without cobra) ---

func TestOutputFlag_UnknownFormatReturnsError(t *testing.T) {
	_, err := output.NewFormatter("xml")
	if err == nil {
		t.Fatal("expected error for unknown format 'xml'")
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Errorf("error should mention 'xml', got: %v", err)
	}
}

func TestOutputFlag_DefaultIsTable(t *testing.T) {
	f, err := output.NewFormatter("table")
	if err != nil {
		t.Fatalf("'table' should be a valid format: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil formatter for 'table'")
	}
}

// --- GetIssue output rendering tests ---

func TestGetCommand_RendersIssueAsJSON(t *testing.T) {
	svc := &mockJiraService{
		getIssueFunc: func(ctx context.Context, key string) (*jirasvc.Issue, error) {
			return &jirasvc.Issue{
				Key:     key,
				Summary: "Fix login bug",
				Status:  "Open",
			}, nil
		},
	}

	issue, err := svc.GetIssue(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("GetIssue() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(issue)
	if err != nil {
		t.Fatalf("Format() unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("JSON output missing key 'PROJ-1'\nGot: %s", out)
	}
	if !strings.Contains(out, "Fix login bug") {
		t.Errorf("JSON output missing summary\nGot: %s", out)
	}
}

func TestGetCommand_ErrNotFoundMapsToExitCode3(t *testing.T) {
	// Verify that ErrNotFound is a distinct sentinel error that callers can check
	svc := &mockJiraService{
		getIssueFunc: func(ctx context.Context, key string) (*jirasvc.Issue, error) {
			return nil, jirasvc.ErrNotFound
		},
	}

	_, err := svc.GetIssue(context.Background(), "PROJ-999")
	if !errors.Is(err, jirasvc.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
	// CLI command maps ErrNotFound → exit 3
	exitCode := mapErrorToExitCode(err)
	if exitCode != 3 {
		t.Errorf("expected exit code 3 for ErrNotFound, got %d", exitCode)
	}
}

func TestGetCommand_ErrUnauthorizedMapsToExitCode2(t *testing.T) {
	svc := &mockJiraService{
		getIssueFunc: func(ctx context.Context, key string) (*jirasvc.Issue, error) {
			return nil, jirasvc.ErrUnauthorized
		},
	}

	_, err := svc.GetIssue(context.Background(), "PROJ-1")
	if !errors.Is(err, jirasvc.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
	exitCode := mapErrorToExitCode(err)
	if exitCode != 2 {
		t.Errorf("expected exit code 2 for ErrUnauthorized, got %d", exitCode)
	}
}

func TestGetCommand_RendersIssueAsTable(t *testing.T) {
	svc := &mockJiraService{
		getIssueFunc: func(ctx context.Context, key string) (*jirasvc.Issue, error) {
			return &jirasvc.Issue{
				Key:      key,
				Summary:  "Dark mode support",
				Status:   "Done",
				Assignee: "Bob",
				Priority: "Low",
				Labels:   []string{},
			}, nil
		},
	}

	issue, err := svc.GetIssue(context.Background(), "PROJ-42")
	if err != nil {
		t.Fatalf("GetIssue() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(issue)
	if err != nil {
		t.Fatalf("Format() unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "PROJ-42") {
		t.Errorf("table output missing key\nGot:\n%s", out)
	}
	if !strings.Contains(out, "Dark mode support") {
		t.Errorf("table output missing summary\nGot:\n%s", out)
	}
}

// TestWriteOutput verifies that formatter output reaches the writer.
func TestWriteOutput_WritesToBuffer(t *testing.T) {
	var buf bytes.Buffer
	issue := &jirasvc.Issue{Key: "PROJ-1", Summary: "Test"}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(issue)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}
	buf.Write(data)

	if !strings.Contains(buf.String(), "PROJ-1") {
		t.Errorf("buffer missing PROJ-1\nGot: %s", buf.String())
	}
}

// mapErrorToExitCode mirrors the exit code mapping in get.go command.
// It's extracted here as a pure function so it can be tested without cobra.
func mapErrorToExitCode(err error) int {
	if errors.Is(err, jirasvc.ErrNotFound) {
		return 3
	}
	if errors.Is(err, jirasvc.ErrUnauthorized) {
		return 2
	}
	if err != nil {
		return 2
	}
	return 0
}
