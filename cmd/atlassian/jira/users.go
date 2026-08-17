package jira

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewUsersCmd returns the "users" subcommand group with "search" subcommand.
func NewUsersCmd(svc jira.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage Jira users",
	}
	cmd.AddCommand(newUsersSearchCmd(svc))
	return cmd
}

// newUsersSearchCmd returns the "users search --query <q> [--max-results N]" command.
func newUsersSearchCmd(svc jira.Service) *cobra.Command {
	var (
		query      string
		maxResults int
		outputFmt  string
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Jira users by name or email",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFmt)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			users, err := svc.LookupAccountID(context.Background(), query, maxResults)
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			data, err := formatter.Format(users)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Name or email to search for (required)")
	cmd.Flags().IntVar(&maxResults, "max-results", 0, "Maximum results to return (default 10)")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "json", "Output format: table, json, yaml")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}
