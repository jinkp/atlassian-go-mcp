package confluence

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/cliutil"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewCommentCmd returns the "comment" subcommand group.
func NewCommentCmd(svc confluence.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Manage Confluence comments",
	}
	cmd.AddCommand(newCommentFooterCmd(svc))
	cmd.AddCommand(newCommentInlineCmd(svc))
	cmd.AddCommand(newCommentChildrenCmd(svc))
	cmd.AddCommand(newCommentAddFooterCmd(svc, auditLog, dryRun))
	cmd.AddCommand(newCommentAddInlineCmd(svc, auditLog, dryRun))
	return cmd
}

// newCommentFooterCmd returns the "comment footer <PAGE_ID>" command.
func newCommentFooterCmd(svc confluence.Service) *cobra.Command {
	var (
		limit     int
		cursor    string
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "footer <PAGE_ID>",
		Short: "List footer comments on a Confluence page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := args[0]

			formatter, err := output.NewFormatter(outputFmt)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			result, err := svc.GetFooterComments(context.Background(), pageID, limit, cursor)
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			data, err := formatter.Format(result)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum comments to return (default 25)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous response")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "json", "Output format: table, json, yaml")
	return cmd
}

// newCommentInlineCmd returns the "comment inline <PAGE_ID>" command.
func newCommentInlineCmd(svc confluence.Service) *cobra.Command {
	var (
		limit     int
		cursor    string
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "inline <PAGE_ID>",
		Short: "List inline comments on a Confluence page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := args[0]

			formatter, err := output.NewFormatter(outputFmt)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			result, err := svc.GetInlineComments(context.Background(), pageID, limit, cursor)
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			data, err := formatter.Format(result)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum comments to return (default 25)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous response")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "json", "Output format: table, json, yaml")
	return cmd
}

// newCommentChildrenCmd returns the "comment children <COMMENT_ID>" command.
func newCommentChildrenCmd(svc confluence.Service) *cobra.Command {
	var (
		limit     int
		cursor    string
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "children <COMMENT_ID>",
		Short: "List child (reply) comments of a Confluence comment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commentID := args[0]

			formatter, err := output.NewFormatter(outputFmt)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			result, err := svc.GetCommentChildren(context.Background(), commentID, limit, cursor)
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			data, err := formatter.Format(result)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum replies to return (default 25)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous response")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "json", "Output format: table, json, yaml")
	return cmd
}

// newCommentAddFooterCmd returns the "comment add-footer <PAGE_ID>" command (write, dry-run aware).
func newCommentAddFooterCmd(svc confluence.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		body     string
		parentID string
	)

	cmd := &cobra.Command{
		Use:   "add-footer <PAGE_ID>",
		Short: "Add a footer comment to a Confluence page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := args[0]

			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(),
					"[DRY RUN] Would add footer comment to page %s\n", pageID)
				return nil
			}

			req := confluence.CreateCommentRequest{
				PageID:          pageID,
				Body:            body,
				ParentCommentID: parentID,
			}

			comment, err := svc.CreateFooterComment(context.Background(), req)
			auditLog.Log(audit.NewEntry("create_confluence_footer_comment", "confluence",
				map[string]any{"page_id": pageID}, err))
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Footer comment added: %s\n", comment.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&body, "body", "", "Comment body in storage XHTML format (required)")
	cmd.Flags().StringVar(&parentID, "parent", "", "Parent comment ID for threaded replies (optional)")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

// newCommentAddInlineCmd returns the "comment add-inline <PAGE_ID>" command (write, dry-run aware).
// --text-selection is required.
func newCommentAddInlineCmd(svc confluence.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		body          string
		textSelection string
		matchCount    int
		matchIndex    int
	)

	cmd := &cobra.Command{
		Use:   "add-inline <PAGE_ID>",
		Short: "Add an inline comment anchored to a text selection on a Confluence page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := args[0]

			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(),
					"[DRY RUN] Would add inline comment to page %s anchored on %q\n", pageID, textSelection)
				return nil
			}

			req := confluence.CreateInlineCommentRequest{
				PageID:                  pageID,
				Body:                    body,
				TextSelection:           textSelection,
				TextSelectionMatchCount: matchCount,
				TextSelectionMatchIndex: matchIndex,
			}

			comment, err := svc.CreateInlineComment(context.Background(), req)
			auditLog.Log(audit.NewEntry("create_confluence_inline_comment", "confluence",
				map[string]any{"page_id": pageID, "text_selection": textSelection}, err))
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Inline comment added: %s\n", comment.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&body, "body", "", "Comment body in storage XHTML format (required)")
	cmd.Flags().StringVar(&textSelection, "text-selection", "", "Text to anchor the inline comment to (required)")
	cmd.Flags().IntVar(&matchCount, "match-count", 0, "Total occurrences of text-selection on the page (optional)")
	cmd.Flags().IntVar(&matchIndex, "match-index", 0, "Zero-based index of the occurrence to anchor to (optional)")
	_ = cmd.MarkFlagRequired("body")
	_ = cmd.MarkFlagRequired("text-selection")
	return cmd
}
