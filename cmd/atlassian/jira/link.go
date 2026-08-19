package jira

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/cliutil"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewLinkCmd returns the "link <INWARD> <OUTWARD> --type <name>" command.
func NewLinkCmd(svc jira.Service, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var linkType string

	cmd := &cobra.Command{
		Use:   "link <INWARD> <OUTWARD>",
		Short: "Create a directed link between two Jira issues",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			inward := args[0]
			outward := args[1]

			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would link %s → %s (type: %s)\n", inward, outward, linkType)
				return nil
			}

			err := svc.LinkIssues(context.Background(), inward, outward, linkType)
			auditLog.Log(audit.NewEntry("link_issues", "jira",
				map[string]any{"inward": inward, "outward": outward, "type": linkType}, err))
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Linked: %s → %s (%s)\n", inward, outward, linkType)
			return nil
		},
	}

	cmd.Flags().StringVar(&linkType, "type", "", "Link type name e.g. Blocks, Clones (required)")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

// NewLinkTypesCmd returns the "link-types" command that lists available issue link types.
func NewLinkTypesCmd(svc jira.Service) *cobra.Command {
	var outputFmt string

	cmd := &cobra.Command{
		Use:   "link-types",
		Short: "List available issue link types",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFmt)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			linkTypes, err := svc.GetIssueLinkTypes(context.Background())
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			data, err := formatter.Format(linkTypes)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFmt, "output", "o", "json", "Output format: table, json, yaml")
	return cmd
}
