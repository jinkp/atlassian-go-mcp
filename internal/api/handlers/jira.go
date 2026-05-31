package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jinkp/atlassian-go-mcp/internal/api"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// JiraHandler handles all /jira/* routes.
type JiraHandler struct {
	svc      jira.Service
	auditLog audit.Logger
}

// NewJiraHandler constructs a JiraHandler.
func NewJiraHandler(svc jira.Service, auditLog audit.Logger) *JiraHandler {
	return &JiraHandler{svc: svc, auditLog: auditLog}
}

// GetIssue handles GET /jira/issues/{key}.
func (h *JiraHandler) GetIssue(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	issue, err := h.svc.GetIssue(r.Context(), key)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, issue)
}

// SearchIssues handles GET /jira/issues?jql=...&maxResults=50.
func (h *JiraHandler) SearchIssues(w http.ResponseWriter, r *http.Request) {
	jql := r.URL.Query().Get("jql")
	maxResults := 50
	if mr := r.URL.Query().Get("maxResults"); mr != "" {
		if v, err := strconv.Atoi(mr); err == nil && v > 0 {
			maxResults = v
		}
	}

	result, err := h.svc.SearchIssues(r.Context(), jql, maxResults)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: result.Issues, Total: result.Total})
}

// createIssueBody is the JSON request body for CreateIssue.
type createIssueBody struct {
	ProjectKey  string   `json:"project_key"`
	IssueType   string   `json:"issue_type"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	AssigneeID  string   `json:"assignee_id"`
	Labels      []string `json:"labels"`
	Priority    string   `json:"priority"`
}

// CreateIssue handles POST /jira/issues.
func (h *JiraHandler) CreateIssue(w http.ResponseWriter, r *http.Request) {
	var body createIssueBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.ProjectKey == "" {
		api.RespondError(w, http.StatusBadRequest, "project_key is required", api.ErrCodeBadRequest)
		return
	}
	if body.Summary == "" {
		api.RespondError(w, http.StatusBadRequest, "summary is required", api.ErrCodeBadRequest)
		return
	}

	req := jira.CreateIssueRequest{
		ProjectKey:  body.ProjectKey,
		IssueType:   body.IssueType,
		Summary:     body.Summary,
		Description: body.Description,
		AssigneeID:  body.AssigneeID,
		Labels:      body.Labels,
		PriorityName: body.Priority,
	}

	result, err := h.svc.CreateIssue(r.Context(), req)
	h.auditLog.Log(audit.NewEntry("create_issue", "jira", map[string]any{"project_key": body.ProjectKey, "summary": body.Summary}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusCreated, result)
}

// updateIssueBody is the JSON request body for UpdateIssue.
type updateIssueBody struct {
	Summary     *string `json:"summary"`
	Description *string `json:"description"`
	AssigneeID  *string `json:"assignee_id"`
	Priority    *string `json:"priority"`
}

// UpdateIssue handles PUT /jira/issues/{key}.
func (h *JiraHandler) UpdateIssue(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	var body updateIssueBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}

	req := jira.UpdateIssueRequest{
		Summary:     body.Summary,
		Description: body.Description,
		AssigneeID:  body.AssigneeID,
		PriorityName: body.Priority,
	}

	err := h.svc.UpdateIssue(r.Context(), key, req)
	h.auditLog.Log(audit.NewEntry("update_issue", "jira", map[string]any{"key": key}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, map[string]string{"status": "updated", "key": key})
}

// GetTransitions handles GET /jira/issues/{key}/transitions.
func (h *JiraHandler) GetTransitions(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	transitions, err := h.svc.GetTransitions(r.Context(), key)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: transitions, Total: len(transitions)})
}

// transitionIssueBody is the JSON request body for TransitionIssue.
type transitionIssueBody struct {
	TransitionID string `json:"transition_id"`
}

// TransitionIssue handles POST /jira/issues/{key}/transitions.
func (h *JiraHandler) TransitionIssue(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	var body transitionIssueBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.TransitionID == "" {
		api.RespondError(w, http.StatusBadRequest, "transition_id is required", api.ErrCodeBadRequest)
		return
	}

	err := h.svc.TransitionIssue(r.Context(), key, body.TransitionID)
	h.auditLog.Log(audit.NewEntry("transition_issue", "jira", map[string]any{"key": key, "transition_id": body.TransitionID}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, map[string]string{"status": "transitioned", "key": key})
}
