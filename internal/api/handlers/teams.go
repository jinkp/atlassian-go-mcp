package handlers

import (
	"net/http"
	"strconv"

	"github.com/jinkp/atlassian-go-mcp/internal/api"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
)

// TeamsHandler handles all /teams/* routes.
// Teams service is read-only — no audit log needed.
type TeamsHandler struct {
	svc teams.TeamsService
}

// NewTeamsHandler constructs a TeamsHandler.
func NewTeamsHandler(svc teams.TeamsService) *TeamsHandler {
	return &TeamsHandler{svc: svc}
}

// GetTeams handles GET /teams?query=...&maxResults=50.
func (h *TeamsHandler) GetTeams(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	maxResults := 50
	if mr := r.URL.Query().Get("maxResults"); mr != "" {
		if v, err := strconv.Atoi(mr); err == nil && v > 0 {
			maxResults = v
		}
	}

	result, err := h.svc.GetTeams(r.Context(), query, maxResults)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: result.Teams, Total: len(result.Teams)})
}

// GetTeam handles GET /teams/{teamId}.
func (h *TeamsHandler) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("teamId")
	team, err := h.svc.GetTeam(r.Context(), teamID)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, team)
}

// GetTeamMembers handles GET /teams/{teamId}/members.
func (h *TeamsHandler) GetTeamMembers(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("teamId")
	maxResults := 50

	members, err := h.svc.GetTeamMembers(r.Context(), teamID, maxResults)
	if err != nil {
		status, code := api.ErrToStatus(err)
		api.RespondError(w, status, err.Error(), code)
		return
	}
	api.RespondJSON(w, http.StatusOK, api.ListResponse{Items: members, Total: len(members)})
}
