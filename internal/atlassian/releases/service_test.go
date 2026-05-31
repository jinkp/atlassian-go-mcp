package releases_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
)

// helpers

func newTestServer(status int, body string) (*httptest.Server, func()) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body)) //nolint:errcheck
	}))
	return srv, srv.Close
}

func newTestServerFunc(fn http.HandlerFunc) (*httptest.Server, func()) {
	srv := httptest.NewServer(fn)
	return srv, srv.Close
}

// --- TestGetReleases ---

func TestGetReleases(t *testing.T) {
	tests := []struct {
		name        string
		serverBody  string
		serverCode  int
		wantErr     error
		wantLen     int
		wantFirst   releases.Release
		wantErrMsg  string
	}{
		{
			name:       "success — returns releases with projectID as string",
			serverCode: http.StatusOK,
			serverBody: `[{"id":"10001","name":"v1.0","description":"First","archived":false,"released":true,"startDate":"2026-01-01","releaseDate":"2026-02-01","projectId":10000}]`,
			wantLen:    1,
			wantFirst: releases.Release{
				ID: "10001", Name: "v1.0", Description: "First",
				Released: true, Archived: false,
				StartDate: "2026-01-01", ReleaseDate: "2026-02-01",
				ProjectID: "10000",
			},
		},
		{
			name:       "empty array returns non-nil empty slice",
			serverCode: http.StatusOK,
			serverBody: `[]`,
			wantLen:    0,
		},
		{
			name:       "401 returns ErrUnauthorized",
			serverCode: http.StatusUnauthorized,
			serverBody: `{"message":"Unauthorized"}`,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name:       "403 returns ErrUnauthorized",
			serverCode: http.StatusForbidden,
			serverBody: `{}`,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name:       "400 returns descriptive error",
			serverCode: http.StatusBadRequest,
			serverBody: `bad request`,
			wantErrMsg: "400",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()

			svc := releases.NewService(srv.Client(), srv.URL)
			result, err := svc.GetReleases(context.Background(), "PROJ")

			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Errorf("error: got %v, want %v", err, tc.wantErr)
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
			if len(result) != tc.wantLen {
				t.Errorf("len: got %d, want %d", len(result), tc.wantLen)
			}
			if tc.wantLen > 0 && result[0] != tc.wantFirst {
				t.Errorf("first release: got %+v, want %+v", result[0], tc.wantFirst)
			}
		})
	}
}

// --- TestGetRelease ---

func TestGetRelease(t *testing.T) {
	tests := []struct {
		name       string
		serverCode int
		serverBody string
		wantErr    error
		wantID     string
	}{
		{
			name:       "200 returns release",
			serverCode: http.StatusOK,
			serverBody: `{"id":"10001","name":"v1.0","archived":false,"released":true,"projectId":10000}`,
			wantID:     "10001",
		},
		{
			name:       "404 returns ErrNotFound",
			serverCode: http.StatusNotFound,
			serverBody: `{}`,
			wantErr:    jira.ErrNotFound,
		},
		{
			name:       "401 returns ErrUnauthorized",
			serverCode: http.StatusUnauthorized,
			serverBody: `{}`,
			wantErr:    jira.ErrUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()

			svc := releases.NewService(srv.Client(), srv.URL)
			result, err := svc.GetRelease(context.Background(), "10001")

			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Errorf("error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID != tc.wantID {
				t.Errorf("ID: got %q, want %q", result.ID, tc.wantID)
			}
		})
	}
}

// --- TestGetReleaseIssueCounts ---

func TestGetReleaseIssueCounts(t *testing.T) {
	tests := []struct {
		name       string
		serverCode int
		serverBody string
		wantErr    error
		wantFix    int
		wantAffects int
	}{
		{
			name:        "200 returns issue counts",
			serverCode:  http.StatusOK,
			serverBody:  `{"fixVersion":5,"affectsVersion":3}`,
			wantFix:     5,
			wantAffects: 3,
		},
		{
			name:       "404 returns ErrNotFound",
			serverCode: http.StatusNotFound,
			serverBody: `{}`,
			wantErr:    jira.ErrNotFound,
		},
		{
			name:       "401 returns ErrUnauthorized",
			serverCode: http.StatusUnauthorized,
			serverBody: `{}`,
			wantErr:    jira.ErrUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()

			svc := releases.NewService(srv.Client(), srv.URL)
			result, err := svc.GetReleaseIssueCounts(context.Background(), "10001")

			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Errorf("error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.FixVersion != tc.wantFix {
				t.Errorf("FixVersion: got %d, want %d", result.FixVersion, tc.wantFix)
			}
			if result.AffectsVersion != tc.wantAffects {
				t.Errorf("AffectsVersion: got %d, want %d", result.AffectsVersion, tc.wantAffects)
			}
		})
	}
}

// --- TestCreateRelease ---

func TestCreateRelease(t *testing.T) {
	tests := []struct {
		name       string
		req        releases.CreateReleaseRequest
		serverCode int
		serverBody string
		wantErr    error
		wantErrMsg string
		wantID     string
	}{
		{
			name:       "201 returns created release",
			req:        releases.CreateReleaseRequest{ProjectID: "10000", Name: "v1.0.0"},
			serverCode: http.StatusCreated,
			serverBody: `{"id":"10001","name":"v1.0.0","archived":false,"released":false,"projectId":10000}`,
			wantID:     "10001",
		},
		{
			name:       "400 returns descriptive error",
			req:        releases.CreateReleaseRequest{ProjectID: "10000", Name: ""},
			serverCode: http.StatusBadRequest,
			serverBody: `Name is required`,
			wantErrMsg: "Name is required",
		},
		{
			name:       "401 returns ErrUnauthorized",
			req:        releases.CreateReleaseRequest{ProjectID: "10000", Name: "v1.0.0"},
			serverCode: http.StatusUnauthorized,
			serverBody: `{}`,
			wantErr:    jira.ErrUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()

			svc := releases.NewService(srv.Client(), srv.URL)
			result, err := svc.CreateRelease(context.Background(), tc.req)

			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Errorf("error: got %v, want %v", err, tc.wantErr)
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
			if result.ID != tc.wantID {
				t.Errorf("ID: got %q, want %q", result.ID, tc.wantID)
			}
		})
	}
}

// TestCreateRelease_SendsProjectIDAsInt verifies the wire format uses integer projectId.
func TestCreateRelease_SendsProjectIDAsInt(t *testing.T) {
	var gotBody map[string]interface{}
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"10001","name":"v1.0","projectId":10000}`)) //nolint:errcheck
	})
	defer close()

	svc := releases.NewService(srv.Client(), srv.URL)
	_, err := svc.CreateRelease(context.Background(), releases.CreateReleaseRequest{ProjectID: "10000", Name: "v1.0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// projectId must be a JSON number, not string
	pid, ok := gotBody["projectId"].(float64)
	if !ok {
		t.Errorf("projectId in wire body was not a number: got %T %v", gotBody["projectId"], gotBody["projectId"])
	}
	if int(pid) != 10000 {
		t.Errorf("projectId: got %v, want 10000", pid)
	}
}

// --- TestUpdateRelease ---

func TestUpdateRelease(t *testing.T) {
	name := "v1.1.0"
	released := true

	tests := []struct {
		name       string
		req        releases.UpdateReleaseRequest
		serverCode int
		serverBody string
		wantErr    error
		wantName   string
	}{
		{
			name:       "200 updates name",
			req:        releases.UpdateReleaseRequest{Name: &name},
			serverCode: http.StatusOK,
			serverBody: `{"id":"10001","name":"v1.1.0","archived":false,"released":false,"projectId":10000}`,
			wantName:   "v1.1.0",
		},
		{
			name:       "200 updates released=true",
			req:        releases.UpdateReleaseRequest{Released: &released},
			serverCode: http.StatusOK,
			serverBody: `{"id":"10001","name":"v1.0","archived":false,"released":true,"projectId":10000}`,
			wantName:   "v1.0",
		},
		{
			name:       "404 returns ErrNotFound",
			req:        releases.UpdateReleaseRequest{Name: &name},
			serverCode: http.StatusNotFound,
			serverBody: `{}`,
			wantErr:    jira.ErrNotFound,
		},
		{
			name:       "401 returns ErrUnauthorized",
			req:        releases.UpdateReleaseRequest{Name: &name},
			serverCode: http.StatusUnauthorized,
			serverBody: `{}`,
			wantErr:    jira.ErrUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()

			svc := releases.NewService(srv.Client(), srv.URL)
			result, err := svc.UpdateRelease(context.Background(), "10001", tc.req)

			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Errorf("error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Name != tc.wantName {
				t.Errorf("Name: got %q, want %q", result.Name, tc.wantName)
			}
		})
	}
}

// TestUpdateRelease_NilFieldsOmitted verifies nil pointer fields are not sent in wire body.
func TestUpdateRelease_NilFieldsOmitted(t *testing.T) {
	var gotBody map[string]interface{}
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"10001","name":"v1.0","projectId":10000}`)) //nolint:errcheck
	})
	defer close()

	name := "v1.1.0"
	svc := releases.NewService(srv.Client(), srv.URL)
	_, err := svc.UpdateRelease(context.Background(), "10001", releases.UpdateReleaseRequest{Name: &name})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only "name" should be in the wire body; released and archived should be absent
	if _, ok := gotBody["released"]; ok {
		t.Error("released should not be present in wire body when nil")
	}
	if _, ok := gotBody["archived"]; ok {
		t.Error("archived should not be present in wire body when nil")
	}
	if gotBody["name"] != "v1.1.0" {
		t.Errorf("name in wire body: got %v, want v1.1.0", gotBody["name"])
	}
}
