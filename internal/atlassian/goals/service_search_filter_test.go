package goals

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// goalNode renders one goals_search edge node with the given id, key and name.
func goalNode(id, key, name string) string {
	return fmt.Sprintf(
		`{"node":{"id":%q,"key":%q,"name":%q,"targetDate":null,"startDate":"","status":{"value":"on_track"},"owner":null}}`,
		id, key, name,
	)
}

// goalPage renders a goals_search response page from the given edges.
func goalPage(hasNextPage bool, endCursor string, edges ...string) string {
	return fmt.Sprintf(
		`{"data":{"goals_search":{"edges":[%s],"pageInfo":{"hasNextPage":%t,"endCursor":%q}}}}`,
		strings.Join(edges, ","), hasNextPage, endCursor,
	)
}

// goalKeys maps goals to their keys for order-sensitive assertions.
func goalKeys(gs []Goal) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.Key
	}
	return out
}

// --- The API ignores searchString, so SearchGoals must filter client-side. ---
//
// Verified against the live tenant: goals_search(searchString:) returns identical
// result sets for "Analytics", "Roberto", "status = on_track" and "zzzznotexist".
// Any filtering the caller asks for therefore has to happen on our side.

func TestSearchGoalsFiltersClientSide(t *testing.T) {
	// One page holding three distinguishable goals.
	onePage := goalPage(false, "",
		goalNode("g1", "ZQEAYBZU-626", "100% de componentes Analytics conectados a Aurora"),
		goalNode("g2", "ZQEAYBZU-987", "100% Llamadas analizadas con AI"),
		goalNode("g3", "ZQEAYBZU-633", "100% de ciclos con validaciones preventivas"),
	)

	tests := []struct {
		name         string
		searchString string
		maxResults   int
		wantKeys     []string
	}{
		{
			name:         "matches goal name as a case-insensitive substring",
			searchString: "analytics",
			wantKeys:     []string{"ZQEAYBZU-626"},
		},
		{
			name:         "matches goal name regardless of the needle casing",
			searchString: "ANALIZADAS",
			wantKeys:     []string{"ZQEAYBZU-987"},
		},
		{
			name:         "matches the human readable goal key",
			searchString: "ZQEAYBZU-633",
			wantKeys:     []string{"ZQEAYBZU-633"},
		},
		{
			name:         "matches a goal key case-insensitively",
			searchString: "zqeaybzu-626",
			wantKeys:     []string{"ZQEAYBZU-626"},
		},
		{
			name:         "a needle matching nothing returns no goals instead of everything",
			searchString: "zzzznotexist",
			wantKeys:     []string{},
		},
		{
			name:         "a JQL-like string is treated as literal text, not as a query language",
			searchString: "status = on_track",
			wantKeys:     []string{},
		},
		{
			name:         "an empty needle returns the page unfiltered",
			searchString: "",
			wantKeys:     []string{"ZQEAYBZU-626", "ZQEAYBZU-987", "ZQEAYBZU-633"},
		},
		{
			name:         "matches are capped at MaxResults",
			searchString: "100%",
			maxResults:   2,
			wantKeys:     []string{"ZQEAYBZU-626", "ZQEAYBZU-987"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(onePage))
			}))
			defer srv.Close()

			svc := newTestService(srv.URL)
			got, err := svc.SearchGoals(context.Background(), SearchGoalsRequest{
				SiteID:       "abc-123",
				SearchString: tc.searchString,
				MaxResults:   tc.maxResults,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Goals == nil {
				t.Fatal("Goals slice must not be nil")
			}

			gotKeys := goalKeys(got.Goals)
			if strings.Join(gotKeys, ",") != strings.Join(tc.wantKeys, ",") {
				t.Errorf("goal keys: got %v, want %v", gotKeys, tc.wantKeys)
			}
		})
	}
}

// --- Sending an argument the API ignores is misleading noise. ---

func TestSearchGoalsDoesNotForwardSearchStringToAPI(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(goalPage(false, "")))
	}))
	defer srv.Close()

	svc := newTestService(srv.URL)
	if _, err := svc.SearchGoals(context.Background(), SearchGoalsRequest{
		SiteID:       "abc-123",
		SearchString: "anything",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed struct {
		Variables map[string]json.RawMessage `json:"variables"`
	}
	if err := json.Unmarshal(captured, &parsed); err != nil {
		t.Fatalf("parsing captured body: %v", err)
	}
	if raw, ok := parsed.Variables["searchString"]; ok {
		t.Errorf("searchString must not be sent to the API because it is ignored there, got %s", raw)
	}
}

// --- Filtering a single page is not enough: matches may live on later pages. ---

func TestSearchGoalsPaginatesWhileFiltering(t *testing.T) {
	// Page 1 holds no match, page 2 holds the only match. A single-page
	// implementation would wrongly report "no goals found".
	pages := map[string]string{
		"": goalPage(true, "cursor-1",
			goalNode("g1", "K-1", "Unrelated goal"),
		),
		"cursor-1": goalPage(false, "",
			goalNode("g2", "K-2", "Analytics rollout"),
		),
	}

	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var parsed struct {
			Variables struct {
				After string `json:"after"`
			} `json:"variables"`
		}
		_ = json.Unmarshal(mustReadBody(r.Body), &parsed)
		page, ok := pages[parsed.Variables.After]
		if !ok {
			t.Errorf("unexpected cursor %q", parsed.Variables.After)
			page = goalPage(false, "")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()

	svc := newTestService(srv.URL)
	got, err := svc.SearchGoals(context.Background(), SearchGoalsRequest{
		SiteID:       "abc-123",
		SearchString: "analytics",
		MaxResults:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"K-2"}; strings.Join(goalKeys(got.Goals), ",") != strings.Join(want, ",") {
		t.Errorf("goal keys: got %v, want %v", goalKeys(got.Goals), want)
	}
	if requests != 2 {
		t.Errorf("API pages fetched: got %d, want 2", requests)
	}
	if got.ScanIncomplete {
		t.Error("ScanIncomplete must be false when every page was scanned")
	}
	if got.HasMore {
		t.Error("HasMore must be false when every page was scanned")
	}
}

// --- A huge site must not hang the caller, and a bounded scan must say so. ---

func TestSearchGoalsBoundsTheScanAndReportsIncompleteness(t *testing.T) {
	// The server always claims another page exists and never yields a match,
	// so the scan can only end by hitting the page budget.
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(goalPage(true, fmt.Sprintf("cursor-%d", requests),
			goalNode(fmt.Sprintf("g%d", requests), fmt.Sprintf("K-%d", requests), "Unrelated goal"),
		)))
	}))
	defer srv.Close()

	svc := newTestService(srv.URL)
	got, err := svc.SearchGoals(context.Background(), SearchGoalsRequest{
		SiteID:       "abc-123",
		SearchString: "analytics",
		MaxResults:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Goals) != 0 {
		t.Errorf("Goals len: got %d, want 0", len(got.Goals))
	}
	if requests != maxFilterScanPages {
		t.Errorf("API pages fetched: got %d, want the page budget %d", requests, maxFilterScanPages)
	}
	if !got.ScanIncomplete {
		t.Error("ScanIncomplete must be true when the page budget stopped the scan early")
	}
	if !got.HasMore {
		t.Error("HasMore must be true when unscanned pages remain")
	}
	if got.NextCursor == "" {
		t.Error("NextCursor must be set so the caller can resume an incomplete scan")
	}
}

// --- An unfiltered search must keep behaving exactly as before. ---

func TestSearchGoalsUnfilteredKeepsSinglePageBehaviour(t *testing.T) {
	var requests int
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		captured = mustReadBody(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Claims more pages exist; an unfiltered search must not chase them.
		_, _ = w.Write([]byte(goalPage(true, "cursor-1", goalNode("g1", "K-1", "Goal One"))))
	}))
	defer srv.Close()

	svc := newTestService(srv.URL)
	got, err := svc.SearchGoals(context.Background(), SearchGoalsRequest{
		SiteID:     "abc-123",
		MaxResults: 7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 1 {
		t.Errorf("API pages fetched: got %d, want 1", requests)
	}
	if !got.HasMore || got.NextCursor != "cursor-1" {
		t.Errorf("pagination passthrough: got HasMore=%v cursor=%q, want true/cursor-1", got.HasMore, got.NextCursor)
	}
	if got.ScanIncomplete {
		t.Error("ScanIncomplete must be false for an unfiltered search; HasMore already conveys paging")
	}

	var parsed struct {
		Variables map[string]json.RawMessage `json:"variables"`
	}
	if err := json.Unmarshal(captured, &parsed); err != nil {
		t.Fatalf("parsing captured body: %v", err)
	}
	if got, want := string(parsed.Variables["first"]), "7"; got != want {
		t.Errorf("first variable: got %s, want %s", got, want)
	}
}

// --- The goal key is part of the contract now, so it must be decoded. ---

func TestSearchGoalsExposesGoalKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(goalPage(false, "", goalNode("g1", "ZQEAYBZU-633", "Goal One"))))
	}))
	defer srv.Close()

	svc := newTestService(srv.URL)
	got, err := svc.SearchGoals(context.Background(), SearchGoalsRequest{SiteID: "abc-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Goals) != 1 {
		t.Fatalf("Goals len: got %d, want 1", len(got.Goals))
	}
	if got.Goals[0].Key != "ZQEAYBZU-633" {
		t.Errorf("Key: got %q, want %q", got.Goals[0].Key, "ZQEAYBZU-633")
	}
}
