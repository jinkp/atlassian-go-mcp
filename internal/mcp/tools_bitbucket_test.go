package mcpserver_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	mcpserver "github.com/jinkp/atlassian-go-mcp/internal/mcp"
)

// mockBitbucketService implements bitbucket.BitbucketService with overridable funcs.
type mockBitbucketService struct {
	listReposFn func(ctx context.Context, ws string) ([]bitbucket.Repository, error)
	listPRsFn   func(ctx context.Context, ws, repo, state string) ([]bitbucket.PullRequest, error)
	getPRFn     func(ctx context.Context, ws, repo string, id int) (*bitbucket.PullRequest, error)
	diffFn      func(ctx context.Context, ws, repo string, id int) (string, error)
	createPRFn  func(ctx context.Context, ws, repo string, req bitbucket.CreatePRRequest) (*bitbucket.PullRequest, error)
	approveFn   func(ctx context.Context, ws, repo string, id int) error
}

func (m *mockBitbucketService) ListRepositories(ctx context.Context, ws string) ([]bitbucket.Repository, error) {
	return m.listReposFn(ctx, ws)
}
func (m *mockBitbucketService) ListPullRequests(ctx context.Context, ws, repo, state string) ([]bitbucket.PullRequest, error) {
	return m.listPRsFn(ctx, ws, repo, state)
}
func (m *mockBitbucketService) GetPullRequest(ctx context.Context, ws, repo string, id int) (*bitbucket.PullRequest, error) {
	return m.getPRFn(ctx, ws, repo, id)
}
func (m *mockBitbucketService) ListPRComments(context.Context, string, string, int) ([]bitbucket.PullRequestComment, error) {
	return nil, nil
}
func (m *mockBitbucketService) ListPRCommits(context.Context, string, string, int) ([]bitbucket.Commit, error) {
	return nil, nil
}
func (m *mockBitbucketService) ListPRFiles(context.Context, string, string, int) ([]bitbucket.PullRequestDiffstat, error) {
	return nil, nil
}
func (m *mockBitbucketService) GetPRDiff(ctx context.Context, ws, repo string, id int) (string, error) {
	return m.diffFn(ctx, ws, repo, id)
}
func (m *mockBitbucketService) ListPRChecks(context.Context, string, string, int) ([]bitbucket.CommitStatus, error) {
	return nil, nil
}
func (m *mockBitbucketService) ListPRReviewers(context.Context, string, string, int) ([]bitbucket.Account, error) {
	return nil, nil
}
func (m *mockBitbucketService) ListBranches(context.Context, string, string) ([]bitbucket.Branch, error) {
	return nil, nil
}
func (m *mockBitbucketService) StaleBranches(context.Context, string, string, int) ([]bitbucket.Branch, error) {
	return nil, nil
}
func (m *mockBitbucketService) ListPipelines(context.Context, string, string) ([]bitbucket.Pipeline, error) {
	return nil, nil
}
func (m *mockBitbucketService) CreatePullRequest(ctx context.Context, ws, repo string, req bitbucket.CreatePRRequest) (*bitbucket.PullRequest, error) {
	return m.createPRFn(ctx, ws, repo, req)
}
func (m *mockBitbucketService) AddPRComment(context.Context, string, string, int, string) (*bitbucket.PullRequestComment, error) {
	return &bitbucket.PullRequestComment{}, nil
}
func (m *mockBitbucketService) UpdatePullRequest(context.Context, string, string, int, bitbucket.UpdatePRRequest) (*bitbucket.PullRequest, error) {
	return &bitbucket.PullRequest{}, nil
}
func (m *mockBitbucketService) ApprovePullRequest(ctx context.Context, ws, repo string, id int) error {
	return m.approveFn(ctx, ws, repo, id)
}
func (m *mockBitbucketService) CreatePRTask(context.Context, string, string, int, string) (*bitbucket.PullRequestTask, error) {
	return &bitbucket.PullRequestTask{}, nil
}
func (m *mockBitbucketService) ResolvePRTask(context.Context, string, string, int, int) error {
	return nil
}
func (m *mockBitbucketService) DeclinePullRequest(context.Context, string, string, int) error {
	return nil
}
func (m *mockBitbucketService) MergePullRequest(context.Context, string, string, int, string, string) (*bitbucket.PullRequest, error) {
	return &bitbucket.PullRequest{}, nil
}
func (m *mockBitbucketService) RunPipeline(context.Context, string, string, string) (*bitbucket.Pipeline, error) {
	return &bitbucket.Pipeline{}, nil
}

func TestToolBitbucketListRepos_UsesEnvWorkspace(t *testing.T) {
	t.Setenv("BITBUCKET_WORKSPACE", "acme")
	var gotWS string
	svc := &mockBitbucketService{
		listReposFn: func(_ context.Context, ws string) ([]bitbucket.Repository, error) {
			gotWS = ws
			return []bitbucket.Repository{{Slug: "r1"}}, nil
		},
	}
	handler := mcpserver.ToolBitbucketListRepos(svc)
	res, err := handler(context.Background(), makeCallToolRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", getResultText(t, res))
	}
	if gotWS != "acme" {
		t.Errorf("expected workspace 'acme' from env, got %q", gotWS)
	}
	if !strings.Contains(getResultText(t, res), "r1") {
		t.Errorf("expected repo slug in output: %s", getResultText(t, res))
	}
}

func TestToolBitbucketListRepos_ParamOverridesEnv(t *testing.T) {
	t.Setenv("BITBUCKET_WORKSPACE", "envws")
	var gotWS string
	svc := &mockBitbucketService{
		listReposFn: func(_ context.Context, ws string) ([]bitbucket.Repository, error) {
			gotWS = ws
			return []bitbucket.Repository{}, nil
		},
	}
	handler := mcpserver.ToolBitbucketListRepos(svc)
	_, _ = handler(context.Background(), makeCallToolRequest(map[string]any{"workspace": "paramws"}))
	if gotWS != "paramws" {
		t.Errorf("expected param to override env, got %q", gotWS)
	}
}

func TestToolBitbucketListRepos_MissingWorkspace(t *testing.T) {
	t.Setenv("BITBUCKET_WORKSPACE", "")
	svc := &mockBitbucketService{
		listReposFn: func(_ context.Context, _ string) ([]bitbucket.Repository, error) { return nil, nil },
	}
	handler := mcpserver.ToolBitbucketListRepos(svc)
	res, _ := handler(context.Background(), makeCallToolRequest(map[string]any{}))
	if !res.IsError {
		t.Fatal("expected error result when no workspace configured")
	}
}

func TestToolBitbucketGetPR_RequiresPositivePRID(t *testing.T) {
	t.Setenv("BITBUCKET_WORKSPACE", "acme")
	t.Setenv("BITBUCKET_REPO", "repo")
	svc := &mockBitbucketService{
		getPRFn: func(context.Context, string, string, int) (*bitbucket.PullRequest, error) { return nil, nil },
	}
	handler := mcpserver.ToolBitbucketGetPR(svc)
	res, _ := handler(context.Background(), makeCallToolRequest(map[string]any{"pr_id": float64(0)}))
	if !res.IsError {
		t.Fatal("expected error result for pr_id <= 0")
	}
}

func TestToolBitbucketGetPR_MissingRepo(t *testing.T) {
	t.Setenv("BITBUCKET_WORKSPACE", "acme")
	t.Setenv("BITBUCKET_REPO", "")
	svc := &mockBitbucketService{
		getPRFn: func(context.Context, string, string, int) (*bitbucket.PullRequest, error) { return nil, nil },
	}
	handler := mcpserver.ToolBitbucketGetPR(svc)
	res, _ := handler(context.Background(), makeCallToolRequest(map[string]any{"pr_id": float64(5)}))
	if !res.IsError {
		t.Fatal("expected error result when no repo configured")
	}
}

func TestToolBitbucketPRDiff_ServiceError(t *testing.T) {
	t.Setenv("BITBUCKET_WORKSPACE", "acme")
	t.Setenv("BITBUCKET_REPO", "repo")
	svc := &mockBitbucketService{
		diffFn: func(context.Context, string, string, int) (string, error) {
			return "", errors.New("boom")
		},
	}
	handler := mcpserver.ToolBitbucketPRDiff(svc)
	res, _ := handler(context.Background(), makeCallToolRequest(map[string]any{"pr_id": float64(2)}))
	if !res.IsError {
		t.Fatal("expected error result when service fails")
	}
}

// --- write tools ---

func TestToolBitbucketCreatePR_BlockedWithoutEnableWrite(t *testing.T) {
	t.Setenv("ENABLE_WRITE", "")
	t.Setenv("BITBUCKET_WORKSPACE", "acme")
	t.Setenv("BITBUCKET_REPO", "repo")
	called := false
	svc := &mockBitbucketService{
		createPRFn: func(context.Context, string, string, bitbucket.CreatePRRequest) (*bitbucket.PullRequest, error) {
			called = true
			return &bitbucket.PullRequest{}, nil
		},
	}
	handler := mcpserver.ToolBitbucketCreatePR(svc, audit.NewNoopLogger())
	res, _ := handler(context.Background(), makeCallToolRequest(map[string]any{
		"title": "x", "source": "feature", "destination": "main",
	}))
	if !res.IsError {
		t.Fatal("expected write to be blocked without ENABLE_WRITE=true")
	}
	if called {
		t.Error("service must NOT be called when write is blocked")
	}
}

func TestToolBitbucketCreatePR_MissingRequired(t *testing.T) {
	t.Setenv("ENABLE_WRITE", "true")
	t.Setenv("BITBUCKET_WORKSPACE", "acme")
	t.Setenv("BITBUCKET_REPO", "repo")
	svc := &mockBitbucketService{
		createPRFn: func(context.Context, string, string, bitbucket.CreatePRRequest) (*bitbucket.PullRequest, error) {
			return &bitbucket.PullRequest{}, nil
		},
	}
	handler := mcpserver.ToolBitbucketCreatePR(svc, audit.NewNoopLogger())
	res, _ := handler(context.Background(), makeCallToolRequest(map[string]any{"title": "x", "source": "feature"}))
	if !res.IsError {
		t.Fatal("expected error when destination is missing")
	}
}

func TestToolBitbucketApprovePR_Success(t *testing.T) {
	t.Setenv("ENABLE_WRITE", "true")
	t.Setenv("BITBUCKET_WORKSPACE", "acme")
	t.Setenv("BITBUCKET_REPO", "repo")
	var gotID int
	svc := &mockBitbucketService{
		approveFn: func(_ context.Context, _, _ string, id int) error {
			gotID = id
			return nil
		},
	}
	handler := mcpserver.ToolBitbucketApprovePR(svc, audit.NewNoopLogger())
	res, err := handler(context.Background(), makeCallToolRequest(map[string]any{"pr_id": float64(11)}))
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", getResultText(t, res))
	}
	if gotID != 11 {
		t.Errorf("expected pr_id 11 forwarded, got %d", gotID)
	}
	if !strings.Contains(getResultText(t, res), "PR #11 approved") {
		t.Errorf("unexpected output: %s", getResultText(t, res))
	}
}
