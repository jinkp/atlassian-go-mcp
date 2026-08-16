package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/jinkp/atlassian-go-mcp/internal/api"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
)

// BitbucketHandler handles all /bitbucket/* routes.
type BitbucketHandler struct {
	svc      bitbucket.BitbucketService
	auditLog audit.Logger
}

// NewBitbucketHandler constructs a BitbucketHandler.
func NewBitbucketHandler(svc bitbucket.BitbucketService, auditLog audit.Logger) *BitbucketHandler {
	return &BitbucketHandler{svc: svc, auditLog: auditLog}
}

// --- helpers ---

// resolveWorkspace reads workspace from the ?workspace= query param, falling back
// to the BITBUCKET_WORKSPACE environment variable.
func resolveWorkspace(r *http.Request) (string, bool) {
	ws := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if ws == "" {
		ws = strings.TrimSpace(os.Getenv("BITBUCKET_WORKSPACE"))
	}
	return ws, ws != ""
}

// resolveWorkspaceRepo resolves workspace and repo (query → env).
func resolveWorkspaceRepo(r *http.Request) (workspace, repo string, ok bool) {
	ws, wsOK := resolveWorkspace(r)
	if !wsOK {
		return "", "", false
	}
	repo = strings.TrimSpace(r.URL.Query().Get("repo"))
	if repo == "" {
		repo = strings.TrimSpace(os.Getenv("BITBUCKET_REPO"))
	}
	if repo == "" {
		return "", "", false
	}
	return ws, repo, true
}

func badWorkspace(w http.ResponseWriter) {
	api.RespondError(w, http.StatusBadRequest, "workspace is required (query param 'workspace' or BITBUCKET_WORKSPACE)", api.ErrCodeBadRequest)
}

func badWorkspaceRepo(w http.ResponseWriter) {
	api.RespondError(w, http.StatusBadRequest, "workspace and repo are required (query params or BITBUCKET_WORKSPACE/BITBUCKET_REPO)", api.ErrCodeBadRequest)
}

// prID reads and validates the {id} path value.
func prID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		api.RespondError(w, http.StatusBadRequest, "pull request id must be a positive integer", api.ErrCodeBadRequest)
		return 0, false
	}
	return id, true
}

func (h *BitbucketHandler) fail(w http.ResponseWriter, err error) {
	status, code := api.ErrToStatus(err)
	api.RespondError(w, status, err.Error(), code)
}

// --- read handlers ---

// GetRepos handles GET /bitbucket/repos.
func (h *BitbucketHandler) GetRepos(w http.ResponseWriter, r *http.Request) {
	ws, ok := resolveWorkspace(r)
	if !ok {
		badWorkspace(w)
		return
	}
	repos, err := h.svc.ListRepositories(r.Context(), ws)
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: repos, Total: len(repos)})
}

// ListPRs handles GET /bitbucket/pullrequests.
func (h *BitbucketHandler) ListPRs(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	state := r.URL.Query().Get("state")
	prs, err := h.svc.ListPullRequests(r.Context(), ws, repo, state)
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: prs, Total: len(prs)})
}

// GetPR handles GET /bitbucket/pullrequests/{id}.
func (h *BitbucketHandler) GetPR(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	pr, err := h.svc.GetPullRequest(r.Context(), ws, repo, id)
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, pr)
}

// GetPRComments handles GET /bitbucket/pullrequests/{id}/comments.
func (h *BitbucketHandler) GetPRComments(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListPRComments(r.Context(), ws, repo, id)
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: items, Total: len(items)})
}

// GetPRCommits handles GET /bitbucket/pullrequests/{id}/commits.
func (h *BitbucketHandler) GetPRCommits(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListPRCommits(r.Context(), ws, repo, id)
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: items, Total: len(items)})
}

// GetPRFiles handles GET /bitbucket/pullrequests/{id}/files.
func (h *BitbucketHandler) GetPRFiles(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListPRFiles(r.Context(), ws, repo, id)
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: items, Total: len(items)})
}

// GetPRDiff handles GET /bitbucket/pullrequests/{id}/diff. Returns text/plain.
func (h *BitbucketHandler) GetPRDiff(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	diff, err := h.svc.GetPRDiff(r.Context(), ws, repo, id)
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(diff))
}

// GetPRChecks handles GET /bitbucket/pullrequests/{id}/checks.
func (h *BitbucketHandler) GetPRChecks(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListPRChecks(r.Context(), ws, repo, id)
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: items, Total: len(items)})
}

// GetPRReviewers handles GET /bitbucket/pullrequests/{id}/reviewers.
func (h *BitbucketHandler) GetPRReviewers(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListPRReviewers(r.Context(), ws, repo, id)
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: items, Total: len(items)})
}

// GetBranches handles GET /bitbucket/branches.
func (h *BitbucketHandler) GetBranches(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	items, err := h.svc.ListBranches(r.Context(), ws, repo)
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: items, Total: len(items)})
}

// GetStaleBranches handles GET /bitbucket/branches/stale?days=N.
func (h *BitbucketHandler) GetStaleBranches(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil {
			days = parsed
		}
	}
	items, err := h.svc.StaleBranches(r.Context(), ws, repo, days)
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: items, Total: len(items)})
}

// GetPipelines handles GET /bitbucket/pipelines.
func (h *BitbucketHandler) GetPipelines(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	items, err := h.svc.ListPipelines(r.Context(), ws, repo)
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: items, Total: len(items)})
}

// --- write handlers (guarded by WriteGuardMiddleware) ---

type createPRBody struct {
	Title             string   `json:"title"`
	Source            string   `json:"source"`
	Destination       string   `json:"destination"`
	Description       string   `json:"description"`
	Reviewers         []string `json:"reviewers"`
	CloseSourceBranch bool     `json:"close_source_branch"`
}

// CreatePR handles POST /bitbucket/pullrequests.
func (h *BitbucketHandler) CreatePR(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	var body createPRBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.Title == "" || body.Source == "" || body.Destination == "" {
		api.RespondError(w, http.StatusBadRequest, "title, source and destination are required", api.ErrCodeBadRequest)
		return
	}
	pr, err := h.svc.CreatePullRequest(r.Context(), ws, repo, bitbucket.CreatePRRequest{
		Title:             body.Title,
		SourceBranch:      body.Source,
		DestinationBranch: body.Destination,
		Description:       body.Description,
		Reviewers:         body.Reviewers,
		CloseSourceBranch: body.CloseSourceBranch,
	})
	h.auditLog.Log(audit.NewEntry("bb_create_pr", "bitbucket", map[string]any{"workspace": ws, "repo": repo}, err))
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusCreated, pr)
}

type commentBody struct {
	Message string `json:"message"`
}

// CommentPR handles POST /bitbucket/pullrequests/{id}/comments.
func (h *BitbucketHandler) CommentPR(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	var body commentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.Message == "" {
		api.RespondError(w, http.StatusBadRequest, "message is required", api.ErrCodeBadRequest)
		return
	}
	comment, err := h.svc.AddPRComment(r.Context(), ws, repo, id, body.Message)
	h.auditLog.Log(audit.NewEntry("bb_comment_pr", "bitbucket", map[string]any{"workspace": ws, "repo": repo, "pr_id": id}, err))
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusCreated, comment)
}

type updatePRBody struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Destination string   `json:"destination"`
	Reviewers   []string `json:"reviewers"`
}

// UpdatePR handles PUT /bitbucket/pullrequests/{id}.
func (h *BitbucketHandler) UpdatePR(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	var body updatePRBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	pr, err := h.svc.UpdatePullRequest(r.Context(), ws, repo, id, bitbucket.UpdatePRRequest{
		Title:             body.Title,
		Description:       body.Description,
		DestinationBranch: body.Destination,
		Reviewers:         body.Reviewers,
	})
	h.auditLog.Log(audit.NewEntry("bb_update_pr", "bitbucket", map[string]any{"workspace": ws, "repo": repo, "pr_id": id}, err))
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, pr)
}

// ApprovePR handles POST /bitbucket/pullrequests/{id}/approve.
func (h *BitbucketHandler) ApprovePR(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	err := h.svc.ApprovePullRequest(r.Context(), ws, repo, id)
	h.auditLog.Log(audit.NewEntry("bb_approve_pr", "bitbucket", map[string]any{"workspace": ws, "repo": repo, "pr_id": id}, err))
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, map[string]any{"pr_id": id, "approved": true})
}

// DeclinePR handles POST /bitbucket/pullrequests/{id}/decline.
func (h *BitbucketHandler) DeclinePR(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	err := h.svc.DeclinePullRequest(r.Context(), ws, repo, id)
	h.auditLog.Log(audit.NewEntry("bb_decline_pr", "bitbucket", map[string]any{"workspace": ws, "repo": repo, "pr_id": id}, err))
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, map[string]any{"pr_id": id, "declined": true})
}

type mergeBody struct {
	Strategy string `json:"strategy"`
	Message  string `json:"message"`
}

// MergePR handles POST /bitbucket/pullrequests/{id}/merge.
func (h *BitbucketHandler) MergePR(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	var body mergeBody
	// Body is optional for merge; ignore decode errors on empty body.
	_ = json.NewDecoder(r.Body).Decode(&body)
	pr, err := h.svc.MergePullRequest(r.Context(), ws, repo, id, body.Strategy, body.Message)
	h.auditLog.Log(audit.NewEntry("bb_merge_pr", "bitbucket", map[string]any{"workspace": ws, "repo": repo, "pr_id": id, "strategy": body.Strategy}, err))
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusOK, pr)
}

type taskBody struct {
	Message string `json:"message"`
}

// CreateTask handles POST /bitbucket/pullrequests/{id}/tasks.
func (h *BitbucketHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	var body taskBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.Message == "" {
		api.RespondError(w, http.StatusBadRequest, "message is required", api.ErrCodeBadRequest)
		return
	}
	task, err := h.svc.CreatePRTask(r.Context(), ws, repo, id, body.Message)
	h.auditLog.Log(audit.NewEntry("bb_create_pr_task", "bitbucket", map[string]any{"workspace": ws, "repo": repo, "pr_id": id}, err))
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusCreated, task)
}

// ResolveTask handles PUT /bitbucket/pullrequests/{id}/tasks/{taskId}.
func (h *BitbucketHandler) ResolveTask(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	id, ok := prID(w, r)
	if !ok {
		return
	}
	taskID, err := strconv.Atoi(r.PathValue("taskId"))
	if err != nil || taskID <= 0 {
		api.RespondError(w, http.StatusBadRequest, "task id must be a positive integer", api.ErrCodeBadRequest)
		return
	}
	svcErr := h.svc.ResolvePRTask(r.Context(), ws, repo, id, taskID)
	h.auditLog.Log(audit.NewEntry("bb_resolve_pr_task", "bitbucket", map[string]any{"workspace": ws, "repo": repo, "pr_id": id, "task_id": taskID}, svcErr))
	if svcErr != nil {
		h.fail(w, svcErr)
		return
	}
	api.RespondJSON(w, http.StatusOK, map[string]any{"pr_id": id, "task_id": taskID, "resolved": true})
}

type runPipelineBody struct {
	Branch string `json:"branch"`
}

// RunPipeline handles POST /bitbucket/pipelines.
func (h *BitbucketHandler) RunPipeline(w http.ResponseWriter, r *http.Request) {
	ws, repo, ok := resolveWorkspaceRepo(r)
	if !ok {
		badWorkspaceRepo(w)
		return
	}
	var body runPipelineBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), api.ErrCodeBadRequest)
		return
	}
	if body.Branch == "" {
		api.RespondError(w, http.StatusBadRequest, "branch is required", api.ErrCodeBadRequest)
		return
	}
	pipeline, err := h.svc.RunPipeline(r.Context(), ws, repo, body.Branch)
	h.auditLog.Log(audit.NewEntry("bb_run_pipeline", "bitbucket", map[string]any{"workspace": ws, "repo": repo, "branch": body.Branch}, err))
	if err != nil {
		h.fail(w, err)
		return
	}
	api.RespondJSON(w, http.StatusCreated, pipeline)
}
