// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel-command wiring: registers hand-authored transcendence commands.

package cli

import (
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelTextQueryCmd(flags))
		addNovelCommandIfAbsent(root, newNovelSnapshotCmd(flags))
		addNovelCommandIfAbsent(root, newNovelPruneCmd(flags))
		addNovelCommandIfAbsent(root, newNovelCascadeCmd(flags))
		addNovelCommandIfAbsent(root, newNovelUsageCmd(flags))
		addNovelCommandIfAbsent(root, newNovelCheckVectorsCmd(flags))
		addNovelCommandIfAbsent(root, newNovelCoverageCmd(flags))
	})
}
