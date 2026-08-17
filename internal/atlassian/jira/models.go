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
	Key       string
	Summary   string
	Status    string
	IssueType string
	Assignee  string
	Priority  string
	Labels    []string
	Created   time.Time
	Updated   time.Time
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
	Key    string          `json:"key"`
	Fields issueFieldsJSON `json:"fields"`
}

type issueFieldsJSON struct {
	Summary   string        `json:"summary"`
	Status    statusJSON    `json:"status"`
	IssueType issueTypeJSON `json:"issuetype"`
	Assignee  *assigneeJSON `json:"assignee"`
	Priority  priorityJSON  `json:"priority"`
	Labels    []string      `json:"labels"`
	Created   string        `json:"created"`
	Updated   string        `json:"updated"`
}

type statusJSON struct {
	Name string `json:"name"`
}

type issueTypeJSON struct {
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
		Key:       r.Key,
		Summary:   r.Fields.Summary,
		Status:    r.Fields.Status.Name,
		IssueType: r.Fields.IssueType.Name,
		Priority:  r.Fields.Priority.Name,
		Labels:    r.Fields.Labels,
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

// --- Phase 2 (expand): New domain types ---

// User represents a Jira user account returned by the user search API.
type User struct {
	AccountID   string
	DisplayName string
	Email       string
	Active      bool
}

// Comment represents a single comment on a Jira issue.
// Body is plain text extracted from the ADF response via adfToPlainText.
type Comment struct {
	ID      string
	Author  string
	Body    string
	Created time.Time
	Updated time.Time
}

// IssueLinkType describes a kind of link between two Jira issues (e.g. "Blocks").
type IssueLinkType struct {
	ID      string
	Name    string
	Inward  string
	Outward string
}

// AddWorklogRequest holds the parameters for logging time on an issue.
// TimeSpent is forwarded as-is (Jira parses "3h 30m", "2h", "30m" natively).
// Comment and Started are optional; empty strings are omitted from the request.
type AddWorklogRequest struct {
	TimeSpent string // required; e.g. "3h 30m"
	Comment   string // optional plain text → ADF
	Started   string // optional ISO 8601; forwarded as-is
}

// Worklog represents a single worklog entry on a Jira issue.
type Worklog struct {
	ID               string
	TimeSpentSeconds int
	Started          time.Time
	Author           string
}

// IssueTypeMeta describes an issue type available for a project (from createmeta).
type IssueTypeMeta struct {
	ID      string
	Name    string
	Desc    string
	Subtask bool
}

// --- Raw API response wrappers for Phase 2 types ---

// userSearchResponse is the JSON array response from GET /rest/api/3/user/search.
// The Jira API returns a flat JSON array, so we decode into a slice directly.
type userItemJSON struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Active       bool   `json:"active"`
}

// ToUser converts a raw user API item to the domain User model.
func (u userItemJSON) ToUser() User {
	return User{
		AccountID:   u.AccountID,
		DisplayName: u.DisplayName,
		Email:       u.EmailAddress,
		Active:      u.Active,
	}
}

// commentAPIResponse maps a single comment from GET /rest/api/3/issue/{key}/comment.
type commentItemJSON struct {
	ID      string                 `json:"id"`
	Author  authorJSON             `json:"author"`
	Body    map[string]interface{} `json:"body"` // ADF document
	Created string                 `json:"created"`
	Updated string                 `json:"updated"`
}

type authorJSON struct {
	DisplayName string `json:"displayName"`
}

// ToComment converts a raw comment API item to the domain Comment model.
// The ADF body is converted to plain text via adfToPlainText.
func (c commentItemJSON) ToComment() Comment {
	comment := Comment{
		ID:     c.ID,
		Author: c.Author.DisplayName,
		Body:   adfToPlainText(c.Body),
	}
	if t, err := time.Parse(jiraTimeLayout, c.Created); err == nil {
		comment.Created = t.UTC()
	}
	if t, err := time.Parse(jiraTimeLayout, c.Updated); err == nil {
		comment.Updated = t.UTC()
	}
	return comment
}

// commentsAPIResponse maps the JSON from GET /rest/api/3/issue/{key}/comment.
type commentsAPIResponse struct {
	Comments []commentItemJSON `json:"comments"`
}

// issueLinkTypeItemJSON maps a single link type from GET /rest/api/3/issueLinkType.
type issueLinkTypeItemJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// ToIssueLinkType converts a raw link type API item to the domain model.
func (l issueLinkTypeItemJSON) ToIssueLinkType() IssueLinkType {
	return IssueLinkType{
		ID:      l.ID,
		Name:    l.Name,
		Inward:  l.Inward,
		Outward: l.Outward,
	}
}

// issueLinkTypesAPIResponse maps the JSON from GET /rest/api/3/issueLinkType.
type issueLinkTypesAPIResponse struct {
	IssueLinkTypes []issueLinkTypeItemJSON `json:"issueLinkTypes"`
}

// worklogItemJSON maps the worklog JSON returned by POST /rest/api/3/issue/{key}/worklog.
type worklogItemJSON struct {
	ID               string     `json:"id"`
	TimeSpentSeconds int        `json:"timeSpentSeconds"`
	Started          string     `json:"started"`
	Author           authorJSON `json:"author"`
}

// ToWorklog converts a raw worklog API item to the domain Worklog model.
func (w worklogItemJSON) ToWorklog() Worklog {
	wl := Worklog{
		ID:               w.ID,
		TimeSpentSeconds: w.TimeSpentSeconds,
		Author:           w.Author.DisplayName,
	}
	if t, err := time.Parse(jiraTimeLayout, w.Started); err == nil {
		wl.Started = t.UTC()
	}
	return wl
}

// issueTypeMetaItemJSON maps a single issue type from the createmeta response.
type issueTypeMetaItemJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Subtask     bool   `json:"subtask"`
}

// ToIssueTypeMeta converts a raw issue type meta item to the domain model.
func (m issueTypeMetaItemJSON) ToIssueTypeMeta() IssueTypeMeta {
	return IssueTypeMeta{
		ID:      m.ID,
		Name:    m.Name,
		Desc:    m.Description,
		Subtask: m.Subtask,
	}
}

// issueTypeMetaAPIResponse decodes both Jira Cloud ("values") and Server/DC
// ("issueTypes") shapes from GET /rest/api/3/issue/createmeta/{key}/issuetypes.
type issueTypeMetaAPIResponse struct {
	Values     []issueTypeMetaItemJSON `json:"values"`
	IssueTypes []issueTypeMetaItemJSON `json:"issueTypes"`
}

// adfToPlainText walks an ADF (Atlassian Document Format) JSON document and
// concatenates all "text" node values found in nested "content" arrays.
// Returns "" on nil or malformed input — never panics.
func adfToPlainText(adf map[string]interface{}) string {
	if adf == nil {
		return ""
	}
	return extractADFText(adf)
}

// extractADFText is the recursive helper used by adfToPlainText.
func extractADFText(node map[string]interface{}) string {
	if node == nil {
		return ""
	}
	result := ""
	// If this node is a "text" node, capture its text value.
	if nodeType, ok := node["type"].(string); ok && nodeType == "text" {
		if text, ok := node["text"].(string); ok {
			result += text
		}
	}
	// Recurse into "content" array regardless of node type.
	content, ok := node["content"].([]interface{})
	if !ok {
		return result
	}
	for _, child := range content {
		childMap, ok := child.(map[string]interface{})
		if !ok {
			continue
		}
		result += extractADFText(childMap)
	}
	return result
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
	Project struct {
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
