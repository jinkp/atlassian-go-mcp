package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
)

func TestNewClient_EmptyBaseURL(t *testing.T) {
	_, err := client.NewClient(client.Config{
		BaseURL:  "",
		Email:    "user@example.com",
		APIToken: "token",
	})
	if err == nil {
		t.Fatal("expected error for empty base URL, got nil")
	}
	if err.Error() == "" {
		t.Fatal("error message should not be empty")
	}
	// Error must mention "base URL"
	if !contains(err.Error(), "base URL") {
		t.Errorf("expected error to contain 'base URL', got: %s", err.Error())
	}
}

func TestNewClient_ValidConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     client.Config
		wantErr bool
	}{
		{
			name: "minimal valid config",
			cfg: client.Config{
				BaseURL:  "https://acme.atlassian.net",
				Email:    "user@example.com",
				APIToken: "secret",
			},
			wantErr: false,
		},
		{
			name: "with explicit timeout and retries",
			cfg: client.Config{
				BaseURL:    "https://acme.atlassian.net",
				Email:      "user@example.com",
				APIToken:   "secret",
				MaxRetries: 5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := client.NewClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && c == nil {
				t.Error("expected non-nil client")
			}
		})
	}
}

func TestNewClient_DefaultsApplied(t *testing.T) {
	c, err := client.NewClient(client.Config{
		BaseURL:  "https://acme.atlassian.net",
		Email:    "user@example.com",
		APIToken: "token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default timeout is 30s
	if c.Timeout().Seconds() != 30 {
		t.Errorf("expected default timeout 30s, got %v", c.Timeout())
	}
	// Default MaxRetries is 3
	if c.MaxRetries() != 3 {
		t.Errorf("expected default MaxRetries 3, got %d", c.MaxRetries())
	}
}

func TestClient_Do_SendsRequest(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := client.NewClient(client.Config{
		BaseURL:  srv.URL,
		Email:    "user@example.com",
		APIToken: "token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/rest/api/3/issue/PROJ-1", nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if !called {
		t.Error("expected server handler to be called")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// contains checks if s contains substr (case-sensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
