// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelResearchCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "research",
		Short:       "research subcommands: competitors, drift, listing, niche, subniches, tags",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelResearchCompetitorsCmd(flags))
	cmd.AddCommand(newNovelResearchDriftCmd(flags))
	cmd.AddCommand(newNovelResearchListingCmd(flags))
	cmd.AddCommand(newNovelResearchNicheCmd(flags))
	cmd.AddCommand(newNovelResearchSubnichesCmd(flags))
	cmd.AddCommand(newNovelResearchTagsCmd(flags))
	return cmd
}
