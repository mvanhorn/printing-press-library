// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelFormatsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "formats",
		Short:       "formats subcommands: gaps",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelFormatsGapsCmd(flags))
	return cmd
}
