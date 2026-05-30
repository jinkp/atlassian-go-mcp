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

// SearchIssues searches issues using JQL.
// Returns ErrInvalidJQL on 400. maxResults defaults to 50 if zero.
func (s *JiraService) SearchIssues(ctx context.Context, jql string, maxResults int) (*SearchResult, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	endpoint := s.baseURL + "/rest/api/3/search"

	params := url.Values{}
	params.Set("jql", jql)
	params.Set("maxResults", strconv.Itoa(maxResults))
	params.Set("startAt", "0")

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
