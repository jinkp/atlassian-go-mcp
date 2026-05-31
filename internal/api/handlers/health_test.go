package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// compile-time check: HealthHandler satisfies http.Handler
var _ http.Handler = (*HealthHandler)(nil)

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		wantStatus int
		wantBody   map[string]string
	}{
		{
			name:       "GET /health returns 200 with status ok",
			method:     http.MethodGet,
			wantStatus: 200,
			wantBody:   map[string]string{"status": "ok"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHealthHandler()
			req := httptest.NewRequest(tc.method, "/health", nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tc.wantStatus)
			}
			ct := w.Header().Get("Content-Type")
			if !strings.Contains(ct, "application/json") {
				t.Errorf("Content-Type: got %q, want application/json", ct)
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("cannot unmarshal body: %v", err)
			}
			for k, v := range tc.wantBody {
				if body[k] != v {
					t.Errorf("body[%q]: got %q, want %q", k, body[k], v)
				}
			}
		})
	}
}
