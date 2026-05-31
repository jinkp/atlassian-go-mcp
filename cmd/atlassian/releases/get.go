package releases

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewGetCmd returns the "releases get <release-id>" command.
func NewGetCmd(svc releases.ReleasesService) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "get <release-id>",
		Short: "Get a Jira release by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			release, err := svc.GetRelease(context.Background(), args[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(releasesExitCode(err))
			}

			data, err := formatter.Format(release)
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
