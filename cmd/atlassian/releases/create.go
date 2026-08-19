package releases

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/cliutil"
	"github.com/spf13/cobra"
)

// NewCreateCmd returns the "releases create --project-id ... --name ..." command.
func NewCreateCmd(svc releases.ReleasesService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		projectID   string
		name        string
		description string
		startDate   string
		releaseDate string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Jira release (version)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would create release: project-id=%s name=%q\n", projectID, name)
				return nil
			}
			req := releases.CreateReleaseRequest{
				ProjectID:   projectID,
				Name:        name,
				Description: description,
				StartDate:   startDate,
				ReleaseDate: releaseDate,
			}

			release, err := svc.CreateRelease(context.Background(), req)
		auditLog.Log(audit.NewEntry("create_release", "releases",
			map[string]any{"project_id": projectID, "name": name}, err))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(releasesExitCode(err))
		}

			fmt.Fprintf(cmd.OutOrStdout(), "Created release: %s %s\n", release.ID, release.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project-id", "", "Project ID (numeric string, required)")
	cmd.Flags().StringVar(&name, "name", "", "Release name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Release description (optional)")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date YYYY-MM-DD (optional)")
	cmd.Flags().StringVar(&releaseDate, "release-date", "", "Release date YYYY-MM-DD (optional)")
	_ = cmd.MarkFlagRequired("project-id")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
