package goals

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// ownerGoalsPage renders a graphStore_atlasGoalHasOwnerInverse response.
func ownerGoalsPage(hasNextPage bool, endCursor string, nodes ...string) string {
	return `{"data":{"graphStore_atlasGoalHasOwnerInverse":{"edges":[` +
		strings.Join(nodes, ",") +
		`],"pageInfo":{"hasNextPage":` + boolJSON(hasNextPage) + `,"endCursor":"` + endCursor + `"}}}}`
}

func boolJSON(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// capturedGQL holds the decoded GraphQL request the test server received.
type capturedGQL struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// serveOwnerGoals starts a test server returning body and capturing the request.
func serveOwnerGoals(t *testing.T, body string) (*httptest.Server, *capturedGQL) {
	t.Helper()
	got := &capturedGQL{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.Unmarshal(mustReadBody(r.Body), got); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

// --- The happy path: a specific owner's goals. ---

func TestGetGoalsByOwnerReturnsThatOwnersGoals(t *testing.T) {
	body := ownerGoalsPage(true, "cursor-1",
		`{"node":{"id":"ari:cloud:townsquare:s:goal/1","key":"ZQEAYBZU-104","name":"Bugs y severidad con SLAs","startDate":"2025-07-01","targetDate":{"label":"September"},"status":{"value":"off_track","score":30},"owner":{"name":"Roberto Dzul","accountId":"712020:aaa"}}}`,
		`{"node":{"id":"ari:cloud:townsquare:s:goal/2","key":"ZQEAYBZU-111","name":"Reducir 20% bugs","targetDate":null,"startDate":"","status":{"value":"off_track","score":null},"owner":{"name":"Roberto Dzul","accountId":"712020:aaa"}}}`,
	)
	srv, _ := serveOwnerGoals(t, body)

	got, err := newTestService(srv.URL).GetGoalsByOwner(context.Background(), GoalsByOwnerRequest{
		SiteID:  "site-abc",
		OwnerID: "712020:aaa",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Goals) != 2 {
		t.Fatalf("Goals len: got %d, want 2", len(got.Goals))
	}
	if got.Goals[0].Key != "ZQEAYBZU-104" {
		t.Errorf("Key: got %q, want ZQEAYBZU-104", got.Goals[0].Key)
	}
	if got.Goals[0].OwnerName != "Roberto Dzul" {
		t.Errorf("OwnerName: got %q, want Roberto Dzul", got.Goals[0].OwnerName)
	}
	if got.Goals[0].TargetDate != "September" {
		t.Errorf("TargetDate: got %q, want September", got.Goals[0].TargetDate)
	}
	if got.Goals[0].Score != 30 {
		t.Errorf("Score: got %d, want 30", got.Goals[0].Score)
	}
	if !got.HasMore || got.NextCursor != "cursor-1" {
		t.Errorf("pagination: got HasMore=%v cursor=%q, want true/cursor-1", got.HasMore, got.NextCursor)
	}
	if got.ScanIncomplete {
		t.Error("ScanIncomplete must stay false: this is a real server-side query, not a client-side scan")
	}
}

// --- Three API requirements that cost real debugging time. Pin them. ---

func TestGetGoalsByOwnerSendsTheRequirementsTheAPIEnforces(t *testing.T) {
	srv, captured := serveOwnerGoals(t, ownerGoalsPage(false, ""))

	if _, err := newTestService(srv.URL).GetGoalsByOwner(context.Background(), GoalsByOwnerRequest{
		SiteID:  "site-abc",
		OwnerID: "712020:aaa",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without the opt-in the gateway refuses with OptInException, and the
	// directive is only legal on the field — on the operation it is rejected
	// as MisplacedDirective.
	if !strings.Contains(captured.Query, `@optIn(to: "GraphStoreAtlasGoalHasOwner")`) {
		t.Errorf("query must opt in to the experimental field\nquery: %s", captured.Query)
	}
	if idx := strings.Index(captured.Query, "@optIn"); idx >= 0 {
		if before := captured.Query[:idx]; !strings.Contains(before, "graphStore_atlasGoalHasOwnerInverse") {
			t.Errorf("@optIn must sit on the field, not on the operation\nquery: %s", captured.Query)
		}
	}

	// Without a query context the API answers "Query context must not be null
	// and should be a valid platform site or workspace ARI".
	if got, want := captured.Variables["queryContext"], "ari:cloud:platform::site/site-abc"; got != want {
		t.Errorf("queryContext: got %v, want %q", got, want)
	}

	// The edge node is a union whose only member is TownsquareGoal, so the
	// selection needs an inline fragment or the server rejects the field.
	if !strings.Contains(captured.Query, "... on TownsquareGoal") {
		t.Errorf("query must select through an inline fragment on TownsquareGoal\nquery: %s", captured.Query)
	}
}

// --- Owners are identified by account ID in our own output; accept that. ---

func TestGetGoalsByOwnerAcceptsAccountIDOrARI(t *testing.T) {
	tests := []struct {
		name    string
		ownerID string
		wantID  string
	}{
		{
			// This is the form search results already print as owner_id, so it
			// must work without the caller hand-building an ARI.
			name:    "bare account id is expanded into a user ARI",
			ownerID: "712020:7bfdc0f9-20c9-41b1-900d-aab305ab0f22",
			wantID:  "ari:cloud:identity::user/712020:7bfdc0f9-20c9-41b1-900d-aab305ab0f22",
		},
		{
			name:    "a full ARI is passed through untouched",
			ownerID: "ari:cloud:identity::user/712020:aaa",
			wantID:  "ari:cloud:identity::user/712020:aaa",
		},
		{
			name:    "surrounding whitespace is trimmed",
			ownerID: "  712020:aaa  ",
			wantID:  "ari:cloud:identity::user/712020:aaa",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, captured := serveOwnerGoals(t, ownerGoalsPage(false, ""))

			if _, err := newTestService(srv.URL).GetGoalsByOwner(context.Background(), GoalsByOwnerRequest{
				SiteID:  "site-abc",
				OwnerID: tc.ownerID,
			}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := captured.Variables["id"]; got != tc.wantID {
				t.Errorf("owner id variable: got %v, want %q", got, tc.wantID)
			}
		})
	}
}

// --- Required input must fail loudly, before any network call. ---

func TestGetGoalsByOwnerRejectsMissingInput(t *testing.T) {
	tests := []struct {
		name       string
		req        GoalsByOwnerRequest
		wantErrMsg string
	}{
		{
			name:       "missing owner",
			req:        GoalsByOwnerRequest{SiteID: "site-abc"},
			wantErrMsg: "owner",
		},
		{
			name:       "missing site",
			req:        GoalsByOwnerRequest{OwnerID: "712020:aaa"},
			wantErrMsg: "site",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			_, err := newTestService(srv.URL).GetGoalsByOwner(context.Background(), tc.req)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.wantErrMsg) {
				t.Errorf("error %q should mention %q", err.Error(), tc.wantErrMsg)
			}
			if called {
				t.Error("invalid input must not reach the API")
			}
		})
	}
}

// --- Defaults, emptiness and failure modes. ---

func TestGetGoalsByOwnerDefaultsMaxResults(t *testing.T) {
	srv, captured := serveOwnerGoals(t, ownerGoalsPage(false, ""))

	if _, err := newTestService(srv.URL).GetGoalsByOwner(context.Background(), GoalsByOwnerRequest{
		SiteID:  "site-abc",
		OwnerID: "712020:aaa",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := captured.Variables["first"], float64(defaultMaxResults); got != want {
		t.Errorf("first: got %v, want %v", got, want)
	}
	if _, ok := captured.Variables["after"]; ok {
		t.Error("after must be omitted when no cursor was given")
	}
}

func TestGetGoalsByOwnerForwardsCursor(t *testing.T) {
	srv, captured := serveOwnerGoals(t, ownerGoalsPage(false, ""))

	if _, err := newTestService(srv.URL).GetGoalsByOwner(context.Background(), GoalsByOwnerRequest{
		SiteID:     "site-abc",
		OwnerID:    "712020:aaa",
		MaxResults: 5,
		Cursor:     "cursor-7",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := captured.Variables["after"]; got != "cursor-7" {
		t.Errorf("after: got %v, want cursor-7", got)
	}
	if got, want := captured.Variables["first"], float64(5); got != want {
		t.Errorf("first: got %v, want %v", got, want)
	}
}

func TestGetGoalsByOwnerAnOwnerWithNoGoalsIsNotAnError(t *testing.T) {
	srv, _ := serveOwnerGoals(t, ownerGoalsPage(false, ""))

	got, err := newTestService(srv.URL).GetGoalsByOwner(context.Background(), GoalsByOwnerRequest{
		SiteID:  "site-abc",
		OwnerID: "712020:nobody",
	})
	if err != nil {
		t.Fatalf("an owner with no goals must not error: %v", err)
	}
	if got.Goals == nil {
		t.Error("Goals must be an empty slice, not nil, so it serialises as []")
	}
	if len(got.Goals) != 0 {
		t.Errorf("Goals len: got %d, want 0", len(got.Goals))
	}
}

func TestGetGoalsByOwnerSkipsUnionMembersThatAreNotGoals(t *testing.T) {
	// The union currently only holds TownsquareGoal, but if the API ever adds a
	// member the inline fragment yields an empty node. Emitting a blank goal
	// would be worse than skipping it.
	body := ownerGoalsPage(false, "",
		`{"node":{}}`,
		`{"node":{"id":"ari:cloud:townsquare:s:goal/2","key":"K-2","name":"Real goal","targetDate":null,"startDate":"","status":{"value":"on_track"},"owner":null}}`,
	)
	srv, _ := serveOwnerGoals(t, body)

	got, err := newTestService(srv.URL).GetGoalsByOwner(context.Background(), GoalsByOwnerRequest{
		SiteID:  "site-abc",
		OwnerID: "712020:aaa",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Goals) != 1 || got.Goals[0].Key != "K-2" {
		t.Errorf("goals: got %+v, want only K-2", got.Goals)
	}
}

func TestGetGoalsByOwnerFailureModes(t *testing.T) {
	tests := []struct {
		name       string
		handler    func(w http.ResponseWriter, r *http.Request)
		wantErrIs  error
		wantErrMsg string
	}{
		{
			name:      "HTTP 401 maps to ErrUnauthorized",
			wantErrIs: jira.ErrUnauthorized,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
		},
		{
			name:       "a GraphQL error is surfaced, not swallowed",
			wantErrMsg: "opt in",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"errors":[{"message":"please specifically opt in"}],"data":null}`))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(tc.handler))
			defer srv.Close()

			_, err := newTestService(srv.URL).GetGoalsByOwner(context.Background(), GoalsByOwnerRequest{
				SiteID:  "site-abc",
				OwnerID: "712020:aaa",
			})
			if tc.wantErrIs != nil {
				if !errors.Is(err, tc.wantErrIs) {
					t.Errorf("error: got %v, want %v", err, tc.wantErrIs)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErrMsg) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrMsg)
			}
		})
	}
}
