// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Household mode command group.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelModeCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "mode",
		Short:       "mode subcommands: run",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelModeRunCmd(flags))
	return cmd
}
