package releases

import (
	"fmt"
	"sort"
	"strings"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// GenerateNotes produces Markdown release notes grouped by issue type.
// Group headings are sorted alphabetically for deterministic output; issues
// within a group preserve their input order.
func GenerateNotes(issues []jira.Issue, releaseName string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Release Notes: %s\n", releaseName))

	if len(issues) == 0 {
		sb.WriteString("\n_No issues linked to this release._\n")
		return sb.String()
	}

	groups := make(map[string][]jira.Issue)
	order := []string{}
	seen := make(map[string]bool)

	for _, iss := range issues {
		t := iss.IssueType
		if t == "" {
			t = "Uncategorized"
		}
		if !seen[t] {
			seen[t] = true
			order = append(order, t)
		}
		groups[t] = append(groups[t], iss)
	}

	sort.Strings(order)

	for _, typeName := range order {
		sb.WriteString(fmt.Sprintf("\n## %s\n\n", typeName))
		for _, iss := range groups[typeName] {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", iss.Key, iss.Summary))
		}
	}

	return sb.String()
}
