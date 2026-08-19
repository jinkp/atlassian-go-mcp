package agile

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/cliutil"
	"github.com/spf13/cobra"
)

// NewSprintCreateCmd returns the "sprint create --board-id --name [--start --end]" command.
func NewSprintCreateCmd(svc agile.AgileService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		boardID   int
		name      string
		startDate string
		endDate   string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new sprint on a board",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would create sprint: board-id=%d name=%q\n", boardID, name)
				return nil
			}
			req := agile.CreateSprintRequest{
				Name:      name,
				BoardID:   boardID,
				StartDate: startDate,
				EndDate:   endDate,
			}

			sprint, err := svc.CreateSprint(context.Background(), req)
		auditLog.Log(audit.NewEntry("create_sprint", "agile",
			map[string]any{"name": name, "board_id": boardID}, err))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(agileExitCode(err))
		}

			fmt.Fprintf(cmd.OutOrStdout(), "Created sprint: %d %s\n", sprint.ID, sprint.Name)
			return nil
		},
	}

	cmd.Flags().IntVar(&boardID, "board-id", 0, "Board ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "Sprint name (required)")
	cmd.Flags().StringVar(&startDate, "start", "", "Start date ISO 8601 e.g. 2026-01-15T00:00:00.000Z (optional)")
	cmd.Flags().StringVar(&endDate, "end", "", "End date ISO 8601 (optional)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
