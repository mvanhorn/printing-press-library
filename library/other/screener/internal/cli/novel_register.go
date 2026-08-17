// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command registration hooks for the Screener.in CLI.
// generate --force preserves implemented bodies.

package cli

import (
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelCompareCmd(flags))
		addNovelCommandIfAbsent(root, newNovelQtrendCmd(flags))
		addNovelCommandIfAbsent(root, newNovelOverlapCmd(flags))
		addNovelCommandIfAbsent(root, newNovelRankCmd(flags))
		addNovelCommandIfAbsent(root, newNovelInsiderFlowCmd(flags))
	})
}
