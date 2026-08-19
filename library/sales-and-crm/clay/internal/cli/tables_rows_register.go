// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Registers the hand-authored `tables rows` command under the generated parent.

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		tablesCmd, _, err := root.Find([]string{"tables"})
		if err == nil && tablesCmd != nil {
			addNovelCommandIfAbsent(tablesCmd, newNovelTablesRowsCmd(flags))
		}
	})
}
