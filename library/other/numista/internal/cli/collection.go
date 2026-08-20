// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// PATCH: hand-written user-collection analysis parent command.
func newCollectionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collection",
		Short: "Compute totals and analyses over a Numista user's collection.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCollectionValueCmd(flags))
	return cmd
}
