// Package handlers provides HTTP handler types for each Atlassian service domain.
// Each handler type holds its service dependency and audit logger (where applicable).
package handlers

import (
	"net/http"

	"github.com/jinkp/atlassian-go-mcp/internal/api"
)

// HealthHandler serves GET /health.
type HealthHandler struct{}

// NewHealthHandler returns a new HealthHandler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// ServeHTTP handles GET /health — returns {"status":"ok"} with HTTP 200.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
