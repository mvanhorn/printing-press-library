// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCreatorCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "creator",
		Short:       "creator subcommands: compare, find, track",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelCreatorCompareCmd(flags))
	cmd.AddCommand(newNovelCreatorFindCmd(flags))
	cmd.AddCommand(newNovelCreatorTrackCmd(flags))
	return cmd
}
