package updatecheck_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/updatecheck"
)

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		wantCmp  int
		wantOK   bool
	}{
		{"equal", "v1.2.2", "v1.2.2", 0, true},
		{"patch behind", "v1.2.2", "v1.2.3", -1, true},
		{"minor behind", "v1.2.9", "v1.3.0", -1, true},
		{"major behind", "v1.9.9", "v2.0.0", -1, true},
		{"ahead", "v1.3.0", "v1.2.9", 1, true},
		{"no v prefix", "1.2.2", "1.2.3", -1, true},
		{"prerelease suffix ignored", "v1.2.2-rc1", "v1.2.2", 0, true},
		{"dev not comparable", "dev", "v1.2.3", 0, false},
		{"garbage latest not comparable", "v1.2.2", "nightly", 0, false},
		{"two-part version", "v1.2", "v1.2.0", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp, ok := updatecheck.CompareSemver(tt.current, tt.latest)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && cmp != tt.wantCmp {
				t.Errorf("cmp = %d, want %d", cmp, tt.wantCmp)
			}
		})
	}
}

func TestFetchLatestTag_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/repos/jinkp/atlassian-go-mcp/releases/latest" {
			t.Errorf("unexpected path: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3","name":"whatever"}`))
	}))
	defer srv.Close()

	tag, err := updatecheck.FetchLatestTag(context.Background(), srv.Client(), srv.URL, "jinkp/atlassian-go-mcp")
	if err != nil {
		t.Fatalf("FetchLatestTag: %v", err)
	}
	if tag != "v1.2.3" {
		t.Errorf("tag = %q, want v1.2.3", tag)
	}
}

func TestFetchLatestTag_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden) // e.g. rate limited
	}))
	defer srv.Close()

	if _, err := updatecheck.FetchLatestTag(context.Background(), srv.Client(), srv.URL, "x/y"); err == nil {
		t.Error("expected an error on non-200 response")
	}
}

func TestFetchLatestTag_EmptyTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"no tag here"}`))
	}))
	defer srv.Close()

	if _, err := updatecheck.FetchLatestTag(context.Background(), srv.Client(), srv.URL, "x/y"); err == nil {
		t.Error("expected an error when tag_name is missing")
	}
}
