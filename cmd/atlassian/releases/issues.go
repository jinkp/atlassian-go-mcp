package releases

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewIssuesCmd returns the "releases issues <release-id>" command.
func NewIssuesCmd(svc releases.ReleasesService) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "issues <release-id>",
		Short: "Get issue counts for a Jira release",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			counts, err := svc.GetReleaseIssueCounts(context.Background(), args[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(releasesExitCode(err))
			}

			data, err := formatter.Format(counts)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	return cmd
}
