// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelEnforcementCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "enforcement",
		Short:       "enforcement subcommands: timeline",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelEnforcementTimelineCmd(flags))
	return cmd
}
