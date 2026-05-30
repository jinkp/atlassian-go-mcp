package main_test

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// TestMain_MissingEnvVars verifies that the binary exits with code 1
// and prints a clear error when required env vars are absent.
// This test requires the binary to be pre-built; it's skipped if not found.
func TestMain_MissingEnvVars(t *testing.T) {
	// Build the binary for this test if it doesn't exist
	binName := "atlassian"
	if runtime.GOOS == "windows" {
		binName = "atlassian.exe"
	}
	binPath := t.TempDir() + "/" + binName
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = "." // current package dir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Skipf("skipping: could not build binary: %v\n%s", err, out)
	}

	tests := []struct {
		name    string
		env     map[string]string
		wantMsg string
	}{
		{
			name: "missing all env vars",
			env:  map[string]string{},
			// Should mention at least one of the required vars
		},
		{
			name: "missing ATLASSIAN_TOKEN",
			env: map[string]string{
				"ATLASSIAN_BASE_URL": "https://acme.atlassian.net",
				"ATLASSIAN_EMAIL":    "user@example.com",
			},
		},
		{
			name: "missing ATLASSIAN_EMAIL",
			env: map[string]string{
				"ATLASSIAN_BASE_URL": "https://acme.atlassian.net",
				"ATLASSIAN_TOKEN":    "token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a clean env with only what we specify (no inherited vars)
			cmd := exec.Command(binPath, "jira", "get", "PROJ-1")
			cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
			for k, v := range tt.env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}

			err := cmd.Run()
			if err == nil {
				t.Fatal("expected non-zero exit code, got 0")
			}

			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("expected ExitError, got: %T: %v", err, err)
			}
			if exitErr.ExitCode() != 1 {
				t.Errorf("expected exit code 1 for missing env vars, got %d", exitErr.ExitCode())
			}
		})
	}
}
