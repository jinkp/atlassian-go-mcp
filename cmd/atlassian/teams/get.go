package teams

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewGetCmd returns the "teams get <team-id>" command.
func NewGetCmd(svc teams.TeamsService) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "get <team-id>",
		Short: "Get an Atlassian team by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID := args[0]

			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			team, err := svc.GetTeam(context.Background(), teamID)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(teamsExitCode(err))
			}

			data, err := formatter.Format(team)
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
