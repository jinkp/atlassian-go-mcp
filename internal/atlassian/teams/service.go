package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

const (
	// teamsBaseURL is the Atlassian Teams Public REST API base URL.
	// This is ALWAYS https://api.atlassian.com — NOT the tenant baseURL (e.g. {org}.atlassian.net).
	teamsBaseURL      = "https://api.atlassian.com"
	defaultMaxResults = 50
)

// TeamsService defines read operations against the Atlassian Teams Public REST API v1.
type TeamsService interface {
	GetTeams(ctx context.Context, query string, maxResults int) (*TeamSearchResult, error)
	GetTeam(ctx context.Context, teamID string) (*Team, error)
	GetTeamMembers(ctx context.Context, teamID string, maxResults int) ([]TeamMember, error)
}

// TeamsRestService implements TeamsService against the Atlassian Teams REST API v1.
type TeamsRestService struct {
	doer  client.HTTPDoer
	orgID string
}

// NewService constructs a TeamsRestService. orgID is the Atlassian organization UUID.
// The base URL is always https://api.atlassian.com (hardcoded — NOT from env ATLASSIAN_BASE_URL).
func NewService(doer client.HTTPDoer, orgID string) TeamsService {
	return &TeamsRestService{doer: doer, orgID: orgID}
}

// orgBase returns the org-scoped base path for Teams API calls.
func (s *TeamsRestService) orgBase() string {
	return teamsBaseURL + "/public/teams/v1/org/" + s.orgID
}

// toTeam converts a wire teamAPIItem to a domain Team.
func toTeam(item teamAPIItem) Team {
	return Team{
		ID:             item.TeamId,
		DisplayName:    item.DisplayName,
		Description:    item.Description,
		OrganizationID: item.OrganizationId,
		TeamType:       item.TeamType,
		State:          item.State,
	}
}

// GetTeams lists teams for the organization with optional query filter and size.
// Returns ErrUnauthorized on 401/403, descriptive error on 400.
// Returns a non-nil empty slice when no teams exist.
func (s *TeamsRestService) GetTeams(ctx context.Context, query string, maxResults int) (*TeamSearchResult, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}
	endpoint := s.orgBase() + "/teams?size=" + strconv.Itoa(maxResults)
	if query != "" {
		endpoint += "&query=" + query
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("teams: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("teams: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success — fall through to decode
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, jira.ErrUnauthorized
	case http.StatusBadRequest:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("teams: get teams 400: %s", string(body))
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("teams: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw teamsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("teams: decoding response: %w", err)
	}

	teamList := make([]Team, len(raw.Entities))
	for i, item := range raw.Entities {
		teamList[i] = toTeam(item)
	}
	return &TeamSearchResult{Teams: teamList, NextCursor: raw.Cursor}, nil
}

// GetTeam fetches a single team by ID.
// Returns ErrUnauthorized on 401/403, ErrNotFound on 404.
func (s *TeamsRestService) GetTeam(ctx context.Context, teamID string) (*Team, error) {
	endpoint := s.orgBase() + "/teams/" + teamID

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("teams: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("teams: request failed: %w", err)
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
		return nil, fmt.Errorf("teams: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var item teamAPIItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("teams: decoding response: %w", err)
	}
	t := toTeam(item)
	return &t, nil
}

// GetTeamMembers fetches members of a team. Uses POST per the Teams REST API contract.
// Returns ErrUnauthorized on 401/403, ErrNotFound on 404.
// Returns a non-nil empty slice when the team has no members.
func (s *TeamsRestService) GetTeamMembers(ctx context.Context, teamID string, maxResults int) ([]TeamMember, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}
	endpoint := s.orgBase() + "/teams/" + teamID + "/members"

	reqBody := membersAPIRequest{First: maxResults}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("teams: marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("teams: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("teams: request failed: %w", err)
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
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("teams: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var raw membersAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("teams: decoding response: %w", err)
	}

	members := make([]TeamMember, len(raw.Results))
	for i, m := range raw.Results {
		members[i] = TeamMember{AccountID: m.AccountId}
	}
	return members, nil
}
