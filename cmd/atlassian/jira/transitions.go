package jira

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewTransitionsCmd returns the "transitions <KEY>" command.
// It lists all available workflow transitions for a Jira issue.
func NewTransitionsCmd(svc jira.Service) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "transitions <KEY>",
		Short: "List available workflow transitions for a Jira issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			transitions, err := svc.GetTransitions(context.Background(), key)
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			data, err := formatter.Format(transitions)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "json", "Output format: table, json, yaml")
	return cmd
}
