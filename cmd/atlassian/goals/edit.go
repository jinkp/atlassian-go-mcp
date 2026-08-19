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

// NewEditCmd returns the "edit <goal-id> [--name ...] [--target-date ...] [--confidence ...] [--archive]" command.
func NewEditCmd(svc goals.GoalsService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		name        string
		targetDate  string
		confidence  string
		archiveFlag string // "true" | "false" | "" (unset)
	)

	cmd := &cobra.Command{
		Use:   "edit <goal-id>",
		Short: "Edit structural fields of an Atlassian Goal (name, targetDate, isArchived)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			goalID := args[0]

			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would edit goal: goal-id=%s\n", goalID)
				return nil
			}

			req := goals.EditGoalRequest{GoalID: goalID}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("target-date") {
				req.TargetDate = &targetDate
			}
			if cmd.Flags().Changed("confidence") {
				req.Confidence = &confidence
			}
			if cmd.Flags().Changed("archive") {
				b := archiveFlag == "true"
				req.Archive = &b
			}

			result, err := svc.EditGoal(context.Background(), req)
			auditLog.Log(audit.NewEntry("edit_goal", "goals",
				map[string]any{"goal_id": goalID}, err))
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(goalsExitCode(err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated goal: %s %s\n", result.ID, result.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New goal name (optional)")
	cmd.Flags().StringVar(&targetDate, "target-date", "", "New target date YYYY-MM-DD (optional)")
	cmd.Flags().StringVar(&confidence, "confidence", "", "Date confidence: QUARTER|DAY|WEEK|MONTH|YEAR (optional)")
	cmd.Flags().StringVar(&archiveFlag, "archive", "", "Archive or unarchive: 'true' to archive, 'false' to unarchive (optional)")
	return cmd
}
