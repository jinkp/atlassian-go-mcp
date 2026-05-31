package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jinkp/atlassian-go-mcp/internal/api"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
)

// ProjectsHandler handles all /projects/* routes.
type ProjectsHandler struct {
	svc      projects.ProjectsService
	auditLog audit.Logger
}

// NewProjectsHandler constructs a ProjectsHandler.
func NewProjectsHandler(svc projects.ProjectsService, auditLog audit.Logger) *ProjectsHandler {
	return &ProjectsHandler{svc: svc, auditLog: auditLog}
}

// SearchProjects handles GET /projects?query=...&maxResults=50.
func (h *ProjectsHandler) SearchProjects(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	maxResults := 50
	if mr := r.URL.Query().Get("maxResults"); mr != "" {
		if v, err := strconv.Atoi(mr); err == nil && v > 0 {
			maxResults = v
		}
	}

	req := projects.SearchProjectsRequest{
		Query:      query,
		MaxResults: maxResults,
	}

	result, err := h.svc.SearchProjects(r.Context(), req)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: result.Projects, Total: result.Total})
}

// GetProject handles GET /projects/{key}.
func (h *ProjectsHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	project, err := h.svc.GetProject(r.Context(), key)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, project)
}

// updateProjectBody is the JSON request body for UpdateProject.
type updateProjectBody struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Lead        *string `json:"lead"`
}

// UpdateProject handles PUT /projects/{key}.
func (h *ProjectsHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	var body updateProjectBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}

	req := projects.UpdateProjectRequest{
		Name:        body.Name,
		Description: body.Description,
		Lead:        body.Lead,
	}

	project, err := h.svc.UpdateProject(r.Context(), key, req)
	h.auditLog.Log(audit.NewEntry("update_project", "projects", map[string]any{"key": key}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, project)
}
