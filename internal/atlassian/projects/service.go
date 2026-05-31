package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

const defaultMaxResults = 50

// ProjectsService defines read and write operations against the Jira Projects REST API v3.
type ProjectsService interface {
	GetProjects(ctx context.Context, maxResults int) ([]Project, error)
	GetProject(ctx context.Context, projectKey string) (*Project, error)
	SearchProjects(ctx context.Context, req SearchProjectsRequest) (*SearchProjectsResult, error)
	UpdateProject(ctx context.Context, projectKey string, req UpdateProjectRequest) (*Project, error)
}

// ProjectsJiraService implements ProjectsService against the Jira REST API v3.
type ProjectsJiraService struct {
	doer    client.HTTPDoer
	baseURL string
}

// NewService constructs a ProjectsJiraService. The doer is typically a *http.Client
// from httptest in tests, or a *client.Client in production.
func NewService(doer client.HTTPDoer, baseURL string) ProjectsService {
	return &ProjectsJiraService{doer: doer, baseURL: baseURL}
}

// toProject converts a wire projectAPIItem to a domain Project.
func toProject(item projectAPIItem) Project {
	lead := ""
	if item.Lead != nil {
		lead = item.Lead.AccountID
	}
	return Project{
		ID:          item.ID,
		Key:         item.Key,
		Name:        item.Name,
		Description: item.Description,
		ProjectType: item.ProjectTypeKey,
		Lead:        lead,
		URL:         item.Self,
	}
}

// GetProjects fetches all projects (paginated via maxResults).
// Returns ErrUnauthorized on 401/403, ErrNotFound on 404, descriptive error on 400.
// Returns a non-nil empty slice when no projects exist.
func (s *ProjectsJiraService) GetProjects(ctx context.Context, maxResults int) ([]Project, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	endpoint := s.baseURL + "/rest/api/3/project?maxResults=" + strconv.Itoa(maxResults)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("projects: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("projects: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success — fall through to decode
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, jira.ErrUnauthorized
	case http.StatusNotFound:
		return nil, jira.ErrNotFound
	case http.StatusBadRequest:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("projects: get projects 400: %s", string(body))
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("projects: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var items []projectAPIItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("projects: decoding response: %w", err)
	}

	result := make([]Project, len(items))
	for i, item := range items {
		result[i] = toProject(item)
	}
	return result, nil
}

// GetProject fetches a single project by key or ID.
// Returns ErrUnauthorized on 401/403, ErrNotFound on 404.
func (s *ProjectsJiraService) GetProject(ctx context.Context, projectKey string) (*Project, error) {
	endpoint := s.baseURL + "/rest/api/3/project/" + projectKey

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("projects: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("projects: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, jira.ErrUnauthorized
	case http.StatusNotFound:
		return nil, jira.ErrNotFound
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("projects: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var item projectAPIItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("projects: decoding response: %w", err)
	}
	p := toProject(item)
	return &p, nil
}

// SearchProjects searches projects with optional query and pagination.
// Returns ErrUnauthorized on 401/403, descriptive error on 400.
func (s *ProjectsJiraService) SearchProjects(ctx context.Context, req SearchProjectsRequest) (*SearchProjectsResult, error) {
	if req.MaxResults <= 0 {
		req.MaxResults = defaultMaxResults
	}

	endpoint := s.baseURL + "/rest/api/3/project/search" +
		"?maxResults=" + strconv.Itoa(req.MaxResults) +
		"&startAt=" + strconv.Itoa(req.StartAt)
	if req.Query != "" {
		endpoint += "&query=" + req.Query
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("projects: building request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("projects: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, jira.ErrUnauthorized
	case http.StatusBadRequest:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("projects: search projects 400: %s", string(body))
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("projects: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw searchProjectsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("projects: decoding response: %w", err)
	}

	projectList := make([]Project, len(raw.Values))
	for i, item := range raw.Values {
		projectList[i] = toProject(item)
	}

	return &SearchProjectsResult{
		Projects:   projectList,
		Total:      raw.Total,
		StartAt:    raw.StartAt,
		MaxResults: raw.MaxResults,
	}, nil
}

// UpdateProject updates a project in Jira. Nil pointer fields are omitted from the PUT body.
// Returns the updated Project on 200, ErrUnauthorized on 401/403, ErrNotFound on 404.
func (s *ProjectsJiraService) UpdateProject(ctx context.Context, projectKey string, req UpdateProjectRequest) (*Project, error) {
	body := updateProjectAPIRequest{}
	if req.Name != nil {
		body.Name = *req.Name
	}
	if req.Description != nil {
		body.Description = *req.Description
	}
	if req.Lead != nil {
		body.LeadAccountID = *req.Lead
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("projects: marshaling update request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut,
		s.baseURL+"/rest/api/3/project/"+projectKey, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("projects: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("projects: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, jira.ErrUnauthorized
	case http.StatusNotFound:
		return nil, jira.ErrNotFound
	case http.StatusBadRequest:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s", string(bodyBytes))
	default:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("projects: unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var item projectAPIItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("projects: decoding response: %w", err)
	}
	p := toProject(item)
	return &p, nil
}
