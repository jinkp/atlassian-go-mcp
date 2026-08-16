package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
)

// mockBitbucketSvc implements bitbucket.BitbucketService for handler tests.
type mockBitbucketSvc struct {
	listReposFn func(ctx context.Context, ws string) ([]bitbucket.Repository, error)
	getPRFn     func(ctx context.Context, ws, repo string, id int) (*bitbucket.PullRequest, error)
	diffFn      func(ctx context.Context, ws, repo string, id int) (string, error)
	createPRFn  func(ctx context.Context, ws, repo string, req bitbucket.CreatePRRequest) (*bitbucket.PullRequest, error)
	mergeFn     func(ctx context.Context, ws, repo string, id int, strategy, message string) (*bitbucket.PullRequest, error)
}

func (m *mockBitbucketSvc) ListRepositories(ctx context.Context, ws string) ([]bitbucket.Repository, error) {
	if m.listReposFn != nil {
		return m.listReposFn(ctx, ws)
	}
	return []bitbucket.Repository{}, nil
}
func (m *mockBitbucketSvc) ListPullRequests(context.Context, string, string, string) ([]bitbucket.PullRequest, error) {
	return []bitbucket.PullRequest{}, nil
}
func (m *mockBitbucketSvc) GetPullRequest(ctx context.Context, ws, repo string, id int) (*bitbucket.PullRequest, error) {
	if m.getPRFn != nil {
		return m.getPRFn(ctx, ws, repo, id)
	}
	return &bitbucket.PullRequest{ID: id}, nil
}
func (m *mockBitbucketSvc) ListPRComments(context.Context, string, string, int) ([]bitbucket.PullRequestComment, error) {
	return []bitbucket.PullRequestComment{}, nil
}
func (m *mockBitbucketSvc) ListPRCommits(context.Context, string, string, int) ([]bitbucket.Commit, error) {
	return []bitbucket.Commit{}, nil
}
func (m *mockBitbucketSvc) ListPRFiles(context.Context, string, string, int) ([]bitbucket.PullRequestDiffstat, error) {
	return []bitbucket.PullRequestDiffstat{}, nil
}
func (m *mockBitbucketSvc) GetPRDiff(ctx context.Context, ws, repo string, id int) (string, error) {
	if m.diffFn != nil {
		return m.diffFn(ctx, ws, repo, id)
	}
	return "", nil
}
func (m *mockBitbucketSvc) ListPRChecks(context.Context, string, string, int) ([]bitbucket.CommitStatus, error) {
	return []bitbucket.CommitStatus{}, nil
}
func (m *mockBitbucketSvc) ListPRReviewers(context.Context, string, string, int) ([]bitbucket.Account, error) {
	return []bitbucket.Account{}, nil
}
func (m *mockBitbucketSvc) ListBranches(context.Context, string, string) ([]bitbucket.Branch, error) {
	return []bitbucket.Branch{}, nil
}
func (m *mockBitbucketSvc) StaleBranches(context.Context, string, string, int) ([]bitbucket.Branch, error) {
	return []bitbucket.Branch{}, nil
}
func (m *mockBitbucketSvc) ListPipelines(context.Context, string, string) ([]bitbucket.Pipeline, error) {
	return []bitbucket.Pipeline{}, nil
}
func (m *mockBitbucketSvc) CreatePullRequest(ctx context.Context, ws, repo string, req bitbucket.CreatePRRequest) (*bitbucket.PullRequest, error) {
	if m.createPRFn != nil {
		return m.createPRFn(ctx, ws, repo, req)
	}
	return &bitbucket.PullRequest{ID: 1, Title: req.Title}, nil
}
func (m *mockBitbucketSvc) AddPRComment(context.Context, string, string, int, string) (*bitbucket.PullRequestComment, error) {
	return &bitbucket.PullRequestComment{ID: 1}, nil
}
func (m *mockBitbucketSvc) UpdatePullRequest(context.Context, string, string, int, bitbucket.UpdatePRRequest) (*bitbucket.PullRequest, error) {
	return &bitbucket.PullRequest{}, nil
}
func (m *mockBitbucketSvc) ApprovePullRequest(context.Context, string, string, int) error { return nil }
func (m *mockBitbucketSvc) DeclinePullRequest(context.Context, string, string, int) error { return nil }
func (m *mockBitbucketSvc) MergePullRequest(ctx context.Context, ws, repo string, id int, strategy, message string) (*bitbucket.PullRequest, error) {
	if m.mergeFn != nil {
		return m.mergeFn(ctx, ws, repo, id, strategy, message)
	}
	return &bitbucket.PullRequest{ID: id, State: "MERGED"}, nil
}
func (m *mockBitbucketSvc) CreatePRTask(context.Context, string, string, int, string) (*bitbucket.PullRequestTask, error) {
	return &bitbucket.PullRequestTask{ID: 1}, nil
}
func (m *mockBitbucketSvc) ResolvePRTask(context.Context, string, string, int, int) error { return nil }
func (m *mockBitbucketSvc) RunPipeline(context.Context, string, string, string) (*bitbucket.Pipeline, error) {
	return &bitbucket.Pipeline{BuildNumber: 1}, nil
}

func TestBitbucketGetRepos_Success(t *testing.T) {
	var gotWS string
	h := NewBitbucketHandler(&mockBitbucketSvc{
		listReposFn: func(_ context.Context, ws string) ([]bitbucket.Repository, error) {
			gotWS = ws
			return []bitbucket.Repository{{Slug: "repo-a"}}, nil
		},
	}, audit.NewNoopLogger())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /bitbucket/repos", h.GetRepos)

	req := httptest.NewRequest(http.MethodGet, "/bitbucket/repos?workspace=acme", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotWS != "acme" {
		t.Errorf("expected workspace 'acme', got %q", gotWS)
	}
	if !strings.Contains(w.Body.String(), "repo-a") {
		t.Errorf("expected repo slug in body, got: %s", w.Body.String())
	}
}

func TestBitbucketGetRepos_MissingWorkspace(t *testing.T) {
	t.Setenv("BITBUCKET_WORKSPACE", "")
	h := NewBitbucketHandler(&mockBitbucketSvc{}, audit.NewNoopLogger())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /bitbucket/repos", h.GetRepos)

	req := httptest.NewRequest(http.MethodGet, "/bitbucket/repos", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when workspace missing, got %d", w.Code)
	}
}

func TestBitbucketGetPRDiff_TextPlain(t *testing.T) {
	h := NewBitbucketHandler(&mockBitbucketSvc{
		diffFn: func(context.Context, string, string, int) (string, error) {
			return "diff --git a/x b/x\n", nil
		},
	}, audit.NewNoopLogger())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /bitbucket/pullrequests/{id}/diff", h.GetPRDiff)

	req := httptest.NewRequest(http.MethodGet, "/bitbucket/pullrequests/5/diff?workspace=acme&repo=repo", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("expected text/plain content type, got %q", ct)
	}
	if !strings.HasPrefix(w.Body.String(), "diff") {
		t.Errorf("expected raw diff body, got: %s", w.Body.String())
	}
}

func TestBitbucketCreatePR_Success(t *testing.T) {
	h := NewBitbucketHandler(&mockBitbucketSvc{
		createPRFn: func(_ context.Context, _, _ string, req bitbucket.CreatePRRequest) (*bitbucket.PullRequest, error) {
			return &bitbucket.PullRequest{ID: 99, Title: req.Title}, nil
		},
	}, audit.NewNoopLogger())
	mux := http.NewServeMux()
	mux.HandleFunc("POST /bitbucket/pullrequests", h.CreatePR)

	body := `{"title":"My PR","source":"feature","destination":"main"}`
	req := httptest.NewRequest(http.MethodPost, "/bitbucket/pullrequests?workspace=acme&repo=repo", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var pr bitbucket.PullRequest
	if err := json.Unmarshal(w.Body.Bytes(), &pr); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if pr.ID != 99 {
		t.Errorf("expected PR id 99, got %d", pr.ID)
	}
}

func TestBitbucketCreatePR_MissingRequired(t *testing.T) {
	h := NewBitbucketHandler(&mockBitbucketSvc{}, audit.NewNoopLogger())
	mux := http.NewServeMux()
	mux.HandleFunc("POST /bitbucket/pullrequests", h.CreatePR)

	body := `{"title":"only title"}`
	req := httptest.NewRequest(http.MethodPost, "/bitbucket/pullrequests?workspace=acme&repo=repo", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing source/destination, got %d", w.Code)
	}
}

func TestBitbucketMergePR_EmptyBodyOK(t *testing.T) {
	h := NewBitbucketHandler(&mockBitbucketSvc{}, audit.NewNoopLogger())
	mux := http.NewServeMux()
	mux.HandleFunc("POST /bitbucket/pullrequests/{id}/merge", h.MergePR)

	req := httptest.NewRequest(http.MethodPost, "/bitbucket/pullrequests/7/merge?workspace=acme&repo=repo", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for merge with empty body, got %d: %s", w.Code, w.Body.String())
	}
}
