package goals

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// newTestService creates a GoalsGraphQLService pointing at the given test server URL.
func newTestService(serverURL string) GoalsService {
	return &GoalsGraphQLService{
		doer:       http.DefaultClient,
		graphqlURL: serverURL + "/gateway/api/graphql",
	}
}

// capturedRequest holds the last request body sent to the test server.
type capturedRequest struct {
	body []byte
}

// mustReadBody reads and returns all bytes from r, ignoring errors.
func mustReadBody(r io.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}

// --- TestGetSiteID ---

func TestGetSiteID(t *testing.T) {
	tests := []struct {
		name        string
		handler     func(w http.ResponseWriter, r *http.Request)
		subdomain   string
		wantID      string
		wantErrIs   error
		wantErrMsg  string
	}{
		{
			name:      "success - valid subdomain",
			subdomain: "myorg",
			wantID:    "abc-123",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"tenantContexts":[{"cloudId":"abc-123"}]}}`))
			},
		},
		{
			name:       "empty contexts - not found",
			subdomain:  "notexist",
			wantID:     "",
			wantErrMsg: "no tenant",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"tenantContexts":[]}}`))
			},
		},
		{
			name:      "HTTP 401 - unauthorized",
			subdomain: "myorg",
			wantID:    "",
			wantErrIs: jira.ErrUnauthorized,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
		},
		{
			name:       "GraphQL errors array - forbidden",
			subdomain:  "myorg",
			wantID:     "",
			wantErrMsg: "forbidden",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"errors":[{"message":"forbidden"}]}`))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(tc.handler))
			defer srv.Close()

			svc := newTestService(srv.URL)
			got, err := svc.GetSiteID(context.Background(), tc.subdomain)

			if tc.wantErrIs != nil {
				if !errors.Is(err, tc.wantErrIs) {
					t.Errorf("error: got %v, want %v", err, tc.wantErrIs)
				}
				return
			}
			if tc.wantErrMsg != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantID {
				t.Errorf("cloudId: got %q, want %q", got, tc.wantID)
			}
		})
	}
}

// --- TestGetGoal ---

func TestGetGoal(t *testing.T) {
	fullGoalJSON := `{
		"data": {
			"goals_byId": {
				"id": "ari:cloud:townsquare:abc:goal/xyz",
				"name": "Increase Revenue",
				"score": 75,
				"targetDate": "2026-12-31",
				"startDate": "2026-01-01",
				"status": {"value": "on_track"},
				"phase": {"name": "in_progress"},
				"owner": {"name": "Alice", "aaid": "acct-001"}
			}
		}
	}`

	tests := []struct {
		name       string
		handler    func(w http.ResponseWriter, r *http.Request)
		goalID     string
		wantGoal   *Goal
		wantErrIs  error
		wantErrMsg string
	}{
		{
			name:   "success - full goal fields",
			goalID: "ari:cloud:townsquare:abc:goal/xyz",
			wantGoal: &Goal{
				ID:         "ari:cloud:townsquare:abc:goal/xyz",
				Name:       "Increase Revenue",
				Status:     "on_track",
				Phase:      "in_progress",
				Score:      75,
				TargetDate: "2026-12-31",
				StartDate:  "2026-01-01",
				OwnerName:  "Alice",
				OwnerID:    "acct-001",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(fullGoalJSON))
			},
		},
		{
			name:      "null goals_byId - ErrNotFound",
			goalID:    "ari:cloud:townsquare:abc:goal/missing",
			wantErrIs: jira.ErrNotFound,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_byId":null}}`))
			},
		},
		{
			name:      "HTTP 401 - unauthorized",
			goalID:    "ari:cloud:townsquare:abc:goal/xyz",
			wantErrIs: jira.ErrUnauthorized,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
		},
		{
			name:       "GraphQL errors array",
			goalID:     "ari:cloud:townsquare:abc:goal/xyz",
			wantErrMsg: "access denied",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"errors":[{"message":"access denied"}]}`))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(tc.handler))
			defer srv.Close()

			svc := newTestService(srv.URL)
			got, err := svc.GetGoal(context.Background(), tc.goalID)

			if tc.wantErrIs != nil {
				if !errors.Is(err, tc.wantErrIs) {
					t.Errorf("error: got %v, want %v", err, tc.wantErrIs)
				}
				return
			}
			if tc.wantErrMsg != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("got nil goal, want non-nil")
			}
			if got.ID != tc.wantGoal.ID {
				t.Errorf("ID: got %q, want %q", got.ID, tc.wantGoal.ID)
			}
			if got.Name != tc.wantGoal.Name {
				t.Errorf("Name: got %q, want %q", got.Name, tc.wantGoal.Name)
			}
			if got.Status != tc.wantGoal.Status {
				t.Errorf("Status: got %q, want %q", got.Status, tc.wantGoal.Status)
			}
			if got.Phase != tc.wantGoal.Phase {
				t.Errorf("Phase: got %q, want %q", got.Phase, tc.wantGoal.Phase)
			}
			if got.Score != tc.wantGoal.Score {
				t.Errorf("Score: got %d, want %d", got.Score, tc.wantGoal.Score)
			}
			if got.TargetDate != tc.wantGoal.TargetDate {
				t.Errorf("TargetDate: got %q, want %q", got.TargetDate, tc.wantGoal.TargetDate)
			}
			if got.OwnerName != tc.wantGoal.OwnerName {
				t.Errorf("OwnerName: got %q, want %q", got.OwnerName, tc.wantGoal.OwnerName)
			}
			if got.OwnerID != tc.wantGoal.OwnerID {
				t.Errorf("OwnerID: got %q, want %q", got.OwnerID, tc.wantGoal.OwnerID)
			}
		})
	}
}

// --- TestSearchGoals ---

func TestSearchGoals(t *testing.T) {
	twoGoalsJSON := `{
		"data": {
			"goals_search": {
				"edges": [
					{"node": {"id":"g1","name":"Goal One","score":50,"targetDate":"","startDate":"","status":{"value":"on_track"},"phase":{"name":"in_progress"},"owner":null}},
					{"node": {"id":"g2","name":"Goal Two","score":80,"targetDate":"","startDate":"","status":{"value":"at_risk"},"phase":{"name":"pending"},"owner":null}}
				],
				"pageInfo": {"hasNextPage":false,"endCursor":""}
			}
		}
	}`

	tests := []struct {
		name         string
		handler      func(w http.ResponseWriter, r *http.Request)
		req          SearchGoalsRequest
		captureBody  bool
		wantLen      int
		wantHasMore  bool
		wantCursor   string
		wantErrIs    error
		wantBodyKey  string // if set, assert request body JSON contains this key
		wantBodyVal  int    // expected int value of wantBodyKey
	}{
		{
			name:        "success - 2 goals returned",
			req:         SearchGoalsRequest{SiteID: "abc-123", SearchString: "status = on_track", MaxResults: 10},
			wantLen:     2,
			wantHasMore: false,
			wantCursor:  "",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(twoGoalsJSON))
			},
		},
		{
			name: "empty result - non-nil empty slice",
			req:  SearchGoalsRequest{SiteID: "abc-123"},
			wantLen: 0,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_search":{"edges":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`))
			},
		},
		{
			name:        "paginated - hasMore=true and cursor",
			req:         SearchGoalsRequest{SiteID: "abc-123", MaxResults: 25},
			wantLen:     1,
			wantHasMore: true,
			wantCursor:  "cursor123",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_search":{"edges":[{"node":{"id":"g1","name":"G","score":0,"targetDate":"","startDate":"","status":{"value":"on_track"},"phase":{"name":"in_progress"},"owner":null}}],"pageInfo":{"hasNextPage":true,"endCursor":"cursor123"}}}}`))
			},
		},
		{
			name:        "MaxResults=0 defaults to 25 in request",
			req:         SearchGoalsRequest{SiteID: "abc-123", MaxResults: 0},
			captureBody: true,
			wantLen:     0,
			wantBodyKey: "first",
			wantBodyVal: 25,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_search":{"edges":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`))
			},
		},
		{
			name:      "HTTP 401 - unauthorized",
			req:       SearchGoalsRequest{SiteID: "abc-123"},
			wantErrIs: jira.ErrUnauthorized,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.captureBody {
					capturedBody = mustReadBody(r.Body)
					// rebuild body as a new reader for subsequent reads — server already read it
				}
				tc.handler(w, r)
			}))
			defer srv.Close()

			svc := newTestService(srv.URL)
			result, err := svc.SearchGoals(context.Background(), tc.req)

			if tc.wantErrIs != nil {
				if !errors.Is(err, tc.wantErrIs) {
					t.Errorf("error: got %v, want %v", err, tc.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("got nil result")
			}
			if result.Goals == nil {
				t.Error("Goals slice must not be nil, got nil")
			}
			if len(result.Goals) != tc.wantLen {
				t.Errorf("Goals len: got %d, want %d", len(result.Goals), tc.wantLen)
			}
			if result.HasMore != tc.wantHasMore {
				t.Errorf("HasMore: got %v, want %v", result.HasMore, tc.wantHasMore)
			}
			if result.NextCursor != tc.wantCursor {
				t.Errorf("NextCursor: got %q, want %q", result.NextCursor, tc.wantCursor)
			}
			if tc.captureBody && tc.wantBodyKey != "" {
				var parsed struct {
					Variables map[string]json.RawMessage `json:"variables"`
				}
				if err := json.Unmarshal(capturedBody, &parsed); err != nil {
					t.Fatalf("parsing captured body: %v", err)
				}
				raw, ok := parsed.Variables[tc.wantBodyKey]
				if !ok {
					t.Fatalf("variable %q not found in request body", tc.wantBodyKey)
				}
				var val float64
				if err := json.Unmarshal(raw, &val); err != nil {
					t.Fatalf("parsing variable %q: %v", tc.wantBodyKey, err)
				}
				if int(val) != tc.wantBodyVal {
					t.Errorf("variable %q: got %d, want %d", tc.wantBodyKey, int(val), tc.wantBodyVal)
				}
			}
		})
	}
}

// --- TestUpdateGoalStatus ---

func TestUpdateGoalStatus(t *testing.T) {
	tests := []struct {
		name           string
		handler        func(w http.ResponseWriter, r *http.Request)
		captureBody    bool
		req            UpdateGoalStatusRequest
		wantErr        bool
		wantErrIs      error
		wantErrMsg     string
		wantBodyHasKey string  // assert JSON request body variables.input does NOT have this key when wantBodyKeyAbsent=true
		wantBodyKeyAbsent bool
		wantBodyContains string // assert captured body contains this substring
	}{
		{
			name: "success - status only",
			req:  UpdateGoalStatusRequest{GoalID: "ari:cloud:townsquare:abc:goal/x", Status: "on_track"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_createUpdate":{"success":true,"errors":[]}}}`))
			},
		},
		{
			name:        "success - with score and summary - body contains score and ADF",
			req:         UpdateGoalStatusRequest{GoalID: "ari:cloud:townsquare:abc:goal/x", Status: "off_track", Score: 75, Summary: "Good progress"},
			captureBody: true,
			wantBodyContains: "Good progress",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_createUpdate":{"success":true,"errors":[]}}}`))
			},
		},
		{
			name:       "mutation returns success:false with error message",
			req:        UpdateGoalStatusRequest{GoalID: "ari:cloud:townsquare:abc:goal/x", Status: "on_track"},
			wantErr:    true,
			wantErrMsg: "goal not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_createUpdate":{"success":false,"errors":[{"message":"goal not found"}]}}}`))
			},
		},
		{
			name:      "HTTP 401 - unauthorized",
			req:       UpdateGoalStatusRequest{GoalID: "ari:cloud:townsquare:abc:goal/x", Status: "on_track"},
			wantErrIs: jira.ErrUnauthorized,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
		},
		{
			name:        "score=0 - not in request body",
			req:         UpdateGoalStatusRequest{GoalID: "ari:cloud:townsquare:abc:goal/x", Status: "on_track", Score: 0},
			captureBody: true,
			wantBodyHasKey: "score",
			wantBodyKeyAbsent: true,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_createUpdate":{"success":true,"errors":[]}}}`))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.captureBody || tc.wantBodyKeyAbsent {
					capturedBody = mustReadBody(r.Body)
				}
				tc.handler(w, r)
			}))
			defer srv.Close()

			svc := newTestService(srv.URL)
			err := svc.UpdateGoalStatus(context.Background(), tc.req)

			if tc.wantErrIs != nil {
				if !errors.Is(err, tc.wantErrIs) {
					t.Errorf("error: got %v, want %v", err, tc.wantErrIs)
				}
				return
			}
			if tc.wantErrMsg != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrMsg)
				}
				return
			}
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Body assertions
			if tc.wantBodyContains != "" {
				if !strings.Contains(string(capturedBody), tc.wantBodyContains) {
					t.Errorf("request body does not contain %q; body: %s", tc.wantBodyContains, string(capturedBody))
				}
			}
			if tc.wantBodyKeyAbsent {
				if strings.Contains(string(capturedBody), `"`+tc.wantBodyHasKey+`"`) {
					t.Errorf("request body should NOT contain key %q; body: %s", tc.wantBodyHasKey, string(capturedBody))
				}
			}
		})
	}
}

// --- TestCreateGoal ---

func TestCreateGoal(t *testing.T) {
	tests := []struct {
		name             string
		handler          func(w http.ResponseWriter, r *http.Request)
		captureBody      bool
		req              CreateGoalRequest
		wantResult       *CreateGoalResult
		wantErrIs        error
		wantErrMsg       string
		wantBodyContains string
	}{
		{
			name: "success - required fields only, confidence defaults to QUARTER",
			req: CreateGoalRequest{
				SiteID:     "abc-123",
				Name:       "Ship Feature X",
				GoalTypeID: "ari:cloud:goal:abc-123:goal-type/act-1/gt-1",
				TargetDate: "2026-12-31",
			},
			captureBody: true,
			wantResult:  &CreateGoalResult{ID: "ari:cloud:townsquare:abc-123:goal/new-1", Name: "Ship Feature X"},
			wantBodyContains: `"confidence":"QUARTER"`,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_create":{"goal":{"id":"ari:cloud:townsquare:abc-123:goal/new-1","name":"Ship Feature X"},"errors":[]}}}`))
			},
		},
		{
			name: "success - with description, body contains ADF summary",
			req: CreateGoalRequest{
				SiteID:      "abc-123",
				Name:        "Ship Feature Y",
				GoalTypeID:  "ari:cloud:goal:abc-123:goal-type/act-1/gt-1",
				TargetDate:  "2026-06-30",
				Confidence:  "MONTH",
				Description: "Launch the new dashboard",
			},
			captureBody:      true,
			wantResult:       &CreateGoalResult{ID: "ari:cloud:townsquare:abc-123:goal/new-2", Name: "Ship Feature Y"},
			wantBodyContains: "Launch the new dashboard",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_create":{"goal":{"id":"ari:cloud:townsquare:abc-123:goal/new-2","name":"Ship Feature Y"},"errors":[]}}}`))
			},
		},
		{
			name: "HTTP 401 - unauthorized",
			req: CreateGoalRequest{
				SiteID:     "abc-123",
				Name:       "X",
				GoalTypeID: "ari:cloud:goal:abc-123:goal-type/act-1/gt-1",
				TargetDate: "2026-12-31",
			},
			wantErrIs: jira.ErrUnauthorized,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
		},
		{
			name: "GraphQL errors array - mutation error",
			req: CreateGoalRequest{
				SiteID:     "abc-123",
				Name:       "X",
				GoalTypeID: "ari:cloud:goal:abc-123:goal-type/act-1/gt-1",
				TargetDate: "2026-12-31",
			},
			wantErrMsg: "invalid goal type",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"errors":[{"message":"invalid goal type"}]}`))
			},
		},
		{
			name: "HTTP 400 - bad request",
			req: CreateGoalRequest{
				SiteID:     "abc-123",
				Name:       "X",
				GoalTypeID: "ari:cloud:goal:abc-123:goal-type/act-1/gt-1",
				TargetDate: "bad-date",
			},
			wantErrMsg: "unexpected status 400",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`bad request`))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.captureBody {
					capturedBody = mustReadBody(r.Body)
				}
				tc.handler(w, r)
			}))
			defer srv.Close()

			svc := newTestService(srv.URL)
			got, err := svc.CreateGoal(context.Background(), tc.req)

			if tc.wantErrIs != nil {
				if !errors.Is(err, tc.wantErrIs) {
					t.Errorf("error: got %v, want %v", err, tc.wantErrIs)
				}
				return
			}
			if tc.wantErrMsg != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("got nil result, want non-nil")
			}
			if got.ID != tc.wantResult.ID {
				t.Errorf("ID: got %q, want %q", got.ID, tc.wantResult.ID)
			}
			if got.Name != tc.wantResult.Name {
				t.Errorf("Name: got %q, want %q", got.Name, tc.wantResult.Name)
			}

			// Body assertions
			if tc.wantBodyContains != "" {
				if !strings.Contains(string(capturedBody), tc.wantBodyContains) {
					t.Errorf("request body does not contain %q; body: %s", tc.wantBodyContains, string(capturedBody))
				}
			}
		})
	}
}

// --- TestEditGoal ---

func TestEditGoal(t *testing.T) {
	ptrStr := func(s string) *string { return &s }
	ptrBool := func(b bool) *bool { return &b }

	tests := []struct {
		name            string
		req             EditGoalRequest
		handler         func(w http.ResponseWriter, r *http.Request)
		wantErr         bool
		wantErrContains string
		wantGoalID      string
		wantGoalName    string
	}{
		{
			name: "success - name updated",
			req:  EditGoalRequest{GoalID: "ari:cloud:townsquare:abc:goal/g1", Name: ptrStr("New Name")},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_edit":{"goal":{"id":"ari:cloud:townsquare:abc:goal/g1","name":"New Name","targetDate":"","isArchived":false},"userErrors":[]}}}`))
			},
			wantGoalID:   "ari:cloud:townsquare:abc:goal/g1",
			wantGoalName: "New Name",
		},
		{
			name: "userErrors returned - propagated as error",
			req:  EditGoalRequest{GoalID: "bad-id"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_edit":{"goal":null,"userErrors":[{"field":"goalId","message":"Goal not found"}]}}}`))
			},
			wantErr:         true,
			wantErrContains: "Goal not found",
		},
		{
			name: "archive goal - success",
			req:  EditGoalRequest{GoalID: "ari:cloud:townsquare:abc:goal/g1", Archive: ptrBool(true)},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"goals_edit":{"goal":{"id":"ari:cloud:townsquare:abc:goal/g1","name":"My Goal","targetDate":"","isArchived":true},"userErrors":[]}}}`))
			},
			wantGoalID:   "ari:cloud:townsquare:abc:goal/g1",
			wantGoalName: "My Goal",
		},
		{
			name: "HTTP 401 - unauthorized",
			req:  EditGoalRequest{GoalID: "g1"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr:         true,
			wantErrContains: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(tc.handler))
			defer srv.Close()

			svc := newTestService(srv.URL)
			got, err := svc.EditGoal(context.Background(), tc.req)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.wantErrContains != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wantErrContains)) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("expected non-nil goal")
			}
			if got.ID != tc.wantGoalID {
				t.Errorf("ID: got %q, want %q", got.ID, tc.wantGoalID)
			}
			if got.Name != tc.wantGoalName {
				t.Errorf("Name: got %q, want %q", got.Name, tc.wantGoalName)
			}
		})
	}
}

// ptr helpers used in tests but not exported (avoid lint warning).
var _ = func() *string { s := ""; return &s }
var _ = errors.New
