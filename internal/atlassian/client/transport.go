package client

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"time"
)

// BasicAuthTransport is an http.RoundTripper that injects an Authorization: Basic
// header on every request. It wraps an inner transport (typically http.DefaultTransport).
// The API token is NEVER included in error messages — errors come from the inner transport.
type BasicAuthTransport struct {
	Email     string
	APIToken  string
	Transport http.RoundTripper // inner; defaults to http.DefaultTransport if nil
}

// RoundTrip clones the request, injects the Basic Auth header, and delegates to the
// inner transport. The original request is never mutated (safe for retries).
func (t *BasicAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	inner := t.Transport
	if inner == nil {
		inner = http.DefaultTransport
	}

	// Clone to avoid mutating the original request (required by http.RoundTripper contract).
	req := r.Clone(r.Context())
	creds := t.Email + ":" + t.APIToken
	encoded := base64.StdEncoding.EncodeToString([]byte(creds))
	req.Header.Set("Authorization", "Basic "+encoded)

	return inner.RoundTrip(req)
}

// RetryTransport is an http.RoundTripper that retries requests on HTTP 429
// (Too Many Requests) with exponential backoff. It respects the Retry-After
// response header when present. It does NOT retry on any other 4xx or 5xx codes.
type RetryTransport struct {
	Transport  http.RoundTripper // inner transport; defaults to http.DefaultTransport if nil
	MaxRetries int               // number of retries (not counting the initial attempt)
	BaseDelay  time.Duration     // initial backoff delay; doubled each retry
	MaxDelay   time.Duration     // backoff cap
}

// RoundTrip sends the request, retrying on 429 up to MaxRetries times.
// Each retry closes the previous response body to avoid resource leaks.
func (t *RetryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	inner := t.Transport
	if inner == nil {
		inner = http.DefaultTransport
	}

	baseDelay := t.BaseDelay
	if baseDelay == 0 {
		baseDelay = 1 * time.Second
	}
	maxDelay := t.MaxDelay
	if maxDelay == 0 {
		maxDelay = 30 * time.Second
	}

	var (
		resp  *http.Response
		err   error
		delay = baseDelay
	)

	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}

		// Clone request per attempt so the body (if any) can be re-read.
		req := r.Clone(r.Context())
		resp, err = inner.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			// Not a 429 — return immediately regardless of status code.
			return resp, nil
		}

		// 429: check for Retry-After header before using exponential backoff.
		if attempt < t.MaxRetries {
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, parseErr := strconv.Atoi(ra); parseErr == nil {
					delay = time.Duration(secs) * time.Second
				}
			}
			// Drain and close the 429 body before retrying.
			resp.Body.Close()
		}
	}

	// All retries exhausted — return the last 429 response.
	return resp, nil
}
