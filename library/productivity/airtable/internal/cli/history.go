// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newHistoryCmd is the parent for hand-built history subcommands.
func newHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Reconstruct historical change timelines from the local mirror",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newHistoryRecordCmd(flags))
	return cmd
}
