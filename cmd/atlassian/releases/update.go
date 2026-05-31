package releases

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/spf13/cobra"
)

// NewUpdateCmd returns the "releases update <release-id> [flags]" command.
func NewUpdateCmd(svc releases.ReleasesService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		name        string
		description string
		releaseDate string
		released    bool
		archived    bool
	)

	cmd := &cobra.Command{
		Use:   "update <release-id>",
		Short: "Update a Jira release (version)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would update release: release-id=%s\n", args[0])
				return nil
			}
			req := releases.UpdateReleaseRequest{}

			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
			}
			if cmd.Flags().Changed("release-date") {
				req.ReleaseDate = &releaseDate
			}
			if cmd.Flags().Changed("released") {
				req.Released = &released
			}
			if cmd.Flags().Changed("archived") {
				req.Archived = &archived
			}

		release, err := svc.UpdateRelease(context.Background(), args[0], req)
		auditLog.Log(audit.NewEntry("update_release", "releases",
			map[string]any{"release_id": args[0]}, err))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(releasesExitCode(err))
		}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated release: %s %s\n", release.ID, release.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New release name (optional)")
	cmd.Flags().StringVar(&description, "description", "", "New description (optional)")
	cmd.Flags().StringVar(&releaseDate, "release-date", "", "New release date YYYY-MM-DD (optional)")
	cmd.Flags().BoolVar(&released, "released", false, "Mark as released (optional)")
	cmd.Flags().BoolVar(&archived, "archived", false, "Archive the release (optional)")
	return cmd
}
