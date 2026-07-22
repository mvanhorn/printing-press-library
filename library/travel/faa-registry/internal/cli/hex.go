// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelHexCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "hex",
		Short:       "hex subcommands: resolve, to-tail, from-tail",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelHexResolveCmd(flags))
	cmd.AddCommand(newHexToTailCmd(flags))
	cmd.AddCommand(newHexFromTailCmd(flags))
	return cmd
}
