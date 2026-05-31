package api

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// RecoverMiddleware is the outermost middleware layer.
// It catches any panic from inner handlers and returns HTTP 500 with INTERNAL_ERROR code.
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("api: panic recovered: %v", rec)
				RespondError(w, http.StatusInternalServerError, fmt.Sprintf("%v", rec), ErrCodeInternal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code for logging.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// LoggingMiddleware logs the HTTP method, path, status code, and duration to stderr.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("api: %s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

// WriteGuardMiddleware blocks POST, PUT, and DELETE requests unless:
//   - readOnly is false AND the X-Enable-Write header is exactly "true"
//
// When blocked, it responds with 403 and WRITE_DISABLED code.
// GET and HEAD requests always pass through.
func WriteGuardMiddleware(readOnly bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
			if readOnly || r.Header.Get("X-Enable-Write") != "true" {
				RespondError(w, http.StatusForbidden, "write operations disabled: set X-Enable-Write: true header", ErrCodeWriteDisabled)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
