// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Bambu job command family.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelJobCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "job",
		Short:       "job subcommands: eta, repeats, timeline",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelJobEtaCmd(flags))
	cmd.AddCommand(newNovelJobRepeatsCmd(flags))
	cmd.AddCommand(newNovelJobTimelineCmd(flags))
	cmd.AddCommand(newBambuDerivedCmd(flags, "job", "current", "Show the current print job", currentJobView))
	cmd.AddCommand(newBambuMetadataCmd(flags, "objects"))
	cmd.AddCommand(newBambuMetadataCmd(flags, "metadata"))
	cmd.AddCommand(newBambuMetadataCmd(flags, "thumbnail"))
	return cmd
}
