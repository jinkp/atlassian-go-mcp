package mcpserver_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

func TestToolValidateReleaseForDeploy_Success(t *testing.T) {
	svc := &mockJiraService{
		searchIssuesFunc: func(_ context.Context, jql string, _ int) (*jira.SearchResult, error) {
			if !strings.Contains(jql, `fixVersion = "v1.0.0"`) {
				t.Errorf("unexpected jql: %s", jql)
			}
			return &jira.SearchResult{Issues: []jira.Issue{
				{Key: "P-1", Status: "Done", IssueType: "Story"},
			}}, nil
		},
	}

	handler := mcpserver.ToolValidateReleaseForDeploy(svc)
	res, err := handler(context.Background(), makeCallToolRequest(map[string]any{
		"project_key":  "PROJ",
		"release_name": "v1.0.0",
	}))
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", getResultText(t, res))
	}

	var out struct {
		Ready  bool     `json:"ready"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(getResultText(t, res)), &out); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if !out.Ready {
		t.Errorf("expected ready=true, got errors %v", out.Errors)
	}
}

func TestToolValidateReleaseForDeploy_MissingArgs(t *testing.T) {
	svc := &mockJiraService{
		searchIssuesFunc: func(_ context.Context, _ string, _ int) (*jira.SearchResult, error) {
			return &jira.SearchResult{}, nil
		},
	}
	handler := mcpserver.ToolValidateReleaseForDeploy(svc)
	res, _ := handler(context.Background(), makeCallToolRequest(map[string]any{
		"project_key": "PROJ",
	}))
	if !res.IsError {
		t.Fatal("expected error result when release_name missing")
	}
}

func TestToolGenerateReleaseNotes_Success(t *testing.T) {
	svc := &mockJiraService{
		searchIssuesFunc: func(_ context.Context, _ string, _ int) (*jira.SearchResult, error) {
			return &jira.SearchResult{Issues: []jira.Issue{
				{Key: "P-1", Summary: "Add login", IssueType: "Story"},
				{Key: "P-2", Summary: "Fix crash", IssueType: "Bug"},
			}}, nil
		},
	}
	handler := mcpserver.ToolGenerateReleaseNotes(svc)
	res, err := handler(context.Background(), makeCallToolRequest(map[string]any{
		"project_key":  "PROJ",
		"release_name": "v2.0.0",
	}))
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", getResultText(t, res))
	}
	md := getResultText(t, res)
	if !strings.Contains(md, "# Release Notes: v2.0.0") {
		t.Errorf("missing title:\n%s", md)
	}
	if !strings.Contains(md, "## Bug") || !strings.Contains(md, "## Story") {
		t.Errorf("missing grouped headings:\n%s", md)
	}
}
