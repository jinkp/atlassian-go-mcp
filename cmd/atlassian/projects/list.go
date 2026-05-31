package projects

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewListCmd returns the "projects list [--max-results 50]" command.
func NewListCmd(svc projects.ProjectsService) *cobra.Command {
	var (
		maxResults   int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Jira projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			result, err := svc.GetProjects(context.Background(), maxResults)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(projectsExitCode(err))
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

	cmd.Flags().IntVar(&maxResults, "max-results", 50, "Maximum number of projects to return")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	return cmd
}
