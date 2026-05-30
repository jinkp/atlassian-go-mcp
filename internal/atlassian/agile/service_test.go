package agile_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// --- helpers ---

func newTestServer(handler http.HandlerFunc) (*httptest.Server, agile.AgileService) {
	srv := httptest.NewServer(handler)
	svc := agile.NewService(srv.Client(), srv.URL)
	return srv, svc
}

// --- TestGetBoards ---

func TestGetBoards(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		projectKey string
		maxResults int
		wantBoards int
		wantErr    error
		wantErrSub string
	}{
		{
			name: "boards returned for project",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.Path, "/rest/agile/1.0/board") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.URL.Query().Get("projectKeyOrId") != "PROJ" {
					t.Errorf("unexpected projectKeyOrId: %s", r.URL.Query().Get("projectKeyOrId"))
				}
				resp := map[string]interface{}{
					"values": []interface{}{
						map[string]interface{}{"id": 1, "name": "PROJ Scrum Board", "type": map[string]interface{}{"name": "scrum"}},
						map[string]interface{}{"id": 2, "name": "PROJ Kanban Board", "type": map[string]interface{}{"name": "kanban"}},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			},
			projectKey: "PROJ",
			maxResults: 50,
			wantBoards: 2,
		},
		{
			name: "no boards returns empty slice not nil",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]interface{}{"values": []interface{}{}}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			},
			projectKey: "EMPTY",
			maxResults: 50,
			wantBoards: 0,
		},
		{
			name: "HTTP 401 returns ErrUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			projectKey: "PROJ",
			maxResults: 50,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name: "HTTP 403 returns ErrUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			projectKey: "PROJ",
			maxResults: 50,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name: "HTTP 404 returns ErrNotFound",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			projectKey: "PROJ",
			maxResults: 50,
			wantErr:    jira.ErrNotFound,
		},
		{
			name: "HTTP 400 returns descriptive error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("invalid project")) //nolint:errcheck
			},
			projectKey: "PROJ",
			maxResults: 50,
			wantErrSub: "invalid project",
		},
		{
			name: "maxResults default 50 when zero",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("maxResults") != "50" {
					t.Errorf("expected maxResults=50, got %s", r.URL.Query().Get("maxResults"))
				}
				resp := map[string]interface{}{"values": []interface{}{}}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			},
			projectKey: "PROJ",
			maxResults: 0, // zero → defaults to 50
			wantBoards: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, svc := newTestServer(tc.handler)
			defer srv.Close()

			boards, err := svc.GetBoards(context.Background(), tc.projectKey, tc.maxResults)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("GetBoards error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if boards == nil {
				t.Fatal("boards slice must not be nil")
			}
			if len(boards) != tc.wantBoards {
				t.Errorf("len(boards): got %d, want %d", len(boards), tc.wantBoards)
			}
			// Verify domain fields on non-empty results
			for _, b := range boards {
				if b.ID == 0 {
					t.Errorf("Board.ID is zero")
				}
				if b.Name == "" {
					t.Errorf("Board.Name is empty")
				}
				if b.Type != "scrum" && b.Type != "kanban" {
					t.Errorf("Board.Type %q not in {scrum,kanban}", b.Type)
				}
			}
		})
	}
}

// --- TestGetSprints ---

func TestGetSprints(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		boardID     int
		state       string
		maxResults  int
		wantSprints int
		wantErr     error
	}{
		{
			name: "all sprints returned for board",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.Path, "/rest/agile/1.0/board/10/sprint") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				// state param must not be set when empty
				if r.URL.Query().Get("state") != "" {
					t.Errorf("state param should be absent, got %q", r.URL.Query().Get("state"))
				}
				resp := map[string]interface{}{
					"values": []interface{}{
						map[string]interface{}{"id": 1, "name": "Sprint 1", "state": "closed", "startDate": "2024-01-01", "endDate": "2024-01-14", "completeDate": "2024-01-14"},
						map[string]interface{}{"id": 2, "name": "Sprint 2", "state": "active", "startDate": "2024-01-15", "endDate": "2024-01-28"},
						map[string]interface{}{"id": 3, "name": "Sprint 3", "state": "future"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			},
			boardID:     10,
			state:       "",
			maxResults:  50,
			wantSprints: 3,
		},
		{
			name: "filter by state=active returns active sprints only",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("state") != "active" {
					t.Errorf("expected state=active, got %q", r.URL.Query().Get("state"))
				}
				resp := map[string]interface{}{
					"values": []interface{}{
						map[string]interface{}{"id": 2, "name": "Sprint 2", "state": "active", "startDate": "2024-01-15", "endDate": "2024-01-28"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			},
			boardID:     10,
			state:       "active",
			maxResults:  50,
			wantSprints: 1,
		},
		{
			name: "kanban board returns empty slice not nil",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]interface{}{"values": []interface{}{}}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			},
			boardID:     20,
			state:       "",
			maxResults:  50,
			wantSprints: 0,
		},
		{
			name: "HTTP 404 returns ErrNotFound",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			boardID:    999,
			state:      "",
			maxResults: 50,
			wantErr:    jira.ErrNotFound,
		},
		{
			name: "HTTP 401 returns ErrUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			boardID:    10,
			state:      "",
			maxResults: 50,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name: "HTTP 403 returns ErrUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			boardID:    10,
			state:      "",
			maxResults: 50,
			wantErr:    jira.ErrUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, svc := newTestServer(tc.handler)
			defer srv.Close()

			sprints, err := svc.GetSprints(context.Background(), tc.boardID, tc.state, tc.maxResults)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("GetSprints error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sprints == nil {
				t.Fatal("sprints slice must not be nil")
			}
			if len(sprints) != tc.wantSprints {
				t.Errorf("len(sprints): got %d, want %d", len(sprints), tc.wantSprints)
			}
			// Verify state field on active sprint
			for _, sp := range sprints {
				if sp.ID == 0 {
					t.Errorf("Sprint.ID is zero")
				}
				if sp.Name == "" {
					t.Errorf("Sprint.Name is empty")
				}
			}
		})
	}
}

// --- TestGetSprintIssues ---

func TestGetSprintIssues(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		sprintID   int
		maxResults int
		wantTotal  int
		wantIssues int
		wantErr    error
	}{
		{
			name: "sprint with 3 issues",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.Path, "/rest/agile/1.0/sprint/100/issue") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				resp := map[string]interface{}{
					"total":      3,
					"startAt":    0,
					"maxResults": 50,
					"issues": []interface{}{
						map[string]interface{}{
							"key": "PROJ-1",
							"fields": map[string]interface{}{
								"summary":  "Issue one",
								"status":   map[string]interface{}{"name": "To Do"},
								"assignee": map[string]interface{}{"displayName": "Alice"},
							},
						},
						map[string]interface{}{
							"key": "PROJ-2",
							"fields": map[string]interface{}{
								"summary":  "Issue two",
								"status":   map[string]interface{}{"name": "In Progress"},
								"assignee": nil,
							},
						},
						map[string]interface{}{
							"key": "PROJ-3",
							"fields": map[string]interface{}{
								"summary": "Issue three",
								"status":  map[string]interface{}{"name": "Done"},
							},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			},
			sprintID:   100,
			maxResults: 50,
			wantTotal:  3,
			wantIssues: 3,
		},
		{
			name: "empty sprint returns Issues=[] Total=0 nil error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]interface{}{
					"total":      0,
					"startAt":    0,
					"maxResults": 50,
					"issues":     []interface{}{},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			},
			sprintID:   200,
			maxResults: 50,
			wantTotal:  0,
			wantIssues: 0,
		},
		{
			name: "unassigned issue has empty Assignee field",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]interface{}{
					"total":      1,
					"startAt":    0,
					"maxResults": 50,
					"issues": []interface{}{
						map[string]interface{}{
							"key": "PROJ-5",
							"fields": map[string]interface{}{
								"summary":  "Unassigned issue",
								"status":   map[string]interface{}{"name": "To Do"},
								"assignee": nil,
							},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			},
			sprintID:   100,
			maxResults: 50,
			wantTotal:  1,
			wantIssues: 1,
		},
		{
			name: "HTTP 404 returns ErrNotFound",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			sprintID:   999,
			maxResults: 50,
			wantErr:    jira.ErrNotFound,
		},
		{
			name: "HTTP 403 returns ErrUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			sprintID:   100,
			maxResults: 50,
			wantErr:    jira.ErrUnauthorized,
		},
		{
			name: "HTTP 401 returns ErrUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			sprintID:   100,
			maxResults: 50,
			wantErr:    jira.ErrUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, svc := newTestServer(tc.handler)
			defer srv.Close()

			result, err := svc.GetSprintIssues(context.Background(), tc.sprintID, tc.maxResults)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("GetSprintIssues error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("SprintIssueResult must not be nil")
			}
			if result.Total != tc.wantTotal {
				t.Errorf("Total: got %d, want %d", result.Total, tc.wantTotal)
			}
			if len(result.Issues) != tc.wantIssues {
				t.Errorf("len(Issues): got %d, want %d", len(result.Issues), tc.wantIssues)
			}
			// Issues slice must never be nil (can be empty)
			if result.Issues == nil {
				t.Error("Issues must be non-nil slice (can be empty)")
			}
			// Verify non-empty results have key and summary
			for _, iss := range result.Issues {
				if iss.Key == "" {
					t.Errorf("SprintIssue.Key is empty")
				}
				if iss.Summary == "" {
					t.Errorf("SprintIssue.Summary is empty")
				}
			}
		})
	}
}

// --- TestUpdateSprint ---

func TestUpdateSprint(t *testing.T) {
	ptr := func(s string) *string { return &s }

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		sprintID   int
		req        agile.UpdateSprintRequest
		wantSprint *agile.Sprint
		wantErr    error
		wantErrSub string
	}{
		{
			name: "name update 200 returns updated sprint",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if !strings.Contains(r.URL.Path, "/rest/agile/1.0/sprint/42") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				// Verify only name field is in body
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode body: %v", err)
				}
				if _, ok := body["state"]; ok {
					t.Error("state should not be present in body when nil")
				}
				if body["name"] != "Sprint 5 Renamed" {
					t.Errorf("name: got %v, want Sprint 5 Renamed", body["name"])
				}
				resp := map[string]interface{}{
					"id": 42, "name": "Sprint 5 Renamed", "state": "active",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			},
			sprintID:   42,
			req:        agile.UpdateSprintRequest{Name: ptr("Sprint 5 Renamed")},
			wantSprint: &agile.Sprint{ID: 42, Name: "Sprint 5 Renamed", State: "active"},
		},
		{
			name: "close active sprint 200 returns closed sprint",
			handler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
				if body["state"] != "closed" {
					t.Errorf("state: got %v, want closed", body["state"])
				}
				resp := map[string]interface{}{
					"id": 42, "name": "Sprint 5", "state": "closed",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			},
			sprintID:   42,
			req:        agile.UpdateSprintRequest{State: ptr("closed")},
			wantSprint: &agile.Sprint{ID: 42, Name: "Sprint 5", State: "closed"},
		},
		{
			name: "close non-active sprint returns 400 with Jira message",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Sprint is not active")) //nolint:errcheck
			},
			sprintID:   99,
			req:        agile.UpdateSprintRequest{State: ptr("closed")},
			wantErrSub: "Sprint is not active",
		},
		{
			name: "HTTP 401 returns ErrUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			sprintID: 42,
			req:      agile.UpdateSprintRequest{Name: ptr("test")},
			wantErr:  jira.ErrUnauthorized,
		},
		{
			name: "HTTP 403 returns ErrUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			sprintID: 42,
			req:      agile.UpdateSprintRequest{Name: ptr("test")},
			wantErr:  jira.ErrUnauthorized,
		},
		{
			name: "HTTP 404 returns ErrNotFound",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			sprintID: 999,
			req:      agile.UpdateSprintRequest{Name: ptr("test")},
			wantErr:  jira.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, svc := newTestServer(tc.handler)
			defer srv.Close()

			sprint, err := svc.UpdateSprint(context.Background(), tc.sprintID, tc.req)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("UpdateSprint error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sprint == nil {
				t.Fatal("sprint must not be nil on success")
			}
			if sprint.ID != tc.wantSprint.ID {
				t.Errorf("Sprint.ID: got %d, want %d", sprint.ID, tc.wantSprint.ID)
			}
			if sprint.Name != tc.wantSprint.Name {
				t.Errorf("Sprint.Name: got %q, want %q", sprint.Name, tc.wantSprint.Name)
			}
			if sprint.State != tc.wantSprint.State {
				t.Errorf("Sprint.State: got %q, want %q", sprint.State, tc.wantSprint.State)
			}
		})
	}
}

// --- TestMoveIssuesToSprint ---

func TestMoveIssuesToSprint(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		sprintID   int
		issueKeys  []string
		wantErr    error
		wantErrSub string
	}{
		{
			name: "success returns nil error with correct body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if !strings.Contains(r.URL.Path, "/rest/agile/1.0/sprint/42/issue") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
				issues, ok := body["issues"].([]interface{})
				if !ok || len(issues) != 2 {
					t.Errorf("issues body: got %v", body["issues"])
				}
				w.WriteHeader(http.StatusNoContent)
			},
			sprintID:  42,
			issueKeys: []string{"PROJ-1", "PROJ-2"},
		},
		{
			name: "idempotent 204 returns nil",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			sprintID:  42,
			issueKeys: []string{"PROJ-1"},
		},
		{
			name: "HTTP 400 returns descriptive error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("invalid issue key")) //nolint:errcheck
			},
			sprintID:   42,
			issueKeys:  []string{"BAD-KEY"},
			wantErrSub: "invalid issue key",
		},
		{
			name: "HTTP 401 returns ErrUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			sprintID:  42,
			issueKeys: []string{"PROJ-1"},
			wantErr:   jira.ErrUnauthorized,
		},
		{
			name: "HTTP 404 returns ErrNotFound",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			sprintID:  999,
			issueKeys: []string{"PROJ-1"},
			wantErr:   jira.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, svc := newTestServer(tc.handler)
			defer srv.Close()

			err := svc.MoveIssuesToSprint(context.Background(), tc.sprintID, tc.issueKeys)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("MoveIssuesToSprint error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// --- TestMoveIssuesToEpic ---

func TestMoveIssuesToEpic(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		epicKey    string
		issueKeys  []string
		wantErr    error
		wantErrSub string
	}{
		{
			name: "success returns nil error with correct body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if !strings.Contains(r.URL.Path, "/rest/agile/1.0/epic/PROJ-100/issue") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
				issues, ok := body["issues"].([]interface{})
				if !ok || len(issues) != 2 {
					t.Errorf("issues body: got %v", body["issues"])
				}
				w.WriteHeader(http.StatusNoContent)
			},
			epicKey:   "PROJ-100",
			issueKeys: []string{"PROJ-1", "PROJ-2"},
		},
		{
			name: "HTTP 400 returns descriptive error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("epic does not exist")) //nolint:errcheck
			},
			epicKey:    "PROJ-999",
			issueKeys:  []string{"PROJ-1"},
			wantErrSub: "epic does not exist",
		},
		{
			name: "HTTP 401 returns ErrUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			epicKey:   "PROJ-100",
			issueKeys: []string{"PROJ-1"},
			wantErr:   jira.ErrUnauthorized,
		},
		{
			name: "HTTP 404 returns ErrNotFound",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			epicKey:   "PROJ-MISSING",
			issueKeys: []string{"PROJ-1"},
			wantErr:   jira.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, svc := newTestServer(tc.handler)
			defer srv.Close()

			err := svc.MoveIssuesToEpic(context.Background(), tc.epicKey, tc.issueKeys)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("MoveIssuesToEpic error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// --- TestCreateSprint ---

func TestCreateSprint(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		req        agile.CreateSprintRequest
		wantSprint *agile.Sprint
		wantErr    error
		wantErrSub string
	}{
		{
			name: "success name+boardID only — 201 returns Sprint with id/name/state",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.URL.Path != "/rest/agile/1.0/sprint" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode body: %v", err)
				}
				if body["name"] != "Sprint 8" {
					t.Errorf("name: got %v, want Sprint 8", body["name"])
				}
				if body["originBoardId"] != float64(10) {
					t.Errorf("originBoardId: got %v, want 10", body["originBoardId"])
				}
				if _, ok := body["startDate"]; ok {
					t.Error("startDate should be absent when empty (omitempty)")
				}
				if _, ok := body["endDate"]; ok {
					t.Error("endDate should be absent when empty (omitempty)")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
					"id": 55, "name": "Sprint 8", "state": "future", "originBoardId": 10,
				})
			},
			req:        agile.CreateSprintRequest{Name: "Sprint 8", BoardID: 10},
			wantSprint: &agile.Sprint{ID: 55, Name: "Sprint 8", State: "future"},
		},
		{
			name: "success with dates — 201, request body contains startDate and endDate",
			handler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode body: %v", err)
				}
				if body["startDate"] != "2024-01-15T00:00:00.000Z" {
					t.Errorf("startDate: got %v, want 2024-01-15T00:00:00.000Z", body["startDate"])
				}
				if body["endDate"] != "2024-01-29T00:00:00.000Z" {
					t.Errorf("endDate: got %v, want 2024-01-29T00:00:00.000Z", body["endDate"])
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
					"id": 55, "name": "Sprint 8", "state": "future",
					"startDate": "2024-01-15T00:00:00.000Z", "endDate": "2024-01-29T00:00:00.000Z",
				})
			},
			req: agile.CreateSprintRequest{
				Name:      "Sprint 8",
				BoardID:   10,
				StartDate: "2024-01-15T00:00:00.000Z",
				EndDate:   "2024-01-29T00:00:00.000Z",
			},
			wantSprint: &agile.Sprint{ID: 55, Name: "Sprint 8", State: "future"},
		},
		{
			name: "HTTP 400 kanban board — error contains Jira body verbatim",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Board does not support sprints")) //nolint:errcheck
			},
			req:        agile.CreateSprintRequest{Name: "Sprint 8", BoardID: 20},
			wantErrSub: "Board does not support sprints",
		},
		{
			name: "HTTP 401 returns ErrUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			req:     agile.CreateSprintRequest{Name: "Sprint 8", BoardID: 10},
			wantErr: jira.ErrUnauthorized,
		},
		{
			name: "HTTP 404 returns ErrNotFound",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			req:     agile.CreateSprintRequest{Name: "Sprint 8", BoardID: 999},
			wantErr: jira.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, svc := newTestServer(tc.handler)
			defer srv.Close()

			sprint, err := svc.CreateSprint(context.Background(), tc.req)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("CreateSprint error: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sprint == nil {
				t.Fatal("sprint must not be nil on success")
			}
			if sprint.ID != tc.wantSprint.ID {
				t.Errorf("Sprint.ID: got %d, want %d", sprint.ID, tc.wantSprint.ID)
			}
			if sprint.Name != tc.wantSprint.Name {
				t.Errorf("Sprint.Name: got %q, want %q", sprint.Name, tc.wantSprint.Name)
			}
			if sprint.State != tc.wantSprint.State {
				t.Errorf("Sprint.State: got %q, want %q", sprint.State, tc.wantSprint.State)
			}
		})
	}
}

// TestGetSprintIssues_UnassignedHasEmptyAssignee verifies that when the assignee
// field is null in the API response, the domain SprintIssue.Assignee is "".
func TestGetSprintIssues_UnassignedHasEmptyAssignee(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"total":      1,
			"startAt":    0,
			"maxResults": 50,
			"issues": []interface{}{
				map[string]interface{}{
					"key": "PROJ-5",
					"fields": map[string]interface{}{
						"summary":  "Unassigned",
						"status":   map[string]interface{}{"name": "To Do"},
						"assignee": nil,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}
	srv, svc := newTestServer(handler)
	defer srv.Close()

	result, err := svc.GetSprintIssues(context.Background(), 100, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].Assignee != "" {
		t.Errorf("Assignee: got %q, want empty string", result.Issues[0].Assignee)
	}
}
