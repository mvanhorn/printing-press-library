// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCustomerCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "customer",
		Short:       "customer subcommands: summary",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelCustomerSummaryCmd(flags))
	return cmd
}
