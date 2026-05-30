package jira

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/spf13/cobra"
)

// NewTransitionCmd returns the "transition <KEY> --transition-id ..." command.
func NewTransitionCmd(svc jira.Service) *cobra.Command {
	var transitionID string

	cmd := &cobra.Command{
		Use:   "transition <KEY>",
		Short: "Apply a workflow transition to a Jira issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			err := svc.TransitionIssue(context.Background(), key, transitionID)
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
