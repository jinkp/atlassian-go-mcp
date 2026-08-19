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

// NewTransitionCmd returns the "transition <KEY> --transition-id ..." command.
func NewTransitionCmd(svc jira.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var transitionID string

	cmd := &cobra.Command{
		Use:   "transition <KEY>",
		Short: "Apply a workflow transition to a Jira issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would transition jira issue: key=%s transition-id=%s\n", key, transitionID)
				return nil
			}

			err := svc.TransitionIssue(context.Background(), key, transitionID)
		auditLog.Log(audit.NewEntry("transition_jira_issue", "jira",
			map[string]any{"issue_key": key, "transition_id": transitionID}, err))
		if err != nil {
			exitCode := exitCodeForError(err)
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(exitCode)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Transitioned: %s\n", key)
			return nil
		},
	}

	cmd.Flags().StringVar(&transitionID, "transition-id", "", "Transition ID from `atlassian jira transitions <KEY>` (required)")
	_ = cmd.MarkFlagRequired("transition-id")
	return cmd
}
