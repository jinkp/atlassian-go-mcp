package agile

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/spf13/cobra"
)

// NewSprintUpdateCmd returns the "sprint update --sprint-id ... [--name --state --start --end]" command.
func NewSprintUpdateCmd(svc agile.AgileService) *cobra.Command {
	var (
		sprintID  int
		name      string
		state     string
		startDate string
		endDate   string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a sprint's name, state, or dates",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := agile.UpdateSprintRequest{}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("state") {
				req.State = &state
			}
			if cmd.Flags().Changed("start") {
				req.StartDate = &startDate
			}
			if cmd.Flags().Changed("end") {
				req.EndDate = &endDate
			}

			sprint, err := svc.UpdateSprint(context.Background(), sprintID, req)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(agileExitCode(err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated sprint: %d %s\n", sprint.ID, sprint.Name)
			return nil
		},
	}

	cmd.Flags().IntVar(&sprintID, "sprint-id", 0, "Sprint ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "New sprint name (optional)")
	cmd.Flags().StringVar(&state, "state", "", "New state: active, closed (optional)")
	cmd.Flags().StringVar(&startDate, "start", "", "New start date ISO 8601 (optional)")
	cmd.Flags().StringVar(&endDate, "end", "", "New end date ISO 8601 (optional)")
	_ = cmd.MarkFlagRequired("sprint-id")
	return cmd
}
