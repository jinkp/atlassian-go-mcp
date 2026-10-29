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
		cursor       string
		maxResults   int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "List Atlassian Goals for a site, optionally narrowed by name or key",
		Long: `List Atlassian Goals for a site.

--query is plain text matched case-insensitively against each goal's name and
key. It is NOT a query language: --query "status = on_track" is matched
literally and finds nothing.

Matching happens on the client because the Goals API ignores its own search
argument and offers no filter. A narrowed search therefore walks pages until it
collects enough matches, bounded by a page budget. If the budget stops the scan
early the output sets scan_incomplete and more matches may exist past
next_cursor, so an empty or short result is not proof that nothing else matches.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			req := goals.SearchGoalsRequest{
				SiteID:       siteID,
				SearchString: query,
				MaxResults:   maxResults,
				Cursor:       cursor,
			}

			result, err := svc.SearchGoals(context.Background(), req)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(goalsExitCode(err))
			}

			// A scan cut short by the page budget proves nothing about the goals
			// it never looked at, so it must never be presented as a complete
			// answer. The warning goes to stderr so `-o json` stays parseable.
			if result.ScanIncomplete {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: scan incomplete — the search stopped before exhausting this site, so more matches may exist. Resume with --cursor %q\n",
					result.NextCursor,
				)
			}

			if len(result.Goals) == 0 {
				if result.ScanIncomplete {
					fmt.Fprintln(cmd.OutOrStdout(), "No matching goals in the pages scanned (scan incomplete — see warning above)")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No goals found")
				}
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
	cmd.Flags().StringVar(&query, "query", "", "Plain text matched against goal name and key, case-insensitive (not a query language)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor to resume from (use next_cursor from a previous run)")
	cmd.Flags().IntVar(&maxResults, "max-results", 25, "Maximum number of goals to return")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	_ = cmd.MarkFlagRequired("site-id")
	return cmd
}
