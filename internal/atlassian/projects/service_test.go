package projects_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
)

// --- helpers ---

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

// --- TestGetProjects ---

func TestGetProjects(t *testing.T) {
	tests := []struct {
		name        string
		serverBody  string
		serverCode  int
		maxResults  int
		wantErr     error
		wantLen     int
		wantFirst   projects.Project
		wantErrMsg  string
	}{
		{
			name:       "success — returns projects with lead as accountId string",
			serverCode: http.StatusOK,
			serverBody: `[{"id":"10000","key":"PROJ","name":"My Project","description":"Desc","projectTypeKey":"software","lead":{"accountId":"acc123","displayName":"John"},"self":"https://example.atlassian.net/rest/api/3/project/10000"}]`,
			maxResults: 50,
			wantLen:    1,
			wantFirst: projects.Project{
				ID: "10000", Key: "PROJ", Name: "My Project", Description: "Desc",
				ProjectType: "software", Lead: "acc123",
				URL: "https://example.atlassian.net/rest/api/3/project/10000",
			},
		},
		{
			name:       "multiple projects returned",
			serverCode: http.StatusOK,
			serverBody: `[{"id":"10000","key":"PROJ","name":"P1","projectTypeKey":"software"},{"id":"10001","key":"PROJ2","name":"P2","projectTypeKey":"business"}]`,
			maxResults: 50,
			wantLen:    2,
		},
		{
			name:       "empty array returns non-nil empty slice",
			serverCode: http.StatusOK,
			serverBody: `[]`,
			maxResults: 50,
			wantLen:    0,
		},
		{
			name:       "project without lead — Lead field is empty string",
			serverCode: http.StatusOK,
			serverBody: `[{"id":"10000","key":"PROJ","name":"My Project","projectTypeKey":"software"}]`,
			maxResults: 50,
			wantLen:    1,
			wantFirst: projects.Project{
				ID: "10000", Key: "PROJ", Name: "My Project",
				ProjectType: "software", Lead: "",
			},
		},
		{
			name:       "401 returns ErrUnauthorized",
			serverCode: http.StatusUnauthorized,
			serverBody: `{"message":"Unauthorized"}`,
			maxResults: 50,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name:       "403 returns ErrUnauthorized",
			serverCode: http.StatusForbidden,
			serverBody: `{}`,
			maxResults: 50,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name:       "400 returns descriptive error",
			serverCode: http.StatusBadRequest,
			serverBody: `bad request`,
			maxResults: 50,
			wantErrMsg: "400",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()

			svc := projects.NewService(srv.Client(), srv.URL)
			result, err := svc.GetProjects(context.Background(), tc.maxResults)

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
			if tc.wantLen > 0 && tc.wantFirst.Key != "" {
				got := result[0]
				if got.ID != tc.wantFirst.ID {
					t.Errorf("ID: got %q, want %q", got.ID, tc.wantFirst.ID)
				}
				if got.Key != tc.wantFirst.Key {
					t.Errorf("Key: got %q, want %q", got.Key, tc.wantFirst.Key)
				}
				if got.ProjectType != tc.wantFirst.ProjectType {
					t.Errorf("ProjectType: got %q, want %q", got.ProjectType, tc.wantFirst.ProjectType)
				}
				if got.Lead != tc.wantFirst.Lead {
					t.Errorf("Lead: got %q, want %q", got.Lead, tc.wantFirst.Lead)
				}
			}
		})
	}
}

// TestGetProjects_DefaultsMaxResults verifies maxResults=0 defaults to 50 in query.
func TestGetProjects_DefaultsMaxResults(t *testing.T) {
	var gotQuery string
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`)) //nolint:errcheck
	})
	defer close()

	svc := projects.NewService(srv.Client(), srv.URL)
	_, err := svc.GetProjects(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotQuery, "maxResults=50") {
		t.Errorf("query %q does not contain maxResults=50", gotQuery)
	}
}

// --- TestGetProject ---

func TestGetProject(t *testing.T) {
	tests := []struct {
		name        string
		projectKey  string
		serverBody  string
		serverCode  int
		wantErr     error
		wantProject projects.Project
		wantErrMsg  string
	}{
		{
			name:       "success — returns project",
			projectKey: "PROJ",
			serverCode: http.StatusOK,
			serverBody: `{"id":"10000","key":"PROJ","name":"My Project","description":"Desc","projectTypeKey":"software","lead":{"accountId":"acc123"},"self":"https://x.atlassian.net"}`,
			wantProject: projects.Project{
				ID: "10000", Key: "PROJ", Name: "My Project",
				Description: "Desc", ProjectType: "software", Lead: "acc123",
			},
		},
		{
			name:       "404 returns ErrNotFound",
			projectKey: "MISSING",
			serverCode: http.StatusNotFound,
			serverBody: `{}`,
			wantErr:    jira.ErrNotFound,
		},
		{
			name:       "401 returns ErrUnauthorized",
			projectKey: "PROJ",
			serverCode: http.StatusUnauthorized,
			serverBody: `{}`,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name:       "403 returns ErrUnauthorized",
			projectKey: "PROJ",
			serverCode: http.StatusForbidden,
			serverBody: `{}`,
			wantErr:    jira.ErrUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()

			svc := projects.NewService(srv.Client(), srv.URL)
			result, err := svc.GetProject(context.Background(), tc.projectKey)

			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Errorf("error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Key != tc.wantProject.Key {
				t.Errorf("Key: got %q, want %q", result.Key, tc.wantProject.Key)
			}
			if result.Lead != tc.wantProject.Lead {
				t.Errorf("Lead: got %q, want %q", result.Lead, tc.wantProject.Lead)
			}
		})
	}
}

// TestGetProject_URLContainsKey verifies the request URL contains the project key.
func TestGetProject_URLContainsKey(t *testing.T) {
	var gotPath string
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"10000","key":"PROJ","name":"My Project","projectTypeKey":"software"}`)) //nolint:errcheck
	})
	defer close()

	svc := projects.NewService(srv.Client(), srv.URL)
	_, err := svc.GetProject(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "/rest/api/3/project/PROJ") {
		t.Errorf("path %q does not contain /rest/api/3/project/PROJ", gotPath)
	}
}

// --- TestSearchProjects ---

func TestSearchProjects(t *testing.T) {
	tests := []struct {
		name        string
		req         projects.SearchProjectsRequest
		serverBody  string
		serverCode  int
		wantErr     error
		wantTotal   int
		wantLen     int
		wantErrMsg  string
	}{
		{
			name:       "success — returns paginated result",
			req:        projects.SearchProjectsRequest{Query: "PROJ", MaxResults: 10},
			serverCode: http.StatusOK,
			serverBody: `{"values":[{"id":"10000","key":"PROJ","name":"My Project","projectTypeKey":"software"},{"id":"10001","key":"PROJ2","name":"Second","projectTypeKey":"business"}],"total":2,"startAt":0,"maxResults":10}`,
			wantTotal:  2,
			wantLen:    2,
		},
		{
			name:       "empty results — Projects non-nil empty slice",
			req:        projects.SearchProjectsRequest{MaxResults: 50},
			serverCode: http.StatusOK,
			serverBody: `{"values":[],"total":0,"startAt":0,"maxResults":50}`,
			wantTotal:  0,
			wantLen:    0,
		},
		{
			name:       "401 returns ErrUnauthorized",
			req:        projects.SearchProjectsRequest{MaxResults: 50},
			serverCode: http.StatusUnauthorized,
			serverBody: `{}`,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name:       "403 returns ErrUnauthorized",
			req:        projects.SearchProjectsRequest{MaxResults: 50},
			serverCode: http.StatusForbidden,
			serverBody: `{}`,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name:       "400 returns descriptive error",
			req:        projects.SearchProjectsRequest{MaxResults: 50},
			serverCode: http.StatusBadRequest,
			serverBody: `invalid query`,
			wantErrMsg: "400",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()

			svc := projects.NewService(srv.Client(), srv.URL)
			result, err := svc.SearchProjects(context.Background(), tc.req)

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
			if result.Total != tc.wantTotal {
				t.Errorf("Total: got %d, want %d", result.Total, tc.wantTotal)
			}
			if len(result.Projects) != tc.wantLen {
				t.Errorf("len(Projects): got %d, want %d", len(result.Projects), tc.wantLen)
			}
			if result.Projects == nil {
				t.Error("Projects must not be nil")
			}
		})
	}
}

// TestSearchProjects_DefaultsMaxResults verifies MaxResults=0 defaults to 50.
func TestSearchProjects_DefaultsMaxResults(t *testing.T) {
	var gotQuery string
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[],"total":0,"startAt":0,"maxResults":50}`)) //nolint:errcheck
	})
	defer close()

	svc := projects.NewService(srv.Client(), srv.URL)
	_, err := svc.SearchProjects(context.Background(), projects.SearchProjectsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotQuery, "maxResults=50") {
		t.Errorf("query %q does not contain maxResults=50", gotQuery)
	}
}

// TestSearchProjects_QueryAndStartAt verifies query and startAt forwarded in URL.
func TestSearchProjects_QueryAndStartAt(t *testing.T) {
	var gotQuery string
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[],"total":0,"startAt":25,"maxResults":10}`)) //nolint:errcheck
	})
	defer close()

	svc := projects.NewService(srv.Client(), srv.URL)
	_, err := svc.SearchProjects(context.Background(), projects.SearchProjectsRequest{
		Query: "PROJ", MaxResults: 10, StartAt: 25,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotQuery, "query=PROJ") {
		t.Errorf("query %q does not contain query=PROJ", gotQuery)
	}
	if !strings.Contains(gotQuery, "maxResults=10") {
		t.Errorf("query %q does not contain maxResults=10", gotQuery)
	}
	if !strings.Contains(gotQuery, "startAt=25") {
		t.Errorf("query %q does not contain startAt=25", gotQuery)
	}
}

// --- TestUpdateProject ---

func TestUpdateProject(t *testing.T) {
	ptrStr := func(s string) *string { return &s }

	tests := []struct {
		name        string
		projectKey  string
		req         projects.UpdateProjectRequest
		serverBody  string
		serverCode  int
		wantErr     error
		wantProject projects.Project
		wantErrMsg  string
	}{
		{
			name:       "success — returns updated project",
			projectKey: "PROJ",
			req:        projects.UpdateProjectRequest{Name: ptrStr("New Name")},
			serverCode: http.StatusOK,
			serverBody: `{"id":"10000","key":"PROJ","name":"New Name","projectTypeKey":"software"}`,
			wantProject: projects.Project{
				ID: "10000", Key: "PROJ", Name: "New Name", ProjectType: "software",
			},
		},
		{
			name:       "nil fields produce minimal request body",
			projectKey: "PROJ",
			req:        projects.UpdateProjectRequest{},
			serverCode: http.StatusOK,
			serverBody: `{"id":"10000","key":"PROJ","name":"My Project","projectTypeKey":"software"}`,
			wantProject: projects.Project{Key: "PROJ"},
		},
		{
			name:       "lead field sent as leadAccountId in body",
			projectKey: "PROJ",
			req:        projects.UpdateProjectRequest{Lead: ptrStr("acc123")},
			serverCode: http.StatusOK,
			serverBody: `{"id":"10000","key":"PROJ","name":"My Project","projectTypeKey":"software","lead":{"accountId":"acc123"}}`,
			wantProject: projects.Project{Key: "PROJ", Lead: "acc123"},
		},
		{
			name:       "404 returns ErrNotFound",
			projectKey: "MISSING",
			req:        projects.UpdateProjectRequest{Name: ptrStr("X")},
			serverCode: http.StatusNotFound,
			serverBody: `{}`,
			wantErr:    jira.ErrNotFound,
		},
		{
			name:       "401 returns ErrUnauthorized",
			projectKey: "PROJ",
			req:        projects.UpdateProjectRequest{},
			serverCode: http.StatusUnauthorized,
			serverBody: `{}`,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name:       "403 returns ErrUnauthorized",
			projectKey: "PROJ",
			req:        projects.UpdateProjectRequest{},
			serverCode: http.StatusForbidden,
			serverBody: `{}`,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name:       "400 returns body message",
			projectKey: "PROJ",
			req:        projects.UpdateProjectRequest{Name: ptrStr("")},
			serverCode: http.StatusBadRequest,
			serverBody: `Name cannot be empty`,
			wantErrMsg: "Name cannot be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()

			svc := projects.NewService(srv.Client(), srv.URL)
			result, err := svc.UpdateProject(context.Background(), tc.projectKey, tc.req)

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
			if result.Key != tc.wantProject.Key {
				t.Errorf("Key: got %q, want %q", result.Key, tc.wantProject.Key)
			}
		})
	}
}

// TestUpdateProject_UsesHTTPPut verifies the PUT method and URL.
func TestUpdateProject_UsesHTTPPut(t *testing.T) {
	var gotMethod, gotPath string
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"10000","key":"PROJ","name":"My Project","projectTypeKey":"software"}`)) //nolint:errcheck
	})
	defer close()

	svc := projects.NewService(srv.Client(), srv.URL)
	name := "My Project"
	_, err := svc.UpdateProject(context.Background(), "PROJ", projects.UpdateProjectRequest{Name: &name})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method: got %q, want PUT", gotMethod)
	}
	if !strings.Contains(gotPath, "/rest/api/3/project/PROJ") {
		t.Errorf("path %q does not contain /rest/api/3/project/PROJ", gotPath)
	}
}

// TestUpdateProject_LeadAccountIDInBody verifies leadAccountId appears in request body.
func TestUpdateProject_LeadAccountIDInBody(t *testing.T) {
	var gotBody string
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		gotBody = string(bodyBytes)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"10000","key":"PROJ","name":"My Project","projectTypeKey":"software","lead":{"accountId":"acc123"}}`)) //nolint:errcheck
	})
	defer close()

	svc := projects.NewService(srv.Client(), srv.URL)
	lead := "acc123"
	_, err := svc.UpdateProject(context.Background(), "PROJ", projects.UpdateProjectRequest{Lead: &lead})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var bodyMap map[string]interface{}
	if err := json.Unmarshal([]byte(gotBody), &bodyMap); err != nil {
		t.Fatalf("body is not valid JSON: %v\nbody: %s", err, gotBody)
	}
	if bodyMap["leadAccountId"] != "acc123" {
		t.Errorf("leadAccountId: got %v, want acc123", bodyMap["leadAccountId"])
	}
}
