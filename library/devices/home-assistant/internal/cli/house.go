// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Household workflow command group.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelHouseCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "house",
		Short:       "house subcommands: check, recap",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelHouseCheckCmd(flags))
	cmd.AddCommand(newNovelHouseRecapCmd(flags))
	return cmd
}
