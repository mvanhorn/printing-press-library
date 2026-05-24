// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newFormulaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "formula",
		Short: "Airtable formula tools — test and validate",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newFormulaTestCmd(flags))
	return cmd
}
