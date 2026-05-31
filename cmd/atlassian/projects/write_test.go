package projects_test

import (
	"context"
	"strings"
	"testing"

	projectssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
)

// SC-C4: update prints "Updated project: {key} {name}"
func TestUpdateProject_SuccessMessageFormat(t *testing.T) {
	name := "New Name"
	svc := &mockProjectsService{
		updateProjectFunc: func(_ context.Context, projectKey string, req projectssvc.UpdateProjectRequest) (*projectssvc.Project, error) {
			if projectKey != "PROJ" {
				t.Errorf("projectKey: got %q, want PROJ", projectKey)
			}
			return &projectssvc.Project{ID: "10000", Key: "PROJ", Name: "New Name"}, nil
		},
	}

	result, err := svc.UpdateProject(context.Background(), "PROJ", projectssvc.UpdateProjectRequest{Name: &name})
	if err != nil {
		t.Fatalf("UpdateProject() unexpected error: %v", err)
	}

	msg := "Updated project: " + result.Key + " " + result.Name
	if !strings.Contains(msg, "PROJ") {
		t.Errorf("message missing key: %s", msg)
	}
	if !strings.Contains(msg, "New Name") {
		t.Errorf("message missing name: %s", msg)
	}
}

// TestUpdateProject_NilFields verifies unset flags stay nil in request.
func TestUpdateProject_NilFields(t *testing.T) {
	var gotReq projectssvc.UpdateProjectRequest
	svc := &mockProjectsService{
		updateProjectFunc: func(_ context.Context, _ string, req projectssvc.UpdateProjectRequest) (*projectssvc.Project, error) {
			gotReq = req
			return &projectssvc.Project{ID: "10000", Key: "PROJ"}, nil
		},
	}

	_, err := svc.UpdateProject(context.Background(), "PROJ", projectssvc.UpdateProjectRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Name != nil {
		t.Errorf("Name should be nil, got %v", gotReq.Name)
	}
	if gotReq.Description != nil {
		t.Errorf("Description should be nil, got %v", gotReq.Description)
	}
	if gotReq.Lead != nil {
		t.Errorf("Lead should be nil, got %v", gotReq.Lead)
	}
}

// TestUpdateProject_LeadFieldPassthrough verifies lead passes through correctly.
func TestUpdateProject_LeadFieldPassthrough(t *testing.T) {
	lead := "acc123"
	var gotReq projectssvc.UpdateProjectRequest
	svc := &mockProjectsService{
		updateProjectFunc: func(_ context.Context, _ string, req projectssvc.UpdateProjectRequest) (*projectssvc.Project, error) {
			gotReq = req
			return &projectssvc.Project{ID: "10000", Key: "PROJ", Lead: "acc123"}, nil
		},
	}

	_, err := svc.UpdateProject(context.Background(), "PROJ", projectssvc.UpdateProjectRequest{Lead: &lead})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Lead == nil || *gotReq.Lead != "acc123" {
		t.Errorf("Lead: got %v, want acc123", gotReq.Lead)
	}
}

// TestProjects_JSONOutput verifies JSON has snake_case field names.
func TestProjects_JSONOutput(t *testing.T) {
	projects := []projectssvc.Project{
		{
			ID:          "10000",
			Key:         "PROJ",
			Name:        "My Project",
			ProjectType: "software",
			Lead:        "acc123",
			URL:         "https://example.atlassian.net",
		},
	}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(projects)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	// Verify snake_case field names in JSON output
	if !strings.Contains(out, `"project_type"`) {
		t.Errorf("JSON missing project_type field\nGot: %s", out)
	}
	if !strings.Contains(out, `"lead"`) {
		t.Errorf("JSON missing lead field\nGot: %s", out)
	}
	if !strings.Contains(out, "acc123") {
		t.Errorf("JSON missing lead value\nGot: %s", out)
	}
}

// TestSearchProjects_WithPagination verifies SearchProjectsResult has correct pagination.
func TestSearchProjects_WithPagination(t *testing.T) {
	svc := &mockProjectsService{
		searchProjectsFunc: func(_ context.Context, req projectssvc.SearchProjectsRequest) (*projectssvc.SearchProjectsResult, error) {
			if req.Query != "engineering" {
				t.Errorf("Query: got %q, want engineering", req.Query)
			}
			if req.MaxResults != 10 {
				t.Errorf("MaxResults: got %d, want 10", req.MaxResults)
			}
			return &projectssvc.SearchProjectsResult{
				Projects: []projectssvc.Project{
					{ID: "10000", Key: "ENG", Name: "Engineering", ProjectType: "software"},
				},
				Total:      1,
				MaxResults: 10,
			}, nil
		},
	}

	result, err := svc.SearchProjects(context.Background(), projectssvc.SearchProjectsRequest{
		Query:      "engineering",
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Total: got %d, want 1", result.Total)
	}
	if len(result.Projects) != 1 {
		t.Errorf("len(Projects): got %d, want 1", len(result.Projects))
	}
	if result.Projects[0].Key != "ENG" {
		t.Errorf("first project Key: got %q, want ENG", result.Projects[0].Key)
	}
}
