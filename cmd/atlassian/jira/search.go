package jira

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewSearchCmd returns the "search --jql '...' [--max-results N]" command.
func NewSearchCmd(svc jira.Service) *cobra.Command {
	var (
		jqlQuery     string
		maxResults   int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Jira issues using JQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jqlQuery == "" {
				fmt.Fprintln(os.Stderr, "error: --jql flag is required")
				os.Exit(1)
			}

			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			result, err := svc.SearchIssues(context.Background(), jqlQuery, maxResults)
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			data, err := formatter.Format(result.Issues)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&jqlQuery, "jql", "", "JQL query string (required)")
	cmd.Flags().IntVar(&maxResults, "max-results", 50, "Maximum number of results to return")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")

	return cmd
}
