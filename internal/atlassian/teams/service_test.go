package teams_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
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

// newTeamsService creates a TeamsRestService pointing at the given httptest server.
// The orgID path is "testorg" — the service builds the URL as:
//   <server.URL> + path-suffix-from-orgBase
// BUT the service hardcodes https://api.atlassian.com as the base. For testing, we need
// to override the base URL so requests hit the test server. We achieve this by constructing
// a test-aware doer that rewrites the host portion of requests.
//
// The simplest approach: wrap http.DefaultClient with a custom Transport that replaces
// the host. We keep it simple and instead use a custom HTTPDoer shim.
type redirectDoer struct {
	client  *http.Client
	origURL string
	newURL  string
}

func (d *redirectDoer) Do(req *http.Request) (*http.Response, error) {
	// Replace the base URL in the request URL
	rawURL := req.URL.String()
	rawURL = strings.Replace(rawURL, d.origURL, d.newURL, 1)
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, rawURL, req.Body)
	if err != nil {
		return nil, err
	}
	for k, v := range req.Header {
		newReq.Header[k] = v
	}
	return d.client.Do(newReq)
}

// newTeamsServiceForTest creates a TeamsService backed by the given httptest server.
// orgID "testorg" is used; the service's https://api.atlassian.com base is redirected to srv.URL.
func newTeamsServiceForTest(srv *httptest.Server) teams.TeamsService {
	doer := &redirectDoer{
		client:  srv.Client(),
		origURL: "https://api.atlassian.com",
		newURL:  srv.URL,
	}
	return teams.NewService(doer, "testorg")
}

// --- TestGetTeams ---

func TestGetTeams(t *testing.T) {
	tests := []struct {
		name        string
		serverCode  int
		serverBody  string
		query       string
		maxResults  int
		wantErr     error
		wantErrMsg  string
		wantLen     int
		wantFirst   teams.Team
		wantCursor  string
	}{
		{
			name:       "success — returns 2 teams",
			serverCode: http.StatusOK,
			serverBody: `{"entities":[{"teamId":"T1","displayName":"Alpha","description":"Design","organizationId":"ORG1","teamType":"OPEN","state":"ACTIVE"},{"teamId":"T2","displayName":"Beta","description":"","organizationId":"ORG1","teamType":"MEMBER_INVITE","state":"ACTIVE"}],"cursor":"next-cursor"}`,
			maxResults: 50,
			wantLen:    2,
			wantFirst: teams.Team{
				ID: "T1", DisplayName: "Alpha", Description: "Design",
				OrganizationID: "ORG1", TeamType: "OPEN", State: "ACTIVE",
			},
			wantCursor: "next-cursor",
		},
		{
			name:       "success — empty entities returns non-nil empty slice",
			serverCode: http.StatusOK,
			serverBody: `{"entities":[],"cursor":""}`,
			maxResults: 50,
			wantLen:    0,
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
			serverBody: `{"message":"Forbidden"}`,
			maxResults: 50,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name:       "400 returns descriptive error",
			serverCode: http.StatusBadRequest,
			serverBody: `bad request`,
			maxResults: 50,
			wantErrMsg: "bad request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()
			svc := newTeamsServiceForTest(srv)

			result, err := svc.GetTeams(context.Background(), tc.query, tc.maxResults)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.wantErr)
				}
				if err != tc.wantErr {
					t.Errorf("error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			if len(result.Teams) != tc.wantLen {
				t.Errorf("len(Teams): got %d, want %d", len(result.Teams), tc.wantLen)
			}
			if tc.wantLen > 0 {
				got := result.Teams[0]
				if got.ID != tc.wantFirst.ID {
					t.Errorf("Teams[0].ID: got %q, want %q", got.ID, tc.wantFirst.ID)
				}
				if got.DisplayName != tc.wantFirst.DisplayName {
					t.Errorf("Teams[0].DisplayName: got %q, want %q", got.DisplayName, tc.wantFirst.DisplayName)
				}
				if got.Description != tc.wantFirst.Description {
					t.Errorf("Teams[0].Description: got %q, want %q", got.Description, tc.wantFirst.Description)
				}
				if got.OrganizationID != tc.wantFirst.OrganizationID {
					t.Errorf("Teams[0].OrganizationID: got %q, want %q", got.OrganizationID, tc.wantFirst.OrganizationID)
				}
				if got.State != tc.wantFirst.State {
					t.Errorf("Teams[0].State: got %q, want %q", got.State, tc.wantFirst.State)
				}
			}
			if tc.wantCursor != "" && result.NextCursor != tc.wantCursor {
				t.Errorf("NextCursor: got %q, want %q", result.NextCursor, tc.wantCursor)
			}
		})
	}
}

// TestGetTeams_DefaultMaxResults verifies that maxResults=0 sends size=50 in the URL.
func TestGetTeams_DefaultMaxResults(t *testing.T) {
	var gotURL string
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"entities":[],"cursor":""}`)) //nolint:errcheck
	})
	defer close()

	svc := newTeamsServiceForTest(srv)
	_, err := svc.GetTeams(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "size=50") {
		t.Errorf("expected URL to contain size=50, got %q", gotURL)
	}
}

// TestGetTeams_QueryParam verifies that a non-empty query is appended to the URL.
func TestGetTeams_QueryParam(t *testing.T) {
	var gotURL string
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"entities":[],"cursor":""}`)) //nolint:errcheck
	})
	defer close()

	svc := newTeamsServiceForTest(srv)
	_, err := svc.GetTeams(context.Background(), "alpha", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "query=alpha") {
		t.Errorf("expected URL to contain query=alpha, got %q", gotURL)
	}
	if !strings.Contains(gotURL, "size=10") {
		t.Errorf("expected URL to contain size=10, got %q", gotURL)
	}
}

// --- TestGetTeam ---

func TestGetTeam(t *testing.T) {
	tests := []struct {
		name       string
		serverCode int
		serverBody string
		teamID     string
		wantErr    error
		wantErrMsg string
		wantTeam   teams.Team
	}{
		{
			name:       "success — returns team",
			serverCode: http.StatusOK,
			serverBody: `{"teamId":"T1","displayName":"Alpha","description":"Design team","organizationId":"ORG1","teamType":"OPEN","state":"ACTIVE"}`,
			teamID:     "T1",
			wantTeam: teams.Team{
				ID: "T1", DisplayName: "Alpha", Description: "Design team",
				OrganizationID: "ORG1", TeamType: "OPEN", State: "ACTIVE",
			},
		},
		{
			name:       "404 returns ErrNotFound",
			serverCode: http.StatusNotFound,
			serverBody: `{"message":"Not Found"}`,
			teamID:     "MISSING",
			wantErr:    jira.ErrNotFound,
		},
		{
			name:       "401 returns ErrUnauthorized",
			serverCode: http.StatusUnauthorized,
			serverBody: `{"message":"Unauthorized"}`,
			teamID:     "T1",
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name:       "403 returns ErrUnauthorized",
			serverCode: http.StatusForbidden,
			serverBody: `{"message":"Forbidden"}`,
			teamID:     "T1",
			wantErr:    jira.ErrUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()
			svc := newTeamsServiceForTest(srv)

			team, err := svc.GetTeam(context.Background(), tc.teamID)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.wantErr)
				}
				if err != tc.wantErr {
					t.Errorf("error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if team == nil {
				t.Fatal("team is nil")
			}
			if team.ID != tc.wantTeam.ID {
				t.Errorf("ID: got %q, want %q", team.ID, tc.wantTeam.ID)
			}
			if team.DisplayName != tc.wantTeam.DisplayName {
				t.Errorf("DisplayName: got %q, want %q", team.DisplayName, tc.wantTeam.DisplayName)
			}
			if team.State != tc.wantTeam.State {
				t.Errorf("State: got %q, want %q", team.State, tc.wantTeam.State)
			}
		})
	}
}

// --- TestGetTeamMembers ---

func TestGetTeamMembers(t *testing.T) {
	tests := []struct {
		name        string
		serverCode  int
		serverBody  string
		teamID      string
		maxResults  int
		wantErr     error
		wantErrMsg  string
		wantLen     int
		wantFirst   teams.TeamMember
	}{
		{
			name:       "success — returns 2 members",
			serverCode: http.StatusOK,
			serverBody: `{"results":[{"accountId":"ACC1"},{"accountId":"ACC2"}]}`,
			teamID:     "T1",
			maxResults: 50,
			wantLen:    2,
			wantFirst:  teams.TeamMember{AccountID: "ACC1"},
		},
		{
			name:       "success — empty results returns non-nil empty slice",
			serverCode: http.StatusOK,
			serverBody: `{"results":[]}`,
			teamID:     "T1",
			maxResults: 50,
			wantLen:    0,
		},
		{
			name:       "404 returns ErrNotFound",
			serverCode: http.StatusNotFound,
			serverBody: `{"message":"Not Found"}`,
			teamID:     "MISSING",
			maxResults: 50,
			wantErr:    jira.ErrNotFound,
		},
		{
			name:       "401 returns ErrUnauthorized",
			serverCode: http.StatusUnauthorized,
			serverBody: `{}`,
			teamID:     "T1",
			maxResults: 50,
			wantErr:    jira.ErrUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, close := newTestServer(tc.serverCode, tc.serverBody)
			defer close()
			svc := newTeamsServiceForTest(srv)

			members, err := svc.GetTeamMembers(context.Background(), tc.teamID, tc.maxResults)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.wantErr)
				}
				if err != tc.wantErr {
					t.Errorf("error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if members == nil {
				t.Fatal("members is nil")
			}
			if len(members) != tc.wantLen {
				t.Errorf("len(members): got %d, want %d", len(members), tc.wantLen)
			}
			if tc.wantLen > 0 {
				if members[0].AccountID != tc.wantFirst.AccountID {
					t.Errorf("members[0].AccountID: got %q, want %q", members[0].AccountID, tc.wantFirst.AccountID)
				}
			}
		})
	}
}

// TestGetTeamMembers_DefaultMaxResults verifies that maxResults=0 sends first=50 in the request body.
func TestGetTeamMembers_DefaultMaxResults(t *testing.T) {
	var gotBody []byte
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results":[]}`)) //nolint:errcheck
	})
	defer close()

	svc := newTeamsServiceForTest(srv)
	_, err := svc.GetTeamMembers(context.Background(), "T1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var reqBody map[string]interface{}
	if err := json.Unmarshal(gotBody, &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	first, ok := reqBody["first"].(float64)
	if !ok {
		t.Fatalf("request body 'first' field missing or wrong type: %v", reqBody)
	}
	if int(first) != 50 {
		t.Errorf("expected first=50 in request body, got %v", first)
	}
}

// TestGetTeamMembers_PostMethod verifies that the members endpoint uses POST.
func TestGetTeamMembers_PostMethod(t *testing.T) {
	var gotMethod string
	srv, close := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results":[]}`)) //nolint:errcheck
	})
	defer close()

	svc := newTeamsServiceForTest(srv)
	_, err := svc.GetTeamMembers(context.Background(), "T1", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST method, got %q", gotMethod)
	}
}
