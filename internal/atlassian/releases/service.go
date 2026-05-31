package releases

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

// ReleasesService defines read and write operations against the Jira Versions REST API v3.
type ReleasesService interface {
	GetReleases(ctx context.Context, projectKey string) ([]Release, error)
	GetRelease(ctx context.Context, releaseID string) (*Release, error)
	GetReleaseIssueCounts(ctx context.Context, releaseID string) (*ReleaseIssueCounts, error)
	CreateRelease(ctx context.Context, req CreateReleaseRequest) (*Release, error)
	UpdateRelease(ctx context.Context, releaseID string, req UpdateReleaseRequest) (*Release, error)
}

// ReleasesJiraService implements ReleasesService against the Jira REST API v3.
type ReleasesJiraService struct {
	doer    client.HTTPDoer
	baseURL string
}

// NewService constructs a ReleasesJiraService. The doer is typically a *http.Client
// from httptest in tests, or a *client.Client in production.
func NewService(doer client.HTTPDoer, baseURL string) ReleasesService {
	return &ReleasesJiraService{doer: doer, baseURL: baseURL}
}

// toRelease converts a wire releasesAPIItem to a domain Release.
// projectId (int) is converted to string via strconv.Itoa.
func toRelease(item releasesAPIItem) Release {
	return Release{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		Archived:    item.Archived,
		Released:    item.Released,
		StartDate:   item.StartDate,
		ReleaseDate: item.ReleaseDate,
		ProjectID:   strconv.Itoa(item.ProjectID),
	}
}

// GetReleases fetches all versions for a project.
// Returns ErrUnauthorized on 401/403, ErrNotFound on 404, descriptive error on 400.
// Returns a non-nil empty slice when the project has no versions.
func (s *ReleasesJiraService) GetReleases(ctx context.Context, projectKey string) ([]Release, error) {
	endpoint := s.baseURL + "/rest/api/3/project/" + projectKey + "/versions"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("releases: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("releases: request failed: %w", err)
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
		return nil, fmt.Errorf("releases: get releases 400: %s", string(body))
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("releases: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var items []releasesAPIItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("releases: decoding response: %w", err)
	}

	releases := make([]Release, len(items))
	for i, item := range items {
		releases[i] = toRelease(item)
	}
	return releases, nil
}

// GetRelease fetches a single version by ID.
// Returns ErrUnauthorized on 401/403, ErrNotFound on 404.
func (s *ReleasesJiraService) GetRelease(ctx context.Context, releaseID string) (*Release, error) {
	endpoint := s.baseURL + "/rest/api/3/version/" + releaseID

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("releases: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("releases: request failed: %w", err)
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
		return nil, fmt.Errorf("releases: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var item releasesAPIItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("releases: decoding response: %w", err)
	}
	r := toRelease(item)
	return &r, nil
}

// GetReleaseIssueCounts fetches related issue counts for a version.
// Returns ErrUnauthorized on 401/403, ErrNotFound on 404.
func (s *ReleasesJiraService) GetReleaseIssueCounts(ctx context.Context, releaseID string) (*ReleaseIssueCounts, error) {
	endpoint := s.baseURL + "/rest/api/3/version/" + releaseID + "/relatedIssueCounts"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("releases: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("releases: request failed: %w", err)
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
		return nil, fmt.Errorf("releases: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw releaseIssueCountsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("releases: decoding response: %w", err)
	}
	return &ReleaseIssueCounts{
		FixVersion:     raw.FixVersion,
		AffectsVersion: raw.AffectsVersion,
	}, nil
}

// CreateRelease creates a new version in Jira. Returns the created Release on 201.
// Returns ErrUnauthorized on 401/403, descriptive error on 400.
func (s *ReleasesJiraService) CreateRelease(ctx context.Context, req CreateReleaseRequest) (*Release, error) {
	projectIDInt, _ := strconv.Atoi(req.ProjectID)
	body := createReleaseAPIRequest{
		ProjectID:   projectIDInt,
		Name:        req.Name,
		Description: req.Description,
		StartDate:   req.StartDate,
		ReleaseDate: req.ReleaseDate,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("releases: marshaling create request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/rest/api/3/version", strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("releases: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("releases: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		// success
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, jira.ErrUnauthorized
	case http.StatusBadRequest:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s", string(bodyBytes))
	default:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("releases: unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var item releasesAPIItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("releases: decoding response: %w", err)
	}
	r := toRelease(item)
	return &r, nil
}

// UpdateRelease updates a version in Jira. Nil pointer fields are omitted from the request.
// Returns the updated Release on 200, ErrUnauthorized on 401/403, ErrNotFound on 404.
func (s *ReleasesJiraService) UpdateRelease(ctx context.Context, releaseID string, req UpdateReleaseRequest) (*Release, error) {
	body := updateReleaseAPIRequest{}
	if req.Name != nil {
		body.Name = *req.Name
	}
	if req.Description != nil {
		body.Description = *req.Description
	}
	if req.Released != nil {
		body.Released = req.Released
	}
	if req.Archived != nil {
		body.Archived = req.Archived
	}
	if req.ReleaseDate != nil {
		body.ReleaseDate = *req.ReleaseDate
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("releases: marshaling update request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut,
		s.baseURL+"/rest/api/3/version/"+releaseID, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("releases: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("releases: request failed: %w", err)
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
		return nil, fmt.Errorf("releases: unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var item releasesAPIItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("releases: decoding response: %w", err)
	}
	r := toRelease(item)
	return &r, nil
}
