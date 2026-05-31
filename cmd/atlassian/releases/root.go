// Package releases provides cobra commands for Jira Releases (Versions) operations.
package releases

import (
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/spf13/cobra"
)

// NewReleasesCmd returns the "releases" subcommand group.
func NewReleasesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "releases",
		Short: "Interact with Jira releases (versions)",
		Long:  "Read and manage Jira project versions using the Jira REST API v3.",
	}
}

// RegisterCommands attaches all releases sub-commands to root.
func RegisterCommands(root *cobra.Command, svc releases.ReleasesService, auditLog audit.Logger, dryRun bool) {
	root.AddCommand(NewListCmd(svc))
	root.AddCommand(NewGetCmd(svc))
	root.AddCommand(NewIssuesCmd(svc))
	root.AddCommand(NewCreateCmd(svc, auditLog, dryRun))
	root.AddCommand(NewUpdateCmd(svc, auditLog, dryRun))
}
