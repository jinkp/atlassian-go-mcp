// Package client provides a reusable HTTP client for Atlassian REST APIs.
// It composes BasicAuthTransport and RetryTransport for auth injection and
// retry-with-backoff without exposing those concerns to callers.
package client

import (
	"errors"
	"net/http"
	"time"
)

const (
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 3
	userAgent         = "atlassian-go-mcp/0.1.0"
)

// HTTPDoer is the minimal interface JiraService (and any other service) needs.
// *http.Client satisfies it; httptest mocks satisfy it too.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Config holds all parameters needed to create an Atlassian API client.
type Config struct {
	BaseURL    string
	Email      string
	APIToken   string
	MaxRetries int
	Timeout    time.Duration
}

// Client wraps an http.Client with Atlassian-specific configuration.
type Client struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	timeout    time.Duration
}

// NewClient constructs a Client from a Config. Returns an error if the
// base URL is empty. Applies sensible defaults for timeout and max retries.
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("base URL must not be empty")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = defaultMaxRetries
	}

	inner := http.DefaultTransport
	auth := &BasicAuthTransport{
		Email:     cfg.Email,
		APIToken:  cfg.APIToken,
		Transport: inner,
	}
	idempotency := &IdempotencyTransport{
		Transport: auth,
	}
	retry := &RetryTransport{
		Transport:  idempotency,
		MaxRetries: maxRetries,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
	}

	return &Client{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Transport: retry,
			Timeout:   timeout,
		},
		maxRetries: maxRetries,
		timeout:    timeout,
	}, nil
}

// Do executes an HTTP request using the underlying http.Client.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", userAgent)
	return c.httpClient.Do(req)
}

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Timeout returns the configured HTTP timeout.
func (c *Client) Timeout() time.Duration {
	return c.timeout
}

// MaxRetries returns the configured max retry count.
func (c *Client) MaxRetries() int {
	return c.maxRetries
}
