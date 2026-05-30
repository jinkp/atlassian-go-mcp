package agile

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewBoardsCmd returns the "boards --project ..." command.
func NewBoardsCmd(svc agile.AgileService) *cobra.Command {
	var (
		project      string
		maxResults   int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "boards",
		Short: "List Jira Software boards for a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			boards, err := svc.GetBoards(context.Background(), project, maxResults)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(agileExitCode(err))
			}

			data, err := formatter.Format(boards)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project key (required)")
	cmd.Flags().IntVar(&maxResults, "max-results", 50, "Maximum number of boards to return")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}

// agileExitCode maps sentinel errors to POSIX-style exit codes.
func agileExitCode(err error) int {
	if err == nil {
		return 0
	}
	switch {
	case isErr(err, jira.ErrNotFound):
		return 3
	case isErr(err, jira.ErrUnauthorized):
		return 2
	case isErr(err, jira.ErrRateLimit):
		return 2
	default:
		return 2
	}
}
