// Package agile provides types and a service client for the Jira Agile REST API v1.0.
// It mirrors the jira package structure: interface, concrete impl, and domain models.
package agile



// Board is the domain model for a Jira Software board.
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "scrum" or "kanban"
}

// Sprint is the domain model for a Jira Software sprint.
type Sprint struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	State        string `json:"state"` // "active", "future", or "closed"
	StartDate    string `json:"start_date,omitempty"`
	EndDate      string `json:"end_date,omitempty"`
	CompleteDate string `json:"complete_date,omitempty"`
	BoardID      int    `json:"board_id"`
}

// SprintIssue is a lightweight issue summary within a sprint.
type SprintIssue struct {
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Assignee string `json:"assignee"` // empty string when unassigned
}

// SprintIssueResult holds a page of sprint issues plus pagination metadata.
type SprintIssueResult struct {
	Issues     []SprintIssue `json:"issues"`
	Total      int           `json:"total"`
	StartAt    int           `json:"start_at"`
	MaxResults int           `json:"max_results"`
}

// UpdateSprintRequest is the public partial-update request for a sprint; nil fields are omitted.
type UpdateSprintRequest struct {
	Name      *string // nil = omit
	State     *string // nil = omit; "closed" closes the sprint
	StartDate *string // nil = omit; ISO 8601 e.g. "2024-01-15T00:00:00.000Z"
	EndDate   *string // nil = omit; ISO 8601
}

// CreateSprintRequest is the public request to create a new sprint on a board.
// StartDate and EndDate are optional; pass "" to omit.
type CreateSprintRequest struct {
	Name      string // required
	BoardID   int    // required; maps to originBoardId in wire
	StartDate string // optional; ISO 8601 e.g. "2024-01-15T00:00:00.000Z"
	EndDate   string // optional; ISO 8601
}

// --- unexported wire types for the Jira Agile REST API ---

// boardsAPIResponse is the JSON shape from GET /rest/agile/1.0/board.
type boardsAPIResponse struct {
	Values []boardAPIItem `json:"values"`
}

type boardAPIItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type struct {
		Name string `json:"name"`
	} `json:"type"`
}

// sprintsAPIResponse is the JSON shape from GET /rest/agile/1.0/board/{id}/sprint.
type sprintsAPIResponse struct {
	Values []sprintAPIItem `json:"values"`
}

type sprintAPIItem struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	State        string `json:"state"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	CompleteDate string `json:"completeDate"`
}

// sprintIssuesAPIResponse is the JSON shape from GET /rest/agile/1.0/sprint/{id}/issue.
type sprintIssuesAPIResponse struct {
	Issues     []sprintIssueAPIItem `json:"issues"`
	Total      int                  `json:"total"`
	StartAt    int                  `json:"startAt"`
	MaxResults int                  `json:"maxResults"`
}

type sprintIssueAPIItem struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name string `json:"name"`
		} `json:"status"`
		Assignee *struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
	} `json:"fields"`
}

// updateSprintAPIRequest is the JSON body for POST /rest/agile/1.0/sprint/{id}.
// omitempty ensures zero-value strings (from nil pointer fields) are excluded.
type updateSprintAPIRequest struct {
	Name      string `json:"name,omitempty"`
	State     string `json:"state,omitempty"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
}

// createSprintAPIRequest is the JSON body for POST /rest/agile/1.0/sprint.
type createSprintAPIRequest struct {
	Name          string `json:"name"`
	OriginBoardID int    `json:"originBoardId"`
	StartDate     string `json:"startDate,omitempty"`
	EndDate       string `json:"endDate,omitempty"`
}

// moveIssuesAPIRequest is the JSON body for POST .../issue endpoints.
type moveIssuesAPIRequest struct {
	Issues []string `json:"issues"`
}

