// Package audit provides structured audit logging for write operations.
// Writes JSON lines to an io.Writer (typically os.Stderr).
// The MCP stdout transport is never touched — all output goes to stderr.
package audit

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// Entry holds the structured fields written for each audited operation.
type Entry struct {
	Timestamp string         `json:"timestamp"`
	Operation string         `json:"operation"`
	Service   string         `json:"service"`
	Args      map[string]any `json:"args,omitempty"`
	Result    string         `json:"result"`
	User      string         `json:"user,omitempty"`
}

// Logger is the interface that audit backends must implement.
type Logger interface {
	Log(entry Entry)
}

// JSONLogger writes one JSON line per Log call to the wrapped io.Writer.
// It is safe to use from multiple goroutines only if w is goroutine-safe;
// os.Stderr satisfies this on all supported platforms.
type JSONLogger struct {
	w io.Writer
}

// NewJSONLogger returns a Logger that writes JSON lines to w.
func NewJSONLogger(w io.Writer) Logger {
	return &JSONLogger{w: w}
}

// Log marshals e to JSON and writes it as a single line (terminated by '\n').
// Marshal errors and write errors are silently discarded — audit is best-effort.
func (l *JSONLogger) Log(e Entry) {
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = l.w.Write(append(b, '\n'))
}

// NoopLogger discards all entries. Used in tests and as a safe default.
type NoopLogger struct{}

// NewNoopLogger returns a Logger that discards all entries without panic.
func NewNoopLogger() Logger {
	return &NoopLogger{}
}

// Log is a no-op.
func (n *NoopLogger) Log(_ Entry) {}

// NewEntry constructs an Entry for the given operation. If err is nil, Result is
// "success"; otherwise it is "error:<err.Error()>". The User field is read from
// the ATLASSIAN_EMAIL environment variable at call time.
func NewEntry(op, svc string, args map[string]any, err error) Entry {
	result := "success"
	if err != nil {
		result = "error:" + err.Error()
	}
	return Entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Operation: op,
		Service:   svc,
		Args:      args,
		Result:    result,
		User:      os.Getenv("ATLASSIAN_EMAIL"),
	}
}
