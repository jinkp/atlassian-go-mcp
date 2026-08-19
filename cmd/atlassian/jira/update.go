package jira

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/cliutil"
	"github.com/spf13/cobra"
)

// NewUpdateCmd returns the "update <KEY> [--summary ...] [--description ...] [--assignee ...] [--priority ...]" command.
func NewUpdateCmd(svc jira.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		summary     string
		description string
		assignee    string
		priority    string
	)

	cmd := &cobra.Command{
		Use:   "update <KEY>",
		Short: "Update fields on an existing Jira issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would update jira issue: key=%s\n", key)
				return nil
			}
			req := jira.UpdateIssueRequest{}

			if cmd.Flags().Changed("summary") {
				req.Summary = &summary
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
			}
			if cmd.Flags().Changed("assignee") {
				req.AssigneeID = &assignee
			}
			if cmd.Flags().Changed("priority") {
				req.PriorityName = &priority
			}

		err := svc.UpdateIssue(context.Background(), key, req)
		auditLog.Log(audit.NewEntry("update_jira_issue", "jira",
			map[string]any{"issue_key": key}, err))
		if err != nil {
			exitCode := exitCodeForError(err)
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(exitCode)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Updated: %s\n", key)
			return nil
		},
	}

	cmd.Flags().StringVar(&summary, "summary", "", "New summary (optional)")
	cmd.Flags().StringVar(&description, "description", "", "New description, plain text (optional)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "New assignee Atlassian account ID (optional)")
	cmd.Flags().StringVar(&priority, "priority", "", "New priority name e.g. High (optional)")
	return cmd
}
