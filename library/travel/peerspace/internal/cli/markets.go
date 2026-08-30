// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelMarketsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "markets",
		Short:       "markets subcommands: neighborhoods, pulse",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelMarketsNeighborhoodsCmd(flags))
	cmd.AddCommand(newNovelMarketsPulseCmd(flags))
	return cmd
}
