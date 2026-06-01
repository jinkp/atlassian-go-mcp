// Package jira provides a read-only SDK for the Jira REST API v3.
package jira

import (
	"errors"
	"time"
)

// jiraTimeLayout is the time format used by Jira REST API v3.
const jiraTimeLayout = "2006-01-02T15:04:05.000-0700"

// Sentinel errors returned by JiraService. Callers can use errors.Is to
// branch on specific error types without string matching.
var (
	// ErrNotFound is returned when a requested issue does not exist (HTTP 404).
	ErrNotFound = errors.New("issue not found")

	// ErrUnauthorized is returned on 401/403 — bad credentials or insufficient permissions.
	ErrUnauthorized = errors.New("unauthorized: check ATLASSIAN_EMAIL and ATLASSIAN_TOKEN")

	// ErrRateLimit is returned on 429 after all retries are exhausted.
	ErrRateLimit = errors.New("rate limited: too many requests")

	// ErrInvalidJQL is returned on 400 with a JQL validation error.
	ErrInvalidJQL = errors.New("invalid JQL expression")

	// ErrConflict is returned on 409 — resource already exists or key already taken.
	ErrConflict = errors.New("conflict: resource already exists")
)

// Issue is the canonical domain model for a Jira issue.
// All service methods return this struct — never raw JSON types.
type Issue struct {
	Key      string
	Summary  string
	Status   string
	Assignee string
	Priority string
	Labels   []string
	Created  time.Time
	Updated  time.Time
}

// SearchResult holds a page of Jira search results.
type SearchResult struct {
	Issues     []Issue
	Total      int
	StartAt    int
	MaxResults int
}

// SearchOptions controls pagination and other search parameters.
type SearchOptions struct {
	MaxResults int // default 50 if zero
	StartAt    int
}

// --- Jira REST API response shapes (internal — not exported to callers) ---

// IssueAPIResponse maps the raw JSON from GET /rest/api/3/issue/{key}.
type IssueAPIResponse struct {
	Key    string           `json:"key"`
	Fields issueFieldsJSON  `json:"fields"`
}

type issueFieldsJSON struct {
	Summary  string          `json:"summary"`
	Status   statusJSON      `json:"status"`
	Assignee *assigneeJSON   `json:"assignee"`
	Priority priorityJSON    `json:"priority"`
	Labels   []string        `json:"labels"`
	Created  string          `json:"created"`
	Updated  string          `json:"updated"`
}

type statusJSON struct {
	Name string `json:"name"`
}

type assigneeJSON struct {
	DisplayName string `json:"displayName"`
}

type priorityJSON struct {
	Name string `json:"name"`
}

// ToIssue converts the raw API response into the domain Issue model.
func (r IssueAPIResponse) ToIssue() Issue {
	issue := Issue{
		Key:      r.Key,
		Summary:  r.Fields.Summary,
		Status:   r.Fields.Status.Name,
		Priority: r.Fields.Priority.Name,
		Labels:   r.Fields.Labels,
	}
	if r.Fields.Labels == nil {
		issue.Labels = []string{}
	}
	if r.Fields.Assignee != nil {
		issue.Assignee = r.Fields.Assignee.DisplayName
	}

	if t, err := time.Parse(jiraTimeLayout, r.Fields.Created); err == nil {
		issue.Created = t.UTC()
	}
	if t, err := time.Parse(jiraTimeLayout, r.Fields.Updated); err == nil {
		issue.Updated = t.UTC()
	}
	return issue
}

// SearchAPIResponse maps the raw JSON from GET /rest/api/3/search/jql.
// Supports both legacy (total/startAt/maxResults) and enhanced (isLast/nextPageToken) fields.
type SearchAPIResponse struct {
	Total      int                `json:"total"`
	StartAt    int                `json:"startAt"`
	MaxResults int                `json:"maxResults"`
	Issues     []IssueAPIResponse `json:"issues"`
	IsLast     *bool              `json:"isLast,omitempty"`
}

// ToSearchResult converts the raw search response to the domain SearchResult.
func (r SearchAPIResponse) ToSearchResult() SearchResult {
	issues := make([]Issue, len(r.Issues))
	for i, raw := range r.Issues {
		issues[i] = raw.ToIssue()
	}
	total := r.Total
	if total == 0 {
		total = len(issues)
	}
	return SearchResult{
		Issues:     issues,
		Total:      total,
		StartAt:    r.StartAt,
		MaxResults: r.MaxResults,
	}
}

// --- Phase 3: Write operation domain types ---

// CreateIssueRequest contains the parameters for creating a new Jira issue.
// Required: ProjectKey, IssueType, Summary. Optional: Description, AssigneeID, Labels, PriorityName.
type CreateIssueRequest struct {
	ProjectKey   string
	IssueType    string
	Summary      string
	Description  string   // plain text; empty = omit from request
	AssigneeID   string   // optional; Jira accountId
	Labels       []string // nil → omitted; non-nil → included (even if empty)
	PriorityName string   // optional; e.g. "High", "Medium"
}

// CreateIssueResponse holds the key and ID of a newly created issue.
type CreateIssueResponse struct {
	Key string `json:"key"`
	ID  string `json:"id"`
}

// UpdateIssueRequest contains optional fields to update on an existing issue.
// A nil pointer means "do not change this field".
type UpdateIssueRequest struct {
	Summary      *string // nil = do not send
	Description  *string // nil = do not send; non-nil = wrap to ADF
	AssigneeID   *string // nil = do not send
	PriorityName *string // nil = do not send
}

// Transition represents a single workflow transition available on a Jira issue.
type Transition struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	StatusCategory string `json:"status_category"` // extracted from to.statusCategory.key
}

// --- Internal ADF helper ---

// plainTextToADF wraps plain text in a minimal Jira v3 ADF document node.
// Output: {"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"<input>"}]}]}
func plainTextToADF(text string) map[string]interface{} {
	return map[string]interface{}{
		"type":    "doc",
		"version": 1,
		"content": []interface{}{
			map[string]interface{}{
				"type": "paragraph",
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": text,
					},
				},
			},
		},
	}
}

// --- Internal wire types for Jira REST API v3 ---

// createIssueAPIRequest is the JSON body for POST /rest/api/3/issue.
type createIssueAPIRequest struct {
	Fields createIssueFields `json:"fields"`
}

type createIssueFields struct {
	Project     struct {
		Key string `json:"key"`
	} `json:"project"`
	IssueType struct {
		Name string `json:"name"`
	} `json:"issuetype"`
	Summary     string                 `json:"summary"`
	Description map[string]interface{} `json:"description,omitempty"`
	Assignee    *assigneeIDJSON        `json:"assignee,omitempty"`
	Labels      []string               `json:"labels,omitempty"`
	Priority    *priorityNameJSON      `json:"priority,omitempty"`
}

type assigneeIDJSON struct {
	AccountID string `json:"accountId"`
}

type priorityNameJSON struct {
	Name string `json:"name"`
}

// updateIssueAPIRequest is the JSON body for PUT /rest/api/3/issue/{key}.
// Fields map is built imperatively — only non-nil fields from UpdateIssueRequest are added.
type updateIssueAPIRequest struct {
	Fields map[string]interface{} `json:"fields"`
}

// transitionAPIRequest is the JSON body for POST /rest/api/3/issue/{key}/transitions.
type transitionAPIRequest struct {
	Transition struct {
		ID string `json:"id"`
	} `json:"transition"`
}

// transitionsAPIResponse is the JSON response from GET /rest/api/3/issue/{key}/transitions.
type transitionsAPIResponse struct {
	Transitions []transitionItemJSON `json:"transitions"`
}

type transitionItemJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   struct {
		StatusCategory struct {
			Key string `json:"key"`
		} `json:"statusCategory"`
	} `json:"to"`
}
