package goals

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/internal/output"
	"github.com/spf13/cobra"
)

// NewGetCmd returns the "get <goal-id-or-key>" command.
func NewGetCmd(svc goals.GoalsService) *cobra.Command {
	var (
		outputFormat string
		siteID       string
	)

	cmd := &cobra.Command{
		Use:   "get <goal-id-or-key>",
		Short: "Fetch an Atlassian Goal by ARI or by key",
		Long: `Fetch an Atlassian Goal.

The argument is either a full ARI, e.g.
"ari:cloud:townsquare:abc:goal/xyz", or the short key the Goals UI shows, e.g.
"ZQEAYBZU-633". Anything that does not start with "ari:" is treated as a key.

A key is only unique within a site, so --site-id is required when passing one.

  atlassian goals get "ari:cloud:townsquare:abc:goal/xyz"
  atlassian goals get ZQEAYBZU-633 --site-id <cloudId>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := strings.TrimSpace(args[0])
			byKey := !strings.HasPrefix(identifier, "ari:")

			// Fail on the missing site before calling out, and say which of the
			// two forms was detected — otherwise a mistyped ARI silently turns
			// into a key lookup and reports a confusing "not found".
			if byKey && strings.TrimSpace(siteID) == "" {
				fmt.Fprintf(os.Stderr,
					"error: %q is not an ARI, so it is treated as a goal key, and --site-id is required to resolve a key\n",
					identifier,
				)
				os.Exit(1)
			}

			formatter, err := output.NewFormatter(outputFormat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			var goal *goals.Goal
			if byKey {
				goal, err = svc.GetGoalByKey(context.Background(), siteID, identifier)
			} else {
				goal, err = svc.GetGoal(context.Background(), identifier)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(goalsExitCode(err))
			}

			data, err := formatter.Format(goal)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&siteID, "site-id", "", "Atlassian cloud site ID (required when the argument is a key, not an ARI)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, yaml")
	return cmd
}
