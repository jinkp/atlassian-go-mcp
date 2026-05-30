package output_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"gopkg.in/yaml.v3"
)

// sampleIssue returns a deterministic Issue for formatting tests.
func sampleIssue() *jira.Issue {
	created, _ := time.Parse(time.RFC3339, "2024-01-15T10:00:00Z")
	updated, _ := time.Parse(time.RFC3339, "2024-01-16T12:00:00Z")
	return &jira.Issue{
		Key:      "PROJ-1",
		Summary:  "Fix login bug",
		Status:   "In Progress",
		Assignee: "Alice",
		Priority: "High",
		Labels:   []string{"backend", "auth"},
		Created:  created,
		Updated:  updated,
	}
}

// --- NewFormatter factory tests ---

func TestNewFormatter_ValidFormats(t *testing.T) {
	tests := []struct {
		format string
	}{
		{"json"},
		{"yaml"},
		{"table"},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			f, err := output.NewFormatter(tt.format)
			if err != nil {
				t.Errorf("NewFormatter(%q) unexpected error: %v", tt.format, err)
			}
			if f == nil {
				t.Errorf("NewFormatter(%q) returned nil formatter", tt.format)
			}
		})
	}
}

func TestNewFormatter_UnknownFormat(t *testing.T) {
	_, err := output.NewFormatter("xml")
	if err == nil {
		t.Fatal("expected error for unknown format 'xml', got nil")
	}
	// Error message should mention the format
	if !strings.Contains(err.Error(), "xml") {
		t.Errorf("error should mention 'xml', got: %v", err)
	}
}

func TestNewFormatter_UnknownFormat_CSV(t *testing.T) {
	// Triangulate: another unknown format also errors
	_, err := output.NewFormatter("csv")
	if err == nil {
		t.Fatal("expected error for unknown format 'csv', got nil")
	}
}

// --- JSONFormatter tests ---

func TestJSONFormatter_FormatIssue(t *testing.T) {
	f, err := output.NewFormatter("json")
	if err != nil {
		t.Fatalf("NewFormatter(json) error: %v", err)
	}

	issue := sampleIssue()
	data, err := f.Format(issue)
	if err != nil {
		t.Fatalf("Format() unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Format() returned empty bytes")
	}

	// Must be valid JSON
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nGot: %s", err, string(data))
	}

	// Must contain the issue key
	if decoded["Key"] != "PROJ-1" {
		t.Errorf("expected Key=PROJ-1 in JSON, got: %v", decoded["Key"])
	}
}

func TestJSONFormatter_FormatIssueSlice(t *testing.T) {
	// Triangulate: slice of issues also produces valid JSON
	f, err := output.NewFormatter("json")
	if err != nil {
		t.Fatalf("NewFormatter(json) error: %v", err)
	}

	issues := []jira.Issue{*sampleIssue(), *sampleIssue()}
	issues[1].Key = "PROJ-2"

	data, err := f.Format(issues)
	if err != nil {
		t.Fatalf("Format([]Issue) unexpected error: %v", err)
	}

	var decoded []map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not valid JSON array: %v\nGot: %s", err, string(data))
	}
	if len(decoded) != 2 {
		t.Errorf("expected 2 items in JSON array, got %d", len(decoded))
	}
}

// --- YAMLFormatter tests ---

func TestYAMLFormatter_FormatIssue(t *testing.T) {
	f, err := output.NewFormatter("yaml")
	if err != nil {
		t.Fatalf("NewFormatter(yaml) error: %v", err)
	}

	issue := sampleIssue()
	data, err := f.Format(issue)
	if err != nil {
		t.Fatalf("Format() unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Format() returned empty bytes")
	}

	// Must be valid YAML
	var decoded map[string]interface{}
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not valid YAML: %v\nGot: %s", err, string(data))
	}

	// YAML key names are lowercase by default
	if decoded["key"] != "PROJ-1" {
		t.Errorf("expected key=PROJ-1 in YAML, got: %v", decoded["key"])
	}
}

func TestYAMLFormatter_FormatIssueSlice(t *testing.T) {
	// Triangulate: slice of issues also produces valid YAML
	f, err := output.NewFormatter("yaml")
	if err != nil {
		t.Fatalf("NewFormatter(yaml) error: %v", err)
	}

	issues := []jira.Issue{*sampleIssue(), *sampleIssue()}
	issues[1].Key = "PROJ-2"

	data, err := f.Format(issues)
	if err != nil {
		t.Fatalf("Format([]Issue) unexpected error: %v", err)
	}

	var decoded []map[string]interface{}
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not valid YAML array: %v\nGot: %s", err, string(data))
	}
	if len(decoded) != 2 {
		t.Errorf("expected 2 items in YAML sequence, got %d", len(decoded))
	}
}

// --- TableFormatter tests ---

func TestTableFormatter_FormatIssue(t *testing.T) {
	f, err := output.NewFormatter("table")
	if err != nil {
		t.Fatalf("NewFormatter(table) error: %v", err)
	}

	issue := sampleIssue()
	data, err := f.Format(issue)
	if err != nil {
		t.Fatalf("Format() unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Format() returned empty bytes")
	}

	output := string(data)

	// Table must contain the key and summary
	if !strings.Contains(output, "PROJ-1") {
		t.Errorf("table output missing 'PROJ-1'\nGot:\n%s", output)
	}
	if !strings.Contains(output, "Fix login bug") {
		t.Errorf("table output missing summary\nGot:\n%s", output)
	}
	if !strings.Contains(output, "In Progress") {
		t.Errorf("table output missing status\nGot:\n%s", output)
	}
}

func TestTableFormatter_FormatIssueSlice(t *testing.T) {
	// Triangulate: multiple issues render multiple rows
	f, err := output.NewFormatter("table")
	if err != nil {
		t.Fatalf("NewFormatter(table) error: %v", err)
	}

	issues := []jira.Issue{*sampleIssue(), *sampleIssue()}
	issues[1].Key = "PROJ-2"
	issues[1].Summary = "Add dark mode"

	data, err := f.Format(issues)
	if err != nil {
		t.Fatalf("Format([]Issue) unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("table missing PROJ-1\nGot:\n%s", out)
	}
	if !strings.Contains(out, "PROJ-2") {
		t.Errorf("table missing PROJ-2\nGot:\n%s", out)
	}
	if !strings.Contains(out, "Add dark mode") {
		t.Errorf("table missing PROJ-2 summary\nGot:\n%s", out)
	}
}
