// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelComplaintsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "complaints",
		Short:       "complaints subcommands: new, list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelComplaintsNewCmd(flags))
	cmd.AddCommand(newNovelComplaintsListCmd(flags))
	return cmd
}
