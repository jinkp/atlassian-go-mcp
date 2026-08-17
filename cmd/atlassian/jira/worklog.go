package jira

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/spf13/cobra"
)

// NewWorklogCmd returns the "worklog" subcommand group with "add" subcommand.
func NewWorklogCmd(svc jira.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worklog",
		Short: "Manage worklogs on Jira issues",
	}
	cmd.AddCommand(newWorklogAddCmd(svc, auditLog, dryRun))
	return cmd
}

// newWorklogAddCmd returns the "worklog add <KEY> --time-spent <s> [--comment <c>] [--started <t>]" command.
func newWorklogAddCmd(svc jira.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		timeSpent string
		comment   string
		started   string
	)

	cmd := &cobra.Command{
		Use:   "add <KEY>",
		Short: "Log time spent on a Jira issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would add worklog to %s: time-spent=%s\n", key, timeSpent)
				return nil
			}

			req := jira.AddWorklogRequest{
				TimeSpent: timeSpent,
				Comment:   comment,
				Started:   started,
			}

			worklog, err := svc.AddWorklog(context.Background(), key, req)
			auditLog.Log(audit.NewEntry("add_worklog", "jira",
				map[string]any{"issue_key": key, "time_spent": timeSpent}, err))
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Worklog added: %s (%ds)\n", worklog.ID, worklog.TimeSpentSeconds)
			return nil
		},
	}

	cmd.Flags().StringVar(&timeSpent, "time-spent", "", "Time spent e.g. '3h 30m', '2h', '30m' (required)")
	cmd.Flags().StringVar(&comment, "comment", "", "Optional comment for the worklog entry")
	cmd.Flags().StringVar(&started, "started", "", "Optional start time in ISO 8601 format")
	_ = cmd.MarkFlagRequired("time-spent")
	return cmd
}
