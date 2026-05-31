// Package teams provides cobra commands for Atlassian Teams operations.
package teams

import (
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	"github.com/spf13/cobra"
)

// NewTeamsCmd returns the "teams" subcommand group.
func NewTeamsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "teams",
		Short: "Interact with Atlassian teams",
		Long:  "Read Atlassian teams using the Teams Public REST API v1.",
	}
}

// RegisterCommands attaches all teams sub-commands to root.
func RegisterCommands(root *cobra.Command, svc teams.TeamsService) {
	root.AddCommand(NewListCmd(svc))
	root.AddCommand(NewGetCmd(svc))
	root.AddCommand(NewMembersCmd(svc))
}
