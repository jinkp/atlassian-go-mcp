package jira

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/spf13/cobra"
)

// NewCreateCmd returns the "create --project ... --type ... --summary ..." command.
func NewCreateCmd(svc jira.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		project     string
		issueType   string
		summary     string
		description string
		assignee    string
		priority    string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Jira issue",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would create jira issue: project=%s type=%s summary=%q\n", project, issueType, summary)
				return nil
			}
			req := jira.CreateIssueRequest{
				ProjectKey:  project,
				IssueType:   issueType,
				Summary:     summary,
				Description: description,
				AssigneeID:  assignee,
				PriorityName: priority,
			}

			resp, err := svc.CreateIssue(context.Background(), req)
		auditLog.Log(audit.NewEntry("create_jira_issue", "jira",
			map[string]any{"project": project, "summary": summary}, err))
		if err != nil {
			exitCode := exitCodeForError(err)
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(exitCode)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", resp.Key)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project key e.g. PROJ (required)")
	cmd.Flags().StringVar(&issueType, "type", "", "Issue type e.g. Bug, Task, Story (required)")
	cmd.Flags().StringVar(&summary, "summary", "", "Issue summary (required)")
	cmd.Flags().StringVar(&description, "description", "", "Issue description (optional)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee Atlassian account ID (optional)")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority name e.g. High, Medium, Low (optional)")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("summary")
	return cmd
}
