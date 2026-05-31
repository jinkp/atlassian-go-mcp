// Package projects provides types and a service client for the Jira Projects REST API v3.
// It mirrors the releases package structure: interface, concrete impl, and domain models.
package projects

// Project is the domain model for a Jira project.
type Project struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ProjectType string `json:"project_type"`
	Lead        string `json:"lead,omitempty"` // lead accountId
	URL         string `json:"url,omitempty"`
}

// SearchProjectsRequest is the input for a paginated project search.
type SearchProjectsRequest struct {
	Query      string
	MaxResults int
	StartAt    int
}

// SearchProjectsResult is the paginated result for SearchProjects.
type SearchProjectsResult struct {
	Projects   []Project `json:"projects"`
	Total      int       `json:"total"`
	StartAt    int       `json:"start_at"`
	MaxResults int       `json:"max_results"`
}

// UpdateProjectRequest is the partial-update request; nil fields are omitted from the PUT body.
type UpdateProjectRequest struct {
	Name        *string
	Description *string
	Lead        *string // accountId
}

// --- unexported wire types for the Jira REST API v3 ---

// projectLeadWire is the nested lead object returned in GET project responses.
type projectLeadWire struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
}

// projectAPIItem is the JSON shape for a single project from the Jira REST API.
type projectAPIItem struct {
	ID             string           `json:"id"`
	Key            string           `json:"key"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	ProjectTypeKey string           `json:"projectTypeKey"`
	Lead           *projectLeadWire `json:"lead"`
	Self           string           `json:"self"`
}

// searchProjectsAPIResponse is the paginated envelope from GET /rest/api/3/project/search.
type searchProjectsAPIResponse struct {
	Values     []projectAPIItem `json:"values"`
	Total      int              `json:"total"`
	StartAt    int              `json:"startAt"`
	MaxResults int              `json:"maxResults"`
}

// updateProjectAPIRequest is the JSON body for PUT /rest/api/3/project/{key}.
// omitempty on all fields — only populated fields are sent.
type updateProjectAPIRequest struct {
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	LeadAccountID string `json:"leadAccountId,omitempty"`
}
