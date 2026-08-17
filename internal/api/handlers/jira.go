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
		ProjectKey:   body.ProjectKey,
		IssueType:    body.IssueType,
		Summary:      body.Summary,
		Description:  body.Description,
		AssigneeID:   body.AssigneeID,
		Labels:       body.Labels,
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
		Summary:      body.Summary,
		Description:  body.Description,
		AssigneeID:   body.AssigneeID,
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

// --- Block 3: 7 new handlers ---

// SearchUsers handles GET /jira/users/search?query=&maxResults=.
func (h *JiraHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		api.RespondError(w, http.StatusBadRequest, "query is required", api.ErrCodeBadRequest)
		return
	}
	maxResults := 10
	if mr := r.URL.Query().Get("maxResults"); mr != "" {
		if v, err := strconv.Atoi(mr); err == nil && v > 0 {
			maxResults = v
		}
	}
	users, err := h.svc.LookupAccountID(r.Context(), query, maxResults)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, users)
}

// addCommentBody is the JSON request body for AddComment.
type addCommentBody struct {
	Body string `json:"body"`
}

// AddComment handles POST /jira/issues/{key}/comments.
// Write-guarded by WriteGuardMiddleware (X-Enable-Write: true required).
func (h *JiraHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	var body addCommentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.Body == "" {
		api.RespondError(w, http.StatusBadRequest, "body is required", api.ErrCodeBadRequest)
		return
	}

	comment, err := h.svc.AddComment(r.Context(), key, body.Body)
	h.auditLog.Log(audit.NewEntry("add_comment", "jira", map[string]any{"key": key}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusCreated, comment)
}

// GetComments handles GET /jira/issues/{key}/comments?maxResults=.
func (h *JiraHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	maxResults := 50
	if mr := r.URL.Query().Get("maxResults"); mr != "" {
		if v, err := strconv.Atoi(mr); err == nil && v > 0 {
			maxResults = v
		}
	}
	comments, err := h.svc.GetComments(r.Context(), key, maxResults)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, comments)
}

// linkIssuesBody is the JSON request body for LinkIssues.
type linkIssuesBody struct {
	InwardIssue  string `json:"inward_issue"`
	OutwardIssue string `json:"outward_issue"`
	LinkType     string `json:"link_type"`
}

// LinkIssues handles POST /jira/issues/links.
// Write-guarded by WriteGuardMiddleware (X-Enable-Write: true required).
func (h *JiraHandler) LinkIssues(w http.ResponseWriter, r *http.Request) {
	var body linkIssuesBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.InwardIssue == "" || body.OutwardIssue == "" || body.LinkType == "" {
		api.RespondError(w, http.StatusBadRequest, "inward_issue, outward_issue and link_type are required", api.ErrCodeBadRequest)
		return
	}

	err := h.svc.LinkIssues(r.Context(), body.InwardIssue, body.OutwardIssue, body.LinkType)
	h.auditLog.Log(audit.NewEntry("link_issues", "jira", map[string]any{"inward": body.InwardIssue, "outward": body.OutwardIssue, "link_type": body.LinkType}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusCreated, map[string]string{"status": "linked"})
}

// GetIssueLinkTypes handles GET /jira/issues/link-types.
func (h *JiraHandler) GetIssueLinkTypes(w http.ResponseWriter, r *http.Request) {
	linkTypes, err := h.svc.GetIssueLinkTypes(r.Context())
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, linkTypes)
}

// addWorklogBody is the JSON request body for AddWorklog.
type addWorklogBody struct {
	TimeSpent string `json:"time_spent"`
	Comment   string `json:"comment"`
	Started   string `json:"started"`
}

// AddWorklog handles POST /jira/issues/{key}/worklogs.
// Write-guarded by WriteGuardMiddleware (X-Enable-Write: true required).
func (h *JiraHandler) AddWorklog(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	var body addWorklogBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.TimeSpent == "" {
		api.RespondError(w, http.StatusBadRequest, "time_spent is required", api.ErrCodeBadRequest)
		return
	}

	req := jira.AddWorklogRequest{
		TimeSpent: body.TimeSpent,
		Comment:   body.Comment,
		Started:   body.Started,
	}

	worklog, err := h.svc.AddWorklog(r.Context(), key, req)
	h.auditLog.Log(audit.NewEntry("add_worklog", "jira", map[string]any{"key": key, "time_spent": body.TimeSpent}, err))
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusCreated, worklog)
}

// GetIssueTypeMetadata handles GET /jira/projects/{key}/issue-types.
func (h *JiraHandler) GetIssueTypeMetadata(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	issueTypes, err := h.svc.GetIssueTypeMetadata(r.Context(), key)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, issueTypes)
}
