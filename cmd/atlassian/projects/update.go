package projects

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/cliutil"
	"github.com/spf13/cobra"
)

// NewUpdateCmd returns the "projects update <KEY> [flags]" command.
func NewUpdateCmd(svc projects.ProjectsService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		name        string
		description string
		lead        string
	)

	cmd := &cobra.Command{
		Use:   "update <project-key>",
		Short: "Update a Jira project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would update project: project-key=%s\n", args[0])
				return nil
			}
			req := projects.UpdateProjectRequest{}

			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
			}
			if cmd.Flags().Changed("lead") {
				req.Lead = &lead
			}

		project, err := svc.UpdateProject(context.Background(), args[0], req)
		auditLog.Log(audit.NewEntry("update_project", "projects",
			map[string]any{"project_key": args[0]}, err))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(projectsExitCode(err))
		}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated project: %s %s\n", project.Key, project.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New project name (optional)")
	cmd.Flags().StringVar(&description, "description", "", "New project description (optional)")
	cmd.Flags().StringVar(&lead, "lead", "", "New lead account ID (optional)")
	return cmd
}
