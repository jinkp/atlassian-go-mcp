// Package jira provides cobra commands for Jira operations.
package jira

import (
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/spf13/cobra"
)

// NewJiraCmd returns the "jira" subcommand group.
// All Jira sub-commands (get, search) are added here.
func NewJiraCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jira",
		Short: "Interact with Jira issues",
		Long:  "Read and search Jira issues using the Jira REST API v3.",
	}
	return cmd
}

// RegisterCommands attaches all jira sub-commands to root.
func RegisterCommands(root *cobra.Command, svc jira.Service, auditLog audit.Logger, dryRun bool) {
	// Existing commands
	root.AddCommand(NewGetCmd(svc))
	root.AddCommand(NewSearchCmd(svc))
	root.AddCommand(NewCreateCmd(svc, auditLog, dryRun))
	root.AddCommand(NewUpdateCmd(svc, auditLog, dryRun))
	root.AddCommand(NewTransitionsCmd(svc))
	root.AddCommand(NewTransitionCmd(svc, auditLog, dryRun))

	// Phase 1 Block 4: new commands
	root.AddCommand(NewUsersCmd(svc))
	root.AddCommand(NewCommentCmd(svc, auditLog, dryRun))
	root.AddCommand(NewLinkCmd(svc, auditLog, dryRun))
	root.AddCommand(NewLinkTypesCmd(svc))
	root.AddCommand(NewWorklogCmd(svc, auditLog, dryRun))
	root.AddCommand(NewIssueTypesCmd(svc))
}
