// Package api provides the HTTP REST server for the Atlassian Go MCP project.
// It exposes all service operations as JSON REST endpoints using net/http stdlib only.
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// Error code constants returned in JSON error envelopes.
const (
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeBadRequest    = "BAD_REQUEST"
	ErrCodeWriteDisabled = "WRITE_DISABLED"
	ErrCodeInternal      = "INTERNAL_ERROR"
	ErrCodeRateLimited   = "RATE_LIMITED"
	ErrCodeConflict      = "CONFLICT"
)

// errorResponse is the canonical JSON shape for all error responses.
type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// ListResponse wraps paginated collections for JSON responses.
// Exported so handler sub-packages can use it.
type ListResponse struct {
	Items any `json:"items"`
	Total int `json:"total"`
}

// listResponse is the unexported alias used in internal tests.
type listResponse = ListResponse

// RespondJSON writes v as JSON with the given status code and Content-Type: application/json.
func RespondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// RespondError writes an error envelope JSON response.
func RespondError(w http.ResponseWriter, status int, msg, code string) {
	RespondJSON(w, status, errorResponse{Error: msg, Code: code})
}

// ErrToStatus maps sentinel errors to HTTP status codes and error codes.
func ErrToStatus(err error) (int, string) {
	switch {
	case errors.Is(err, jira.ErrNotFound):
		return http.StatusNotFound, ErrCodeNotFound
	case errors.Is(err, jira.ErrUnauthorized):
		return http.StatusUnauthorized, ErrCodeUnauthorized
	case errors.Is(err, jira.ErrRateLimit):
		return http.StatusTooManyRequests, ErrCodeRateLimited
	case errors.Is(err, jira.ErrInvalidJQL):
		return http.StatusBadRequest, ErrCodeBadRequest
	case errors.Is(err, jira.ErrConflict):
		return http.StatusConflict, ErrCodeConflict
	// Bitbucket sentinels (separate error set, same HTTP semantics).
	case errors.Is(err, bitbucket.ErrNotFound):
		return http.StatusNotFound, ErrCodeNotFound
	case errors.Is(err, bitbucket.ErrUnauthorized):
		return http.StatusUnauthorized, ErrCodeUnauthorized
	case errors.Is(err, bitbucket.ErrRateLimit):
		return http.StatusTooManyRequests, ErrCodeRateLimited
	// Confluence sentinels (separate error set, same HTTP semantics).
	case errors.Is(err, confluence.ErrNotFound):
		return http.StatusNotFound, ErrCodeNotFound
	case errors.Is(err, confluence.ErrUnauthorized):
		return http.StatusUnauthorized, ErrCodeUnauthorized
	case errors.Is(err, confluence.ErrRateLimit):
		return http.StatusTooManyRequests, ErrCodeRateLimited
	case errors.Is(err, confluence.ErrConflict):
		return http.StatusConflict, ErrCodeConflict
	default:
		return http.StatusInternalServerError, ErrCodeInternal
	}
}
