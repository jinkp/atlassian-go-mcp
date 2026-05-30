package goals

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/spf13/cobra"
)

// NewCreateCmd returns the "create --site-id --name --type-id --target-date [opts]" command.
func NewCreateCmd(svc goals.GoalsService) *cobra.Command {
	var (
		siteID      string
		name        string
		typeID      string
		targetDate  string
		confidence  string
		description string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Atlassian Goal",
		Long: `Create a new Goal for a site. Requires the goal type ID (an ARI) which can be
obtained from your Atlassian admin or workspace settings.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			req := goals.CreateGoalRequest{
				SiteID:      siteID,
				Name:        name,
				GoalTypeID:  typeID,
				TargetDate:  targetDate,
				Confidence:  confidence,
				Description: description,
			}

			result, err := svc.CreateGoal(context.Background(), req)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(goalsExitCode(err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created goal: %s %s\n", result.ID, result.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&siteID, "site-id", "", "Atlassian cloud site ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "Goal name (required)")
	cmd.Flags().StringVar(&typeID, "type-id", "", "Goal type ARI (required, obtain from Atlassian admin)")
	cmd.Flags().StringVar(&targetDate, "target-date", "", "Target date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&confidence, "confidence", "", "Confidence: QUARTER|DAY|WEEK|MONTH|YEAR (default QUARTER)")
	cmd.Flags().StringVar(&description, "description", "", "Goal description, plain text (optional)")
	_ = cmd.MarkFlagRequired("site-id")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("type-id")
	_ = cmd.MarkFlagRequired("target-date")
	return cmd
}
