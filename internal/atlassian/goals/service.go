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
	GetGoalMetrics(ctx context.Context, goalID string) ([]MetricTarget, error)
	CreateMetric(ctx context.Context, req CreateMetricRequest) (*MetricTarget, error)
	UpdateMetricValue(ctx context.Context, req UpdateMetricValueRequest) (*MetricValue, error)
	UpdateMetricTarget(ctx context.Context, req UpdateMetricTargetRequest) error
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
    id name startDate
    targetDate { label }
    status { value score }
    owner { ... on AtlassianAccountUser { name accountId } }
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

	const query = `query SearchGoals($containerId: ID!, $searchString: String, $first: Int, $after: String) {
  goals_search(containerId: $containerId, searchString: $searchString, first: $first, after: $after, sort: [NAME_ASC]) {
    edges {
      node {
        id name startDate
        targetDate { label }
        status { value score }
        owner { ... on AtlassianAccountUser { name accountId } }
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

// GetGoalMetrics returns all MetricTargets attached to a goal.
// Returns an empty slice (not an error) when the goal has no metrics or goals_byId is null.
func (s *GoalsGraphQLService) GetGoalMetrics(ctx context.Context, goalID string) ([]MetricTarget, error) {
	const query = `query GetGoalMetrics($goalId: ID!) {
  goals_byId(goalId: $goalId) {
    metricTargets {
      edges {
        node {
          id startValue targetValue
          snapshotValue { value time }
          metric { id name type archived latestValue { id value time } }
        }
      }
    }
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

	var data goalsGetMetricsData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, fmt.Errorf("goals: decoding metricTargets: %w", err)
	}
	if data.GoalsByID == nil {
		return []MetricTarget{}, nil
	}

	edges := data.GoalsByID.MetricTargets.Edges
	result := make([]MetricTarget, len(edges))
	for i, edge := range edges {
		result[i] = edge.Node.toMetricTarget()
	}
	return result, nil
}

// CreateMetric creates a new metric and attaches it to the given goal in one GraphQL mutation.
// Returns the newly created MetricTarget from the goal's updated metricTargets list.
func (s *GoalsGraphQLService) CreateMetric(ctx context.Context, req CreateMetricRequest) (*MetricTarget, error) {
	const mutationQuery = `mutation CreateMetric($input: TownsquareGoalsCreateAddMetricTargetInput!) {
  goals_createAndAddMetricTarget(input: $input) {
    success
    errors { message }
    goal {
      metricTargets {
        edges { node { id startValue targetValue metric { id name type archived } } }
      }
    }
  }
}`
	input := createAndAddMetricTargetInput{
		GoalID:      req.GoalID,
		StartValue:  req.StartValue,
		TargetValue: req.TargetValue,
		CreateMetric: createMetricInlineInput{
			GoalID: req.GoalID,
			Name:   req.Name,
			Type:   req.MetricType,
			Value:  req.InitialValue,
		},
	}
	variables := map[string]any{"input": input}

	envelope, err := s.doGraphQL(ctx, mutationQuery, variables)
	if err != nil {
		return nil, err
	}
	if msg := firstGraphQLError(envelope.Errors); msg != "" {
		return nil, fmt.Errorf("goals: GraphQL error: %s", msg)
	}

	var data createAndAddMetricResponseData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, fmt.Errorf("goals: decoding createAndAddMetricTarget: %w", err)
	}
	payload := data.GoalsCreateAndAddMetricTarget
	if !payload.Success {
		if len(payload.Errors) > 0 {
			return nil, fmt.Errorf("%s", payload.Errors[0].Message)
		}
		return nil, fmt.Errorf("goals: createMetric failed with no error detail")
	}
	if payload.Goal == nil || len(payload.Goal.MetricTargets.Edges) == 0 {
		return nil, fmt.Errorf("goals: createMetric returned no metric targets")
	}
	edges := payload.Goal.MetricTargets.Edges
	mt := edges[len(edges)-1].Node.toMetricTarget()
	return &mt, nil
}

// UpdateMetricValue adds a new value datapoint to an existing metric.
// Returns the created MetricValue with its ID, value, and timestamp.
func (s *GoalsGraphQLService) UpdateMetricValue(ctx context.Context, req UpdateMetricValueRequest) (*MetricValue, error) {
	const mutationQuery = `mutation UpdateMetricValue($input: TownsquareGoalsCreateMetricValueInput!) {
  goals_createMetricValue(input: $input) {
    success
    errors { message }
    metricValue { id value time }
  }
}`
	input := createMetricValueInput{
		MetricID: req.MetricID,
		Value:    req.Value,
		Time:     req.Time,
	}
	variables := map[string]any{"input": input}

	envelope, err := s.doGraphQL(ctx, mutationQuery, variables)
	if err != nil {
		return nil, err
	}
	if msg := firstGraphQLError(envelope.Errors); msg != "" {
		return nil, fmt.Errorf("goals: GraphQL error: %s", msg)
	}

	var data createMetricValueResponseData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, fmt.Errorf("goals: decoding createMetricValue: %w", err)
	}
	payload := data.GoalsCreateMetricValue
	if !payload.Success {
		if len(payload.Errors) > 0 {
			return nil, fmt.Errorf("%s", payload.Errors[0].Message)
		}
		return nil, fmt.Errorf("goals: updateMetricValue failed with no error detail")
	}
	if payload.MetricValue == nil {
		return nil, fmt.Errorf("goals: updateMetricValue returned nil metricValue")
	}
	return &MetricValue{
		ID:    payload.MetricValue.ID,
		Value: payload.MetricValue.Value,
		Time:  payload.MetricValue.Time,
	}, nil
}

// UpdateMetricTarget updates the start, current, and/or target values of a MetricTarget.
// Only non-nil fields in the request are sent to the API.
func (s *GoalsGraphQLService) UpdateMetricTarget(ctx context.Context, req UpdateMetricTargetRequest) error {
	const mutationQuery = `mutation UpdateMetricTarget($input: TownsquareGoalsEditMetricTargetInput!) {
  goals_editMetricTarget(input: $input) {
    success
    errors { message }
  }
}`
	input := editMetricTargetInput{
		MetricTargetID: req.MetricTargetID,
		CurrentValue:   req.CurrentValue,
		StartValue:     req.StartValue,
		TargetValue:    req.TargetValue,
	}
	variables := map[string]any{"input": input}

	envelope, err := s.doGraphQL(ctx, mutationQuery, variables)
	if err != nil {
		return err
	}
	if msg := firstGraphQLError(envelope.Errors); msg != "" {
		return fmt.Errorf("goals: GraphQL error: %s", msg)
	}

	var data editMetricTargetResponseData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return fmt.Errorf("goals: decoding editMetricTarget: %w", err)
	}
	payload := data.GoalsEditMetricTarget
	if !payload.Success {
		if len(payload.Errors) > 0 {
			return fmt.Errorf("%s", payload.Errors[0].Message)
		}
		return fmt.Errorf("goals: updateMetricTarget failed with no error detail")
	}
	return nil
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
