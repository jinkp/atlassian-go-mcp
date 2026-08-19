package confluence_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
)

// --- Fixture helpers ---

// pageFixture returns a minimal Confluence page API JSON response.
func pageFixture(id, title, spaceID string, version int, body string) string {
	return `{
		"id": "` + id + `",
		"title": "` + title + `",
		"spaceId": "` + spaceID + `",
		"status": "current",
		"version": {"number": ` + itoa(version) + `},
		"body": {"storage": {"value": "` + body + `"}},
		"_links": {"webui": "/wiki/spaces/DEV/pages/` + id + `"},
		"createdAt": "2024-01-01T10:00:00.000Z"
	}`
}

// pageRefFixture returns a lightweight page reference JSON item.
func pageRefFixture(id, title string) string {
	return `{"id": "` + id + `", "title": "` + title + `", "status": "current", "type": "page"}`
}

// spaceFixture returns a minimal space JSON item.
func spaceFixture(id, key, name string) string {
	return `{"id": "` + id + `", "key": "` + key + `", "name": "` + name + `", "type": "global", "status": "current"}`
}

// commentFixture returns a minimal comment JSON item.
func commentFixture(id, body string, version int) string {
	return `{
		"id": "` + id + `",
		"version": {"number": ` + itoa(version) + `},
		"body": {"storage": {"value": "` + body + `"}},
		"createdAt": "2024-01-01T10:00:00.000Z"
	}`
}

// pageListFixture wraps a list of page JSON items with optional _links.next.
func pageListFixture(items []string, next string) string {
	results := "[" + strings.Join(items, ",") + "]"
	links := `{}`
	if next != "" {
		links = `{"next": "` + next + `"}`
	}
	return `{"results": ` + results + `, "_links": ` + links + `}`
}

// commentListFixture wraps a list of comment JSON items with optional _links.next.
func commentListFixture(items []string, next string) string {
	results := "[" + strings.Join(items, ",") + "]"
	links := `{}`
	if next != "" {
		links = `{"next": "` + next + `"}`
	}
	return `{"results": ` + results + `, "_links": ` + links + `}`
}

// searchFixture wraps a list of search result JSON items.
func searchResultFixture(contentID, title, spaceKey, excerpt string) string {
	return `{
		"content": {"id": "` + contentID + `", "type": "page"},
		"title": "` + title + `",
		"excerpt": "` + excerpt + `",
		"space": {"key": "` + spaceKey + `"}
	}`
}

// itoa converts an int to its string representation for use in JSON fixture strings.
func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// --- extractCursor unit tests ---

func TestExtractCursor(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "valid URL with cursor",
			input: "/wiki/api/v2/spaces/DEV/pages?limit=25&cursor=abc123",
			want:  "abc123",
		},
		{
			name:  "full URL with cursor",
			input: "https://example.atlassian.net/wiki/api/v2/spaces/DEV/pages?limit=25&cursor=xyz%3D%3D",
			want:  "xyz==",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "URL without cursor param",
			input: "/wiki/api/v2/spaces/DEV/pages?limit=25",
			want:  "",
		},
		{
			name:  "malformed URL",
			input: "://bad url%%here",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// extractCursor is unexported; we test it indirectly through GetPagesInSpace.
			// For direct coverage, we exercise via a list endpoint that echoes _links.next.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				next := tt.input
				links := `{}`
				if next != "" {
					raw, _ := json.Marshal(next)
					links = `{"next": ` + string(raw) + `}`
				}
				resp := `{"results": [], "_links": ` + links + `}`
				w.Write([]byte(resp)) //nolint:errcheck
			}))
			defer srv.Close()

			svc := confluence.NewService(srv.Client(), srv.URL)
			list, err := svc.GetPagesInSpace(context.Background(), "SPACE", 5, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if list.NextCursor != tt.want {
				t.Errorf("NextCursor: want %q, got %q", tt.want, list.NextCursor)
			}
		})
	}
}

// --- GetPage tests ---

func TestConfluenceService_GetPage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/pages/PAGE-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("body-format") != "storage" {
			t.Errorf("expected body-format=storage, got %q", r.URL.Query().Get("body-format"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(pageFixture("PAGE-1", "My Page", "SPACE-DEV", 5, "<p>Hello</p>"))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	page, err := svc.GetPage(context.Background(), "PAGE-1", "storage")
	if err != nil {
		t.Fatalf("GetPage() unexpected error: %v", err)
	}
	if page.ID != "PAGE-1" {
		t.Errorf("ID: expected PAGE-1, got %s", page.ID)
	}
	if page.Title != "My Page" {
		t.Errorf("Title: expected 'My Page', got %s", page.Title)
	}
	if page.SpaceID != "SPACE-DEV" {
		t.Errorf("SpaceID: expected SPACE-DEV, got %s", page.SpaceID)
	}
	if page.VersionNumber != 5 {
		t.Errorf("VersionNumber: expected 5, got %d", page.VersionNumber)
	}
	if page.Body != "<p>Hello</p>" {
		t.Errorf("Body: expected '<p>Hello</p>', got %q", page.Body)
	}
}

func TestConfluenceService_GetPage_DefaultBodyFormat(t *testing.T) {
	var gotBodyFormat string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBodyFormat = r.URL.Query().Get("body-format")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(pageFixture("P1", "Title", "S1", 1, ""))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetPage(context.Background(), "P1", "") // empty bodyFormat → default "storage"
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBodyFormat != "storage" {
		t.Errorf("expected default body-format=storage, got %q", gotBodyFormat)
	}
}

func TestConfluenceService_GetPage_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetPage(context.Background(), "MISSING", "storage")
	if !errors.Is(err, confluence.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestConfluenceService_GetPage_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetPage(context.Background(), "PAGE-1", "storage")
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_GetPage_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetPage(context.Background(), "PAGE-1", "storage")
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for 403, got: %v", err)
	}
}

func TestConfluenceService_GetPage_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetPage(context.Background(), "PAGE-1", "storage")
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- GetPagesInSpace tests ---

func TestConfluenceService_GetPagesInSpace_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/spaces/SPACE-1/pages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		items := []string{
			pageFixture("P1", "Page One", "SPACE-1", 1, ""),
			pageFixture("P2", "Page Two", "SPACE-1", 2, ""),
		}
		w.Write([]byte(pageListFixture(items, ""))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	list, err := svc.GetPagesInSpace(context.Background(), "SPACE-1", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(list.Results))
	}
	if list.Results[0].ID != "P1" {
		t.Errorf("Results[0].ID: expected P1, got %s", list.Results[0].ID)
	}
	if list.NextCursor != "" {
		t.Errorf("expected empty NextCursor, got %q", list.NextCursor)
	}
}

func TestConfluenceService_GetPagesInSpace_WithCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		items := []string{pageFixture("P3", "Page Three", "S1", 1, "")}
		// Return a _links.next with cursor
		w.Write([]byte(pageListFixture(items, "/wiki/api/v2/spaces/S1/pages?limit=1&cursor=next-token"))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	list, err := svc.GetPagesInSpace(context.Background(), "S1", 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if list.NextCursor != "next-token" {
		t.Errorf("NextCursor: expected 'next-token', got %q", list.NextCursor)
	}
}

func TestConfluenceService_GetPagesInSpace_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Empty results array — return a pageListAPIResponse with empty results
		w.Write([]byte(`{"results": [], "_links": {}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	list, err := svc.GetPagesInSpace(context.Background(), "EMPTY-SPACE", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if list.Results == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(list.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(list.Results))
	}
	if list.NextCursor != "" {
		t.Errorf("expected empty NextCursor for empty space, got %q", list.NextCursor)
	}
}

func TestConfluenceService_GetPagesInSpace_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetPagesInSpace(context.Background(), "BAD", 10, "")
	if !errors.Is(err, confluence.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestConfluenceService_GetPagesInSpace_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetPagesInSpace(context.Background(), "S1", 10, "")
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_GetPagesInSpace_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetPagesInSpace(context.Background(), "S1", 10, "")
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- GetSpaces tests ---

func TestConfluenceService_GetSpaces_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/spaces" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		items := []string{spaceFixture("S1", "DEV", "Development"), spaceFixture("S2", "OPS", "Operations")}
		w.Write([]byte(`{"results": [` + strings.Join(items, ",") + `], "_links": {}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	list, err := svc.GetSpaces(context.Background(), 10, "", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Results) != 2 {
		t.Fatalf("expected 2 spaces, got %d", len(list.Results))
	}
	if list.Results[0].Key != "DEV" {
		t.Errorf("Results[0].Key: expected DEV, got %s", list.Results[0].Key)
	}
}

func TestConfluenceService_GetSpaces_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetSpaces(context.Background(), 10, "", nil, "")
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_GetSpaces_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetSpaces(context.Background(), 10, "", nil, "")
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- GetPageDescendants tests ---

func TestConfluenceService_GetPageDescendants_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/pages/PAGE-1/descendants" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		items := []string{pageRefFixture("CHILD-1", "Child Page")}
		w.Write([]byte(pageListFixture(items, ""))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	list, err := svc.GetPageDescendants(context.Background(), "PAGE-1", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Results) != 1 {
		t.Fatalf("expected 1 descendant, got %d", len(list.Results))
	}
	if list.Results[0].ID != "CHILD-1" {
		t.Errorf("Results[0].ID: expected CHILD-1, got %s", list.Results[0].ID)
	}
}

func TestConfluenceService_GetPageDescendants_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetPageDescendants(context.Background(), "BAD", 10, "")
	if !errors.Is(err, confluence.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestConfluenceService_GetPageDescendants_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetPageDescendants(context.Background(), "PAGE-1", 10, "")
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_GetPageDescendants_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetPageDescendants(context.Background(), "PAGE-1", 10, "")
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- GetFooterComments tests ---

func TestConfluenceService_GetFooterComments_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/pages/PAGE-1/footer-comments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		items := []string{commentFixture("C1", "<p>Great page!</p>", 1)}
		w.Write([]byte(commentListFixture(items, ""))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	list, err := svc.GetFooterComments(context.Background(), "PAGE-1", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Results) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(list.Results))
	}
	if list.Results[0].ID != "C1" {
		t.Errorf("Results[0].ID: expected C1, got %s", list.Results[0].ID)
	}
}

func TestConfluenceService_GetFooterComments_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(commentListFixture([]string{}, ""))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	list, err := svc.GetFooterComments(context.Background(), "PAGE-1", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if list.Results == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(list.Results) != 0 {
		t.Errorf("expected 0 comments, got %d", len(list.Results))
	}
}

func TestConfluenceService_GetFooterComments_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetFooterComments(context.Background(), "BAD", 10, "")
	if !errors.Is(err, confluence.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestConfluenceService_GetFooterComments_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetFooterComments(context.Background(), "PAGE-1", 10, "")
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_GetFooterComments_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetFooterComments(context.Background(), "PAGE-1", 10, "")
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- GetInlineComments tests ---

func TestConfluenceService_GetInlineComments_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/pages/PAGE-1/inline-comments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		items := []string{commentFixture("IC1", "<p>Inline note</p>", 1)}
		w.Write([]byte(commentListFixture(items, ""))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	list, err := svc.GetInlineComments(context.Background(), "PAGE-1", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Results) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(list.Results))
	}
	if list.Results[0].ID != "IC1" {
		t.Errorf("Results[0].ID: expected IC1, got %s", list.Results[0].ID)
	}
}

func TestConfluenceService_GetInlineComments_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetInlineComments(context.Background(), "BAD", 10, "")
	if !errors.Is(err, confluence.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestConfluenceService_GetInlineComments_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetInlineComments(context.Background(), "PAGE-1", 10, "")
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_GetInlineComments_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetInlineComments(context.Background(), "PAGE-1", 10, "")
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- GetCommentChildren tests ---

func TestConfluenceService_GetCommentChildren_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/footer-comments/C1/children" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		items := []string{commentFixture("C2", "<p>Reply</p>", 1)}
		w.Write([]byte(commentListFixture(items, ""))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	list, err := svc.GetCommentChildren(context.Background(), "C1", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Results) != 1 {
		t.Fatalf("expected 1 child, got %d", len(list.Results))
	}
	if list.Results[0].ID != "C2" {
		t.Errorf("Results[0].ID: expected C2, got %s", list.Results[0].ID)
	}
}

func TestConfluenceService_GetCommentChildren_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetCommentChildren(context.Background(), "BAD", 10, "")
	if !errors.Is(err, confluence.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestConfluenceService_GetCommentChildren_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetCommentChildren(context.Background(), "C1", 10, "")
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_GetCommentChildren_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.GetCommentChildren(context.Background(), "C1", 10, "")
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- CreatePage tests ---

func TestConfluenceService_CreatePage_Success(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/pages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(pageFixture("NEW-1", "New Page", "SPACE-DEV", 1, "<p>Content</p>"))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	page, err := svc.CreatePage(context.Background(), confluence.CreatePageRequest{
		SpaceID: "SPACE-DEV",
		Title:   "New Page",
		Body:    "<p>Content</p>",
	})
	if err != nil {
		t.Fatalf("CreatePage() unexpected error: %v", err)
	}
	if page.ID != "NEW-1" {
		t.Errorf("ID: expected NEW-1, got %s", page.ID)
	}
	if page.Title != "New Page" {
		t.Errorf("Title: expected 'New Page', got %s", page.Title)
	}

	// Verify the request body sends storage representation.
	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	bodyField, ok := body["body"].(map[string]interface{})
	if !ok {
		t.Fatalf("body field is not object, got %T", body["body"])
	}
	if bodyField["representation"] != "storage" {
		t.Errorf("body.representation: expected 'storage', got %v", bodyField["representation"])
	}
	if bodyField["value"] != "<p>Content</p>" {
		t.Errorf("body.value: expected '<p>Content</p>', got %v", bodyField["value"])
	}
	if body["status"] != "current" {
		t.Errorf("status: expected 'current' default, got %v", body["status"])
	}
}

func TestConfluenceService_CreatePage_WithParentID(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(pageFixture("CHILD-1", "Child", "S1", 1, ""))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.CreatePage(context.Background(), confluence.CreatePageRequest{
		SpaceID:  "S1",
		Title:    "Child",
		Body:     "<p>text</p>",
		ParentID: "PARENT-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["parentId"] != "PARENT-1" {
		t.Errorf("parentId: expected 'PARENT-1', got %v", body["parentId"])
	}
}

func TestConfluenceService_CreatePage_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.CreatePage(context.Background(), confluence.CreatePageRequest{SpaceID: "BAD", Title: "X", Body: "<p>x</p>"})
	if !errors.Is(err, confluence.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestConfluenceService_CreatePage_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.CreatePage(context.Background(), confluence.CreatePageRequest{SpaceID: "S1", Title: "X", Body: "<p>x</p>"})
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_CreatePage_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.CreatePage(context.Background(), confluence.CreatePageRequest{SpaceID: "S1", Title: "X", Body: "<p>x</p>"})
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- UpdatePage tests ---

// TestConfluenceService_UpdatePage_VersionSupplied verifies that when version_number
// is explicitly provided, the service sends it as-is (no extra GET).
func TestConfluenceService_UpdatePage_VersionSupplied(t *testing.T) {
	requestCount := 0
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s (request #%d)", r.Method, requestCount)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(pageFixture("PAGE-1", "Updated", "S1", 3, "<p>updated</p>"))) //nolint:errcheck
	}))
	defer srv.Close()

	version := 3
	svc := confluence.NewService(srv.Client(), srv.URL)
	page, err := svc.UpdatePage(context.Background(), confluence.UpdatePageRequest{
		PageID:        "PAGE-1",
		Title:         "Updated",
		Body:          "<p>updated</p>",
		VersionNumber: &version,
	})
	if err != nil {
		t.Fatalf("UpdatePage() unexpected error: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected exactly 1 request (no auto-GET), got %d", requestCount)
	}
	if page.Title != "Updated" {
		t.Errorf("Title: expected 'Updated', got %s", page.Title)
	}

	// Verify version in body.
	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	versionField, ok := body["version"].(map[string]interface{})
	if !ok {
		t.Fatalf("version field not object, got %T", body["version"])
	}
	if versionField["number"] != float64(3) {
		t.Errorf("version.number: expected 3, got %v", versionField["number"])
	}
}

// TestConfluenceService_UpdatePage_AutoIncrement verifies Decision 2: when version
// is nil, the service GETs the page, reads current version, and PUTs with current+1.
func TestConfluenceService_UpdatePage_AutoIncrement(t *testing.T) {
	requestCount := 0
	var putBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			// First request: GET page returning version 7.
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(pageFixture("PAGE-1", "Current Title", "S1", 7, "<p>old</p>"))) //nolint:errcheck
		case http.MethodPut:
			// Second request: PUT update.
			putBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(pageFixture("PAGE-1", "New Title", "S1", 8, "<p>new</p>"))) //nolint:errcheck
		default:
			t.Errorf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	page, err := svc.UpdatePage(context.Background(), confluence.UpdatePageRequest{
		PageID:        "PAGE-1",
		Title:         "New Title",
		Body:          "<p>new</p>",
		VersionNumber: nil, // Trigger auto-increment
	})
	if err != nil {
		t.Fatalf("UpdatePage() unexpected error: %v", err)
	}

	// Must have made exactly 2 requests: GET then PUT.
	if requestCount != 2 {
		t.Errorf("expected 2 requests (GET + PUT), got %d", requestCount)
	}
	if page.Title != "New Title" {
		t.Errorf("Title: expected 'New Title', got %s", page.Title)
	}

	// Verify the PUT was sent with version = 7 + 1 = 8.
	var body map[string]interface{}
	if err := json.Unmarshal(putBody, &body); err != nil {
		t.Fatalf("failed to parse PUT body: %v", err)
	}
	versionField, ok := body["version"].(map[string]interface{})
	if !ok {
		t.Fatalf("version field not object, got %T", body["version"])
	}
	if versionField["number"] != float64(8) {
		t.Errorf("version.number in PUT: expected 8 (7+1), got %v", versionField["number"])
	}
}

// TestConfluenceService_UpdatePage_Conflict verifies that a 409 response maps to ErrConflict.
func TestConfluenceService_UpdatePage_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	version := 1
	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.UpdatePage(context.Background(), confluence.UpdatePageRequest{
		PageID:        "PAGE-1",
		Title:         "X",
		Body:          "<p>x</p>",
		VersionNumber: &version,
	})
	if !errors.Is(err, confluence.ErrConflict) {
		t.Errorf("expected ErrConflict for 409, got: %v", err)
	}
}

func TestConfluenceService_UpdatePage_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	version := 1
	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.UpdatePage(context.Background(), confluence.UpdatePageRequest{
		PageID:        "MISSING",
		Title:         "X",
		Body:          "<p>x</p>",
		VersionNumber: &version,
	})
	if !errors.Is(err, confluence.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestConfluenceService_UpdatePage_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	version := 1
	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.UpdatePage(context.Background(), confluence.UpdatePageRequest{
		PageID:        "PAGE-1",
		Title:         "X",
		Body:          "<p>x</p>",
		VersionNumber: &version,
	})
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_UpdatePage_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	version := 1
	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.UpdatePage(context.Background(), confluence.UpdatePageRequest{
		PageID:        "PAGE-1",
		Title:         "X",
		Body:          "<p>x</p>",
		VersionNumber: &version,
	})
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// TestConfluenceService_UpdatePage_AutoIncrementGetFails verifies that when the
// internal GET fails during auto-increment, the error is propagated correctly.
func TestConfluenceService_UpdatePage_AutoIncrementGetFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The GET for version fetch returns 404.
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.UpdatePage(context.Background(), confluence.UpdatePageRequest{
		PageID:        "MISSING",
		Title:         "X",
		Body:          "<p>x</p>",
		VersionNumber: nil, // triggers auto-increment GET
	})
	if err == nil {
		t.Fatal("expected error when GET fails, got nil")
	}
	// The error should wrap ErrNotFound from the internal GET.
	if !errors.Is(err, confluence.ErrNotFound) {
		t.Errorf("expected wrapped ErrNotFound, got: %v", err)
	}
}

// --- CreateFooterComment tests ---

func TestConfluenceService_CreateFooterComment_Success(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/footer-comments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(commentFixture("FC1", "<p>My comment</p>", 1))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	comment, err := svc.CreateFooterComment(context.Background(), confluence.CreateCommentRequest{
		PageID: "PAGE-1",
		Body:   "<p>My comment</p>",
	})
	if err != nil {
		t.Fatalf("CreateFooterComment() unexpected error: %v", err)
	}
	if comment.ID != "FC1" {
		t.Errorf("ID: expected FC1, got %s", comment.ID)
	}

	// Verify body contains storage representation.
	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	bodyField, ok := body["body"].(map[string]interface{})
	if !ok {
		t.Fatalf("body field not object, got %T", body["body"])
	}
	if bodyField["representation"] != "storage" {
		t.Errorf("body.representation: expected 'storage', got %v", bodyField["representation"])
	}
}

func TestConfluenceService_CreateFooterComment_WithParent(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(commentFixture("REPLY-1", "<p>reply</p>", 1))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateFooterComment(context.Background(), confluence.CreateCommentRequest{
		PageID:          "PAGE-1",
		Body:            "<p>reply</p>",
		ParentCommentID: "C1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["parentCommentId"] != "C1" {
		t.Errorf("parentCommentId: expected 'C1', got %v", body["parentCommentId"])
	}
}

func TestConfluenceService_CreateFooterComment_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateFooterComment(context.Background(), confluence.CreateCommentRequest{PageID: "BAD", Body: "<p>x</p>"})
	if !errors.Is(err, confluence.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestConfluenceService_CreateFooterComment_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateFooterComment(context.Background(), confluence.CreateCommentRequest{PageID: "P1", Body: "<p>x</p>"})
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_CreateFooterComment_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateFooterComment(context.Background(), confluence.CreateCommentRequest{PageID: "P1", Body: "<p>x</p>"})
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- CreateInlineComment tests ---

func TestConfluenceService_CreateInlineComment_Success(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/inline-comments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(commentFixture("IC1", "<p>Inline comment</p>", 1))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	comment, err := svc.CreateInlineComment(context.Background(), confluence.CreateInlineCommentRequest{
		PageID:        "PAGE-1",
		Body:          "<p>Inline comment</p>",
		TextSelection: "deploy pipeline",
	})
	if err != nil {
		t.Fatalf("CreateInlineComment() unexpected error: %v", err)
	}
	if comment.ID != "IC1" {
		t.Errorf("ID: expected IC1, got %s", comment.ID)
	}

	// Verify the request body includes inlineCommentProperties with textSelection.
	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	props, ok := body["inlineCommentProperties"].(map[string]interface{})
	if !ok {
		t.Fatalf("inlineCommentProperties not object, got %T", body["inlineCommentProperties"])
	}
	if props["textSelection"] != "deploy pipeline" {
		t.Errorf("textSelection: expected 'deploy pipeline', got %v", props["textSelection"])
	}
}

func TestConfluenceService_CreateInlineComment_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateInlineComment(context.Background(), confluence.CreateInlineCommentRequest{
		PageID:        "BAD",
		Body:          "<p>x</p>",
		TextSelection: "text",
	})
	if !errors.Is(err, confluence.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestConfluenceService_CreateInlineComment_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateInlineComment(context.Background(), confluence.CreateInlineCommentRequest{
		PageID:        "P1",
		Body:          "<p>x</p>",
		TextSelection: "text",
	})
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_CreateInlineComment_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateInlineComment(context.Background(), confluence.CreateInlineCommentRequest{
		PageID:        "P1",
		Body:          "<p>x</p>",
		TextSelection: "text",
	})
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- SearchContent tests ---

func TestConfluenceService_SearchContent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/rest/api/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("cql") == "" {
			t.Error("expected cql query parameter")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		item := searchResultFixture("P1", "Deploy Pipeline", "DEV", "...deploy pipeline steps...")
		w.Write([]byte(`{"results": [` + item + `]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	results, err := svc.SearchContent(context.Background(), "type=page AND space=DEV", 10)
	if err != nil {
		t.Fatalf("SearchContent() unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ContentID != "P1" {
		t.Errorf("ContentID: expected P1, got %s", results[0].ContentID)
	}
	if results[0].Title != "Deploy Pipeline" {
		t.Errorf("Title: expected 'Deploy Pipeline', got %s", results[0].Title)
	}
	if results[0].SpaceKey != "DEV" {
		t.Errorf("SpaceKey: expected DEV, got %s", results[0].SpaceKey)
	}
	if results[0].Excerpt != "...deploy pipeline steps..." {
		t.Errorf("Excerpt: expected '...deploy pipeline steps...', got %q", results[0].Excerpt)
	}
}

func TestConfluenceService_SearchContent_NoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results": []}`)) //nolint:errcheck
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	results, err := svc.SearchContent(context.Background(), "type=page AND title='NonExistent'", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestConfluenceService_SearchContent_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.SearchContent(context.Background(), "type=page", 10)
	if !errors.Is(err, confluence.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestConfluenceService_SearchContent_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := confluence.NewService(srv.Client(), srv.URL)
	_, err := svc.SearchContent(context.Background(), "type=page", 10)
	if !errors.Is(err, confluence.ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got: %v", err)
	}
}

// --- Service interface compliance ---

// TestConfluenceService_ImplementsInterface verifies that *ConfluenceService satisfies
// the Service interface at compile time (fails to compile if interface is not implemented).
func TestConfluenceService_ImplementsInterface(t *testing.T) {
	var _ confluence.Service = (*confluence.ConfluenceService)(nil)
}
