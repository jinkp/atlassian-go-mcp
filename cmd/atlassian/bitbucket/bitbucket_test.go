package bitbucket_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	atlclibitbucket "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/bitbucket"
	bbsvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/spf13/cobra"
)

// mockBB implements bbsvc.BitbucketService with overridable read/write funcs.
type mockBB struct {
	listReposFn func(ctx context.Context, ws string) ([]bbsvc.Repository, error)
	createPRFn  func(ctx context.Context, ws, repo string, req bbsvc.CreatePRRequest) (*bbsvc.PullRequest, error)
}

func (m *mockBB) ListRepositories(ctx context.Context, ws string) ([]bbsvc.Repository, error) {
	return m.listReposFn(ctx, ws)
}
func (m *mockBB) ListPullRequests(context.Context, string, string, string) ([]bbsvc.PullRequest, error) {
	return []bbsvc.PullRequest{}, nil
}
func (m *mockBB) GetPullRequest(context.Context, string, string, int) (*bbsvc.PullRequest, error) {
	return &bbsvc.PullRequest{}, nil
}
func (m *mockBB) ListPRComments(context.Context, string, string, int) ([]bbsvc.PullRequestComment, error) {
	return nil, nil
}
func (m *mockBB) ListPRCommits(context.Context, string, string, int) ([]bbsvc.Commit, error) {
	return nil, nil
}
func (m *mockBB) ListPRFiles(context.Context, string, string, int) ([]bbsvc.PullRequestDiffstat, error) {
	return nil, nil
}
func (m *mockBB) GetPRDiff(context.Context, string, string, int) (string, error) { return "", nil }
func (m *mockBB) ListPRChecks(context.Context, string, string, int) ([]bbsvc.CommitStatus, error) {
	return nil, nil
}
func (m *mockBB) ListPRReviewers(context.Context, string, string, int) ([]bbsvc.Account, error) {
	return nil, nil
}
func (m *mockBB) ListBranches(context.Context, string, string) ([]bbsvc.Branch, error) {
	return nil, nil
}
func (m *mockBB) StaleBranches(context.Context, string, string, int) ([]bbsvc.Branch, error) {
	return nil, nil
}
func (m *mockBB) ListPipelines(context.Context, string, string) ([]bbsvc.Pipeline, error) {
	return nil, nil
}
func (m *mockBB) CreatePullRequest(ctx context.Context, ws, repo string, req bbsvc.CreatePRRequest) (*bbsvc.PullRequest, error) {
	return m.createPRFn(ctx, ws, repo, req)
}
func (m *mockBB) AddPRComment(context.Context, string, string, int, string) (*bbsvc.PullRequestComment, error) {
	return &bbsvc.PullRequestComment{}, nil
}
func (m *mockBB) UpdatePullRequest(context.Context, string, string, int, bbsvc.UpdatePRRequest) (*bbsvc.PullRequest, error) {
	return &bbsvc.PullRequest{}, nil
}
func (m *mockBB) ApprovePullRequest(context.Context, string, string, int) error { return nil }
func (m *mockBB) CreatePRTask(context.Context, string, string, int, string) (*bbsvc.PullRequestTask, error) {
	return &bbsvc.PullRequestTask{}, nil
}
func (m *mockBB) ResolvePRTask(context.Context, string, string, int, int) error { return nil }
func (m *mockBB) DeclinePullRequest(context.Context, string, string, int) error { return nil }
func (m *mockBB) MergePullRequest(context.Context, string, string, int, string, string) (*bbsvc.PullRequest, error) {
	return &bbsvc.PullRequest{}, nil
}
func (m *mockBB) RunPipeline(context.Context, string, string, string) (*bbsvc.Pipeline, error) {
	return &bbsvc.Pipeline{}, nil
}

func buildRoot(svc bbsvc.BitbucketService, dryRun bool) *cobra.Command {
	root := atlclibitbucket.NewBitbucketCmd()
	atlclibitbucket.RegisterCommands(root, svc, audit.NewNoopLogger(), dryRun)
	return root
}

func TestReposCommand_JSONOutput(t *testing.T) {
	var gotWS string
	svc := &mockBB{
		listReposFn: func(_ context.Context, ws string) ([]bbsvc.Repository, error) {
			gotWS = ws
			return []bbsvc.Repository{{Slug: "repo-a", Name: "Repo A"}}, nil
		},
	}
	root := buildRoot(svc, false)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"repos", "--workspace", "acme", "-o", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if gotWS != "acme" {
		t.Errorf("expected workspace 'acme' from --workspace flag, got %q", gotWS)
	}
	if !strings.Contains(buf.String(), "repo-a") {
		t.Errorf("expected repo slug in JSON output, got:\n%s", buf.String())
	}
}

func TestReposCommand_UsesEnvWorkspace(t *testing.T) {
	t.Setenv("BITBUCKET_WORKSPACE", "envws")
	var gotWS string
	svc := &mockBB{
		listReposFn: func(_ context.Context, ws string) ([]bbsvc.Repository, error) {
			gotWS = ws
			return []bbsvc.Repository{}, nil
		},
	}
	root := buildRoot(svc, false)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"repos", "-o", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if gotWS != "envws" {
		t.Errorf("expected workspace from BITBUCKET_WORKSPACE env, got %q", gotWS)
	}
}

func TestPRCreate_DryRunDoesNotCallService(t *testing.T) {
	t.Setenv("BITBUCKET_WORKSPACE", "acme")
	t.Setenv("BITBUCKET_REPO", "repo")
	called := false
	svc := &mockBB{
		createPRFn: func(context.Context, string, string, bbsvc.CreatePRRequest) (*bbsvc.PullRequest, error) {
			called = true
			return &bbsvc.PullRequest{}, nil
		},
	}
	root := buildRoot(svc, true) // dryRun
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"pr", "create", "--title", "T", "--source", "feat", "--destination", "main"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if called {
		t.Error("service must NOT be called in --dry-run mode")
	}
	if !strings.Contains(buf.String(), "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] output, got:\n%s", buf.String())
	}
}

func TestPRMerge_DryRun(t *testing.T) {
	t.Setenv("BITBUCKET_WORKSPACE", "acme")
	t.Setenv("BITBUCKET_REPO", "repo")
	svc := &mockBB{}
	root := buildRoot(svc, true) // dryRun
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"pr", "merge", "42", "--strategy", "squash"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if !strings.Contains(buf.String(), "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] output, got:\n%s", buf.String())
	}
}

func TestPipelineRun_DryRun(t *testing.T) {
	t.Setenv("BITBUCKET_WORKSPACE", "acme")
	t.Setenv("BITBUCKET_REPO", "repo")
	svc := &mockBB{}
	root := buildRoot(svc, true) // dryRun
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"pipeline", "run", "--branch", "main"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if !strings.Contains(buf.String(), "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] output, got:\n%s", buf.String())
	}
}
