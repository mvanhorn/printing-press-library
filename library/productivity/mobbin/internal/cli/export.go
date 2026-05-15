// Copyright 2026 darin-kishore. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// PATCH: Add a registered export workflow shim for local-store artifacts.
func newExportCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export local Mobbin research artifacts from the SQLite store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return flags.printJSON(cmd, map[string]any{"status": "use deck or grab for file exports", "store": defaultStorePath()})
		},
	}
}
