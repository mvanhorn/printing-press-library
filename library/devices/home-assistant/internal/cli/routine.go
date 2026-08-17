// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Routine intelligence command group.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelRoutineCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "routine",
		Short:       "routine subcommands: impact, lint, why",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelRoutineImpactCmd(flags))
	cmd.AddCommand(newNovelRoutineLintCmd(flags))
	cmd.AddCommand(newNovelRoutineWhyCmd(flags))
	return cmd
}
