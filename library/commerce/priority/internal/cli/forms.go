// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelFormsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "forms",
		Short:       "Tenant schema intelligence: list, describe, search, diff, licensed, refresh",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelFormsDiffCmd(flags))
	cmd.AddCommand(newFormsListCmd(flags))
	cmd.AddCommand(newFormsDescribeCmd(flags))
	cmd.AddCommand(newFormsRefreshCmd(flags))
	cmd.AddCommand(newNovelFormsLicensedCmd(flags))
	cmd.AddCommand(newNovelFormsSearchCmd(flags))
	return cmd
}
