// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Registers the hand-authored sync command. Kept in its own file so
// regeneration preserves it.

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newCrestronSyncCmd(flags))
	})
}
