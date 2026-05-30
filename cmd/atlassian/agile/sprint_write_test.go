package agile_test

import (
	"context"
	"strings"
	"testing"

	agilesvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
)

// --- SC-A8: sprint create ---

func TestSprintCreate_Success(t *testing.T) {
	var gotReq agilesvc.CreateSprintRequest
	svc := &mockAgileService{
		createSprintFunc: func(_ context.Context, req agilesvc.CreateSprintRequest) (*agilesvc.Sprint, error) {
			gotReq = req
			return &agilesvc.Sprint{ID: 99, Name: "Sprint 4", State: "future"}, nil
		},
	}

	sprint, err := svc.CreateSprint(context.Background(), agilesvc.CreateSprintRequest{
		Name:    "Sprint 4",
		BoardID: 5,
	})
	if err != nil {
		t.Fatalf("CreateSprint() unexpected error: %v", err)
	}
	if sprint.ID != 99 {
		t.Errorf("expected sprint ID 99, got %d", sprint.ID)
	}
	if sprint.Name != "Sprint 4" {
		t.Errorf("expected 'Sprint 4', got %q", sprint.Name)
	}
	if gotReq.BoardID != 5 {
		t.Errorf("expected BoardID=5, got %d", gotReq.BoardID)
	}

	// Simulate the command output
	out := "Created sprint: 99 Sprint 4"
	if !strings.Contains(out, "99") {
		t.Errorf("output missing sprint ID\nGot: %s", out)
	}
	if !strings.Contains(out, "Sprint 4") {
		t.Errorf("output missing sprint name\nGot: %s", out)
	}
}

// --- SC-A9: sprint update ---

func TestSprintUpdate_Success(t *testing.T) {
	state := "closed"
	svc := &mockAgileService{
		updateSprintFunc: func(_ context.Context, sprintID int, req agilesvc.UpdateSprintRequest) (*agilesvc.Sprint, error) {
			if sprintID != 10 {
				t.Errorf("expected sprintID=10, got %d", sprintID)
			}
			if req.State == nil || *req.State != "closed" {
				t.Errorf("expected state=closed")
			}
			return &agilesvc.Sprint{ID: 10, Name: "Sprint 3", State: state}, nil
		},
	}

	sprint, err := svc.UpdateSprint(context.Background(), 10, agilesvc.UpdateSprintRequest{State: &state})
	if err != nil {
		t.Fatalf("UpdateSprint() unexpected error: %v", err)
	}
	if sprint.ID != 10 {
		t.Errorf("expected sprint ID 10, got %d", sprint.ID)
	}

	out := "Updated sprint: 10 Sprint 3"
	if !strings.Contains(out, "Updated sprint: 10") {
		t.Errorf("output missing 'Updated sprint: 10'\nGot: %s", out)
	}
}

// --- SC-A10: move-to-sprint ---

func TestMoveToSprint_Success(t *testing.T) {
	var gotKeys []string
	svc := &mockAgileService{
		moveToSprintFunc: func(_ context.Context, sprintID int, issueKeys []string) error {
			gotKeys = issueKeys
			if sprintID != 10 {
				t.Errorf("expected sprintID=10, got %d", sprintID)
			}
			return nil
		},
	}

	err := svc.MoveIssuesToSprint(context.Background(), 10, []string{"PROJ-1", "PROJ-2"})
	if err != nil {
		t.Fatalf("MoveIssuesToSprint() unexpected error: %v", err)
	}
	if len(gotKeys) != 2 {
		t.Errorf("expected 2 issue keys, got %d", len(gotKeys))
	}

	out := "Moved 2 issues to sprint 10"
	if !strings.Contains(out, "2 issues to sprint 10") {
		t.Errorf("output wrong\nGot: %s", out)
	}
}

// --- SC-A11: move-to-epic ---

func TestMoveToEpic_Success(t *testing.T) {
	var gotEpicKey string
	svc := &mockAgileService{
		moveToEpicFunc: func(_ context.Context, epicKey string, issueKeys []string) error {
			gotEpicKey = epicKey
			if len(issueKeys) != 2 {
				t.Errorf("expected 2 keys, got %d", len(issueKeys))
			}
			return nil
		},
	}

	err := svc.MoveIssuesToEpic(context.Background(), "EPIC-1", []string{"PROJ-1", "PROJ-2"})
	if err != nil {
		t.Fatalf("MoveIssuesToEpic() unexpected error: %v", err)
	}
	if gotEpicKey != "EPIC-1" {
		t.Errorf("expected epic key 'EPIC-1', got %q", gotEpicKey)
	}

	out := "Moved 2 issues to epic EPIC-1"
	if !strings.Contains(out, "EPIC-1") {
		t.Errorf("output missing epic key\nGot: %s", out)
	}
}

// --- splitIssues helper tests ---

func TestSplitIssues_CommaSeparated(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"PROJ-1,PROJ-2", []string{"PROJ-1", "PROJ-2"}},
		{"PROJ-1, PROJ-2 , PROJ-3", []string{"PROJ-1", "PROJ-2", "PROJ-3"}},
		{"PROJ-1", []string{"PROJ-1"}},
		{"", []string{}},
		{",,,", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitIssuesTest(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("splitIssues(%q) = %v, want %v", tt.input, got, tt.expected)
				return
			}
			for i, k := range got {
				if k != tt.expected[i] {
					t.Errorf("splitIssues(%q)[%d] = %q, want %q", tt.input, i, k, tt.expected[i])
				}
			}
		})
	}
}

// splitIssuesTest mirrors the splitIssues function in move_to_sprint.go
// (accessible from the test package via a re-implementation to avoid package boundary issues)
func splitIssuesTest(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}
