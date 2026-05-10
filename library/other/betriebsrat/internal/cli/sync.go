// Copyright 2026 dawid-piaskowski. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "No-op: all BetrVG knowledge is embedded",
		Long: `All BetrVG knowledge is embedded in this binary. No sync required.

Source: gesetze-im-internet.de (Bundesministerium der Justiz)
Run 'betriebsrat doctor' to verify setup.`,
		Example: "  betriebsrat sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "All BetrVG knowledge is embedded. No sync required.")
			fmt.Fprintln(cmd.OutOrStdout(), "Source: gesetze-im-internet.de (Bundesministerium der Justiz)")
			fmt.Fprintln(cmd.OutOrStdout(), "Run 'betriebsrat doctor' to verify setup.")
			return nil
		},
	}

	return cmd
}
