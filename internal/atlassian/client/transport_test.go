package client_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
)

// --- BasicAuthTransport tests ---

func TestBasicAuthTransport_InjectsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &client.BasicAuthTransport{
		Email:     "user@example.com",
		APIToken:  "my-secret-token",
		Transport: http.DefaultTransport,
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@example.com:my-secret-token"))
	if gotAuth != expected {
		t.Errorf("expected Authorization=%q, got %q", expected, gotAuth)
	}
}

func TestBasicAuthTransport_DifferentCredentials(t *testing.T) {
	// Triangulate: different user+token produce different Base64
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &client.BasicAuthTransport{
		Email:     "admin@corp.com",
		APIToken:  "another-token-xyz",
		Transport: http.DefaultTransport,
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin@corp.com:another-token-xyz"))
	if gotAuth != expected {
		t.Errorf("expected Authorization=%q, got %q", expected, gotAuth)
	}
}

func TestBasicAuthTransport_DoesNotLeakTokenInError(t *testing.T) {
	// Point at an invalid address to force a network error
	transport := &client.BasicAuthTransport{
		Email:     "user@example.com",
		APIToken:  "super-secret-token-12345",
		Transport: http.DefaultTransport,
	}

	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:1", nil)
	_, err := transport.RoundTrip(req)
	if err == nil {
		t.Skip("expected network error, but got none — skipping token leak check")
	}

	if strings.Contains(err.Error(), "super-secret-token-12345") {
		t.Errorf("token leaked in error: %v", err)
	}
}

// --- RetryTransport tests ---

func TestRetryTransport_RetriesOn429(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &client.RetryTransport{
		Transport:  http.DefaultTransport,
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond, // fast for tests
		MaxDelay:   10 * time.Millisecond,
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 after retries, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 calls (2 x 429 + 1 x 200), got %d", atomic.LoadInt32(&callCount))
	}
}

func TestRetryTransport_RespectsRetryAfterHeader(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0") // 0s for test speed
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &client.RetryTransport{
		Transport:  http.DefaultTransport,
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	start := time.Now()
	resp, err := transport.RoundTrip(req)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("RoundTrip() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	// With Retry-After: 0, should be fast (< 500ms)
	if elapsed > 500*time.Millisecond {
		t.Errorf("expected fast retry with Retry-After: 0, took %v", elapsed)
	}
}

func TestRetryTransport_NoRetryOn401(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	transport := &client.RetryTransport{
		Transport:  http.DefaultTransport,
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected exactly 1 call for 401 (no retry), got %d", atomic.LoadInt32(&callCount))
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 response, got %d", resp.StatusCode)
	}
}

func TestRetryTransport_NoRetryOn500(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	transport := &client.RetryTransport{
		Transport:  http.DefaultTransport,
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected exactly 1 call for 500 (no retry), got %d", atomic.LoadInt32(&callCount))
	}
}

func TestRetryTransport_StopsAfterMaxRetries(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusTooManyRequests) // always 429
	}))
	defer srv.Close()

	transport := &client.RetryTransport{
		Transport:  http.DefaultTransport,
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// 1 initial + 3 retries = 4 total
	if atomic.LoadInt32(&callCount) != 4 {
		t.Errorf("expected 4 total calls (1 + 3 retries), got %d", atomic.LoadInt32(&callCount))
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected final 429, got %d", resp.StatusCode)
	}
}

func TestRetryTransport_ExponentialBackoff(t *testing.T) {
	// We verify the delay pattern: each call should be delayed longer.
	// Use timestamps from the server side to measure actual delays.
	var timestamps []time.Time
	var mu int32 // use atomic for simplicity

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&mu, 1)
		timestamps = append(timestamps, time.Now())
		w.WriteHeader(http.StatusTooManyRequests) // always retry
	}))
	defer srv.Close()

	baseDelay := 10 * time.Millisecond
	transport := &client.RetryTransport{
		Transport:  http.DefaultTransport,
		MaxRetries: 3,
		BaseDelay:  baseDelay,
		MaxDelay:   500 * time.Millisecond,
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, _ := transport.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}

	if len(timestamps) < 2 {
		t.Fatalf("expected at least 2 server calls, got %d", len(timestamps))
	}

	// First gap (after initial call) should be at least baseDelay
	gap1 := timestamps[1].Sub(timestamps[0])
	if gap1 < baseDelay/2 {
		t.Errorf("expected gap1 >= %v, got %v", baseDelay/2, gap1)
	}
	fmt.Printf("Backoff gaps: gap1=%v", gap1)
	if len(timestamps) >= 3 {
		gap2 := timestamps[2].Sub(timestamps[1])
		fmt.Printf(", gap2=%v", gap2)
		// Second gap should be approximately 2x the first (exponential)
		if gap2 < gap1 {
			t.Errorf("expected exponential backoff: gap2 (%v) should be >= gap1 (%v)", gap2, gap1)
		}
	}
	fmt.Println()
}
