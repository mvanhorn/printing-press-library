// Copyright 2026 darin-kishore. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// PATCH: Add a registered tail workflow shim for recent local-store activity.
func newTailCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "tail",
		Short: "Show recent local Mobbin sync activity.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return flags.printJSON(cmd, map[string]any{"status": "recent activity is available through sync and audit", "store": defaultStorePath()})
		},
	}
}
