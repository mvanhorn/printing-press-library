// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Energy analysis command group.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelEnergyCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "energy",
		Short:       "energy subcommands: standby",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelEnergyStandbyCmd(flags))
	return cmd
}
