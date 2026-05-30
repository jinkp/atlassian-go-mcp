// Package agile provides cobra commands for Jira Agile operations.
package agile

import (
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/spf13/cobra"
)

// NewAgileCmd returns the "agile" subcommand group.
func NewAgileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agile",
		Short: "Interact with Jira Agile boards and sprints",
		Long:  "Read and manage Jira Software boards, sprints, and epics using the Agile REST API v1.",
	}
}

// RegisterCommands attaches all agile sub-commands to root.
func RegisterCommands(root *cobra.Command, svc agile.AgileService) {
	root.AddCommand(NewBoardsCmd(svc))
	root.AddCommand(NewSprintsCmd(svc))

	// "sprint" sub-group groups sprint-scoped commands
	sprintGroup := &cobra.Command{
		Use:   "sprint",
		Short: "Sprint operations (active, issues, create, update)",
	}
	sprintGroup.AddCommand(NewSprintActiveCmd(svc))
	sprintGroup.AddCommand(NewSprintIssuesCmd(svc))
	sprintGroup.AddCommand(NewSprintCreateCmd(svc))
	sprintGroup.AddCommand(NewSprintUpdateCmd(svc))
	root.AddCommand(sprintGroup)

	root.AddCommand(NewMoveToSprintCmd(svc))
	root.AddCommand(NewMoveToEpicCmd(svc))
}
