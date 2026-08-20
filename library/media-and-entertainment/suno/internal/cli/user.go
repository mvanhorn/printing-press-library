// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newUserCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "user",
		Short:  "Your account, config, and personalization",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newUserConfigCmd(flags))
	cmd.AddCommand(newUserPersonalizationCmd(flags))
	cmd.AddCommand(newUserPersonalizationMemoryCmd(flags))
	return cmd
}
