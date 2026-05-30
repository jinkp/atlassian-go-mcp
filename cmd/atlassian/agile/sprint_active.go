package agile

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewSprintActiveCmd returns the "sprint active --board-id ..." command.
// It calls GetSprints with state="active" and returns the first result.
func NewSprintActiveCmd(svc agile.AgileService) *cobra.Command {
	var (
		boardID      int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "active",
		Short: "Show the active sprint for a board",
		RunE: func(cmd *cobra.Command, args []string) error {
			sprints, err := svc.GetSprints(context.Background(), boardID, "active", 1)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(agileExitCode(err))
			}

			if len(sprints) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no active sprint for board %d\n", boardID)
				return nil
			}

			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			// Wrap in a single-element slice so the sprint header+row renders correctly.
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
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}
