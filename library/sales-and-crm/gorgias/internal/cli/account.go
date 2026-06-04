// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newAccountCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "account",
		Short:  "Account-level settings and tenant metadata",
		Hidden: true,
	}

	cmd.AddCommand(newAccountGetCmd(flags))
	cmd.AddCommand(newAccountSettingsCreateCmd(flags))
	cmd.AddCommand(newAccountSettingsListCmd(flags))
	cmd.AddCommand(newAccountSettingsUpdateCmd(flags))
	return cmd
}
