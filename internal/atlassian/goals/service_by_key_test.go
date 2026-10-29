package goals

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// --- Fetching a goal by its short, human-readable key. ---

func TestGetGoalByKeyReturnsTheGoal(t *testing.T) {
	body := `{"data":{"goals_byKey":{"id":"ari:cloud:townsquare:s:goal/0f6","key":"ZQEAYBZU-987","name":"100% Llamadas analizadas con AI","startDate":null,"targetDate":{"label":"June"},"status":{"value":"pending","score":null},"owner":{"name":"Andres Mena Dorantes","accountId":"712020:2ee"}}}}`
	srv, captured := serveOwnerGoals(t, body)

	got, err := newTestService(srv.URL).GetGoalByKey(context.Background(), "site-abc", "ZQEAYBZU-987")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Key != "ZQEAYBZU-987" {
		t.Errorf("Key: got %q, want ZQEAYBZU-987", got.Key)
	}
	if got.ID != "ari:cloud:townsquare:s:goal/0f6" {
		t.Errorf("ID: got %q", got.ID)
	}
	if got.Name != "100% Llamadas analizadas con AI" {
		t.Errorf("Name: got %q", got.Name)
	}
	if got.Status != "pending" {
		t.Errorf("Status: got %q, want pending", got.Status)
	}
	if got.TargetDate != "June" {
		t.Errorf("TargetDate: got %q, want June", got.TargetDate)
	}
	if got.OwnerName != "Andres Mena Dorantes" {
		t.Errorf("OwnerName: got %q", got.OwnerName)
	}

	if got, want := captured.Variables["containerId"], "ari:cloud:townsquare::site/site-abc"; got != want {
		t.Errorf("containerId: got %v, want %q", got, want)
	}
	if got := captured.Variables["goalKey"]; got != "ZQEAYBZU-987" {
		t.Errorf("goalKey: got %v, want ZQEAYBZU-987", got)
	}
}

// --- An unknown key must be a clear not-found, not a wall of GraphQL noise. ---
//
// The API answers an unknown key with a GraphQL error carrying
// extensions.statusCode 404 and a message like
// "Exception while fetching data (/goals_byKey) : Goal with key X does not
// exist for <cloudId>". Surfacing that verbatim would be useless to a caller
// and would give the wrong CLI exit code, so it maps to ErrNotFound like the
// ARI-based lookup already does.

func TestGetGoalByKeyUnknownKeyIsNotFound(t *testing.T) {
	body := `{"errors":[{"message":"Exception while fetching data (/goals_byKey) : Goal with key ZQEAYBZU-99999 does not exist for 5669cd27","extensions":{"statusCode":404,"classification":"DataFetchingException"}}],"data":{"goals_byKey":null}}`
	srv, _ := serveOwnerGoals(t, body)

	_, err := newTestService(srv.URL).GetGoalByKey(context.Background(), "site-abc", "ZQEAYBZU-99999")
	if !errors.Is(err, jira.ErrNotFound) {
		t.Fatalf("error: got %v, want ErrNotFound", err)
	}
	// The bare sentinel reads "issue not found", which is wrong for a goal and
	// names nothing. The message must identify what was actually missing.
	for _, want := range []string{"ZQEAYBZU-99999", "site-abc"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err.Error(), want)
		}
	}
}

func TestGetGoalByKeyNullGoalWithoutErrorsIsAlsoNotFound(t *testing.T) {
	srv, _ := serveOwnerGoals(t, `{"data":{"goals_byKey":null}}`)

	_, err := newTestService(srv.URL).GetGoalByKey(context.Background(), "site-abc", "ZQEAYBZU-1")
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("error: got %v, want ErrNotFound", err)
	}
}

// --- A non-404 GraphQL error must stay a real error, not become "not found". ---

func TestGetGoalByKeyOtherGraphQLErrorsAreNotSwallowedAsNotFound(t *testing.T) {
	body := `{"errors":[{"message":"something else broke","extensions":{"statusCode":500}}],"data":null}`
	srv, _ := serveOwnerGoals(t, body)

	_, err := newTestService(srv.URL).GetGoalByKey(context.Background(), "site-abc", "ZQEAYBZU-1")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if errors.Is(err, jira.ErrNotFound) {
		t.Error("a 500 must not be reported as not found")
	}
	if !strings.Contains(err.Error(), "something else broke") {
		t.Errorf("error %q should carry the original message", err.Error())
	}
}

// --- Required input fails before the network call. ---

func TestGetGoalByKeyRejectsMissingInput(t *testing.T) {
	tests := []struct {
		name       string
		siteID     string
		goalKey    string
		wantErrMsg string
	}{
		{name: "missing site", siteID: "", goalKey: "ZQEAYBZU-1", wantErrMsg: "site"},
		{name: "missing key", siteID: "site-abc", goalKey: "", wantErrMsg: "key"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			_, err := newTestService(srv.URL).GetGoalByKey(context.Background(), tc.siteID, tc.goalKey)
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

func TestGetGoalByKeyUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := newTestService(srv.URL).GetGoalByKey(context.Background(), "site-abc", "ZQEAYBZU-1")
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("error: got %v, want ErrUnauthorized", err)
	}
}

// --- Adding statusCode decoding must not change how other calls report errors. ---

func TestGraphQLErrorsWithoutStatusCodeStillSurfaceTheirMessage(t *testing.T) {
	srv, _ := serveOwnerGoals(t, `{"errors":[{"message":"forbidden"}]}`)

	_, err := newTestService(srv.URL).GetSiteID(context.Background(), "myorg")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error %q should contain the GraphQL message", err.Error())
	}
}
