package projects

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewSearchCmd returns the "projects search --query ..." command.
func NewSearchCmd(svc projects.ProjectsService) *cobra.Command {
	var (
		query        string
		maxResults   int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Jira projects by name or key",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			result, err := svc.SearchProjects(context.Background(), projects.SearchProjectsRequest{
				Query:      query,
				MaxResults: maxResults,
			})
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

	cmd.Flags().StringVar(&query, "query", "", "Search query (optional — omit to list all)")
	cmd.Flags().IntVar(&maxResults, "max-results", 50, "Maximum number of results to return")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	return cmd
}
