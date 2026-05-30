package goals

import (
	"errors"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// goalsExitCode maps sentinel errors to POSIX-style exit codes.
func goalsExitCode(err error) int {
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
