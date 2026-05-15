// Copyright 2026 darin-kishore. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// PATCH: Add a registered analytics workflow shim for local Mobbin benchmarks.
func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "analytics",
		Short: "Summarize local Mobbin benchmark and audit data.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return flags.printJSON(cmd, map[string]any{"status": "use bench, cross, and audit for analytics", "store": defaultStorePath()})
		},
	}
}
