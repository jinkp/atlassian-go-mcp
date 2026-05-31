package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// mockAgileService implements agile.AgileService for testing.
type mockAgileService struct {
	getBoardsFunc          func(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error)
	getSprintsFunc         func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error)
	getSprintIssuesFunc    func(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error)
	updateSprintFunc       func(ctx context.Context, sprintID int, req agile.UpdateSprintRequest) (*agile.Sprint, error)
	moveIssuesToSprintFunc func(ctx context.Context, sprintID int, issueKeys []string) error
	moveIssuesToEpicFunc   func(ctx context.Context, epicKey string, issueKeys []string) error
	createSprintFunc       func(ctx context.Context, req agile.CreateSprintRequest) (*agile.Sprint, error)
}

func (m *mockAgileService) GetBoards(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error) {
	if m.getBoardsFunc != nil {
		return m.getBoardsFunc(ctx, projectKey, maxResults)
	}
	return []agile.Board{}, nil
}
func (m *mockAgileService) GetSprints(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
	if m.getSprintsFunc != nil {
		return m.getSprintsFunc(ctx, boardID, state, maxResults)
	}
	return []agile.Sprint{}, nil
}
func (m *mockAgileService) GetSprintIssues(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error) {
	if m.getSprintIssuesFunc != nil {
		return m.getSprintIssuesFunc(ctx, sprintID, maxResults)
	}
	return &agile.SprintIssueResult{Issues: []agile.SprintIssue{}, Total: 0}, nil
}
func (m *mockAgileService) UpdateSprint(ctx context.Context, sprintID int, req agile.UpdateSprintRequest) (*agile.Sprint, error) {
	if m.updateSprintFunc != nil {
		return m.updateSprintFunc(ctx, sprintID, req)
	}
	return &agile.Sprint{ID: sprintID}, nil
}
func (m *mockAgileService) MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error {
	if m.moveIssuesToSprintFunc != nil {
		return m.moveIssuesToSprintFunc(ctx, sprintID, issueKeys)
	}
	return nil
}
func (m *mockAgileService) MoveIssuesToEpic(ctx context.Context, epicKey string, issueKeys []string) error {
	if m.moveIssuesToEpicFunc != nil {
		return m.moveIssuesToEpicFunc(ctx, epicKey, issueKeys)
	}
	return nil
}
func (m *mockAgileService) CreateSprint(ctx context.Context, req agile.CreateSprintRequest) (*agile.Sprint, error) {
	if m.createSprintFunc != nil {
		return m.createSprintFunc(ctx, req)
	}
	return &agile.Sprint{ID: 1, Name: req.Name}, nil
}

func TestAgileGetBoards(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		mockFn      func(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:  "success returns boards list",
			query: "project=PROJ",
			mockFn: func(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error) {
				return []agile.Board{{ID: 1, Name: "PROJ Board", Type: "scrum"}}, nil
			},
			wantStatus:  200,
			wantContain: `"total":1`,
		},
		{
			name:  "unauthorized returns 401",
			query: "project=PROJ",
			mockFn: func(ctx context.Context, projectKey string, maxResults int) ([]agile.Board, error) {
				return nil, jira.ErrUnauthorized
			},
			wantStatus:  401,
			wantContain: `"code":"UNAUTHORIZED"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewAgileHandler(&mockAgileService{getBoardsFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /agile/boards", h.GetBoards)

			req := httptest.NewRequest(http.MethodGet, "/agile/boards?"+tc.query, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

func TestAgileGetSprints(t *testing.T) {
	tests := []struct {
		name        string
		boardID     string
		mockFn      func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error)
		wantStatus  int
		wantContain string
	}{
		{
			name:    "success returns sprints",
			boardID: "1",
			mockFn: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
				return []agile.Sprint{{ID: 10, Name: "Sprint 1", State: "active"}}, nil
			},
			wantStatus:  200,
			wantContain: `"total":1`,
		},
		{
			name:    "not found returns 404",
			boardID: "999",
			mockFn: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
				return nil, jira.ErrNotFound
			},
			wantStatus:  404,
			wantContain: `"code":"NOT_FOUND"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewAgileHandler(&mockAgileService{getSprintsFunc: tc.mockFn}, audit.NewNoopLogger())

			mux := http.NewServeMux()
			mux.HandleFunc("GET /agile/boards/{boardId}/sprints", h.GetSprints)

			req := httptest.NewRequest(http.MethodGet, "/agile/boards/"+tc.boardID+"/sprints", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

func TestAgileGetActiveSprint(t *testing.T) {
	t.Run("success returns active sprint", func(t *testing.T) {
		h := NewAgileHandler(&mockAgileService{
			getSprintsFunc: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
				return []agile.Sprint{{ID: 5, Name: "Active Sprint", State: "active"}}, nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("GET /agile/boards/{boardId}/sprints/active", h.GetActiveSprint)

		req := httptest.NewRequest(http.MethodGet, "/agile/boards/1/sprints/active", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "Active Sprint") {
			t.Errorf("body %q does not contain 'Active Sprint'", w.Body.String())
		}
	})

	t.Run("no active sprint returns 404", func(t *testing.T) {
		h := NewAgileHandler(&mockAgileService{
			getSprintsFunc: func(ctx context.Context, boardID int, state string, maxResults int) ([]agile.Sprint, error) {
				return []agile.Sprint{}, nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("GET /agile/boards/{boardId}/sprints/active", h.GetActiveSprint)

		req := httptest.NewRequest(http.MethodGet, "/agile/boards/1/sprints/active", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 404 {
			t.Errorf("status: got %d, want 404", w.Code)
		}
	})
}

func TestAgileGetSprintIssues(t *testing.T) {
	t.Run("success returns sprint issues", func(t *testing.T) {
		h := NewAgileHandler(&mockAgileService{
			getSprintIssuesFunc: func(ctx context.Context, sprintID int, maxResults int) (*agile.SprintIssueResult, error) {
				return &agile.SprintIssueResult{
					Issues: []agile.SprintIssue{{Key: "PROJ-1", Summary: "Fix bug"}},
					Total:  1,
				}, nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("GET /agile/sprints/{sprintId}/issues", h.GetSprintIssues)

		req := httptest.NewRequest(http.MethodGet, "/agile/sprints/10/issues", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"total":1`) {
			t.Errorf("body %q does not contain total:1", w.Body.String())
		}
	})
}

func TestAgileCreateSprint(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]any
		mockFn      func(ctx context.Context, req agile.CreateSprintRequest) (*agile.Sprint, error)
		wantStatus  int
		wantContain string
	}{
		{
			name: "success returns 201",
			body: map[string]any{"name": "Sprint 5", "board_id": float64(1)},
			mockFn: func(ctx context.Context, req agile.CreateSprintRequest) (*agile.Sprint, error) {
				return &agile.Sprint{ID: 5, Name: "Sprint 5"}, nil
			},
			wantStatus:  201,
			wantContain: "Sprint 5",
		},
		{
			name:        "missing name returns 400",
			body:        map[string]any{"board_id": float64(1)},
			mockFn:      nil,
			wantStatus:  400,
			wantContain: `"code":"BAD_REQUEST"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewAgileHandler(&mockAgileService{createSprintFunc: tc.mockFn}, audit.NewNoopLogger())
			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/agile/sprints", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.CreateSprint(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantContain) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.wantContain)
			}
		})
	}
}

func TestAgileUpdateSprint(t *testing.T) {
	t.Run("success returns updated sprint", func(t *testing.T) {
		h := NewAgileHandler(&mockAgileService{
			updateSprintFunc: func(ctx context.Context, sprintID int, req agile.UpdateSprintRequest) (*agile.Sprint, error) {
				return &agile.Sprint{ID: sprintID, Name: "Updated"}, nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("PUT /agile/sprints/{sprintId}", h.UpdateSprint)

		name := "Updated"
		body, _ := json.Marshal(map[string]any{"name": name})
		req := httptest.NewRequest(http.MethodPut, "/agile/sprints/10", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
	})
}

func TestAgileMoveIssuesToSprint(t *testing.T) {
	t.Run("success returns ok", func(t *testing.T) {
		h := NewAgileHandler(&mockAgileService{
			moveIssuesToSprintFunc: func(ctx context.Context, sprintID int, issueKeys []string) error {
				return nil
			},
		}, audit.NewNoopLogger())

		mux := http.NewServeMux()
		mux.HandleFunc("POST /agile/sprints/{sprintId}/issues", h.MoveIssuesToSprint)

		body, _ := json.Marshal(map[string]any{"issue_keys": []string{"PROJ-1", "PROJ-2"}})
		req := httptest.NewRequest(http.MethodPost, "/agile/sprints/10/issues", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("status: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
	})
}
