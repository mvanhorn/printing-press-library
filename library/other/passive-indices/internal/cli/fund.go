// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelFundCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "fund",
		Short:       "ETFs and index funds — detail, search, screening, and comparison (indiapassivefunds.com)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelFundNfoCmd(flags))
	cmd.AddCommand(newNovelFundRawCmd(flags))
	cmd.AddCommand(newFundGetCmd(flags))
	cmd.AddCommand(newFundSearchCmd(flags))
	cmd.AddCommand(newFundScreenCmd(flags))
	cmd.AddCommand(newFundTimeseriesCmd(flags))
	cmd.AddCommand(newFundCompareListCmd(flags))
	return cmd
}
