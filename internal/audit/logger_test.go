package audit_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
)

// --- TestJSONLogger ---

func TestJSONLogger(t *testing.T) {
	tests := []struct {
		name        string
		entry       audit.Entry
		wantOp      string
		wantResult  string
		wantService string
	}{
		{
			name: "success entry contains all required fields",
			entry: audit.Entry{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Operation: "create_jira_issue",
				Service:   "jira",
				Args:      map[string]any{"project_key": "PROJ"},
				Result:    "success",
				User:      "user@example.com",
			},
			wantOp:      "create_jira_issue",
			wantResult:  "success",
			wantService: "jira",
		},
		{
			name: "error entry result captured",
			entry: audit.Entry{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Operation: "update_jira_issue",
				Service:   "jira",
				Result:    "error:not found",
				User:      "user@example.com",
			},
			wantOp:      "update_jira_issue",
			wantResult:  "error:not found",
			wantService: "jira",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := audit.NewJSONLogger(&buf)
			logger.Log(tc.entry)

			line := buf.String()
			if !strings.HasSuffix(line, "\n") {
				t.Error("expected log line to end with newline")
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &decoded); err != nil {
				t.Fatalf("log output is not valid JSON: %v\nOutput: %s", err, line)
			}

			// Verify required fields present
			for _, key := range []string{"timestamp", "operation", "service", "result"} {
				if _, ok := decoded[key]; !ok {
					t.Errorf("missing key %q in JSON output", key)
				}
			}

			// Verify timestamp parseable as RFC3339
			ts, ok := decoded["timestamp"].(string)
			if !ok {
				t.Fatalf("timestamp is not a string")
			}
			if _, err := time.Parse(time.RFC3339, ts); err != nil {
				t.Errorf("timestamp %q is not RFC3339: %v", ts, err)
			}

			if got := decoded["operation"].(string); got != tc.wantOp {
				t.Errorf("operation: got %q, want %q", got, tc.wantOp)
			}
			if got := decoded["result"].(string); got != tc.wantResult {
				t.Errorf("result: got %q, want %q", got, tc.wantResult)
			}
			if got := decoded["service"].(string); got != tc.wantService {
				t.Errorf("service: got %q, want %q", got, tc.wantService)
			}
		})
	}
}

// --- TestNewEntry ---

func TestNewEntry_Success(t *testing.T) {
	t.Setenv("ATLASSIAN_EMAIL", "admin@corp.com")

	entry := audit.NewEntry("create_sprint", "agile", map[string]any{"name": "Sprint 1"}, nil)

	if entry.Result != "success" {
		t.Errorf("expected result='success', got %q", entry.Result)
	}
	if entry.User != "admin@corp.com" {
		t.Errorf("expected user='admin@corp.com', got %q", entry.User)
	}
	if entry.Operation != "create_sprint" {
		t.Errorf("expected operation='create_sprint', got %q", entry.Operation)
	}
	if entry.Service != "agile" {
		t.Errorf("expected service='agile', got %q", entry.Service)
	}
	if _, err := time.Parse(time.RFC3339, entry.Timestamp); err != nil {
		t.Errorf("timestamp %q not RFC3339: %v", entry.Timestamp, err)
	}
}

func TestNewEntry_Error(t *testing.T) {
	entry := audit.NewEntry("update_sprint", "agile", nil, fmt.Errorf("timeout"))

	if entry.Result != "error:timeout" {
		t.Errorf("expected result='error:timeout', got %q", entry.Result)
	}
}

// --- TestNoopLogger ---

func TestNoopLogger_NoPanic(t *testing.T) {
	logger := audit.NewNoopLogger()
	// Must not panic
	logger.Log(audit.Entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Operation: "test",
		Service:   "test",
		Result:    "success",
	})
}
