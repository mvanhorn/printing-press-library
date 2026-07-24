// Copyright 2026 Arun Gopalakrishnan and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelAuthCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "auth",
		Short:       "auth subcommands: check",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelAuthCheckCmd(flags))
	return cmd
}
