// Package goals provides cobra commands for Atlassian Goals operations.
package goals

import (
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/spf13/cobra"
)

// NewGoalsCmd returns the "goals" subcommand group.
func NewGoalsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "goals",
		Short: "Interact with Atlassian Goals",
		Long:  "Read and manage Atlassian Goals via the platform GraphQL API.",
	}
}

// RegisterCommands attaches all goals sub-commands to root.
func RegisterCommands(root *cobra.Command, svc goals.GoalsService, auditLog audit.Logger, dryRun bool) {
	root.AddCommand(NewSiteIDCmd(svc))
	root.AddCommand(NewGetCmd(svc))
	root.AddCommand(NewSearchCmd(svc))
	root.AddCommand(NewCreateCmd(svc, auditLog, dryRun))
	root.AddCommand(NewUpdateCmd(svc, auditLog, dryRun))
	root.AddCommand(NewEditCmd(svc, auditLog, dryRun))
}
