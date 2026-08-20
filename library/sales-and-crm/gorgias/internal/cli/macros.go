// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newMacrosCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "macros",
		Short:  "Reusable canned-reply templates with variables and actions",
		Hidden: true,
	}

	cmd.AddCommand(newMacrosArchiveCmd(flags))
	cmd.AddCommand(newMacrosCreateCmd(flags))
	cmd.AddCommand(newMacrosDeleteCmd(flags))
	cmd.AddCommand(newMacrosGetCmd(flags))
	cmd.AddCommand(newMacrosListCmd(flags))
	cmd.AddCommand(newMacrosUnarchiveCmd(flags))
	cmd.AddCommand(newMacrosUpdateCmd(flags))
	return cmd
}
