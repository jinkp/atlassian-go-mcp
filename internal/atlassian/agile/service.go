package agile

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

// AgileService defines read and write operations against the Jira Agile REST API v1.0.
type AgileService interface {
	GetBoards(ctx context.Context, projectKey string, maxResults int) ([]Board, error)
	GetSprints(ctx context.Context, boardID int, state string, maxResults int) ([]Sprint, error)
	GetSprintIssues(ctx context.Context, sprintID int, maxResults int) (*SprintIssueResult, error)
	UpdateSprint(ctx context.Context, sprintID int, req UpdateSprintRequest) (*Sprint, error)
	MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error
	MoveIssuesToEpic(ctx context.Context, epicKey string, issueKeys []string) error
	CreateSprint(ctx context.Context, req CreateSprintRequest) (*Sprint, error)
}

// AgileJiraService implements AgileService against the Jira Agile REST API.
type AgileJiraService struct {
	doer    client.HTTPDoer
	baseURL string
}

// NewService constructs an AgileJiraService. The doer is typically a *http.Client
// from httptest in tests, or a *client.Client in production.
func NewService(doer client.HTTPDoer, baseURL string) AgileService {
	return &AgileJiraService{
		doer:    doer,
		baseURL: baseURL,
	}
}

// GetBoards fetches all Jira Software boards matching the given projectKey.
// Returns ErrUnauthorized on 401/403, ErrNotFound on 404, descriptive error on 400.
// Returns an empty (non-nil) slice when no boards exist for the project.
func (s *AgileJiraService) GetBoards(ctx context.Context, projectKey string, maxResults int) ([]Board, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	endpoint := s.baseURL + "/rest/agile/1.0/board" +
		"?projectKeyOrId=" + projectKey +
		"&maxResults=" + strconv.Itoa(maxResults)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("agile: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agile: request failed: %w", err)
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
		return nil, fmt.Errorf("agile: get boards 400: %s", string(body))
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agile: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw boardsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("agile: decoding boards response: %w", err)
	}

	boards := make([]Board, len(raw.Values))
	for i, item := range raw.Values {
		boards[i] = Board{
			ID:   item.ID,
			Name: item.Name,
			Type: item.Type.Name,
		}
	}
	return boards, nil
}

// GetSprints fetches sprints for a board. state filters by sprint state
// ("active", "future", "closed"); pass "" for all. Returns ErrUnauthorized on
// 401/403, ErrNotFound on 404. Returns an empty (non-nil) slice when none exist.
func (s *AgileJiraService) GetSprints(ctx context.Context, boardID int, state string, maxResults int) ([]Sprint, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	endpoint := s.baseURL + "/rest/agile/1.0/board/" + strconv.Itoa(boardID) + "/sprint" +
		"?maxResults=" + strconv.Itoa(maxResults)
	if state != "" {
		endpoint += "&state=" + state
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("agile: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agile: request failed: %w", err)
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
		return nil, fmt.Errorf("agile: get sprints 400: %s", string(body))
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agile: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw sprintsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("agile: decoding sprints response: %w", err)
	}

	sprints := make([]Sprint, len(raw.Values))
	for i, item := range raw.Values {
		sprints[i] = Sprint{
			ID:           item.ID,
			Name:         item.Name,
			State:        item.State,
			StartDate:    item.StartDate,
			EndDate:      item.EndDate,
			CompleteDate: item.CompleteDate,
			BoardID:      boardID,
		}
	}
	return sprints, nil
}

// GetSprintIssues fetches issues for a sprint. Returns a non-nil SprintIssueResult
// even when the sprint is empty. Returns ErrUnauthorized on 401/403, ErrNotFound on 404.
func (s *AgileJiraService) GetSprintIssues(ctx context.Context, sprintID int, maxResults int) (*SprintIssueResult, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	endpoint := s.baseURL + "/rest/agile/1.0/sprint/" + strconv.Itoa(sprintID) + "/issue" +
		"?maxResults=" + strconv.Itoa(maxResults)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("agile: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agile: request failed: %w", err)
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
		return nil, fmt.Errorf("agile: get sprint issues 400: %s", string(body))
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agile: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var raw sprintIssuesAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("agile: decoding sprint issues response: %w", err)
	}

	issues := make([]SprintIssue, len(raw.Issues))
	for i, item := range raw.Issues {
		assignee := ""
		if item.Fields.Assignee != nil {
			assignee = item.Fields.Assignee.DisplayName
		}
		issues[i] = SprintIssue{
			Key:      item.Key,
			Summary:  item.Fields.Summary,
			Status:   item.Fields.Status.Name,
			Assignee: assignee,
		}
	}

	return &SprintIssueResult{
		Issues:     issues,
		Total:      raw.Total,
		StartAt:    raw.StartAt,
		MaxResults: raw.MaxResults,
	}, nil
}

// UpdateSprint updates a sprint's name, state, or dates.
// Returns the updated Sprint on 200, ErrUnauthorized on 401/403, ErrNotFound on 404.
// Returns a descriptive error on 400 (e.g. "Sprint is not active").
func (s *AgileJiraService) UpdateSprint(ctx context.Context, sprintID int, req UpdateSprintRequest) (*Sprint, error) {
	body := updateSprintAPIRequest{}
	if req.Name != nil {
		body.Name = *req.Name
	}
	if req.State != nil {
		body.State = *req.State
	}
	if req.StartDate != nil {
		body.StartDate = *req.StartDate
	}
	if req.EndDate != nil {
		body.EndDate = *req.EndDate
	}

	endpoint := s.baseURL + "/rest/agile/1.0/sprint/" + strconv.Itoa(sprintID)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("agile: marshaling update sprint request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("agile: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("agile: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through to decode
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, jira.ErrUnauthorized
	case http.StatusNotFound:
		return nil, jira.ErrNotFound
	case http.StatusBadRequest:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s", string(bodyBytes))
	default:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agile: unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var raw sprintAPIItem
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("agile: decoding update sprint response: %w", err)
	}

	return &Sprint{
		ID:           raw.ID,
		Name:         raw.Name,
		State:        raw.State,
		StartDate:    raw.StartDate,
		EndDate:      raw.EndDate,
		CompleteDate: raw.CompleteDate,
	}, nil
}

// MoveIssuesToSprint moves the given issue keys into the specified sprint.
// Returns nil on 204, ErrUnauthorized on 401/403, ErrNotFound on 404.
func (s *AgileJiraService) MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error {
	endpoint := s.baseURL + "/rest/agile/1.0/sprint/" + strconv.Itoa(sprintID) + "/issue"

	body := moveIssuesAPIRequest{Issues: issueKeys}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("agile: marshaling move issues request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("agile: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return fmt.Errorf("agile: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return jira.ErrUnauthorized
	case http.StatusNotFound:
		return jira.ErrNotFound
	case http.StatusBadRequest:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(bodyBytes))
	default:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agile: unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

// CreateSprint creates a new sprint on the given board.
// Returns the created Sprint on 201, ErrUnauthorized on 401/403, ErrNotFound on 404.
// Returns a descriptive error on 400 (e.g. "Board does not support sprints").
func (s *AgileJiraService) CreateSprint(ctx context.Context, req CreateSprintRequest) (*Sprint, error) {
	endpoint := s.baseURL + "/rest/agile/1.0/sprint"

	body := createSprintAPIRequest{
		Name:          req.Name,
		OriginBoardID: req.BoardID,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("agile: marshaling create sprint request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("agile: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("agile: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		// fall through to decode
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, jira.ErrUnauthorized
	case http.StatusNotFound:
		return nil, jira.ErrNotFound
	case http.StatusBadRequest:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s", string(bodyBytes))
	default:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agile: unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var raw sprintAPIItem
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("agile: decoding create sprint response: %w", err)
	}

	return &Sprint{
		ID:           raw.ID,
		Name:         raw.Name,
		State:        raw.State,
		StartDate:    raw.StartDate,
		EndDate:      raw.EndDate,
		CompleteDate: raw.CompleteDate,
	}, nil
}

// MoveIssuesToEpic links the given issue keys to the specified epic.
// Returns nil on 204, ErrUnauthorized on 401/403, ErrNotFound on 404.
func (s *AgileJiraService) MoveIssuesToEpic(ctx context.Context, epicKey string, issueKeys []string) error {
	endpoint := s.baseURL + "/rest/agile/1.0/epic/" + epicKey + "/issue"

	body := moveIssuesAPIRequest{Issues: issueKeys}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("agile: marshaling move issues request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("agile: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return fmt.Errorf("agile: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return jira.ErrUnauthorized
	case http.StatusNotFound:
		return jira.ErrNotFound
	case http.StatusBadRequest:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(bodyBytes))
	default:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agile: unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}
}
