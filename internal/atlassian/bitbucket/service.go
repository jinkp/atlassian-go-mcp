package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
)

// CloudBaseURL is the Bitbucket Cloud REST API v2.0 base URL.
const CloudBaseURL = "https://api.bitbucket.org/2.0"

const staleDefaultDays = 30

// Sentinel errors, aligned with the other atlassian service packages.
var (
	// ErrUnauthorized is returned on 401/403 — bad credentials or insufficient permissions.
	ErrUnauthorized = errors.New("unauthorized: check BITBUCKET_USERNAME and BITBUCKET_API_TOKEN")
	// ErrNotFound is returned when a requested resource does not exist (HTTP 404).
	ErrNotFound = errors.New("bitbucket: resource not found")
	// ErrRateLimit is returned on 429 after all retries are exhausted.
	ErrRateLimit = errors.New("rate limited: too many requests")
)

// BitbucketService defines read operations against the Bitbucket Cloud REST API v2.0.
type BitbucketService interface {
	ListRepositories(ctx context.Context, workspace string) ([]Repository, error)
	ListPullRequests(ctx context.Context, workspace, repo, state string) ([]PullRequest, error)
	GetPullRequest(ctx context.Context, workspace, repo string, id int) (*PullRequest, error)
	ListPRComments(ctx context.Context, workspace, repo string, id int) ([]PullRequestComment, error)
	ListPRCommits(ctx context.Context, workspace, repo string, id int) ([]Commit, error)
	ListPRFiles(ctx context.Context, workspace, repo string, id int) ([]PullRequestDiffstat, error)
	GetPRDiff(ctx context.Context, workspace, repo string, id int) (string, error)
	ListPRChecks(ctx context.Context, workspace, repo string, id int) ([]CommitStatus, error)
	ListPRReviewers(ctx context.Context, workspace, repo string, id int) ([]Account, error)
	ListBranches(ctx context.Context, workspace, repo string) ([]Branch, error)
	StaleBranches(ctx context.Context, workspace, repo string, days int) ([]Branch, error)
	ListPipelines(ctx context.Context, workspace, repo string) ([]Pipeline, error)

	// write operations
	CreatePullRequest(ctx context.Context, workspace, repo string, req CreatePRRequest) (*PullRequest, error)
	AddPRComment(ctx context.Context, workspace, repo string, id int, message string) (*PullRequestComment, error)
	UpdatePullRequest(ctx context.Context, workspace, repo string, id int, req UpdatePRRequest) (*PullRequest, error)
	ApprovePullRequest(ctx context.Context, workspace, repo string, id int) error
	DeclinePullRequest(ctx context.Context, workspace, repo string, id int) error
	MergePullRequest(ctx context.Context, workspace, repo string, id int, strategy, message string) (*PullRequest, error)
	CreatePRTask(ctx context.Context, workspace, repo string, id int, message string) (*PullRequestTask, error)
	ResolvePRTask(ctx context.Context, workspace, repo string, prID, taskID int) error
	RunPipeline(ctx context.Context, workspace, repo, branch string) (*Pipeline, error)
}

// BitbucketCloudService implements BitbucketService against api.bitbucket.org.
type BitbucketCloudService struct {
	doer    client.HTTPDoer
	baseURL string
}

// NewService constructs a BitbucketCloudService. The doer is typically a
// *client.Client configured with the Bitbucket base URL and username:apiToken
// auth, or an *http.Client from httptest in tests.
func NewService(doer client.HTTPDoer, baseURL string) BitbucketService {
	if baseURL == "" {
		baseURL = CloudBaseURL
	}
	return &BitbucketCloudService{doer: doer, baseURL: strings.TrimRight(baseURL, "/")}
}

// resolveURL returns an absolute URL. Bitbucket pagination "next" links are already
// absolute; relative paths are joined to the configured base URL.
func (s *BitbucketCloudService) resolveURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return s.baseURL + path
}

// getJSON performs a GET and decodes the JSON body into out.
func (s *BitbucketCloudService) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.resolveURL(path), nil)
	if err != nil {
		return fmt.Errorf("bitbucket: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.doer.Do(req)
	if err != nil {
		return fmt.Errorf("bitbucket: request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := statusError(resp); err != nil {
		return err
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("bitbucket: decoding response: %w", err)
	}
	return nil
}

// getText performs a GET and returns the raw text body (used for PR diffs).
func (s *BitbucketCloudService) getText(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.resolveURL(path), nil)
	if err != nil {
		return "", fmt.Errorf("bitbucket: building request: %w", err)
	}
	req.Header.Set("Accept", "text/plain")

	resp, err := s.doer.Do(req)
	if err != nil {
		return "", fmt.Errorf("bitbucket: request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := statusError(resp); err != nil {
		return "", err
	}
	var sb strings.Builder
	if _, err := io.Copy(&sb, resp.Body); err != nil {
		return "", fmt.Errorf("bitbucket: reading response: %w", err)
	}
	return sb.String(), nil
}

// statusError maps a non-2xx response to a sentinel or descriptive error.
// On success (2xx) it returns nil without consuming the body.
func statusError(resp *http.Response) error {
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimit
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bitbucket: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

// paginate follows Bitbucket "next" links and accumulates all values.
func paginate[T any](ctx context.Context, s *BitbucketCloudService, path string) ([]T, error) {
	var values []T
	current := path
	for current != "" {
		var page paginatedResponse[T]
		if err := s.getJSON(ctx, current, &page); err != nil {
			return nil, err
		}
		values = append(values, page.Values...)
		current = page.Next
	}
	if values == nil {
		values = []T{}
	}
	return values, nil
}

// --- read operations ---

// ListRepositories lists repositories in a workspace.
func (s *BitbucketCloudService) ListRepositories(ctx context.Context, workspace string) ([]Repository, error) {
	return paginate[Repository](ctx, s, fmt.Sprintf("/repositories/%s", workspace))
}

// ListPullRequests lists pull requests for a repository, optionally filtered by state.
// state may be "" (defaults to OPEN on the Bitbucket side) or OPEN/MERGED/DECLINED/SUPERSEDED.
func (s *BitbucketCloudService) ListPullRequests(ctx context.Context, workspace, repo, state string) ([]PullRequest, error) {
	path := fmt.Sprintf("/repositories/%s/%s/pullrequests", workspace, repo)
	if st := strings.TrimSpace(state); st != "" {
		q := url.Values{}
		q.Add("state", strings.ToUpper(st))
		path += "?" + q.Encode()
	}
	return paginate[PullRequest](ctx, s, path)
}

// GetPullRequest fetches a single pull request by ID.
func (s *BitbucketCloudService) GetPullRequest(ctx context.Context, workspace, repo string, id int) (*PullRequest, error) {
	var pr PullRequest
	if err := s.getJSON(ctx, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d", workspace, repo, id), &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// ListPRComments lists comments on a pull request.
func (s *BitbucketCloudService) ListPRComments(ctx context.Context, workspace, repo string, id int) ([]PullRequestComment, error) {
	return paginate[PullRequestComment](ctx, s, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/comments", workspace, repo, id))
}

// ListPRCommits lists commits in a pull request.
func (s *BitbucketCloudService) ListPRCommits(ctx context.Context, workspace, repo string, id int) ([]Commit, error) {
	return paginate[Commit](ctx, s, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/commits", workspace, repo, id))
}

// ListPRFiles lists changed files (diffstat) in a pull request.
func (s *BitbucketCloudService) ListPRFiles(ctx context.Context, workspace, repo string, id int) ([]PullRequestDiffstat, error) {
	return paginate[PullRequestDiffstat](ctx, s, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/diffstat", workspace, repo, id))
}

// GetPRDiff returns the raw unified diff for a pull request.
func (s *BitbucketCloudService) GetPRDiff(ctx context.Context, workspace, repo string, id int) (string, error) {
	return s.getText(ctx, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/diff", workspace, repo, id))
}

// ListPRChecks lists build/pipeline status checks for a pull request.
func (s *BitbucketCloudService) ListPRChecks(ctx context.Context, workspace, repo string, id int) ([]CommitStatus, error) {
	return paginate[CommitStatus](ctx, s, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/statuses", workspace, repo, id))
}

// ListPRReviewers returns the reviewers of a pull request (derived from the PR object).
func (s *BitbucketCloudService) ListPRReviewers(ctx context.Context, workspace, repo string, id int) ([]Account, error) {
	pr, err := s.GetPullRequest(ctx, workspace, repo, id)
	if err != nil {
		return nil, err
	}
	if pr.Reviewers == nil {
		return []Account{}, nil
	}
	return pr.Reviewers, nil
}

// ListBranches lists branches in a repository.
func (s *BitbucketCloudService) ListBranches(ctx context.Context, workspace, repo string) ([]Branch, error) {
	return paginate[Branch](ctx, s, fmt.Sprintf("/repositories/%s/%s/refs/branches", workspace, repo))
}

// StaleBranches returns branches whose tip commit is older than `days` days.
// days <= 0 defaults to 30. Branches with unparseable dates are skipped.
func (s *BitbucketCloudService) StaleBranches(ctx context.Context, workspace, repo string, days int) ([]Branch, error) {
	if days <= 0 {
		days = staleDefaultDays
	}
	branches, err := s.ListBranches(ctx, workspace, repo)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	stale := make([]Branch, 0, len(branches))
	for _, b := range branches {
		t, err := time.Parse(time.RFC3339, b.Target.Date)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			stale = append(stale, b)
		}
	}
	return stale, nil
}

// ListPipelines lists the 20 most recent pipelines for a repository (newest first).
func (s *BitbucketCloudService) ListPipelines(ctx context.Context, workspace, repo string) ([]Pipeline, error) {
	q := url.Values{}
	q.Set("sort", "-created_on")
	q.Set("pagelen", "20")
	return paginate[Pipeline](ctx, s, fmt.Sprintf("/repositories/%s/%s/pipelines/?%s", workspace, repo, q.Encode()))
}

// --- write operations ---

// reviewerRef is the wire shape for a reviewer reference (UUID or nickname).
type reviewerRef struct {
	UUID     string `json:"uuid,omitempty"`
	Nickname string `json:"nickname,omitempty"`
}

// resolveReviewerRefs converts reviewer strings into wire refs. A value wrapped in
// braces ("{uuid}") is treated as a UUID; anything else is treated as a nickname.
func resolveReviewerRefs(reviewers []string) []reviewerRef {
	refs := make([]reviewerRef, 0, len(reviewers))
	for _, r := range reviewers {
		trimmed := strings.TrimSpace(r)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
			refs = append(refs, reviewerRef{UUID: trimmed})
		} else {
			refs = append(refs, reviewerRef{Nickname: trimmed})
		}
	}
	return refs
}

// sendJSON performs a POST/PATCH with an optional JSON body and optionally decodes
// the response into out. A 204/empty body is treated as success with no decode.
func (s *BitbucketCloudService) sendJSON(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("bitbucket: marshaling request: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.resolveURL(path), reader)
	if err != nil {
		return fmt.Errorf("bitbucket: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.doer.Do(req)
	if err != nil {
		return fmt.Errorf("bitbucket: request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := statusError(resp); err != nil {
		return err
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("bitbucket: decoding response: %w", err)
	}
	return nil
}

// CreatePullRequest creates a pull request. Title, SourceBranch and DestinationBranch are required.
func (s *BitbucketCloudService) CreatePullRequest(ctx context.Context, workspace, repo string, req CreatePRRequest) (*PullRequest, error) {
	title := strings.TrimSpace(req.Title)
	source := strings.TrimSpace(req.SourceBranch)
	destination := strings.TrimSpace(req.DestinationBranch)
	if title == "" {
		return nil, fmt.Errorf("bitbucket: pull request title is required")
	}
	if source == "" {
		return nil, fmt.Errorf("bitbucket: source branch is required")
	}
	if destination == "" {
		return nil, fmt.Errorf("bitbucket: destination branch is required")
	}

	payload := map[string]any{
		"title":       title,
		"source":      map[string]any{"branch": map[string]string{"name": source}},
		"destination": map[string]any{"branch": map[string]string{"name": destination}},
	}
	if desc := strings.TrimSpace(req.Description); desc != "" {
		payload["description"] = desc
	}
	if refs := resolveReviewerRefs(req.Reviewers); len(refs) > 0 {
		payload["reviewers"] = refs
	}
	if req.CloseSourceBranch {
		payload["close_source_branch"] = true
	}

	var pr PullRequest
	if err := s.sendJSON(ctx, http.MethodPost, fmt.Sprintf("/repositories/%s/%s/pullrequests", workspace, repo), payload, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// AddPRComment posts a comment on a pull request and returns the created comment.
func (s *BitbucketCloudService) AddPRComment(ctx context.Context, workspace, repo string, id int, message string) (*PullRequestComment, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, fmt.Errorf("bitbucket: comment message cannot be empty")
	}
	payload := map[string]any{"content": map[string]string{"raw": message}}
	var comment PullRequestComment
	if err := s.sendJSON(ctx, http.MethodPost, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/comments", workspace, repo, id), payload, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// UpdatePullRequest applies a partial update to a pull request. At least one field must be set.
func (s *BitbucketCloudService) UpdatePullRequest(ctx context.Context, workspace, repo string, id int, req UpdatePRRequest) (*PullRequest, error) {
	payload := map[string]any{}
	if title := strings.TrimSpace(req.Title); title != "" {
		payload["title"] = title
	}
	if desc := strings.TrimSpace(req.Description); desc != "" {
		payload["description"] = desc
	}
	if dest := strings.TrimSpace(req.DestinationBranch); dest != "" {
		payload["destination"] = map[string]any{"branch": map[string]string{"name": dest}}
	}
	if req.Reviewers != nil {
		payload["reviewers"] = resolveReviewerRefs(req.Reviewers)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("bitbucket: at least one field (title, description, destination, reviewers) must be provided")
	}

	var pr PullRequest
	if err := s.sendJSON(ctx, http.MethodPut, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d", workspace, repo, id), payload, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// ApprovePullRequest approves a pull request. Idempotent: re-approving is safe.
func (s *BitbucketCloudService) ApprovePullRequest(ctx context.Context, workspace, repo string, id int) error {
	return s.sendJSON(ctx, http.MethodPost, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/approve", workspace, repo, id), map[string]any{}, nil)
}

// CreatePRTask creates a task on a pull request and returns the created task.
func (s *BitbucketCloudService) CreatePRTask(ctx context.Context, workspace, repo string, id int, message string) (*PullRequestTask, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, fmt.Errorf("bitbucket: task message cannot be empty")
	}
	payload := map[string]any{"content": map[string]string{"raw": message}}
	var task PullRequestTask
	if err := s.sendJSON(ctx, http.MethodPost, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/tasks", workspace, repo, id), payload, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// ResolvePRTask marks a pull request task as resolved.
func (s *BitbucketCloudService) ResolvePRTask(ctx context.Context, workspace, repo string, prID, taskID int) error {
	payload := map[string]string{"state": "RESOLVED"}
	return s.sendJSON(ctx, http.MethodPut, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/tasks/%d", workspace, repo, prID, taskID), payload, nil)
}

// DeclinePullRequest declines (rejects) a pull request.
func (s *BitbucketCloudService) DeclinePullRequest(ctx context.Context, workspace, repo string, id int) error {
	return s.sendJSON(ctx, http.MethodPost, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/decline", workspace, repo, id), map[string]any{}, nil)
}

// MergePullRequest merges a pull request. strategy is optional (empty uses the repo default);
// message is an optional custom merge commit message.
func (s *BitbucketCloudService) MergePullRequest(ctx context.Context, workspace, repo string, id int, strategy, message string) (*PullRequest, error) {
	normalized, err := normalizeMergeStrategy(strategy)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"type": "pullrequest_merge_parameters"}
	if normalized != "" {
		payload["merge_strategy"] = normalized
	}
	if m := strings.TrimSpace(message); m != "" {
		payload["message"] = m
	}
	var pr PullRequest
	if err := s.sendJSON(ctx, http.MethodPost, fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/merge", workspace, repo, id), payload, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// RunPipeline triggers a pipeline for the given branch and returns the created pipeline.
func (s *BitbucketCloudService) RunPipeline(ctx context.Context, workspace, repo, branch string) (*Pipeline, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil, fmt.Errorf("bitbucket: branch is required to run a pipeline")
	}
	payload := map[string]any{
		"target": map[string]any{
			"ref_type": "branch",
			"type":     "pipeline_ref_target",
			"ref_name": branch,
		},
	}
	var pipeline Pipeline
	if err := s.sendJSON(ctx, http.MethodPost, fmt.Sprintf("/repositories/%s/%s/pipelines/", workspace, repo), payload, &pipeline); err != nil {
		return nil, err
	}
	return &pipeline, nil
}

// normalizeMergeStrategy validates and normalizes a merge strategy name.
// Empty input returns "" (use repo default).
func normalizeMergeStrategy(strategy string) (string, error) {
	strategy = strings.TrimSpace(strategy)
	switch strategy {
	case "":
		return "", nil
	case "merge", "merge_commit":
		return "merge_commit", nil
	case "squash":
		return "squash", nil
	case "fast_forward":
		return "fast_forward", nil
	case "squash_fast_forward":
		return "squash_fast_forward", nil
	case "rebase_fast_forward":
		return "rebase_fast_forward", nil
	case "rebase_merge":
		return "rebase_merge", nil
	default:
		return "", fmt.Errorf("bitbucket: invalid merge strategy %q (supported: merge_commit, squash, fast_forward, squash_fast_forward, rebase_fast_forward, rebase_merge)", strategy)
	}
}
