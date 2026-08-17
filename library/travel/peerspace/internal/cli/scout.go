// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelScoutCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "scout",
		Short:       "scout subcommands: budget, capacity, gaps, multi-city",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelScoutBudgetCmd(flags))
	cmd.AddCommand(newNovelScoutCapacityCmd(flags))
	cmd.AddCommand(newNovelScoutGapsCmd(flags))
	cmd.AddCommand(newNovelScoutMultiCityCmd(flags))
	return cmd
}
