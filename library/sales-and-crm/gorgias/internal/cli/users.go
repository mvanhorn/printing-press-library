// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newUsersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "users",
		Short:  "Agents and admin users on the Gorgias account",
		Hidden: true,
	}

	cmd.AddCommand(newUsersCreateCmd(flags))
	cmd.AddCommand(newUsersDeleteCmd(flags))
	cmd.AddCommand(newUsersGetCmd(flags))
	cmd.AddCommand(newUsersListCmd(flags))
	cmd.AddCommand(newUsersUpdateCmd(flags))
	return cmd
}
