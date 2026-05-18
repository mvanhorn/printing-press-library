// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// PATCH: hand-written local price watchlist command group.
func newWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Track price changes for a set of Numista types over time.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchlistAddCmd(flags))
	cmd.AddCommand(newWatchlistListCmd(flags))
	cmd.AddCommand(newWatchlistRemoveCmd(flags))
	cmd.AddCommand(newWatchlistCheckCmd(flags))
	return cmd
}
