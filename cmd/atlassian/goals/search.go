package goals

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewSearchCmd returns the "search --site-id ... [--query ...] [--max-results N]" command.
func NewSearchCmd(svc goals.GoalsService) *cobra.Command {
	var (
		siteID       string
		query        string
		maxResults   int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Atlassian Goals for a site",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := goals.SearchGoalsRequest{
				SiteID:       siteID,
				SearchString: query,
				MaxResults:   maxResults,
			}

			result, err := svc.SearchGoals(context.Background(), req)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(goalsExitCode(err))
			}

			if len(result.Goals) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No goals found")
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

	cmd.Flags().StringVar(&siteID, "site-id", "", "Atlassian cloud site ID (required, use `goals site-id` to obtain)")
	cmd.Flags().StringVar(&query, "query", "", "Search string (optional)")
	cmd.Flags().IntVar(&maxResults, "max-results", 25, "Maximum number of goals to return")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	_ = cmd.MarkFlagRequired("site-id")
	return cmd
}
