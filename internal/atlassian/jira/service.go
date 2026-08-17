package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
)

const defaultMaxResults = 50

// Service defines all Jira operations: read (Phase 1-2) and write (Phase 3).
// Both CLI commands and MCP tools depend on this interface.
type Service interface {
	GetIssue(ctx context.Context, key string) (*Issue, error)
	SearchIssues(ctx context.Context, jql string, maxResults int) (*SearchResult, error)
	// Phase 3 write methods:
	CreateIssue(ctx context.Context, req CreateIssueRequest) (*CreateIssueResponse, error)
	UpdateIssue(ctx context.Context, key string, req UpdateIssueRequest) error
	GetTransitions(ctx context.Context, key string) ([]Transition, error)
	TransitionIssue(ctx context.Context, key string, transitionID string) error
	// Phase 2 expansion — 7 new methods (4 read, 3 write):
	LookupAccountID(ctx context.Context, query string, maxResults int) ([]User, error)
	AddComment(ctx context.Context, key string, body string) (*Comment, error)
	GetComments(ctx context.Context, key string, maxResults int) ([]Comment, error)
	LinkIssues(ctx context.Context, inward, outward, linkTypeName string) error
	GetIssueLinkTypes(ctx context.Context) ([]IssueLinkType, error)
	AddWorklog(ctx context.Context, key string, req AddWorklogRequest) (*Worklog, error)
	GetIssueTypeMetadata(ctx context.Context, projectKey string) ([]IssueTypeMeta, error)
}

// JiraService implements Service against the Jira REST API v3.
type JiraService struct {
	doer    client.HTTPDoer
	baseURL string
}

// NewService constructs a JiraService. The doer is typically a *http.Client
// from httptest in tests, or a *client.Client in production.
func NewService(doer client.HTTPDoer, baseURL string) *JiraService {
	return &JiraService{
		doer:    doer,
		baseURL: baseURL,
	}
}

// GetIssue fetches a single Jira issue by key.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403.
func (s *JiraService) GetIssue(ctx context.Context, key string) (*Issue, error) {
	endpoint := s.baseURL + "/rest/api/3/issue/" + url.PathEscape(key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success — fall through to decode
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusTooManyRequests:
		return nil, ErrRateLimit
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw IssueAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("jira: decoding response: %w", err)
	}

	issue := raw.ToIssue()
	return &issue, nil
}

// CreateIssue creates a new Jira issue.
// Returns ErrUnauthorized on 401/403, ErrConflict on 409, ErrRateLimit on 429.
func (s *JiraService) CreateIssue(ctx context.Context, req CreateIssueRequest) (*CreateIssueResponse, error) {
	endpoint := s.baseURL + "/rest/api/3/issue"

	// Build the request body
	fields := createIssueFields{}
	fields.Project.Key = req.ProjectKey
	fields.IssueType.Name = req.IssueType
	fields.Summary = req.Summary

	if req.Description != "" {
		fields.Description = plainTextToADF(req.Description)
	}
	if req.AssigneeID != "" {
		fields.Assignee = &assigneeIDJSON{AccountID: req.AssigneeID}
	}
	if req.Labels != nil {
		fields.Labels = req.Labels
	}
	if req.PriorityName != "" {
		fields.Priority = &priorityNameJSON{Name: req.PriorityName}
	}

	apiReq := createIssueAPIRequest{Fields: fields}
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("jira: marshaling create request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		// success — fall through to decode
	case http.StatusBadRequest:
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: create issue 400: %s", string(respBody))
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusConflict:
		return nil, ErrConflict
	case http.StatusTooManyRequests:
		return nil, ErrRateLimit
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result CreateIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jira: decoding create response: %w", err)
	}
	return &result, nil
}

// UpdateIssue updates fields on an existing Jira issue.
// Only non-nil fields in req are sent. Returns ErrNotFound on 404, ErrUnauthorized on 401/403.
func (s *JiraService) UpdateIssue(ctx context.Context, key string, req UpdateIssueRequest) error {
	endpoint := s.baseURL + "/rest/api/3/issue/" + url.PathEscape(key)

	// Build fields map imperatively — only include non-nil fields
	fields := make(map[string]interface{})
	if req.Summary != nil {
		fields["summary"] = *req.Summary
	}
	if req.Description != nil {
		fields["description"] = plainTextToADF(*req.Description)
	}
	if req.AssigneeID != nil {
		fields["assignee"] = map[string]interface{}{"accountId": *req.AssigneeID}
	}
	if req.PriorityName != nil {
		fields["priority"] = map[string]interface{}{"name": *req.PriorityName}
	}

	apiReq := updateIssueAPIRequest{Fields: fields}
	body, err := json.Marshal(apiReq)
	if err != nil {
		return fmt.Errorf("jira: marshaling update request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("jira: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusBadRequest:
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: update issue 400: %s", string(respBody))
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimit
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
}

// GetTransitions returns the available workflow transitions for a Jira issue.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403.
func (s *JiraService) GetTransitions(ctx context.Context, key string) ([]Transition, error) {
	endpoint := s.baseURL + "/rest/api/3/issue/" + url.PathEscape(key) + "/transitions"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success — fall through to decode
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusTooManyRequests:
		return nil, ErrRateLimit
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var raw transitionsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("jira: decoding transitions response: %w", err)
	}

	transitions := make([]Transition, len(raw.Transitions))
	for i, t := range raw.Transitions {
		transitions[i] = Transition{
			ID:             t.ID,
			Name:           t.Name,
			StatusCategory: t.To.StatusCategory.Key,
		}
	}
	return transitions, nil
}

// TransitionIssue applies a workflow transition to a Jira issue.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, descriptive error on 400.
func (s *JiraService) TransitionIssue(ctx context.Context, key string, transitionID string) error {
	endpoint := s.baseURL + "/rest/api/3/issue/" + url.PathEscape(key) + "/transitions"

	var apiReq transitionAPIRequest
	apiReq.Transition.ID = transitionID

	body, err := json.Marshal(apiReq)
	if err != nil {
		return fmt.Errorf("jira: marshaling transition request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("jira: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusBadRequest:
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: transition issue 400: %s", string(respBody))
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimit
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
}

// --- Phase 2 expansion: 7 new service methods ---

const defaultAccountLookupMaxResults = 10

// LookupAccountID searches for Jira users by name or email.
// maxResults defaults to 10 when <= 0 to minimise token footprint in MCP contexts.
// Returns ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *JiraService) LookupAccountID(ctx context.Context, query string, maxResults int) ([]User, error) {
	if maxResults <= 0 {
		maxResults = defaultAccountLookupMaxResults
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("maxResults", strconv.Itoa(maxResults))
	endpoint := s.baseURL + "/rest/api/3/user/search?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success — fall through to decode
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusTooManyRequests:
		return nil, ErrRateLimit
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw []userItemJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("jira: decoding user search response: %w", err)
	}

	users := make([]User, len(raw))
	for i, u := range raw {
		users[i] = u.ToUser()
	}
	return users, nil
}

// AddComment posts a new comment on a Jira issue.
// body is plain text and is converted to ADF via plainTextToADF before sending.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *JiraService) AddComment(ctx context.Context, key string, body string) (*Comment, error) {
	endpoint := s.baseURL + "/rest/api/3/issue/" + url.PathEscape(key) + "/comment"

	apiBody := map[string]interface{}{
		"body": plainTextToADF(body),
	}
	encoded, err := json.Marshal(apiBody)
	if err != nil {
		return nil, fmt.Errorf("jira: marshaling comment request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		// success — fall through to decode
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusTooManyRequests:
		return nil, ErrRateLimit
	case http.StatusBadRequest:
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: add comment 400: %s", string(respBody))
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var raw commentItemJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("jira: decoding comment response: %w", err)
	}

	comment := raw.ToComment()
	return &comment, nil
}

// GetComments retrieves comments for a Jira issue.
// maxResults defaults to defaultMaxResults (50) when <= 0.
// Returns an empty (non-nil) slice when the issue has no comments.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *JiraService) GetComments(ctx context.Context, key string, maxResults int) ([]Comment, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	params := url.Values{}
	params.Set("maxResults", strconv.Itoa(maxResults))
	endpoint := s.baseURL + "/rest/api/3/issue/" + url.PathEscape(key) + "/comment?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success — fall through to decode
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusTooManyRequests:
		return nil, ErrRateLimit
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw commentsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("jira: decoding comments response: %w", err)
	}

	comments := make([]Comment, len(raw.Comments))
	for i, c := range raw.Comments {
		comments[i] = c.ToComment()
	}
	return comments, nil
}

// LinkIssues creates a directed link between two Jira issues.
// linkTypeName must match a name in GET /rest/api/3/issueLinkType (e.g. "Blocks").
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *JiraService) LinkIssues(ctx context.Context, inward, outward, linkTypeName string) error {
	endpoint := s.baseURL + "/rest/api/3/issueLink"

	apiBody := map[string]interface{}{
		"type":         map[string]string{"name": linkTypeName},
		"inwardIssue":  map[string]string{"key": inward},
		"outwardIssue": map[string]string{"key": outward},
	}
	encoded, err := json.Marshal(apiBody)
	if err != nil {
		return fmt.Errorf("jira: marshaling link request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("jira: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	case http.StatusTooManyRequests:
		return ErrRateLimit
	case http.StatusBadRequest:
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: link issues 400: %s", string(respBody))
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
}

// GetIssueLinkTypes returns all available issue link types for this Jira instance.
// Returns ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *JiraService) GetIssueLinkTypes(ctx context.Context) ([]IssueLinkType, error) {
	endpoint := s.baseURL + "/rest/api/3/issueLinkType"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success — fall through to decode
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusTooManyRequests:
		return nil, ErrRateLimit
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw issueLinkTypesAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("jira: decoding link types response: %w", err)
	}

	linkTypes := make([]IssueLinkType, len(raw.IssueLinkTypes))
	for i, lt := range raw.IssueLinkTypes {
		linkTypes[i] = lt.ToIssueLinkType()
	}
	return linkTypes, nil
}

// AddWorklog logs time spent on a Jira issue.
// req.TimeSpent is forwarded as-is; req.Comment (if non-empty) is converted to ADF;
// req.Started (if non-empty) is forwarded as-is.
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *JiraService) AddWorklog(ctx context.Context, key string, req AddWorklogRequest) (*Worklog, error) {
	endpoint := s.baseURL + "/rest/api/3/issue/" + url.PathEscape(key) + "/worklog"

	apiBody := map[string]interface{}{
		"timeSpent": req.TimeSpent,
	}
	if req.Comment != "" {
		apiBody["comment"] = plainTextToADF(req.Comment)
	}
	if req.Started != "" {
		apiBody["started"] = req.Started
	}

	encoded, err := json.Marshal(apiBody)
	if err != nil {
		return nil, fmt.Errorf("jira: marshaling worklog request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		// success — fall through to decode
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusTooManyRequests:
		return nil, ErrRateLimit
	case http.StatusBadRequest:
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: add worklog 400: %s", string(respBody))
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var raw worklogItemJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("jira: decoding worklog response: %w", err)
	}

	wl := raw.ToWorklog()
	return &wl, nil
}

// GetIssueTypeMetadata returns the available issue types for a Jira project.
// Handles both Jira Cloud response ("values" key) and Server/DC ("issueTypes" key).
// Returns ErrNotFound on 404, ErrUnauthorized on 401/403, ErrRateLimit on 429.
func (s *JiraService) GetIssueTypeMetadata(ctx context.Context, projectKey string) ([]IssueTypeMeta, error) {
	endpoint := s.baseURL + "/rest/api/3/issue/createmeta/" + url.PathEscape(projectKey) + "/issuetypes"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success — fall through to decode
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusTooManyRequests:
		return nil, ErrRateLimit
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw issueTypeMetaAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("jira: decoding issue type metadata response: %w", err)
	}

	// Cloud returns "values"; Server/DC returns "issueTypes" — try "values" first.
	items := raw.Values
	if len(items) == 0 {
		items = raw.IssueTypes
	}

	issueTypes := make([]IssueTypeMeta, len(items))
	for i, it := range items {
		issueTypes[i] = it.ToIssueTypeMeta()
	}
	return issueTypes, nil
}

// SearchIssues searches issues using JQL.
// Returns ErrInvalidJQL on 400. maxResults defaults to 50 if zero.
func (s *JiraService) SearchIssues(ctx context.Context, jql string, maxResults int) (*SearchResult, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	endpoint := s.baseURL + "/rest/api/3/search/jql"

	params := url.Values{}
	params.Set("jql", jql)
	params.Set("maxResults", strconv.Itoa(maxResults))
	params.Set("fields", "summary,status,assignee,priority,issuetype,created,updated,description,project,labels")

	fullURL := endpoint + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success — fall through to decode
	case http.StatusBadRequest:
		return nil, ErrInvalidJQL
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusTooManyRequests:
		return nil, ErrRateLimit
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw SearchAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("jira: decoding search response: %w", err)
	}

	result := raw.ToSearchResult()
	return &result, nil
}
