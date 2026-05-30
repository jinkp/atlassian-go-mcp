package agile

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewSprintsCmd returns the "sprints --board-id ... [--state ...]" command.
func NewSprintsCmd(svc agile.AgileService) *cobra.Command {
	var (
		boardID      int
		state        string
		maxResults   int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "sprints",
		Short: "List sprints for a board",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			sprints, err := svc.GetSprints(context.Background(), boardID, state, maxResults)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(agileExitCode(err))
			}

			data, err := formatter.Format(sprints)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().IntVar(&boardID, "board-id", 0, "Board ID (required)")
	cmd.Flags().StringVar(&state, "state", "", "Filter by state: active, future, or closed (empty = all)")
	cmd.Flags().IntVar(&maxResults, "max-results", 50, "Maximum number of sprints to return")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}
