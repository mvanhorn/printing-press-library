// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// Root wiring for every hand-built command group. Each group is attached with
// an explicit rootCmd.AddCommand guarded by hasRealCommand, so a group the
// generated root already attached is never added twice, and the static
// command-tree checks can see the full path of every novel command.
func init() {
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		if !hasRealCommand(rootCmd, "mirror") {
			rootCmd.AddCommand(newMirrorCmd(flags))
		}
		if !hasRealCommand(rootCmd, "budgets-remote") {
			rootCmd.AddCommand(newBudgetsRemoteCmd(flags))
		}
		if !hasRealCommand(rootCmd, "report") {
			rootCmd.AddCommand(newReportCmd(flags))
		}
		if !hasRealCommand(rootCmd, "import-csv") {
			rootCmd.AddCommand(newImportCSVCmd(flags))
		}
		if !hasRealCommand(rootCmd, "ledger") {
			rootCmd.AddCommand(newNovelLedgerCmd(flags))
		}
		if !hasRealCommand(rootCmd, "audit") {
			rootCmd.AddCommand(newNovelAuditCmd(flags))
		}
		if !hasRealCommand(rootCmd, "categorize") {
			rootCmd.AddCommand(newNovelCategorizeCmd(flags))
		}
		if !hasRealCommand(rootCmd, "templates") {
			rootCmd.AddCommand(newNovelTemplatesCmd(flags))
		}
		if !hasRealCommand(rootCmd, "changes") {
			rootCmd.AddCommand(newNovelChangesCmd(flags))
		}
	})
}

// hasRealCommand reports whether parent already has a non-scaffold child
// named name. A leftover TODO scaffold with that name is removed so the real
// command replaces it.
func hasRealCommand(parent *cobra.Command, name string) bool {
	for _, existing := range parent.Commands() {
		if existing.Name() != name {
			continue
		}
		if !isNovelScaffoldCommand(existing) {
			return true
		}
		parent.RemoveCommand(existing)
	}
	return false
}
