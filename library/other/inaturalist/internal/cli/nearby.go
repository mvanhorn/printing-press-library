// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Nearby commands provide privacy-safe field discovery workflows.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelNearbyCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "nearby",
		Short:       "nearby subcommands: highlights, seasonal-shift",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelNearbyHighlightsCmd(flags))
	cmd.AddCommand(newNovelNearbySeasonalShiftCmd(flags))
	return cmd
}
