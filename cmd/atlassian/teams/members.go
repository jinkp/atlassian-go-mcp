package teams

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewMembersCmd returns the "teams members <team-id> [--max-results 50]" command.
func NewMembersCmd(svc teams.TeamsService) *cobra.Command {
	var (
		maxResults   int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "members <team-id>",
		Short: "List members of an Atlassian team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID := args[0]

			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			members, err := svc.GetTeamMembers(context.Background(), teamID, maxResults)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(teamsExitCode(err))
			}

			data, err := formatter.Format(members)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().IntVar(&maxResults, "max-results", 50, "Maximum number of members to return")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	return cmd
}
