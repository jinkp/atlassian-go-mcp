package jira

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewGetCmd returns the "get <KEY>" command.
// It accepts a --output flag and maps service errors to exit codes.
func NewGetCmd(svc jira.Service) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "get <KEY>",
		Short: "Fetch a Jira issue by key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			issue, err := svc.GetIssue(context.Background(), key)
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			data, err := formatter.Format(issue)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	return cmd
}

// exitCodeForError maps known sentinel errors to POSIX-style exit codes.
// 0 = success, 1 = usage/user error, 2 = auth/API error, 3 = not found.
func exitCodeForError(err error) int {
	if errors.Is(err, jira.ErrNotFound) {
		return 3
	}
	if errors.Is(err, jira.ErrUnauthorized) {
		return 2
	}
	if errors.Is(err, jira.ErrInvalidJQL) {
		return 1
	}
	if errors.Is(err, jira.ErrRateLimit) {
		return 2
	}
	return 2
}
