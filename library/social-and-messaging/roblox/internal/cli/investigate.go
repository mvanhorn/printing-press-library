// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelInvestigateCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "investigate",
		Short:       "investigate subcommands: group, user",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelInvestigateGroupCmd(flags))
	cmd.AddCommand(newNovelInvestigateUserCmd(flags))
	return cmd
}
