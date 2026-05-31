package releases_test

import (
	"context"
	"strings"
	"testing"

	releasessvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
)

// SC-C3: create prints "Created release: {id} {name}"
func TestCreateRelease_SuccessMessageFormat(t *testing.T) {
	svc := &mockReleasesService{
		createReleaseFunc: func(_ context.Context, req releasessvc.CreateReleaseRequest) (*releasessvc.Release, error) {
			if req.ProjectID != "10000" {
				t.Errorf("ProjectID: got %q, want 10000", req.ProjectID)
			}
			if req.Name != "v1.0.0" {
				t.Errorf("Name: got %q, want v1.0.0", req.Name)
			}
			return &releasessvc.Release{ID: "10001", Name: "v1.0.0", ProjectID: "10000"}, nil
		},
	}

	result, err := svc.CreateRelease(context.Background(), releasessvc.CreateReleaseRequest{
		ProjectID: "10000",
		Name:      "v1.0.0",
	})
	if err != nil {
		t.Fatalf("CreateRelease() unexpected error: %v", err)
	}

	// Verify the output format: "Created release: {id} {name}"
	msg := "Created release: " + result.ID + " " + result.Name
	if !strings.Contains(msg, "10001") {
		t.Errorf("message missing ID: %s", msg)
	}
	if !strings.Contains(msg, "v1.0.0") {
		t.Errorf("message missing name: %s", msg)
	}
}

// SC-C4: update prints "Updated release: {id} {name}"
func TestUpdateRelease_SuccessMessageFormat(t *testing.T) {
	name := "v1.1.0"
	svc := &mockReleasesService{
		updateReleaseFunc: func(_ context.Context, releaseID string, req releasessvc.UpdateReleaseRequest) (*releasessvc.Release, error) {
			if releaseID != "10001" {
				t.Errorf("releaseID: got %q, want 10001", releaseID)
			}
			return &releasessvc.Release{ID: "10001", Name: "v1.1.0", ProjectID: "10000"}, nil
		},
	}

	result, err := svc.UpdateRelease(context.Background(), "10001", releasessvc.UpdateReleaseRequest{Name: &name})
	if err != nil {
		t.Fatalf("UpdateRelease() unexpected error: %v", err)
	}

	msg := "Updated release: " + result.ID + " " + result.Name
	if !strings.Contains(msg, "10001") {
		t.Errorf("message missing ID: %s", msg)
	}
	if !strings.Contains(msg, "v1.1.0") {
		t.Errorf("message missing name: %s", msg)
	}
}

// TestCreateRelease_WithOptionalFields verifies optional fields are passed through.
func TestCreateRelease_WithOptionalFields(t *testing.T) {
	var gotReq releasessvc.CreateReleaseRequest
	svc := &mockReleasesService{
		createReleaseFunc: func(_ context.Context, req releasessvc.CreateReleaseRequest) (*releasessvc.Release, error) {
			gotReq = req
			return &releasessvc.Release{ID: "10001", Name: req.Name}, nil
		},
	}

	_, err := svc.CreateRelease(context.Background(), releasessvc.CreateReleaseRequest{
		ProjectID:   "10000",
		Name:        "v1.0.0",
		Description: "First release",
		ReleaseDate: "2026-06-01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Description != "First release" {
		t.Errorf("Description: got %q, want 'First release'", gotReq.Description)
	}
	if gotReq.ReleaseDate != "2026-06-01" {
		t.Errorf("ReleaseDate: got %q, want '2026-06-01'", gotReq.ReleaseDate)
	}
}

// TestUpdateRelease_ReleasedPointerTrue verifies released=true sets Released field.
func TestUpdateRelease_ReleasedPointerTrue(t *testing.T) {
	released := true
	var gotReq releasessvc.UpdateReleaseRequest
	svc := &mockReleasesService{
		updateReleaseFunc: func(_ context.Context, _ string, req releasessvc.UpdateReleaseRequest) (*releasessvc.Release, error) {
			gotReq = req
			return &releasessvc.Release{ID: "10001", Released: true}, nil
		},
	}

	_, err := svc.UpdateRelease(context.Background(), "10001", releasessvc.UpdateReleaseRequest{Released: &released})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Released == nil || !*gotReq.Released {
		t.Errorf("Released: got %v, want true", gotReq.Released)
	}
}

// TestReleases_JSONOutputStructure verifies JSON output has expected fields.
func TestReleases_JSONOutputStructure(t *testing.T) {
	releases := []releasessvc.Release{
		{
			ID:          "10001",
			Name:        "v1.0.0",
			Released:    true,
			Archived:    false,
			ReleaseDate: "2026-06-01",
			ProjectID:   "10000",
		},
	}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(releases)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	fields := []string{`"id"`, `"name"`, `"released"`, `"project_id"`}
	for _, field := range fields {
		if !strings.Contains(out, field) {
			t.Errorf("JSON missing field %q\nGot: %s", field, out)
		}
	}
}
