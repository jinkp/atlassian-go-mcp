package confluence

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewSpacesCmd returns the "spaces" command that lists Confluence spaces.
func NewSpacesCmd(svc confluence.Service) *cobra.Command {
	var (
		limit     int
		cursor    string
		keys      string
		spaceType string
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "spaces",
		Short: "List Confluence spaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter, err := output.NewFormatter(outputFmt)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			var keyList []string
			if keys != "" {
				for _, k := range strings.Split(keys, ",") {
					k = strings.TrimSpace(k)
					if k != "" {
						keyList = append(keyList, k)
					}
				}
			}

			result, err := svc.GetSpaces(context.Background(), limit, cursor, keyList, spaceType)
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

	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum spaces to return (default 25)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous response")
	cmd.Flags().StringVar(&keys, "keys", "", "Comma-separated list of space keys to filter by")
	cmd.Flags().StringVar(&spaceType, "type", "", "Space type filter: global or personal")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "json", "Output format: table, json, yaml")
	return cmd
}
