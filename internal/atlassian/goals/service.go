package goals

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

const defaultMaxResults = 25

// GoalsService defines read and write operations against the Atlassian Goals GraphQL API.
type GoalsService interface {
	GetSiteID(ctx context.Context, subdomain string) (string, error)
	GetGoal(ctx context.Context, goalID string) (*Goal, error)
	SearchGoals(ctx context.Context, req SearchGoalsRequest) (*GoalSearchResult, error)
	UpdateGoalStatus(ctx context.Context, req UpdateGoalStatusRequest) error
	CreateGoal(ctx context.Context, req CreateGoalRequest) (*CreateGoalResult, error)
	EditGoal(ctx context.Context, req EditGoalRequest) (*Goal, error)
}

// GoalsGraphQLService implements GoalsService via the Atlassian platform GraphQL gateway.
type GoalsGraphQLService struct {
	doer       client.HTTPDoer
	graphqlURL string // baseURL + "/gateway/api/graphql"
}

// NewService constructs a GoalsGraphQLService.
func NewService(doer client.HTTPDoer, baseURL string) GoalsService {
	return &GoalsGraphQLService{
		doer:       doer,
		graphqlURL: baseURL + "/gateway/api/graphql",
	}
}

// doGraphQL posts a GraphQL request and returns the raw response envelope.
// Handles HTTP-level errors (401/403 → ErrUnauthorized) and decodes the envelope.
// Callers MUST check envelope.Errors before reading envelope.Data.
func (s *GoalsGraphQLService) doGraphQL(ctx context.Context, query string, variables map[string]any) (*graphQLResponse, error) {
	reqBody := graphQLRequest{Query: query, Variables: variables}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("goals: marshaling GraphQL request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.graphqlURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("goals: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("goals: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through to decode
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, jira.ErrUnauthorized
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("goals: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var envelope graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("goals: decoding GraphQL response: %w", err)
	}
	return &envelope, nil
}

// GetSiteID resolves a subdomain (e.g. "myorg") to an Atlassian cloudId using tenantContexts.
func (s *GoalsGraphQLService) GetSiteID(ctx context.Context, subdomain string) (string, error) {
	const query = `query GetSiteID($hostNames: [String!]!) {
  tenantContexts(hostNames: $hostNames) {
    cloudId
  }
}`
	variables := map[string]any{
		"hostNames": []string{subdomain + ".atlassian.net"},
	}

	envelope, err := s.doGraphQL(ctx, query, variables)
	if err != nil {
		return "", err
	}
	if msg := firstGraphQLError(envelope.Errors); msg != "" {
		return "", fmt.Errorf("goals: GraphQL error: %s", msg)
	}

	var data tenantContextsData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return "", fmt.Errorf("goals: decoding tenantContexts: %w", err)
	}
	if len(data.TenantContexts) == 0 || data.TenantContexts[0].CloudID == "" {
		return "", fmt.Errorf("goals: no tenant found for subdomain %q", subdomain)
	}
	return data.TenantContexts[0].CloudID, nil
}

// GetGoal fetches a single goal by its ARI (Atlassian Resource Identifier).
func (s *GoalsGraphQLService) GetGoal(ctx context.Context, goalID string) (*Goal, error) {
	const query = `query GetGoal($goalId: ID!) {
  goals_byId(goalId: $goalId) {
    id name score targetDate startDate
    status { value }
    phase { name }
    owner { name aaid }
  }
}`
	variables := map[string]any{"goalId": goalID}

	envelope, err := s.doGraphQL(ctx, query, variables)
	if err != nil {
		return nil, err
	}
	if msg := firstGraphQLError(envelope.Errors); msg != "" {
		return nil, fmt.Errorf("goals: GraphQL error: %s", msg)
	}

	var data goalsByIdData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, fmt.Errorf("goals: decoding goals_byId: %w", err)
	}
	if data.GoalsByID == nil {
		return nil, jira.ErrNotFound
	}
	g := data.GoalsByID.toGoal()
	return &g, nil
}

// SearchGoals searches for goals using the goals_search query with JQL-like syntax.
func (s *GoalsGraphQLService) SearchGoals(ctx context.Context, req SearchGoalsRequest) (*GoalSearchResult, error) {
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	containerId := "ari:cloud:townsquare::site/" + req.SiteID

	const query = `query SearchGoals($containerId: String!, $searchString: String, $first: Int, $after: String) {
  goals_search(containerId: $containerId, searchString: $searchString, first: $first, after: $after, sort: [NAME_ASC]) {
    edges {
      node {
        id name score targetDate startDate
        status { value }
        phase { name }
        owner { name aaid }
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}`
	variables := map[string]any{
		"containerId":  containerId,
		"searchString": req.SearchString,
		"first":        maxResults,
	}
	if req.Cursor != "" {
		variables["after"] = req.Cursor
	}

	envelope, err := s.doGraphQL(ctx, query, variables)
	if err != nil {
		return nil, err
	}
	if msg := firstGraphQLError(envelope.Errors); msg != "" {
		return nil, fmt.Errorf("goals: GraphQL error: %s", msg)
	}

	var data goalsSearchData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, fmt.Errorf("goals: decoding goals_search: %w", err)
	}

	goalsList := make([]Goal, len(data.GoalsSearch.Edges))
	for i, edge := range data.GoalsSearch.Edges {
		goalsList[i] = edge.Node.toGoal()
	}

	return &GoalSearchResult{
		Goals:      goalsList,
		HasMore:    data.GoalsSearch.PageInfo.HasNextPage,
		NextCursor: data.GoalsSearch.PageInfo.EndCursor,
	}, nil
}

// CreateGoal creates a new goal via the goals_create GraphQL mutation.
func (s *GoalsGraphQLService) CreateGoal(ctx context.Context, req CreateGoalRequest) (*CreateGoalResult, error) {
	const mutationQuery = `mutation CreateGoal($input: goals_CreateGoalInput!) {
  goals_create(input: $input) {
    goal { id name }
    errors { message }
  }
}`

	confidence := req.Confidence
	if confidence == "" {
		confidence = "QUARTER"
	}

	input := createGoalAPIInput{
		ContainerID: "ari:cloud:townsquare::site/" + req.SiteID,
		Name:        req.Name,
		GoalTypeID:  req.GoalTypeID,
		TargetDate: createGoalTargetDate{
			Date:       req.TargetDate,
			Confidence: confidence,
		},
	}
	if req.Description != "" {
		input.Summary = plainTextToADF(req.Description)
	}

	variables := map[string]any{"input": input}

	envelope, err := s.doGraphQL(ctx, mutationQuery, variables)
	if err != nil {
		return nil, err
	}
	if msg := firstGraphQLError(envelope.Errors); msg != "" {
		return nil, fmt.Errorf("goals: GraphQL error: %s", msg)
	}

	var data goalsCreateData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, fmt.Errorf("goals: decoding goals_create: %w", err)
	}
	if len(data.GoalsCreate.Errors) > 0 {
		return nil, fmt.Errorf("%s", data.GoalsCreate.Errors[0].Message)
	}

	return &CreateGoalResult{
		ID:   data.GoalsCreate.Goal.ID,
		Name: data.GoalsCreate.Goal.Name,
	}, nil
}

// EditGoal updates structural fields of a goal (name, targetDate, isArchived) via the goals_edit mutation.
// Returns the updated goal or an error. Business validation errors are returned via userErrors in the
// response payload — these are distinct from transport-level errors in the root errors[] field.
func (s *GoalsGraphQLService) EditGoal(ctx context.Context, req EditGoalRequest) (*Goal, error) {
	const mutationQuery = `mutation EditGoal($input: goals_EditGoalInput!) {
  goals_edit(input: $input) {
    goal { id name targetDate isArchived }
    userErrors { field message }
  }
}`

	input := editGoalAPIInput{
		GoalID:     req.GoalID,
		Name:       req.Name,
		IsArchived: req.Archive,
	}
	if req.TargetDate != nil {
		conf := "QUARTER"
		if req.Confidence != nil {
			conf = *req.Confidence
		}
		input.TargetDate = &editGoalTargetDate{
			Date:       *req.TargetDate,
			Confidence: conf,
		}
	}

	variables := map[string]any{"input": input}

	envelope, err := s.doGraphQL(ctx, mutationQuery, variables)
	if err != nil {
		return nil, err
	}
	if msg := firstGraphQLError(envelope.Errors); msg != "" {
		return nil, fmt.Errorf("goals: GraphQL error: %s", msg)
	}

	var data goalsEditData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, fmt.Errorf("goals: decoding goals_edit: %w", err)
	}
	if len(data.GoalsEdit.UserErrors) > 0 {
		return nil, fmt.Errorf("%s", data.GoalsEdit.UserErrors[0].Message)
	}
	if data.GoalsEdit.Goal == nil {
		return nil, fmt.Errorf("goals: edit returned nil goal")
	}

	g := &Goal{
		ID:         data.GoalsEdit.Goal.ID,
		Name:       data.GoalsEdit.Goal.Name,
		TargetDate: data.GoalsEdit.Goal.TargetDate,
	}
	return g, nil
}

// UpdateGoalStatus posts a check-in update to a goal with new status, optional score, optional summary.
func (s *GoalsGraphQLService) UpdateGoalStatus(ctx context.Context, req UpdateGoalStatusRequest) error {
	const mutationQuery = `mutation UpdateGoalStatus($input: GoalsCreateUpdateInput!) {
  goals_createUpdate(input: $input) {
    success
    errors { message }
  }
}`
	input := goalsCreateUpdateInput{
		GoalID: req.GoalID,
		Status: req.Status,
	}
	if req.Score != 0 {
		score := req.Score
		input.Score = &score
	}
	if req.Summary != "" {
		input.Summary = plainTextToADF(req.Summary)
	}

	variables := map[string]any{"input": input}

	envelope, err := s.doGraphQL(ctx, mutationQuery, variables)
	if err != nil {
		return err
	}
	if msg := firstGraphQLError(envelope.Errors); msg != "" {
		return fmt.Errorf("goals: GraphQL error: %s", msg)
	}

	var data goalsCreateUpdateData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return fmt.Errorf("goals: decoding goals_createUpdate: %w", err)
	}
	if !data.GoalsCreateUpdate.Success {
		if len(data.GoalsCreateUpdate.Errors) > 0 {
			return fmt.Errorf("%s", data.GoalsCreateUpdate.Errors[0].Message)
		}
		return fmt.Errorf("goals: update failed with no error detail")
	}
	return nil
}
