package goals

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewByOwnerCmd returns the "by-owner --site-id ... --owner ..." command.
func NewByOwnerCmd(svc goals.GoalsService) *cobra.Command {
	var (
		siteID       string
		owner        string
		cursor       string
		maxResults   int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "by-owner",
		Short: "List the goals owned by one person",
		Long: `List the goals owned by one person.

--owner takes either a bare Atlassian account ID, exactly as the owner_id field
of "goals search" prints it, or a full "ari:cloud:identity::user/..." ARI.

Unlike "goals search --query", this filters on the server, so the result is
complete for the page requested and never reports an incomplete scan.

Example:

  atlassian goals search --site-id <cloudId> -o json      # read owner_id here
  atlassian goals by-owner --site-id <cloudId> --owner 712020:7bfdc0f9-...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := svc.GetGoalsByOwner(context.Background(), goals.GoalsByOwnerRequest{
				SiteID:     siteID,
				OwnerID:    owner,
				MaxResults: maxResults,
				Cursor:     cursor,
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(goalsExitCode(err))
			}

			if len(result.Goals) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No goals found for that owner")
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
	cmd.Flags().StringVar(&owner, "owner", "", "Owner account ID or full user ARI (required)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor to resume from (use next_cursor from a previous run)")
	cmd.Flags().IntVar(&maxResults, "max-results", 25, "Maximum number of goals to return")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	_ = cmd.MarkFlagRequired("site-id")
	_ = cmd.MarkFlagRequired("owner")
	return cmd
}
