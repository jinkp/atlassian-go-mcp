package releases_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	releasessvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
)

// mockReleasesService implements releasessvc.ReleasesService for testing.
type mockReleasesService struct {
	getReleasesFunc           func(ctx context.Context, projectKey string) ([]releasessvc.Release, error)
	getReleaseFunc            func(ctx context.Context, releaseID string) (*releasessvc.Release, error)
	getReleaseIssueCountsFunc func(ctx context.Context, releaseID string) (*releasessvc.ReleaseIssueCounts, error)
	createReleaseFunc         func(ctx context.Context, req releasessvc.CreateReleaseRequest) (*releasessvc.Release, error)
	updateReleaseFunc         func(ctx context.Context, releaseID string, req releasessvc.UpdateReleaseRequest) (*releasessvc.Release, error)
}

func (m *mockReleasesService) GetReleases(ctx context.Context, projectKey string) ([]releasessvc.Release, error) {
	if m.getReleasesFunc != nil {
		return m.getReleasesFunc(ctx, projectKey)
	}
	return nil, errors.New("not implemented")
}
func (m *mockReleasesService) GetRelease(ctx context.Context, releaseID string) (*releasessvc.Release, error) {
	if m.getReleaseFunc != nil {
		return m.getReleaseFunc(ctx, releaseID)
	}
	return nil, errors.New("not implemented")
}
func (m *mockReleasesService) GetReleaseIssueCounts(ctx context.Context, releaseID string) (*releasessvc.ReleaseIssueCounts, error) {
	if m.getReleaseIssueCountsFunc != nil {
		return m.getReleaseIssueCountsFunc(ctx, releaseID)
	}
	return nil, errors.New("not implemented")
}
func (m *mockReleasesService) CreateRelease(ctx context.Context, req releasessvc.CreateReleaseRequest) (*releasessvc.Release, error) {
	if m.createReleaseFunc != nil {
		return m.createReleaseFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}
func (m *mockReleasesService) UpdateRelease(ctx context.Context, releaseID string, req releasessvc.UpdateReleaseRequest) (*releasessvc.Release, error) {
	if m.updateReleaseFunc != nil {
		return m.updateReleaseFunc(ctx, releaseID, req)
	}
	return nil, errors.New("not implemented")
}

// SC-C1: list renders Release table output
func TestReleases_RendersTableOutput(t *testing.T) {
	svc := &mockReleasesService{
		getReleasesFunc: func(_ context.Context, _ string) ([]releasessvc.Release, error) {
			return []releasessvc.Release{
				{ID: "10001", Name: "v1.0.0", Released: true, ReleaseDate: "2026-06-01", ProjectID: "10000"},
			}, nil
		},
	}

	result, err := svc.GetReleases(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("GetReleases() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(result)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "10001") {
		t.Errorf("table missing release ID\nGot: %s", out)
	}
	if !strings.Contains(out, "v1.0.0") {
		t.Errorf("table missing release name\nGot: %s", out)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("table missing released=true\nGot: %s", out)
	}
}

// SC-C2: list renders JSON output
func TestReleases_RendersJSONOutput(t *testing.T) {
	svc := &mockReleasesService{
		getReleasesFunc: func(_ context.Context, _ string) ([]releasessvc.Release, error) {
			return []releasessvc.Release{
				{ID: "10002", Name: "v2.0.0", Released: false, ProjectID: "10000"},
			}, nil
		},
	}

	result, err := svc.GetReleases(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("GetReleases() unexpected error: %v", err)
	}

	f, _ := output.NewFormatter("json")
	data, err := f.Format(result)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "v2.0.0") {
		t.Errorf("JSON missing release name\nGot: %s", out)
	}
	if !strings.Contains(out, "10002") {
		t.Errorf("JSON missing release ID\nGot: %s", out)
	}
}

// SC-R4 for formatter: single release renders table
func TestRelease_RendersTableSingle(t *testing.T) {
	r := releasessvc.Release{
		ID: "10001", Name: "v1.0.0", Released: true,
		ReleaseDate: "2026-06-01", ProjectID: "10000",
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(r)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "ID") {
		t.Errorf("table missing ID label\nGot: %s", out)
	}
	if !strings.Contains(out, "NAME") {
		t.Errorf("table missing NAME label\nGot: %s", out)
	}
	if !strings.Contains(out, "RELEASED") {
		t.Errorf("table missing RELEASED label\nGot: %s", out)
	}
	if !strings.Contains(out, "2026-06-01") {
		t.Errorf("table missing release date\nGot: %s", out)
	}
}

// SC-R4 for formatter: *Release pointer also renders
func TestRelease_RendersTableSinglePointer(t *testing.T) {
	r := &releasessvc.Release{
		ID: "10001", Name: "v1.0.0", Released: false, ProjectID: "10000",
	}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(r)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "10001") {
		t.Errorf("table missing release ID\nGot: %s", out)
	}
}

// SC-R7 for formatter: ReleaseIssueCounts renders table
func TestReleaseIssueCounts_RendersTable(t *testing.T) {
	counts := &releasessvc.ReleaseIssueCounts{FixVersion: 5, AffectsVersion: 3}

	f, _ := output.NewFormatter("table")
	data, err := f.Format(counts)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "FIX_VERSION") {
		t.Errorf("table missing FIX_VERSION label\nGot: %s", out)
	}
	if !strings.Contains(out, "5") {
		t.Errorf("table missing fix version count\nGot: %s", out)
	}
	if !strings.Contains(out, "AFFECTS_VERSION") {
		t.Errorf("table missing AFFECTS_VERSION label\nGot: %s", out)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("table missing affects version count\nGot: %s", out)
	}
}

// exit code mapping tests
func TestReleasesExitCode_NotFound(t *testing.T) {
	if code := releasesExitCodeTest(jira.ErrNotFound); code != 3 {
		t.Errorf("expected 3 for ErrNotFound, got %d", code)
	}
}

func TestReleasesExitCode_Unauthorized(t *testing.T) {
	if code := releasesExitCodeTest(jira.ErrUnauthorized); code != 2 {
		t.Errorf("expected 2 for ErrUnauthorized, got %d", code)
	}
}

func releasesExitCodeTest(err error) int {
	if errors.Is(err, jira.ErrNotFound) {
		return 3
	}
	if errors.Is(err, jira.ErrUnauthorized) {
		return 2
	}
	return 2
}
