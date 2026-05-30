package agile

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/spf13/cobra"
)

// NewMoveToSprintCmd returns the "move-to-sprint --sprint-id ... --issues PROJ-1,PROJ-2" command.
func NewMoveToSprintCmd(svc agile.AgileService) *cobra.Command {
	var (
		sprintID   int
		issuesFlag string
	)

	cmd := &cobra.Command{
		Use:   "move-to-sprint",
		Short: "Move issues into a sprint",
		RunE: func(cmd *cobra.Command, args []string) error {
			keys := splitIssues(issuesFlag)
			if len(keys) == 0 {
				fmt.Fprintln(os.Stderr, "error: --issues must contain at least one issue key")
				os.Exit(1)
			}

			err := svc.MoveIssuesToSprint(context.Background(), sprintID, keys)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(agileExitCode(err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Moved %d issues to sprint %d\n", len(keys), sprintID)
			return nil
		},
	}

	cmd.Flags().IntVar(&sprintID, "sprint-id", 0, "Sprint ID (required)")
	cmd.Flags().StringVar(&issuesFlag, "issues", "", "Comma-separated issue keys e.g. PROJ-1,PROJ-2 (required)")
	_ = cmd.MarkFlagRequired("sprint-id")
	_ = cmd.MarkFlagRequired("issues")
	return cmd
}

// splitIssues splits a comma-separated issue key string, trimming whitespace and
// discarding empty entries.
func splitIssues(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}
