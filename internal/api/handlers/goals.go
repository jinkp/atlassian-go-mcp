package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jinkp/atlassian-go-mcp/internal/api"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
)

// GoalsHandler handles all /goals/* routes.
type GoalsHandler struct {
	svc      goals.GoalsService
	auditLog audit.Logger
}

// NewGoalsHandler constructs a GoalsHandler.
func NewGoalsHandler(svc goals.GoalsService, auditLog audit.Logger) *GoalsHandler {
	return &GoalsHandler{svc: svc, auditLog: auditLog}
}

// GetSiteID handles GET /goals/site-id?subdomain=...
func (h *GoalsHandler) GetSiteID(w http.ResponseWriter, r *http.Request) {
	subdomain := r.URL.Query().Get("subdomain")
	if subdomain == "" {
		api.RespondError(w, http.StatusBadRequest, "subdomain query param is required", api.ErrCodeBadRequest)
		return
	}

	siteID, err := h.svc.GetSiteID(r.Context(), subdomain)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, map[string]string{"site_id": siteID})
}

// SearchGoals handles GET /goals?siteId=...&query=...
func (h *GoalsHandler) SearchGoals(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("siteId")
	query := r.URL.Query().Get("query")

	req := goals.SearchGoalsRequest{
		SiteID:       siteID,
		SearchString: query,
	}

	result, err := h.svc.SearchGoals(r.Context(), req)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: result.Goals, Total: len(result.Goals)})
}

// GetGoal handles GET /goals/{goalId}.
func (h *GoalsHandler) GetGoal(w http.ResponseWriter, r *http.Request) {
	goalID := r.PathValue("goalId")
	goal, err := h.svc.GetGoal(r.Context(), goalID)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, goal)
}

// createGoalBody is the JSON request body for CreateGoal.
type createGoalBody struct {
	SiteID      string `json:"site_id"`
	Name        string `json:"name"`
	GoalTypeID  string `json:"goal_type_id"`
	TargetDate  string `json:"target_date"`
	Confidence  string `json:"confidence"`
	Description string `json:"description"`
}

// CreateGoal handles POST /goals.
func (h *GoalsHandler) CreateGoal(w http.ResponseWriter, r *http.Request) {
	var body createGoalBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.Name == "" {
		api.RespondError(w, http.StatusBadRequest, "name is required", api.ErrCodeBadRequest)
		return
	}

	req := goals.CreateGoalRequest{
		SiteID:      body.SiteID,
		Name:        body.Name,
		GoalTypeID:  body.GoalTypeID,
		TargetDate:  body.TargetDate,
		Confidence:  body.Confidence,
		Description: body.Description,
	}

	result, err := h.svc.CreateGoal(r.Context(), req)
	h.auditLog.Log(audit.NewEntry("create_goal", "goals", map[string]any{"name": body.Name, "site_id": body.SiteID}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusCreated, result)
}

// updateGoalStatusBody is the JSON request body for UpdateGoalStatus.
type updateGoalStatusBody struct {
	Status  string `json:"status"`
	Score   int    `json:"score"`
	Summary string `json:"summary"`
}

// UpdateGoalStatus handles PUT /goals/{goalId}/status.
func (h *GoalsHandler) UpdateGoalStatus(w http.ResponseWriter, r *http.Request) {
	goalID := r.PathValue("goalId")

	var body updateGoalStatusBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}

	req := goals.UpdateGoalStatusRequest{
		GoalID:  goalID,
		Status:  body.Status,
		Score:   body.Score,
		Summary: body.Summary,
	}

	err := h.svc.UpdateGoalStatus(r.Context(), req)
	h.auditLog.Log(audit.NewEntry("update_goal_status", "goals", map[string]any{"goal_id": goalID, "status": body.Status}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, map[string]string{"status": "updated", "goal_id": goalID})
}

// editGoalBody is the JSON request body for EditGoal.
type editGoalBody struct {
	Name       *string `json:"name"`
	TargetDate *string `json:"target_date"`
	Confidence *string `json:"confidence"`
	Archive    *bool   `json:"archive"`
}

// EditGoal handles PUT /goals/{goalId}.
func (h *GoalsHandler) EditGoal(w http.ResponseWriter, r *http.Request) {
	goalID := r.PathValue("goalId")

	var body editGoalBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}

	req := goals.EditGoalRequest{
		GoalID:     goalID,
		Name:       body.Name,
		TargetDate: body.TargetDate,
		Confidence: body.Confidence,
		Archive:    body.Archive,
	}

	goal, err := h.svc.EditGoal(r.Context(), req)
	h.auditLog.Log(audit.NewEntry("edit_goal", "goals", map[string]any{"goal_id": goalID}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, goal)
}
