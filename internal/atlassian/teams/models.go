// Package teams provides types and a service client for the Atlassian Teams Public REST API v1.
// It mirrors the projects package structure: interface, concrete impl, and domain models.
package teams

// Team is the domain model for an Atlassian team.
type Team struct {
	ID             string `json:"id"`
	DisplayName    string `json:"display_name"`
	Description    string `json:"description,omitempty"`
	OrganizationID string `json:"organization_id"`
	TeamType       string `json:"team_type,omitempty"`
	State          string `json:"state,omitempty"`
}

// TeamMember is a member of an Atlassian team.
type TeamMember struct {
	AccountID string `json:"account_id"`
}

// TeamSearchResult is the paginated result for GetTeams.
type TeamSearchResult struct {
	Teams      []Team `json:"teams"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// --- unexported wire types for the Atlassian Teams REST API v1 ---

// teamAPIItem is the JSON shape for a single team from the Teams REST API.
type teamAPIItem struct {
	TeamId         string `json:"teamId"`
	DisplayName    string `json:"displayName"`
	Description    string `json:"description"`
	OrganizationId string `json:"organizationId"`
	TeamType       string `json:"teamType"`
	State          string `json:"state"`
}

// teamsAPIResponse is the paginated envelope from GET /public/teams/v1/org/{orgId}/teams.
type teamsAPIResponse struct {
	Entities []teamAPIItem `json:"entities"`
	Cursor   string        `json:"cursor"`
}

// memberAPIItem is the JSON shape for a single team member from the Teams REST API.
type memberAPIItem struct {
	AccountId string `json:"accountId"`
}

// membersAPIResponse is the response envelope from POST /public/teams/v1/org/{orgId}/teams/{teamId}/members.
type membersAPIResponse struct {
	Results []memberAPIItem `json:"results"`
}

// membersAPIRequest is the JSON body for the members POST endpoint.
type membersAPIRequest struct {
	After string `json:"after,omitempty"`
	First int    `json:"first"`
}
