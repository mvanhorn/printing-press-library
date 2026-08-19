// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Registers the hand-authored formulas command family.

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelFormulasCmd(flags))
	})
}
