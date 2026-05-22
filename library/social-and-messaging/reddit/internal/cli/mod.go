// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// newModCmd is a compact alias parent for the most-used moderation novel
// features. The spec-derived `moderation` resource handles the full surface;
// `mod` provides the convenience shortcuts moderators reach for daily.
func newModCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mod",
		Short: "Mod-team primitives: queue (age-sorted), reporters, ghost-actions, remove-batch",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newModQueueAgeCmd(flags))
	cmd.AddCommand(newModReportersCmd(flags))
	cmd.AddCommand(newModGhostActionsCmd(flags))
	cmd.AddCommand(newModRemoveBatchCmd(flags))
	return cmd
}
