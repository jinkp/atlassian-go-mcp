package agile

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewSprintIssuesCmd returns the "sprint issues --sprint-id ..." command.
func NewSprintIssuesCmd(svc agile.AgileService) *cobra.Command {
	var (
		sprintID     int
		maxResults   int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "issues",
		Short: "List issues in a sprint",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := svc.GetSprintIssues(context.Background(), sprintID, maxResults)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(agileExitCode(err))
			}

			if len(result.Issues) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No issues found")
				return nil
			}

			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			data, err := formatter.Format(result)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().IntVar(&sprintID, "sprint-id", 0, "Sprint ID (required)")
	cmd.Flags().IntVar(&maxResults, "max-results", 50, "Maximum number of issues to return")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	_ = cmd.MarkFlagRequired("sprint-id")
	return cmd
}
