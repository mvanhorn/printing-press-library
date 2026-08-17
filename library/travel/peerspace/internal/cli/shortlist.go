// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelShortlistCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "shortlist",
		Short: "shortlist subcommands: add, compare, create-board, delta, drift, export, hydrate, similar",
		// Parent stays readable for discovery; write children mark mcp:read-only=false.
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelShortlistAddCmd(flags))
	cmd.AddCommand(newNovelShortlistCompareCmd(flags))
	cmd.AddCommand(newNovelShortlistCreateBoardCmd(flags))
	cmd.AddCommand(newNovelShortlistDeltaCmd(flags))
	cmd.AddCommand(newNovelShortlistDriftCmd(flags))
	cmd.AddCommand(newNovelShortlistExportCmd(flags))
	cmd.AddCommand(newNovelShortlistHydrateCmd(flags))
	cmd.AddCommand(newNovelShortlistSimilarCmd(flags))
	return cmd
}
