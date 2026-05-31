package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jinkp/atlassian-go-mcp/internal/api"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
)

// ReleasesHandler handles all /releases/* routes.
type ReleasesHandler struct {
	svc      releases.ReleasesService
	auditLog audit.Logger
}

// NewReleasesHandler constructs a ReleasesHandler.
func NewReleasesHandler(svc releases.ReleasesService, auditLog audit.Logger) *ReleasesHandler {
	return &ReleasesHandler{svc: svc, auditLog: auditLog}
}

// GetReleases handles GET /releases?project=...
func (h *ReleasesHandler) GetReleases(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	result, err := h.svc.GetReleases(r.Context(), project)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: result, Total: len(result)})
}

// GetRelease handles GET /releases/{releaseId}.
func (h *ReleasesHandler) GetRelease(w http.ResponseWriter, r *http.Request) {
	releaseID := r.PathValue("releaseId")
	release, err := h.svc.GetRelease(r.Context(), releaseID)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, release)
}

// GetReleaseIssues handles GET /releases/{releaseId}/issues.
func (h *ReleasesHandler) GetReleaseIssues(w http.ResponseWriter, r *http.Request) {
	releaseID := r.PathValue("releaseId")
	counts, err := h.svc.GetReleaseIssueCounts(r.Context(), releaseID)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, counts)
}

// createReleaseBody is the JSON request body for CreateRelease.
type createReleaseBody struct {
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	StartDate   string `json:"start_date"`
	ReleaseDate string `json:"release_date"`
}

// CreateRelease handles POST /releases.
func (h *ReleasesHandler) CreateRelease(w http.ResponseWriter, r *http.Request) {
	var body createReleaseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.Name == "" {
		api.RespondError(w, http.StatusBadRequest, "name is required", api.ErrCodeBadRequest)
		return
	}

	req := releases.CreateReleaseRequest{
		ProjectID:   body.ProjectID,
		Name:        body.Name,
		Description: body.Description,
		StartDate:   body.StartDate,
		ReleaseDate: body.ReleaseDate,
	}

	release, err := h.svc.CreateRelease(r.Context(), req)
	h.auditLog.Log(audit.NewEntry("create_release", "releases", map[string]any{"name": body.Name, "project_id": body.ProjectID}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusCreated, release)
}

// updateReleaseBody is the JSON request body for UpdateRelease.
type updateReleaseBody struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Released    *bool   `json:"released"`
	Archived    *bool   `json:"archived"`
	ReleaseDate *string `json:"release_date"`
}

// UpdateRelease handles PUT /releases/{releaseId}.
func (h *ReleasesHandler) UpdateRelease(w http.ResponseWriter, r *http.Request) {
	releaseID := r.PathValue("releaseId")

	var body updateReleaseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}

	req := releases.UpdateReleaseRequest{
		Name:        body.Name,
		Description: body.Description,
		Released:    body.Released,
		Archived:    body.Archived,
		ReleaseDate: body.ReleaseDate,
	}

	release, err := h.svc.UpdateRelease(r.Context(), releaseID, req)
	h.auditLog.Log(audit.NewEntry("update_release", "releases", map[string]any{"release_id": releaseID}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, release)
}
