// Package projects provides cobra commands for Jira Projects operations.
package projects

import (
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/spf13/cobra"
)

// NewProjectsCmd returns the "projects" subcommand group.
func NewProjectsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "projects",
		Short: "Interact with Jira projects",
		Long:  "Read and manage Jira projects using the Jira REST API v3.",
	}
}

// RegisterCommands attaches all projects sub-commands to root.
func RegisterCommands(root *cobra.Command, svc projects.ProjectsService, auditLog audit.Logger, dryRun bool) {
	root.AddCommand(NewListCmd(svc))
	root.AddCommand(NewGetCmd(svc))
	root.AddCommand(NewSearchCmd(svc))
	root.AddCommand(NewUpdateCmd(svc, auditLog, dryRun))
}
