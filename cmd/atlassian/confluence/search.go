package confluence

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewSearchCmd returns the "search <CQL>" command.
func NewSearchCmd(svc confluence.Service) *cobra.Command {
	var (
		limit     int
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "search <CQL>",
		Short: "Search Confluence content using CQL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cql := args[0]

			formatter, err := output.NewFormatter(outputFmt)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			results, err := svc.SearchContent(context.Background(), cql, limit)
			if err != nil {
				exitCode := exitCodeForError(err)
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(exitCode)
			}

			data, err := formatter.Format(results)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum results to return (default 25)")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "json", "Output format: table, json, yaml")
	return cmd
}
