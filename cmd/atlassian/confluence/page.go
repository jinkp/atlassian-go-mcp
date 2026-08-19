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

// NewPageCmd returns the "page" subcommand group.
func NewPageCmd(svc confluence.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "Manage Confluence pages",
	}
	cmd.AddCommand(newPageGetCmd(svc))
	cmd.AddCommand(newPageListCmd(svc))
	cmd.AddCommand(newPageDescendantsCmd(svc))
	cmd.AddCommand(newPageCreateCmd(svc, auditLog, dryRun))
	cmd.AddCommand(newPageUpdateCmd(svc, auditLog, dryRun))
	return cmd
}

// newPageGetCmd returns the "page get <PAGE_ID>" command.
func newPageGetCmd(svc confluence.Service) *cobra.Command {
	var (
		bodyFormat string
		outputFmt  string
	)

	cmd := &cobra.Command{
		Use:   "get <PAGE_ID>",
		Short: "Fetch a Confluence page by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := args[0]

			formatter, err := output.NewFormatter(outputFmt)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			page, err := svc.GetPage(context.Background(), pageID, bodyFormat)
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			data, err := formatter.Format(page)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&bodyFormat, "body-format", "storage", "Body representation format (storage, atlas_doc_format, view)")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "json", "Output format: table, json, yaml")
	return cmd
}

// newPageListCmd returns the "page list <SPACE_ID>" command (GetPagesInSpace).
func newPageListCmd(svc confluence.Service) *cobra.Command {
	var (
		limit     int
		cursor    string
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "list <SPACE_ID>",
		Short: "List pages in a Confluence space",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spaceID := args[0]

			formatter, err := output.NewFormatter(outputFmt)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			result, err := svc.GetPagesInSpace(context.Background(), spaceID, limit, cursor)
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

	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum pages to return (default 25)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous response")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "json", "Output format: table, json, yaml")
	return cmd
}

// newPageDescendantsCmd returns the "page descendants <PAGE_ID>" command.
func newPageDescendantsCmd(svc confluence.Service) *cobra.Command {
	var (
		limit     int
		cursor    string
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "descendants <PAGE_ID>",
		Short: "List descendant pages of a Confluence page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := args[0]

			formatter, err := output.NewFormatter(outputFmt)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			result, err := svc.GetPageDescendants(context.Background(), pageID, limit, cursor)
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

	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum descendants to return (default 25)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous response")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "json", "Output format: table, json, yaml")
	return cmd
}

// newPageCreateCmd returns the "page create" command (write, dry-run aware).
func newPageCreateCmd(svc confluence.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		spaceID  string
		title    string
		body     string
		parentID string
		status   string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Confluence page",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(),
					"[DRY RUN] Would create page %q in space %s\n", title, spaceID)
				return nil
			}

			req := confluence.CreatePageRequest{
				SpaceID:  spaceID,
				Title:    title,
				Body:     body,
				ParentID: parentID,
				Status:   status,
			}

			page, err := svc.CreatePage(context.Background(), req)
			auditLog.Log(audit.NewEntry("create_confluence_page", "confluence",
				map[string]any{"space_id": spaceID, "title": title}, err))
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created page: %s (ID: %s)\n", page.Title, page.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&spaceID, "space-id", "", "Confluence space ID (required)")
	cmd.Flags().StringVar(&title, "title", "", "Page title (required)")
	cmd.Flags().StringVar(&body, "body", "", "Page body in storage XHTML format (required)")
	cmd.Flags().StringVar(&parentID, "parent-id", "", "Parent page ID (optional)")
	cmd.Flags().StringVar(&status, "status", "current", "Page status: current or draft")
	_ = cmd.MarkFlagRequired("space-id")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

// newPageUpdateCmd returns the "page update <PAGE_ID>" command (write, dry-run aware).
func newPageUpdateCmd(svc confluence.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		title   string
		body    string
		status  string
		version int
	)

	cmd := &cobra.Command{
		Use:   "update <PAGE_ID>",
		Short: "Update an existing Confluence page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := args[0]

			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(),
					"[DRY RUN] Would update page %s with title %q\n", pageID, title)
				return nil
			}

			req := confluence.UpdatePageRequest{
				PageID: pageID,
				Title:  title,
				Body:   body,
				Status: status,
			}
			if cmd.Flags().Changed("version") {
				req.VersionNumber = &version
			}

			page, err := svc.UpdatePage(context.Background(), req)
			auditLog.Log(audit.NewEntry("update_confluence_page", "confluence",
				map[string]any{"page_id": pageID, "title": title}, err))
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated page: %s (ID: %s)\n", page.Title, page.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New page title (required)")
	cmd.Flags().StringVar(&body, "body", "", "New page body in storage XHTML format (required)")
	cmd.Flags().StringVar(&status, "status", "current", "Page status: current or draft")
	cmd.Flags().IntVar(&version, "version", 0, "Explicit version number (omit to auto-fetch and increment)")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

// exitCodeForError maps known confluence sentinel errors to POSIX-style exit codes.
// 0 = success, 1 = user error, 2 = auth/API error, 3 = not found.
func exitCodeForError(err error) int {
	if err == nil {
		return 0
	}
	switch {
	case isErr(err, confluence.ErrNotFound):
		return 3
	case isErr(err, confluence.ErrUnauthorized):
		return 2
	case isErr(err, confluence.ErrConflict):
		return 2
	case isErr(err, confluence.ErrRateLimit):
		return 2
	default:
		return 2
	}
}

// isErr reports whether target matches any error in err's chain.
func isErr(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
		} else {
			break
		}
	}
	return false
}
