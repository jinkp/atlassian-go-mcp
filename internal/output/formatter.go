// Package output provides format-agnostic output for CLI commands.
// Callers select a Formatter via NewFormatter and then call Format(v) — they
// never need to know whether the output is JSON, YAML, or a table.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"gopkg.in/yaml.v3"
)

// Formatter serialises any value to bytes in a specific human or machine format.
type Formatter interface {
	Format(v interface{}) ([]byte, error)
}

// NewFormatter returns the Formatter matching format ("json", "yaml", "table").
// Returns an error for any unknown format string.
func NewFormatter(format string) (Formatter, error) {
	switch strings.ToLower(format) {
	case "json":
		return &JSONFormatter{}, nil
	case "yaml":
		return &YAMLFormatter{}, nil
	case "table":
		return &TableFormatter{}, nil
	default:
		return nil, fmt.Errorf("unknown output format %q: must be json, yaml, or table", format)
	}
}

// JSONFormatter serialises values as indented JSON.
type JSONFormatter struct{}

// Format marshals v to indented JSON bytes.
func (f *JSONFormatter) Format(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// YAMLFormatter serialises values as YAML.
type YAMLFormatter struct{}

// Format marshals v to YAML bytes using gopkg.in/yaml.v3.
func (f *YAMLFormatter) Format(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

// TableFormatter renders values as human-readable tab-separated tables using
// text/tabwriter for aligned columns.
type TableFormatter struct{}

// Format renders v as a plain-text table.
// Supports jira.Issue, []jira.Issue, jira.SearchResult,
// agile.Board, agile.Sprint, agile.SprintIssueResult,
// goals.Goal, goals.GoalSearchResult, and goals.CreateGoalResult.
func (f *TableFormatter) Format(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	switch val := v.(type) {
	// --- jira ---
	case *jira.Issue:
		f.writeIssueTable(w, *val)
	case jira.Issue:
		f.writeIssueTable(w, val)
	case []jira.Issue:
		f.writeIssueHeader(w)
		for _, issue := range val {
			f.writeIssueRow(w, issue)
		}
	case *jira.SearchResult:
		f.writeIssueHeader(w)
		for _, issue := range val.Issues {
			f.writeIssueRow(w, issue)
		}
	// --- agile ---
	case []agile.Board:
		f.writeBoardHeader(w)
		for _, b := range val {
			f.writeBoardRow(w, b)
		}
	case []agile.Sprint:
		f.writeSprintHeader(w)
		for _, s := range val {
			f.writeSprintRow(w, s)
		}
	case *agile.SprintIssueResult:
		f.writeSprintIssueHeader(w)
		for _, i := range val.Issues {
			f.writeSprintIssueRow(w, i)
		}
	case []agile.SprintIssue:
		f.writeSprintIssueHeader(w)
		for _, i := range val {
			f.writeSprintIssueRow(w, i)
		}
	// --- goals ---
	case *goals.Goal:
		f.writeGoalTable(w, *val)
	case goals.Goal:
		f.writeGoalTable(w, val)
	case []goals.Goal:
		f.writeGoalHeader(w)
		for _, g := range val {
			f.writeGoalRow(w, g)
		}
	case *goals.GoalSearchResult:
		f.writeGoalHeader(w)
		for _, g := range val.Goals {
			f.writeGoalRow(w, g)
		}
	default:
		// Fallback: JSON-encode unknown types
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("table formatter: unsupported type %T: %w", v, err)
		}
		return data, nil
	}

	w.Flush()
	return buf.Bytes(), nil
}

func (f *TableFormatter) writeIssueTable(w *tabwriter.Writer, issue jira.Issue) {
	fmt.Fprintf(w, "KEY\t%s\n", issue.Key)
	fmt.Fprintf(w, "SUMMARY\t%s\n", issue.Summary)
	fmt.Fprintf(w, "STATUS\t%s\n", issue.Status)
	fmt.Fprintf(w, "ASSIGNEE\t%s\n", issue.Assignee)
	fmt.Fprintf(w, "PRIORITY\t%s\n", issue.Priority)
	fmt.Fprintf(w, "LABELS\t%s\n", strings.Join(issue.Labels, ", "))
	fmt.Fprintf(w, "CREATED\t%s\n", issue.Created.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(w, "UPDATED\t%s\n", issue.Updated.Format("2006-01-02 15:04:05 UTC"))
}

func (f *TableFormatter) writeIssueHeader(w *tabwriter.Writer) {
	fmt.Fprintf(w, "KEY\tSUMMARY\tSTATUS\tASSIGNEE\tPRIORITY\n")
	fmt.Fprintf(w, "---\t-------\t------\t--------\t--------\n")
}

func (f *TableFormatter) writeIssueRow(w *tabwriter.Writer, issue jira.Issue) {
	summary := issue.Summary
	if len(summary) > 60 {
		summary = summary[:57] + "..."
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		issue.Key, summary, issue.Status, issue.Assignee, issue.Priority)
}

// --- agile table helpers ---

func (f *TableFormatter) writeBoardHeader(w *tabwriter.Writer) {
	fmt.Fprintf(w, "ID\tNAME\tTYPE\n")
	fmt.Fprintf(w, "--\t----\t----\n")
}

func (f *TableFormatter) writeBoardRow(w *tabwriter.Writer, b agile.Board) {
	fmt.Fprintf(w, "%d\t%s\t%s\n", b.ID, b.Name, b.Type)
}

func (f *TableFormatter) writeSprintHeader(w *tabwriter.Writer) {
	fmt.Fprintf(w, "ID\tNAME\tSTATE\tSTART\tEND\n")
	fmt.Fprintf(w, "--\t----\t-----\t-----\t---\n")
}

func (f *TableFormatter) writeSprintRow(w *tabwriter.Writer, s agile.Sprint) {
	fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", s.ID, s.Name, s.State, s.StartDate, s.EndDate)
}

func (f *TableFormatter) writeSprintIssueHeader(w *tabwriter.Writer) {
	fmt.Fprintf(w, "KEY\tSUMMARY\tSTATUS\tASSIGNEE\n")
	fmt.Fprintf(w, "---\t-------\t------\t--------\n")
}

func (f *TableFormatter) writeSprintIssueRow(w *tabwriter.Writer, i agile.SprintIssue) {
	summary := i.Summary
	if len(summary) > 60 {
		summary = summary[:57] + "..."
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", i.Key, summary, i.Status, i.Assignee)
}

// --- goals table helpers ---

func (f *TableFormatter) writeGoalTable(w *tabwriter.Writer, g goals.Goal) {
	fmt.Fprintf(w, "ID\t%s\n", g.ID)
	fmt.Fprintf(w, "NAME\t%s\n", g.Name)
	fmt.Fprintf(w, "STATUS\t%s\n", g.Status)
	fmt.Fprintf(w, "PHASE\t%s\n", g.Phase)
	fmt.Fprintf(w, "SCORE\t%d\n", g.Score)
	fmt.Fprintf(w, "TARGET_DATE\t%s\n", g.TargetDate)
	fmt.Fprintf(w, "OWNER\t%s\n", g.OwnerName)
}

func (f *TableFormatter) writeGoalHeader(w *tabwriter.Writer) {
	fmt.Fprintf(w, "NAME\tSTATUS\tPHASE\tSCORE\tTARGET_DATE\n")
	fmt.Fprintf(w, "----\t------\t-----\t-----\t-----------\n")
}

func (f *TableFormatter) writeGoalRow(w *tabwriter.Writer, g goals.Goal) {
	name := g.Name
	if len(name) > 50 {
		name = name[:47] + "..."
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", name, g.Status, g.Phase, g.Score, g.TargetDate)
}
