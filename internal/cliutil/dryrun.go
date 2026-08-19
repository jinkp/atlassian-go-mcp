// Package cliutil holds small shared helpers for the atlassian CLI commands.
package cliutil

import "github.com/spf13/cobra"

// ResolveDryRun returns true if dry-run is active, reading the LIVE value of the
// persistent --dry-run flag at execution time and OR-ing it with the value
// captured at command construction. This corrects a wiring bug where the flag
// value was captured before cobra parsed flags. `captured` keeps unit tests
// that inject dryRun directly working.
func ResolveDryRun(cmd *cobra.Command, captured bool) bool {
	if captured {
		return true
	}
	if f := cmd.Flags().Lookup("dry-run"); f != nil {
		if v, err := cmd.Flags().GetBool("dry-run"); err == nil {
			return v
		}
	}
	// Fall back to the root's persistent flags in case the inherited flag
	// is not merged into cmd.Flags() for this invocation.
	if root := cmd.Root(); root != nil {
		if v, err := root.PersistentFlags().GetBool("dry-run"); err == nil {
			return v
		}
	}
	return false
}
