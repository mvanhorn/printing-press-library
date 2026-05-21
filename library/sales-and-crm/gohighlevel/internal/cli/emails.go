// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-05-20: add email command group with list and stats subcommands)

package cli

import (
	"github.com/spf13/cobra"
)

func newEmailsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "email",
		Short: "Manage email campaigns",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newEmailsListCmd(flags))
	cmd.AddCommand(newEmailsStatsCmd(flags))
	return cmd
}
