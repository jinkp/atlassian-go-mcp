package bitbucket

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/spf13/cobra"
)

// newPRCheckoutCmd checks out the PR's source branch locally via git.
// This is a LOCAL operation (no Bitbucket mutation): it fetches origin and
// checks out the branch in the current git repository.
func newPRCheckoutCmd(svc bitbucket.BitbucketService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkout <pr-id>",
		Short: "Check out a pull request's source branch locally (requires a git repo)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			id, err := parsePRID(args[0])
			if err != nil {
				fail(err)
			}
			pr, err := svc.GetPullRequest(context.Background(), ws, repo, id)
			if err != nil {
				fail(err)
			}
			branch := pr.Source.Branch.Name
			if err := checkoutBranch(context.Background(), branch); err != nil {
				fail(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Checked out branch %q for PR #%d\n", branch, id)
			return nil
		},
	}
	return cmd
}

// newPROpenCmd opens the PR's HTML page in the default browser.
func newPROpenCmd(svc bitbucket.BitbucketService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open <pr-id>",
		Short: "Open a pull request in the default browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, repo, err := resolveWorkspaceRepo(cmd)
			if err != nil {
				fail(err)
			}
			id, err := parsePRID(args[0])
			if err != nil {
				fail(err)
			}
			pr, err := svc.GetPullRequest(context.Background(), ws, repo, id)
			if err != nil {
				fail(err)
			}
			if err := openURL(pr.Links.HTML.Href); err != nil {
				fail(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Opened PR #%d in browser\n", id)
			return nil
		},
	}
	return cmd
}

// checkoutBranch fetches origin and checks out the given branch locally.
func checkoutBranch(ctx context.Context, branch string) error {
	if !isSafeBranchName(branch) {
		return fmt.Errorf("unsafe source branch name: %q", branch)
	}
	fetch := exec.CommandContext(ctx, "git", "fetch", "origin")
	fetch.Stdout = os.Stdout
	fetch.Stderr = os.Stderr
	if err := fetch.Run(); err != nil {
		return fmt.Errorf("could not fetch origin: %w", err)
	}
	checkout := exec.CommandContext(ctx, "git", "checkout", branch)
	checkout.Stdout = os.Stdout
	checkout.Stderr = os.Stderr
	if err := checkout.Run(); err != nil {
		return fmt.Errorf("could not check out branch %q: %w", branch, err)
	}
	return nil
}

// isSafeBranchName rejects branch names that could be interpreted as flags or
// contain control/whitespace characters (command-injection hardening).
func isSafeBranchName(branch string) bool {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" || trimmed != branch || strings.HasPrefix(trimmed, "-") {
		return false
	}
	for _, c := range branch {
		if c <= 0x1f || c == 0x7f || c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			return false
		}
	}
	return true
}

// openURL opens a validated http(s) URL in the OS default browser.
func openURL(rawURL string) error {
	safeURL, err := resolveSafeURL(rawURL)
	if err != nil {
		return err
	}
	file, args := openCommand(safeURL)
	if err := exec.Command(file, args...).Start(); err != nil {
		return fmt.Errorf("could not open URL in the default browser: %w", err)
	}
	return nil
}

func openCommand(target string) (string, []string) {
	switch runtime.GOOS {
	case "windows":
		return "cmd", []string{"/c", "start", "", target}
	case "darwin":
		return "open", []string{target}
	default:
		return "xdg-open", []string{target}
	}
}

func resolveSafeURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed != value {
		return "", fmt.Errorf("pull request URL is invalid or unsafe")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("pull request URL is invalid or unsafe")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("pull request URL is invalid or unsafe")
	}
	return parsed.String(), nil
}
