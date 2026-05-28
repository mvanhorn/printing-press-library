// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

func newContactCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "contact <subcommand> [args...]",
		Hidden: true,
		Short:  "Cross-entity views centered on one contact (use 'contacts' for resource CRUD)",
		RunE:   parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newContact360Cmd(flags))
	return cmd
}
