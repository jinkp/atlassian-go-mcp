package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jinkp/atlassian-go-mcp/internal/api"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// AgileHandler handles all /agile/* routes.
type AgileHandler struct {
	svc      agile.AgileService
	auditLog audit.Logger
}

// NewAgileHandler constructs an AgileHandler.
func NewAgileHandler(svc agile.AgileService, auditLog audit.Logger) *AgileHandler {
	return &AgileHandler{svc: svc, auditLog: auditLog}
}

// GetBoards handles GET /agile/boards?project=...
func (h *AgileHandler) GetBoards(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	maxResults := 50
	if mr := r.URL.Query().Get("maxResults"); mr != "" {
		if v, err := strconv.Atoi(mr); err == nil && v > 0 {
			maxResults = v
		}
	}

	boards, err := h.svc.GetBoards(r.Context(), project, maxResults)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: boards, Total: len(boards)})
}

// GetSprints handles GET /agile/boards/{boardId}/sprints?state=...
func (h *AgileHandler) GetSprints(w http.ResponseWriter, r *http.Request) {
	boardID, err := strconv.Atoi(r.PathValue("boardId"))
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid boardId", api.ErrCodeBadRequest)
		return
	}
	state := r.URL.Query().Get("state")
	maxResults := 50

	sprints, err := h.svc.GetSprints(r.Context(), boardID, state, maxResults)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: sprints, Total: len(sprints)})
}

// GetActiveSprint handles GET /agile/boards/{boardId}/sprints/active.
func (h *AgileHandler) GetActiveSprint(w http.ResponseWriter, r *http.Request) {
	boardID, err := strconv.Atoi(r.PathValue("boardId"))
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid boardId", api.ErrCodeBadRequest)
		return
	}

	sprints, err := h.svc.GetSprints(r.Context(), boardID, "active", 1)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	if len(sprints) == 0 {
		api.RespondError(w, http.StatusNotFound, "no active sprint found", api.ErrCodeNotFound)
		return
	}
	api.RespondJSON(w, http.StatusOK, sprints[0])
}

// GetSprintIssues handles GET /agile/sprints/{sprintId}/issues.
func (h *AgileHandler) GetSprintIssues(w http.ResponseWriter, r *http.Request) {
	sprintID, err := strconv.Atoi(r.PathValue("sprintId"))
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid sprintId", api.ErrCodeBadRequest)
		return
	}
	maxResults := 50

	result, err := h.svc.GetSprintIssues(r.Context(), sprintID, maxResults)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: result.Issues, Total: result.Total})
}

// createSprintBody is the JSON request body for CreateSprint.
type createSprintBody struct {
	Name      string `json:"name"`
	BoardID   int    `json:"board_id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// CreateSprint handles POST /agile/sprints.
func (h *AgileHandler) CreateSprint(w http.ResponseWriter, r *http.Request) {
	var body createSprintBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.Name == "" {
		api.RespondError(w, http.StatusBadRequest, "name is required", api.ErrCodeBadRequest)
		return
	}

	req := agile.CreateSprintRequest{
		Name:      body.Name,
		BoardID:   body.BoardID,
		StartDate: body.StartDate,
		EndDate:   body.EndDate,
	}

	sprint, err := h.svc.CreateSprint(r.Context(), req)
	h.auditLog.Log(audit.NewEntry("create_sprint", "agile", map[string]any{"name": body.Name, "board_id": body.BoardID}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusCreated, sprint)
}

// updateSprintBody is the JSON request body for UpdateSprint.
type updateSprintBody struct {
	Name      *string `json:"name"`
	State     *string `json:"state"`
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
}

// UpdateSprint handles PUT /agile/sprints/{sprintId}.
func (h *AgileHandler) UpdateSprint(w http.ResponseWriter, r *http.Request) {
	sprintID, err := strconv.Atoi(r.PathValue("sprintId"))
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid sprintId", api.ErrCodeBadRequest)
		return
	}

	var body updateSprintBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}

	req := agile.UpdateSprintRequest{
		Name:      body.Name,
		State:     body.State,
		StartDate: body.StartDate,
		EndDate:   body.EndDate,
	}

	sprint, err := h.svc.UpdateSprint(r.Context(), sprintID, req)
	h.auditLog.Log(audit.NewEntry("update_sprint", "agile", map[string]any{"sprint_id": sprintID}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, sprint)
}

// moveIssuesToSprintBody is the JSON request body for MoveIssuesToSprint.
type moveIssuesToSprintBody struct {
	IssueKeys []string `json:"issue_keys"`
}

// MoveIssuesToSprint handles POST /agile/sprints/{sprintId}/issues.
func (h *AgileHandler) MoveIssuesToSprint(w http.ResponseWriter, r *http.Request) {
	sprintID, err := strconv.Atoi(r.PathValue("sprintId"))
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid sprintId", api.ErrCodeBadRequest)
		return
	}

	var body moveIssuesToSprintBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}

	err = h.svc.MoveIssuesToSprint(r.Context(), sprintID, body.IssueKeys)
	h.auditLog.Log(audit.NewEntry("move_issues_to_sprint", "agile", map[string]any{"sprint_id": sprintID, "count": len(body.IssueKeys)}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		// agile errors may not be jira sentinels — map ErrNotFound explicitly
		if err == jira.ErrNotFound {
			api.RespondError(w, http.StatusNotFound, err.Error(), api.ErrCodeNotFound)
			return
		}
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, map[string]string{"status": "moved"})
}
