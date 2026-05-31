package releases

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewListCmd returns the "releases list --project ..." command.
func NewListCmd(svc releases.ReleasesService) *cobra.Command {
	var (
		project      string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Jira releases for a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			result, err := svc.GetReleases(context.Background(), project)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(releasesExitCode(err))
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

	cmd.Flags().StringVar(&project, "project", "", "Project key (required)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}
