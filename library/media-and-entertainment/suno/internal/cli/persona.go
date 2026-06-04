// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newPersonaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "persona",
		Short:  "Voice personas",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newPersonaGetCmd(flags))
	cmd.AddCommand(newPersonaUsageCmd(flags))
	return cmd
}
