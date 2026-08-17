// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Household maintenance command group.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelHealthCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "health",
		Short:       "health subcommands: hotspots",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelHealthHotspotsCmd(flags))
	return cmd
}
