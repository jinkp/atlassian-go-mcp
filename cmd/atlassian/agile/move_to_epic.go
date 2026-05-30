package agile

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/spf13/cobra"
)

// NewMoveToEpicCmd returns the "move-to-epic --epic-key ... --issues PROJ-1,PROJ-2" command.
func NewMoveToEpicCmd(svc agile.AgileService) *cobra.Command {
	var (
		epicKey    string
		issuesFlag string
	)

	cmd := &cobra.Command{
		Use:   "move-to-epic",
		Short: "Link issues to an epic",
		RunE: func(cmd *cobra.Command, args []string) error {
			keys := splitIssues(issuesFlag)
			if len(keys) == 0 {
				fmt.Fprintln(os.Stderr, "error: --issues must contain at least one issue key")
				os.Exit(1)
			}

			err := svc.MoveIssuesToEpic(context.Background(), epicKey, keys)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(agileExitCode(err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Moved %d issues to epic %s\n", len(keys), epicKey)
			return nil
		},
	}

	cmd.Flags().StringVar(&epicKey, "epic-key", "", "Epic issue key e.g. EPIC-1 (required)")
	cmd.Flags().StringVar(&issuesFlag, "issues", "", "Comma-separated issue keys e.g. PROJ-1,PROJ-2 (required)")
	_ = cmd.MarkFlagRequired("epic-key")
	_ = cmd.MarkFlagRequired("issues")
	return cmd
}
