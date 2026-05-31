package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

func TestRespondJSON(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        any
		wantStatus  int
		wantContain string
	}{
		{
			name:        "200 with struct body",
			status:      http.StatusOK,
			body:        map[string]string{"key": "PROJ-1"},
			wantStatus:  200,
			wantContain: `"key":"PROJ-1"`,
		},
		{
			name:        "201 with struct body",
			status:      http.StatusCreated,
			body:        map[string]string{"id": "10001"},
			wantStatus:  201,
			wantContain: `"id":"10001"`,
		},
		{
			name:        "sets application/json content-type",
			status:      http.StatusOK,
			body:        map[string]string{"status": "ok"},
			wantStatus:  200,
			wantContain: "ok",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			RespondJSON(w, tc.status, tc.body)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tc.wantStatus)
			}
			ct := w.Header().Get("Content-Type")
			if !strings.Contains(ct, "application/json") {
				t.Errorf("Content-Type: got %q, want application/json", ct)
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

func TestRespondError(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		msg        string
		code       string
		wantStatus int
		wantCode   string
	}{
		{"not found", 404, "not found", ErrCodeNotFound, 404, ErrCodeNotFound},
		{"unauthorized", 401, "unauthorized", ErrCodeUnauthorized, 401, ErrCodeUnauthorized},
		{"internal", 500, "internal error", ErrCodeInternal, 500, ErrCodeInternal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			RespondError(w, tc.status, tc.msg, tc.code)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tc.wantStatus)
			}
			var resp errorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("cannot unmarshal error response: %v", err)
			}
			if resp.Code != tc.wantCode {
				t.Errorf("code: got %q, want %q", resp.Code, tc.wantCode)
			}
			if resp.Error == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

func TestErrToStatusPublic(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"ErrNotFound", jira.ErrNotFound, 404, ErrCodeNotFound},
		{"ErrUnauthorized", jira.ErrUnauthorized, 401, ErrCodeUnauthorized},
		{"ErrRateLimit", jira.ErrRateLimit, 429, ErrCodeRateLimited},
		{"ErrInvalidJQL", jira.ErrInvalidJQL, 400, ErrCodeBadRequest},
		{"ErrConflict", jira.ErrConflict, 409, ErrCodeConflict},
		{"unknown error", errors.New("something else"), 500, ErrCodeInternal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, code := ErrToStatus(tc.err)
			if status != tc.wantStatus {
				t.Errorf("status: got %d, want %d", status, tc.wantStatus)
			}
			if code != tc.wantCode {
				t.Errorf("code: got %q, want %q", code, tc.wantCode)
			}
		})
	}
}

func TestListResponse(t *testing.T) {
	t.Run("list response has items and total", func(t *testing.T) {
		w := httptest.NewRecorder()
		items := []string{"a", "b", "c"}
		RespondJSON(w, 200, listResponse{Items: items, Total: len(items)})

		var result listResponse
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("cannot unmarshal: %v", err)
		}
		if result.Total != 3 {
			t.Errorf("total: got %d, want 3", result.Total)
		}
	})
}
