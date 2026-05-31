package teams

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewListCmd returns the "teams list [--query ...] [--max-results 50]" command.
func NewListCmd(svc teams.TeamsService) *cobra.Command {
	var (
		query        string
		maxResults   int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Atlassian teams",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			result, err := svc.GetTeams(context.Background(), query, maxResults)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(teamsExitCode(err))
			}

			data, err := formatter.Format(result.Teams)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Search query to filter teams by name")
	cmd.Flags().IntVar(&maxResults, "max-results", 50, "Maximum number of teams to return")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	return cmd
}
