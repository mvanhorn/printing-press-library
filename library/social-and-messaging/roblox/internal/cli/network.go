// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelNetworkCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "network",
		Short:       "network subcommands: overlap",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelNetworkOverlapCmd(flags))
	return cmd
}
