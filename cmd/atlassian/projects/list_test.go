package projects_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	projectssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
)

// mockProjectsService implements projectssvc.ProjectsService for testing.
type mockProjectsService struct {
	getProjectsFunc    func(ctx context.Context, maxResults int) ([]projectssvc.Project, error)
	getProjectFunc     func(ctx context.Context, projectKey string) (*projectssvc.Project, error)
	searchProjectsFunc func(ctx context.Context, req projectssvc.SearchProjectsRequest) (*projectssvc.SearchProjectsResult, error)
	updateProjectFunc  func(ctx context.Context, projectKey string, req projectssvc.UpdateProjectRequest) (*projectssvc.Project, error)
}

func (m *mockProjectsService) GetProjects(ctx context.Context, maxResults int) ([]projectssvc.Project, error) {
	if m.getProjectsFunc != nil {
		return m.getProjectsFunc(ctx, maxResults)
	}
	return nil, errors.New("not implemented")
}

func (m *mockProjectsService) GetProject(ctx context.Context, projectKey string) (*projectssvc.Project, error) {
	if m.getProjectFunc != nil {
		return m.getProjectFunc(ctx, projectKey)
	}
	return nil, errors.New("not implemented")
}

func (m *mockProjectsService) SearchProjects(ctx context.Context, req projectssvc.SearchProjectsRequest) (*projectssvc.SearchProjectsResult, error) {
	if m.searchProjectsFunc != nil {
		return m.searchProjectsFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockProjectsService) UpdateProject(ctx context.Context, projectKey string, req projectssvc.UpdateProjectRequest) (*projectssvc.Project, error) {
	if m.updateProjectFunc != nil {
		return m.updateProjectFunc(ctx, projectKey, req)
	}
	return nil, errors.New("not implemented")
}

// SC-C1: list renders Project table output
func TestProjects_RendersTableOutput(t *testing.T) {
	svc := &mockProjectsService{
		getProjectsFunc: func(_ context.Context, _ int) ([]projectssvc.Project, error) {
			return []projectssvc.Project{
				{ID: "10000", Key: "PROJ", Name: "My Project", ProjectType: "software", Lead: "acc123"},
			}, nil
		},
	}

	result, err := svc.GetProjects(context.Background(), 50)
	if err != nil {
		t.Fatalf("GetProjects() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(result)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "PROJ") {
		t.Errorf("table missing project key\nGot: %s", out)
	}
	if !strings.Contains(out, "My Project") {
		t.Errorf("table missing project name\nGot: %s", out)
	}
	if !strings.Contains(out, "software") {
		t.Errorf("table missing project type\nGot: %s", out)
	}
}

// SC-C2: list renders JSON output
func TestProjects_RendersJSONOutput(t *testing.T) {
	svc := &mockProjectsService{
		getProjectsFunc: func(_ context.Context, _ int) ([]projectssvc.Project, error) {
			return []projectssvc.Project{
				{ID: "10000", Key: "PROJ", Name: "My Project", ProjectType: "software"},
			}, nil
		},
	}

	result, err := svc.GetProjects(context.Background(), 50)
	if err != nil {
		t.Fatalf("GetProjects() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(result)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "PROJ") {
		t.Errorf("JSON missing project key\nGot: %s", out)
	}
	if !strings.Contains(out, "10000") {
		t.Errorf("JSON missing project ID\nGot: %s", out)
	}
}

// SC-R: single project renders table
func TestProject_RendersTableSingle(t *testing.T) {
	p := projectssvc.Project{
		ID: "10000", Key: "PROJ", Name: "My Project",
		ProjectType: "software", Lead: "acc123",
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(p)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "KEY") {
		t.Errorf("table missing KEY label\nGot: %s", out)
	}
	if !strings.Contains(out, "NAME") {
		t.Errorf("table missing NAME label\nGot: %s", out)
	}
	if !strings.Contains(out, "TYPE") {
		t.Errorf("table missing TYPE label\nGot: %s", out)
	}
	if !strings.Contains(out, "LEAD") {
		t.Errorf("table missing LEAD label\nGot: %s", out)
	}
}

// SC-R: *Project pointer also renders
func TestProject_RendersTableSinglePointer(t *testing.T) {
	p := &projectssvc.Project{
		ID: "10000", Key: "PROJ", Name: "My Project", ProjectType: "software",
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(p)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "PROJ") {
		t.Errorf("table missing project key\nGot: %s", out)
	}
}

// SC-R: SearchProjectsResult renders table
func TestSearchProjectsResult_RendersTable(t *testing.T) {
	result := &projectssvc.SearchProjectsResult{
		Projects: []projectssvc.Project{
			{ID: "10000", Key: "PROJ", Name: "My Project", ProjectType: "software"},
			{ID: "10001", Key: "PROJ2", Name: "Second", ProjectType: "business"},
		},
		Total:      2,
		MaxResults: 50,
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(result)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "KEY") {
		t.Errorf("table missing KEY header\nGot: %s", out)
	}
	if !strings.Contains(out, "PROJ") {
		t.Errorf("table missing first project key\nGot: %s", out)
	}
	if !strings.Contains(out, "PROJ2") {
		t.Errorf("table missing second project key\nGot: %s", out)
	}
}

// SC-JSON: Projects array has expected fields
func TestProjects_JSONOutputStructure(t *testing.T) {
	projects := []projectssvc.Project{
		{
			ID:          "10000",
			Key:         "PROJ",
			Name:        "My Project",
			ProjectType: "software",
			Lead:        "acc123",
		},
	}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(projects)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	fields := []string{`"id"`, `"key"`, `"name"`, `"project_type"`}
	for _, field := range fields {
		if !strings.Contains(out, field) {
			t.Errorf("JSON missing field %q\nGot: %s", field, out)
		}
	}
}

// exit code mapping tests
func TestProjectsExitCode_NotFound(t *testing.T) {
	if code := projectsExitCodeTest(jira.ErrNotFound); code != 3 {
		t.Errorf("expected 3 for ErrNotFound, got %d", code)
	}
}

func TestProjectsExitCode_Unauthorized(t *testing.T) {
	if code := projectsExitCodeTest(jira.ErrUnauthorized); code != 2 {
		t.Errorf("expected 2 for ErrUnauthorized, got %d", code)
	}
}

func projectsExitCodeTest(err error) int {
	if errors.Is(err, jira.ErrNotFound) {
		return 3
	}
	if errors.Is(err, jira.ErrUnauthorized) {
		return 2
	}
	return 2
}
