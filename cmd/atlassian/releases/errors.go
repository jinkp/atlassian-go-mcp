package releases

import (
	"errors"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// releasesExitCode maps sentinel errors to POSIX-style exit codes.
func releasesExitCode(err error) int {
	if err == nil {
		return 0
	}
	switch {
	case errors.Is(err, jira.ErrNotFound):
		return 3
	case errors.Is(err, jira.ErrUnauthorized):
		return 2
	case errors.Is(err, jira.ErrRateLimit):
		return 2
	default:
		return 2
	}
}
