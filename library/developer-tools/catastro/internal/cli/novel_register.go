// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-written: registers all novel features onto the root command tree.

package cli

import "github.com/spf13/cobra"

// registerNovelCommands wires every hand-written novel feature onto rootCmd.
// Called from root.go's newRootCmd after the generated commands are added.
func registerNovelCommands(rootCmd *cobra.Command, flags *rootFlags) {
	rootCmd.AddCommand(newGeocodeCmd(flags))
	rootCmd.AddCommand(newReconcileCmd(flags))
	rootCmd.AddCommand(newEnrichCmd(flags))
	rootCmd.AddCommand(newNeighborsCmd(flags))
	rootCmd.AddCommand(newReportCmd(flags))
	rootCmd.AddCommand(newStaleCmd(flags))
	rootCmd.AddCommand(newCoverageCmd(flags))
	rootCmd.AddCommand(newAnalyzeAreaCmd(flags))
	rootCmd.AddCommand(newWatchCmd(flags))
	rootCmd.AddCommand(newExportCmd(flags))
	rootCmd.AddCommand(newWMSCmd(flags))
	rootCmd.AddCommand(newSRSCmd(flags))
	rootCmd.AddCommand(newCacheCmd(flags))
}
