package projects

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewGetCmd returns the "projects get <KEY>" command.
func NewGetCmd(svc projects.ProjectsService) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "get <project-key>",
		Short: "Get a Jira project by key or ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			project, err := svc.GetProject(context.Background(), args[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(projectsExitCode(err))
			}

			data, err := formatter.Format(project)
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
