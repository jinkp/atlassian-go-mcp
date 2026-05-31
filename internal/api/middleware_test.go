package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler is a simple handler that always returns 200 with {"status":"ok"}.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
})

func TestWriteGuardMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		header      string // value for X-Enable-Write, empty = not set
		readOnly    bool
		wantStatus  int
		wantBlocked bool
	}{
		{
			name:        "GET always passes (no header needed)",
			method:      http.MethodGet,
			header:      "",
			readOnly:    false,
			wantStatus:  200,
			wantBlocked: false,
		},
		{
			name:        "GET passes even when read-only",
			method:      http.MethodGet,
			header:      "",
			readOnly:    true,
			wantStatus:  200,
			wantBlocked: false,
		},
		{
			name:        "POST blocked without header",
			method:      http.MethodPost,
			header:      "",
			readOnly:    false,
			wantStatus:  403,
			wantBlocked: true,
		},
		{
			name:        "POST allowed with X-Enable-Write: true",
			method:      http.MethodPost,
			header:      "true",
			readOnly:    false,
			wantStatus:  200,
			wantBlocked: false,
		},
		{
			name:        "PUT blocked without header",
			method:      http.MethodPut,
			header:      "",
			readOnly:    false,
			wantStatus:  403,
			wantBlocked: true,
		},
		{
			name:        "PUT allowed with X-Enable-Write: true",
			method:      http.MethodPut,
			header:      "true",
			readOnly:    false,
			wantStatus:  200,
			wantBlocked: false,
		},
		{
			name:        "POST blocked when read-only=true even with header",
			method:      http.MethodPost,
			header:      "true",
			readOnly:    true,
			wantStatus:  403,
			wantBlocked: true,
		},
		{
			name:        "POST blocked with wrong header value",
			method:      http.MethodPost,
			header:      "1",
			readOnly:    false,
			wantStatus:  403,
			wantBlocked: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := WriteGuardMiddleware(tc.readOnly, okHandler)

			req := httptest.NewRequest(tc.method, "/test", nil)
			if tc.header != "" {
				req.Header.Set("X-Enable-Write", tc.header)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tc.wantStatus)
			}
			if tc.wantBlocked {
				var resp errorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("cannot unmarshal blocked response: %v", err)
				}
				if resp.Code != ErrCodeWriteDisabled {
					t.Errorf("code: got %q, want %q", resp.Code, ErrCodeWriteDisabled)
				}
			}
		})
	}
}

func TestRecoverMiddleware(t *testing.T) {
	t.Run("panic returns 500 with INTERNAL_ERROR code", func(t *testing.T) {
		panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("something went wrong")
		})

		handler := RecoverMiddleware(panicHandler)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status: got %d, want 500", w.Code)
		}
		var resp errorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("cannot unmarshal panic response: %v", err)
		}
		if resp.Code != ErrCodeInternal {
			t.Errorf("code: got %q, want %q", resp.Code, ErrCodeInternal)
		}
	})

	t.Run("non-panic handler passes through", func(t *testing.T) {
		handler := RecoverMiddleware(okHandler)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status: got %d, want 200", w.Code)
		}
	})
}

func TestLoggingMiddleware(t *testing.T) {
	t.Run("passes request to next handler", func(t *testing.T) {
		handler := LoggingMiddleware(okHandler)
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status: got %d, want 200", w.Code)
		}
	})

	t.Run("preserves response body", func(t *testing.T) {
		handler := LoggingMiddleware(okHandler)
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		body := w.Body.String()
		if body == "" {
			t.Error("response body should not be empty")
		}
	})
}
