package releases_test

import (
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
)

func TestGenerateNotes_Empty(t *testing.T) {
	md := releases.GenerateNotes(nil, "v1.0.0")
	if !strings.Contains(md, "# Release Notes: v1.0.0") {
		t.Errorf("missing title, got:\n%s", md)
	}
	if !strings.Contains(md, "_No issues linked to this release._") {
		t.Errorf("missing empty marker, got:\n%s", md)
	}
}

func TestGenerateNotes_GroupedAndSorted(t *testing.T) {
	issues := []jira.Issue{
		{Key: "P-3", Summary: "Fix crash", IssueType: "Bug"},
		{Key: "P-1", Summary: "Add login", IssueType: "Story"},
		{Key: "P-2", Summary: "Second bug", IssueType: "Bug"},
	}
	md := releases.GenerateNotes(issues, "v2.0.0")

	// Groups sorted alphabetically → Bug before Story.
	bugIdx := strings.Index(md, "## Bug")
	storyIdx := strings.Index(md, "## Story")
	if bugIdx == -1 || storyIdx == -1 {
		t.Fatalf("expected both group headings, got:\n%s", md)
	}
	if bugIdx > storyIdx {
		t.Errorf("expected Bug group before Story group, got:\n%s", md)
	}

	// Issues within a group preserve input order (P-3 before P-2).
	p3 := strings.Index(md, "P-3: Fix crash")
	p2 := strings.Index(md, "P-2: Second bug")
	if p3 == -1 || p2 == -1 || p3 > p2 {
		t.Errorf("expected P-3 before P-2 within Bug group, got:\n%s", md)
	}
}

func TestGenerateNotes_UncategorizedFallback(t *testing.T) {
	issues := []jira.Issue{{Key: "P-1", Summary: "No type", IssueType: ""}}
	md := releases.GenerateNotes(issues, "v3.0.0")
	if !strings.Contains(md, "## Uncategorized") {
		t.Errorf("expected Uncategorized group for empty issue type, got:\n%s", md)
	}
}
