// Package confluence provides cobra commands for Confluence operations.
package confluence

import (
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/spf13/cobra"
)

// NewConfluenceCmd returns the "confluence" subcommand group.
// All Confluence sub-commands are added via RegisterCommands.
func NewConfluenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "confluence",
		Short: "Interact with Confluence pages, spaces, and comments",
		Long:  "Read and manage Confluence pages, spaces, and comments using the Confluence Cloud REST API.",
	}
	return cmd
}

// RegisterCommands attaches all confluence sub-commands to root.
func RegisterCommands(root *cobra.Command, svc confluence.Service, auditLog audit.Logger, dryRun bool) {
	// Page subgroup
	root.AddCommand(NewPageCmd(svc, auditLog, dryRun))

	// Spaces
	root.AddCommand(NewSpacesCmd(svc))

	// Comment subgroup
	root.AddCommand(NewCommentCmd(svc, auditLog, dryRun))

	// Search
	root.AddCommand(NewSearchCmd(svc))
}
