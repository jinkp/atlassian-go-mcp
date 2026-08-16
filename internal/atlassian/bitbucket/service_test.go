package bitbucket_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
)

func newSvc(t *testing.T, handler http.HandlerFunc) bitbucket.BitbucketService {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return bitbucket.NewService(srv.Client(), srv.URL)
}

func TestListRepositories_Pagination(t *testing.T) {
	var calls int
	svc := newSvc(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/repositories/acme" {
			// second page uses absolute "next" URL pointing back to same server
		}
		switch calls {
		case 1:
			next := fmt.Sprintf("http://%s/repositories/acme?page=2", r.Host)
			fmt.Fprintf(w, `{"values":[{"slug":"repo-a"}],"next":%q}`, next)
		default:
			fmt.Fprint(w, `{"values":[{"slug":"repo-b"}]}`)
		}
	})

	repos, err := svc.ListRepositories(context.Background(), "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 || repos[0].Slug != "repo-a" || repos[1].Slug != "repo-b" {
		t.Fatalf("expected [repo-a repo-b], got %+v", repos)
	}
	if calls != 2 {
		t.Errorf("expected 2 paginated calls, got %d", calls)
	}
}

func TestListPullRequests_StateFilter(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("state"); got != "MERGED" {
			t.Errorf("expected state=MERGED, got %q", got)
		}
		fmt.Fprint(w, `{"values":[{"id":7,"title":"Fix"}]}`)
	})

	prs, err := svc.ListPullRequests(context.Background(), "acme", "repo", "merged")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 || prs[0].ID != 7 {
		t.Fatalf("expected 1 PR id=7, got %+v", prs)
	}
}

func TestGetPullRequest_NotFound(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"message":"not found"}}`)
	})

	_, err := svc.GetPullRequest(context.Background(), "acme", "repo", 99)
	if !errors.Is(err, bitbucket.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetPullRequest_Unauthorized(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := svc.GetPullRequest(context.Background(), "acme", "repo", 1)
	if !errors.Is(err, bitbucket.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestGetPRDiff_Text(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "diff --git a/x b/x\n+added\n")
	})
	diff, err := svc.GetPRDiff(context.Background(), "acme", "repo", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff == "" || diff[:4] != "diff" {
		t.Fatalf("unexpected diff body: %q", diff)
	}
}

func TestListPRReviewers_DerivedFromPR(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"id":1,"reviewers":[{"display_name":"Alice"},{"display_name":"Bob"}]}`)
	})
	reviewers, err := svc.ListPRReviewers(context.Background(), "acme", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reviewers) != 2 || reviewers[0].DisplayName != "Alice" {
		t.Fatalf("unexpected reviewers: %+v", reviewers)
	}
}

func TestStaleBranches_FiltersByDate(t *testing.T) {
	old := time.Now().Add(-60 * 24 * time.Hour).Format(time.RFC3339)
	recent := time.Now().Add(-2 * 24 * time.Hour).Format(time.RFC3339)
	svc := newSvc(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"values":[{"name":"old","target":{"date":%q}},{"name":"fresh","target":{"date":%q}}]}`, old, recent)
	})

	stale, err := svc.StaleBranches(context.Background(), "acme", "repo", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stale) != 1 || stale[0].Name != "old" {
		t.Fatalf("expected only 'old' branch stale, got %+v", stale)
	}
}

func TestListPipelines_Empty(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"values":[]}`)
	})
	pipes, err := svc.ListPipelines(context.Background(), "acme", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pipes == nil || len(pipes) != 0 {
		t.Fatalf("expected non-nil empty slice, got %+v", pipes)
	}
}

func TestCreatePullRequest_PostsPayload(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "My PR" {
			t.Errorf("expected title 'My PR', got %v", body["title"])
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":42,"title":"My PR"}`)
	})

	pr, err := svc.CreatePullRequest(context.Background(), "acme", "repo", bitbucket.CreatePRRequest{
		Title: "My PR", SourceBranch: "feature", DestinationBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.ID != 42 {
		t.Errorf("expected id 42, got %d", pr.ID)
	}
}

func TestCreatePullRequest_ValidatesRequired(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("service must not call the API when required fields are missing")
	})
	_, err := svc.CreatePullRequest(context.Background(), "acme", "repo", bitbucket.CreatePRRequest{Title: "x"})
	if err == nil {
		t.Fatal("expected validation error for missing source/destination")
	}
}

func TestUpdatePullRequest_UsesPUT(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT (per Bitbucket docs), got %s", r.Method)
		}
		fmt.Fprint(w, `{"id":5,"title":"Updated"}`)
	})
	title := "Updated"
	pr, err := svc.UpdatePullRequest(context.Background(), "acme", "repo", 5, bitbucket.UpdatePRRequest{Title: title})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Title != "Updated" {
		t.Errorf("expected title 'Updated', got %q", pr.Title)
	}
}

func TestUpdatePullRequest_RequiresAtLeastOneField(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("service must not call the API with an empty update")
	})
	_, err := svc.UpdatePullRequest(context.Background(), "acme", "repo", 5, bitbucket.UpdatePRRequest{})
	if err == nil {
		t.Fatal("expected error when no update fields are provided")
	}
}

func TestResolvePRTask_UsesPUT(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT (per Bitbucket docs), got %s", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["state"] != "RESOLVED" {
			t.Errorf("expected state RESOLVED, got %v", body["state"])
		}
		w.WriteHeader(http.StatusOK)
	})
	if err := svc.ResolvePRTask(context.Background(), "acme", "repo", 5, 9); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeclinePullRequest_PostsDecline(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/decline") {
			t.Errorf("expected /decline endpoint, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	if err := svc.DeclinePullRequest(context.Background(), "acme", "repo", 7); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergePullRequest_PostsStrategy(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["merge_strategy"] != "squash" {
			t.Errorf("expected merge_strategy squash, got %v", body["merge_strategy"])
		}
		fmt.Fprint(w, `{"id":8,"state":"MERGED"}`)
	})
	pr, err := svc.MergePullRequest(context.Background(), "acme", "repo", 8, "squash", "done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.State != "MERGED" {
		t.Errorf("expected state MERGED, got %q", pr.State)
	}
}

func TestMergePullRequest_InvalidStrategy(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("service must not call the API with an invalid strategy")
	})
	_, err := svc.MergePullRequest(context.Background(), "acme", "repo", 8, "bogus", "")
	if err == nil {
		t.Fatal("expected error for invalid merge strategy")
	}
}

func TestRunPipeline_PostsTargetBranch(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		target, _ := body["target"].(map[string]any)
		if target["ref_name"] != "main" {
			t.Errorf("expected ref_name main, got %v", target["ref_name"])
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"uuid":"{p1}","build_number":10}`)
	})
	pipe, err := svc.RunPipeline(context.Background(), "acme", "repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pipe.BuildNumber != 10 {
		t.Errorf("expected build number 10, got %d", pipe.BuildNumber)
	}
}

func TestRunPipeline_RequiresBranch(t *testing.T) {
	svc := newSvc(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("service must not call the API without a branch")
	})
	_, err := svc.RunPipeline(context.Background(), "acme", "repo", "")
	if err == nil {
		t.Fatal("expected error when branch is empty")
	}
}
