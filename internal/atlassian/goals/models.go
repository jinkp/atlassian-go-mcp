// Package goals provides types and a service client for the Atlassian Goals GraphQL API.
// It mirrors the agile package structure: interface, concrete impl, and domain models.
package goals

import (
	"encoding/json"
)

// Goal is the domain model for an Atlassian Goal.
type Goal struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`      // on_track | off_track | at_risk
	Phase      string `json:"phase"`       // pending | in_progress | done | paused | cancelled
	Score      int    `json:"score"`
	TargetDate string `json:"target_date,omitempty"`
	StartDate  string `json:"start_date,omitempty"`
	OwnerName  string `json:"owner_name,omitempty"`
	OwnerID    string `json:"owner_id,omitempty"` // Atlassian account ID (aaid)
}

// SearchGoalsRequest holds search parameters for goals_search.
type SearchGoalsRequest struct {
	SiteID       string // required; used to build containerId ARI
	SearchString string // optional; JQL-like Goals search syntax
	MaxResults   int    // default 25 when zero
	Cursor       string // optional; for cursor-based pagination
}

// GoalSearchResult holds a paginated page of goals.
type GoalSearchResult struct {
	Goals      []Goal `json:"goals"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// UpdateGoalStatusRequest holds parameters for goals_createUpdate mutation.
type UpdateGoalStatusRequest struct {
	GoalID  string // required; ARI e.g. "ari:cloud:townsquare:{siteId}:goal/{uuid}"
	Status  string // required; on_track | off_track | at_risk
	Score   int    // optional; 0 = omit
	Summary string // optional; plain text; wrapped to ADF internally
}

// CreateGoalRequest holds parameters for the goals_create mutation.
type CreateGoalRequest struct {
	SiteID      string // required; used to build containerId ARI
	Name        string // required
	GoalTypeID  string // required; per-tenant ARI
	TargetDate  string // required; YYYY-MM-DD
	Confidence  string // optional; default "QUARTER"; QUARTER|DAY|WEEK|MONTH|YEAR
	Description string // optional; plain text; wrapped to ADF as summary if non-empty
}

// CreateGoalResult holds the newly created goal's identity.
type CreateGoalResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// --- unexported wire types for GraphQL request/response ---

// graphQLRequest is the standard GraphQL-over-HTTP request body.
type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphQLResponse is the standard GraphQL-over-HTTP response envelope.
// Both data and errors can coexist. Always check errors first.
type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}

type graphQLError struct {
	Message string `json:"message"`
}

// tenantContextsData is the shape of {"data":{"tenantContexts":[{"cloudId":"..."}]}}
type tenantContextsData struct {
	TenantContexts []struct {
		CloudID string `json:"cloudId"`
	} `json:"tenantContexts"`
}

// goalsByIdData is the shape of {"data":{"goals_byId":{...}}}
type goalsByIdData struct {
	GoalsByID *goalAPIItem `json:"goals_byId"`
}

// goalsSearchData is the shape of {"data":{"goals_search":{...}}}
type goalsSearchData struct {
	GoalsSearch struct {
		Edges []struct {
			Node goalAPIItem `json:"node"`
		} `json:"edges"`
		PageInfo struct {
			HasNextPage bool   `json:"hasNextPage"`
			EndCursor   string `json:"endCursor"`
		} `json:"pageInfo"`
	} `json:"goals_search"`
}

// goalAPIItem is the wire-format goal node returned by goals_byId and goals_search edges.
type goalAPIItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TargetDate *struct {
		Label string `json:"label"`
	} `json:"targetDate"`
	StartDate string `json:"startDate"`
	Status    struct {
		Value string   `json:"value"`
		Score *float64 `json:"score"`
	} `json:"status"`
	Owner *struct {
		Name      string `json:"name"`
		AccountID string `json:"accountId"`
	} `json:"owner"`
}

// toGoal converts a wire goalAPIItem to the domain Goal model.
func (item goalAPIItem) toGoal() Goal {
	g := Goal{
		ID:        item.ID,
		Name:      item.Name,
		Status:    item.Status.Value,
		StartDate: item.StartDate,
	}
	if item.Status.Score != nil {
		g.Score = int(*item.Status.Score)
	}
	if item.TargetDate != nil {
		g.TargetDate = item.TargetDate.Label
	}
	if item.Owner != nil {
		g.OwnerName = item.Owner.Name
		g.OwnerID = item.Owner.AccountID
	}
	return g
}

// goalsCreateUpdateData is the shape of {"data":{"goals_createUpdate":{...}}}
type goalsCreateUpdateData struct {
	GoalsCreateUpdate struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	} `json:"goals_createUpdate"`
}

// goalsCreateUpdateInput is the variables map for the goals_createUpdate mutation.
// Score is omitted when zero (use pointer to allow omitempty).
type goalsCreateUpdateInput struct {
	GoalID  string `json:"goalId"`
	Status  string `json:"status"`
	Score   *int   `json:"score,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// plainTextToADF wraps plain text in a minimal ADF document string (JSON-encoded).
// Required by goals_createUpdate summary field.
func plainTextToADF(text string) string {
	doc := map[string]any{
		"version": 1,
		"type":    "doc",
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": text,
					},
				},
			},
		},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

// createGoalAPIInput is the variables map input for the goals_create mutation.
type createGoalAPIInput struct {
	ContainerID string               `json:"containerId"`
	Name        string               `json:"name"`
	GoalTypeID  string               `json:"goalTypeId"`
	TargetDate  createGoalTargetDate `json:"targetDate"`
	Summary     string               `json:"summary,omitempty"`
}

// createGoalTargetDate holds the target date + confidence for goal creation.
type createGoalTargetDate struct {
	Date       string `json:"date"`
	Confidence string `json:"confidence"`
}

// goalsCreateData is the shape of {"data":{"goals_create":{...}}}
type goalsCreateData struct {
	GoalsCreate struct {
		Goal struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"goal"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	} `json:"goals_create"`
}

// EditGoalRequest holds parameters for the goals_edit mutation.
// All fields except GoalID are optional (nil = omit).
type EditGoalRequest struct {
	GoalID     string  // required; ARI
	Name       *string // optional; new goal name
	TargetDate *string // optional; YYYY-MM-DD
	Confidence *string // optional; default "QUARTER" when TargetDate set and Confidence nil
	Archive    *bool   // optional; true = archive, false = unarchive
}

// editGoalAPIInput is the input type for the goals_edit mutation.
type editGoalAPIInput struct {
	GoalID     string              `json:"goalId"`
	Name       *string             `json:"name,omitempty"`
	TargetDate *editGoalTargetDate `json:"targetDate,omitempty"`
	IsArchived *bool               `json:"isArchived,omitempty"`
}

// editGoalTargetDate holds the target date + confidence for goal edits.
type editGoalTargetDate struct {
	Date       string `json:"date"`
	Confidence string `json:"confidence"`
}

// goalsEditResponseGoal is the wire-format goal returned by goals_edit.
type goalsEditResponseGoal struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TargetDate string `json:"targetDate"`
	IsArchived bool   `json:"isArchived"`
}

// goalsEditData is the shape of {"data":{"goals_edit":{"goal":{...},"userErrors":[...]}}}
type goalsEditData struct {
	GoalsEdit struct {
		Goal       *goalsEditResponseGoal `json:"goal"`
		UserErrors []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"userErrors"`
	} `json:"goals_edit"`
}

// firstGraphQLError returns the message of the first error in the errors array, or "".
func firstGraphQLError(errs []graphQLError) string {
	if len(errs) > 0 {
		return errs[0].Message
	}
	return ""
}

// --- Domain models for Goal Metrics ---

// Metric is the domain model for an Atlassian Goal metric.
// Type is one of CURRENCY | NUMERIC | PERCENTAGE.
type Metric struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Archived    bool         `json:"archived"`
	LatestValue *MetricValue `json:"latest_value,omitempty"`
}

// MetricValue is a single value datapoint for a metric.
type MetricValue struct {
	ID    string  `json:"id"`
	Value float64 `json:"value"`
	Time  string  `json:"time,omitempty"`
}

// MetricTarget links a Metric to a Goal with start/target/current values.
type MetricTarget struct {
	ID           string   `json:"id"`
	Metric       Metric   `json:"metric"`
	StartValue   float64  `json:"start_value"`
	TargetValue  float64  `json:"target_value"`
	CurrentValue *float64 `json:"current_value,omitempty"`
}

// CreateMetricRequest holds parameters for creating a new metric attached to a goal.
type CreateMetricRequest struct {
	GoalID       string  // required
	Name         string  // required
	MetricType   string  // required: CURRENCY | NUMERIC | PERCENTAGE
	StartValue   float64 // required
	TargetValue  float64 // required
	InitialValue float64 // required; maps to createMetric.value (initial current value)
}

// UpdateMetricValueRequest holds parameters for adding a value datapoint to a metric.
type UpdateMetricValueRequest struct {
	MetricID string  // required
	Value    float64 // required
	Time     string  // optional ISO 8601
}

// UpdateMetricTargetRequest holds parameters for updating a MetricTarget's values.
type UpdateMetricTargetRequest struct {
	MetricTargetID string   // required
	CurrentValue   *float64 // optional; nil = omit
	StartValue     *float64 // optional; nil = omit
	TargetValue    *float64 // optional; nil = omit
}

// --- Wire types for Goal Metrics (unexported) ---

// goalsGetMetricsData decodes the goals_byId.metricTargets response.
// Separate from goalsByIdData to avoid conflicting selection sets.
type goalsGetMetricsData struct {
	GoalsByID *struct {
		MetricTargets struct {
			Edges []struct {
				Node metricTargetAPIItem `json:"node"`
			} `json:"edges"`
		} `json:"metricTargets"`
	} `json:"goals_byId"`
}

// metricTargetAPIItem is the wire-format MetricTarget node returned by GraphQL.
type metricTargetAPIItem struct {
	ID          string  `json:"id"`
	StartValue  float64 `json:"startValue"`
	TargetValue float64 `json:"targetValue"`
	SnapshotValue *struct {
		Value float64 `json:"value"`
		Time  string  `json:"time"`
	} `json:"snapshotValue"`
	Metric struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		Archived bool   `json:"archived"`
		LatestValue *struct {
			ID    string  `json:"id"`
			Value float64 `json:"value"`
			Time  string  `json:"time"`
		} `json:"latestValue"`
	} `json:"metric"`
}

// toMetricTarget converts a wire metricTargetAPIItem to the domain MetricTarget model.
func (item metricTargetAPIItem) toMetricTarget() MetricTarget {
	mt := MetricTarget{
		ID:          item.ID,
		StartValue:  item.StartValue,
		TargetValue: item.TargetValue,
		Metric: Metric{
			ID:       item.Metric.ID,
			Name:     item.Metric.Name,
			Type:     item.Metric.Type,
			Archived: item.Metric.Archived,
		},
	}
	if item.SnapshotValue != nil {
		v := item.SnapshotValue.Value
		mt.CurrentValue = &v
	}
	if item.Metric.LatestValue != nil {
		mt.Metric.LatestValue = &MetricValue{
			ID:    item.Metric.LatestValue.ID,
			Value: item.Metric.LatestValue.Value,
			Time:  item.Metric.LatestValue.Time,
		}
	}
	return mt
}

// createAndAddMetricTargetInput is the GraphQL input for goals_createAndAddMetricTarget.
type createAndAddMetricTargetInput struct {
	GoalID       string                  `json:"goalId"`
	StartValue   float64                 `json:"startValue"`
	TargetValue  float64                 `json:"targetValue"`
	CreateMetric createMetricInlineInput `json:"createMetric"`
}

// createMetricInlineInput is the nested createMetric field inside createAndAddMetricTargetInput.
type createMetricInlineInput struct {
	GoalID string  `json:"goalId"`
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Value  float64 `json:"value"`
}

// createAndAddMetricResponseData decodes the goals_createAndAddMetricTarget response.
type createAndAddMetricResponseData struct {
	GoalsCreateAndAddMetricTarget struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Goal *struct {
			MetricTargets struct {
				Edges []struct {
					Node metricTargetAPIItem `json:"node"`
				} `json:"edges"`
			} `json:"metricTargets"`
		} `json:"goal"`
	} `json:"goals_createAndAddMetricTarget"`
}

// createMetricValueInput is the GraphQL input for goals_createMetricValue.
type createMetricValueInput struct {
	MetricID string  `json:"metricId"`
	Value    float64 `json:"value"`
	Time     string  `json:"time,omitempty"`
}

// createMetricValueResponseData decodes the goals_createMetricValue response.
type createMetricValueResponseData struct {
	GoalsCreateMetricValue struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
		MetricValue *metricValueAPIItem `json:"metricValue"`
	} `json:"goals_createMetricValue"`
}

// metricValueAPIItem is the wire-format MetricValue returned by goals_createMetricValue.
type metricValueAPIItem struct {
	ID    string  `json:"id"`
	Value float64 `json:"value"`
	Time  string  `json:"time"`
}

// editMetricTargetInput is the GraphQL input for goals_editMetricTarget.
type editMetricTargetInput struct {
	MetricTargetID string   `json:"metricTargetId"`
	CurrentValue   *float64 `json:"currentValue,omitempty"`
	StartValue     *float64 `json:"startValue,omitempty"`
	TargetValue    *float64 `json:"targetValue,omitempty"`
}

// editMetricTargetResponseData decodes the goals_editMetricTarget response.
type editMetricTargetResponseData struct {
	GoalsEditMetricTarget struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	} `json:"goals_editMetricTarget"`
}


