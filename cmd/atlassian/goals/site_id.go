package goals

import (
	"context"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/spf13/cobra"
)

// NewSiteIDCmd returns the "site-id --subdomain ..." command.
// It resolves a subdomain to an Atlassian cloudId (site ID).
func NewSiteIDCmd(svc goals.GoalsService) *cobra.Command {
	var subdomain string

	cmd := &cobra.Command{
		Use:   "site-id",
		Short: "Resolve an Atlassian subdomain to a cloud site ID",
		Long: `Resolves a subdomain (e.g. "myorg") to the Atlassian cloudId required by other goals commands.
Example: atlassian goals site-id --subdomain myorg`,
		RunE: func(cmd *cobra.Command, args []string) error {
			siteID, err := svc.GetSiteID(context.Background(), subdomain)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(goalsExitCode(err))
			}

			fmt.Fprintln(cmd.OutOrStdout(), siteID)
			return nil
		},
	}

	cmd.Flags().StringVar(&subdomain, "subdomain", "", "Atlassian subdomain e.g. myorg (required)")
	_ = cmd.MarkFlagRequired("subdomain")
	return cmd
}
