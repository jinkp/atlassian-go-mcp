package jira

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewCommentCmd returns the "comment" subcommand group with "add" and "list" subcommands.
func NewCommentCmd(svc jira.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Manage comments on Jira issues",
	}
	cmd.AddCommand(newCommentAddCmd(svc, auditLog, dryRun))
	cmd.AddCommand(newCommentListCmd(svc))
	return cmd
}

// newCommentAddCmd returns the "comment add <KEY> <body...>" command.
// Body is accepted as one or more positional args after KEY and joined with spaces.
func newCommentAddCmd(svc jira.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <KEY> <body>",
		Short: "Add a comment to a Jira issue",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			body := strings.Join(args[1:], " ")

			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would add comment to %s: %q\n", key, body)
				return nil
			}

			comment, err := svc.AddComment(context.Background(), key, body)
			auditLog.Log(audit.NewEntry("add_comment_to_issue", "jira",
				map[string]any{"issue_key": key}, err))
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Comment added: %s\n", comment.ID)
			return nil
		},
	}
	return cmd
}

// newCommentListCmd returns the "comment list <KEY>" command.
func newCommentListCmd(svc jira.Service) *cobra.Command {
	var (
		maxResults int
		outputFmt  string
	)

	cmd := &cobra.Command{
		Use:   "list <KEY>",
		Short: "List comments on a Jira issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			formatter, err := output.NewFormatter(outputFmt)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			comments, err := svc.GetComments(context.Background(), key, maxResults)
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			data, err := formatter.Format(comments)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().IntVar(&maxResults, "max-results", 0, "Maximum comments to return (default 50)")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "json", "Output format: table, json, yaml")
	return cmd
}
