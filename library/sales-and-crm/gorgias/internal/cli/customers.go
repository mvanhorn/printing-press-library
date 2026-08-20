// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newCustomersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "customers",
		Short:  "Read and write Gorgias customer records (CRM core)",
		Hidden: true,
	}

	cmd.AddCommand(newCustomersCreateCmd(flags))
	cmd.AddCommand(newCustomersCustomFieldsListCmd(flags))
	cmd.AddCommand(newCustomersCustomFieldsSetCmd(flags))
	cmd.AddCommand(newCustomersCustomFieldsSetAllCmd(flags))
	cmd.AddCommand(newCustomersCustomFieldsUnsetCmd(flags))
	cmd.AddCommand(newCustomersDataUpdateCmd(flags))
	cmd.AddCommand(newCustomersDeleteCmd(flags))
	cmd.AddCommand(newCustomersDeleteAllCmd(flags))
	cmd.AddCommand(newCustomersGetCmd(flags))
	cmd.AddCommand(newCustomersListCmd(flags))
	cmd.AddCommand(newCustomersMergeCmd(flags))
	cmd.AddCommand(newCustomersUpdateCmd(flags))
	return cmd
}
