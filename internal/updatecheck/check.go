// Package updatecheck performs a read-only check against the GitHub Releases
// API to tell the user whether a newer version is available. It never downloads
// or modifies anything.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// DefaultAPIBase is the GitHub REST API base URL.
const DefaultAPIBase = "https://api.github.com"

// Doer is the minimal HTTP client contract (satisfied by *http.Client).
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// FetchLatestTag queries the GitHub "latest release" endpoint for repo (e.g.
// "jinkp/atlassian-go-mcp") and returns its tag_name (e.g. "v1.2.2").
func FetchLatestTag(ctx context.Context, doer Doer, apiBase, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimRight(apiBase, "/"), repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "atlassian-mcp-updatecheck")

	resp, err := doer.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github API returned HTTP %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decoding release response: %w", err)
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("release response had no tag_name")
	}
	return payload.TagName, nil
}

// CompareSemver compares two "vX.Y.Z" version strings. It returns:
//
//	-1 if current < latest  (an update is available)
//	 0 if current == latest
//	 1 if current > latest  (ahead of the latest release, e.g. a dev build)
//
// The second return value is false when either version is not a parseable
// semantic version (e.g. "dev"), in which case the int result is meaningless.
func CompareSemver(current, latest string) (int, bool) {
	c, ok1 := parseSemver(current)
	l, ok2 := parseSemver(latest)
	if !ok1 || !ok2 {
		return 0, false
	}
	for i := 0; i < 3; i++ {
		if c[i] != l[i] {
			if c[i] < l[i] {
				return -1, true
			}
			return 1, true
		}
	}
	return 0, true
}

// parseSemver extracts [major, minor, patch] from a "vX.Y.Z" string, ignoring
// any pre-release/build metadata suffix. Missing components default to 0.
func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return [3]int{}, false
	}
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < 3; i++ {
		if i >= len(parts) {
			out[i] = 0
			continue
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
