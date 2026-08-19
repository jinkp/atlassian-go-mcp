package goals

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/internal/cliutil"
	"github.com/spf13/cobra"
)

// NewUpdateCmd returns the "update --goal-id --status [--score N] [--summary ...]" command.
func NewUpdateCmd(svc goals.GoalsService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		goalID  string
		status  string
		score   int
		summary string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Post a status update (check-in) to an Atlassian Goal",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would update goal status: goal-id=%s status=%s\n", goalID, status)
				return nil
			}
			req := goals.UpdateGoalStatusRequest{
				GoalID:  goalID,
				Status:  status,
				Score:   score,
				Summary: summary,
			}

			err := svc.UpdateGoalStatus(context.Background(), req)
		auditLog.Log(audit.NewEntry("update_goal_status", "goals",
			map[string]any{"goal_id": goalID, "status": status}, err))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(goalsExitCode(err))
		}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated goal: %s\n", goalID)
			return nil
		},
	}

	cmd.Flags().StringVar(&goalID, "goal-id", "", "Goal ARI (required)")
	cmd.Flags().StringVar(&status, "status", "", "New status: on_track, off_track, at_risk (required)")
	cmd.Flags().IntVar(&score, "score", 0, "Progress score 0-100 (optional, 0 = omit)")
	cmd.Flags().StringVar(&summary, "summary", "", "Check-in summary, plain text (optional)")
	_ = cmd.MarkFlagRequired("goal-id")
	_ = cmd.MarkFlagRequired("status")
	return cmd
}
