// Package jira provides cobra commands for Jira operations.
package jira

import (
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
