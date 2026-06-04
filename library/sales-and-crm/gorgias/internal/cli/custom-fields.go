// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newCustomFieldsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "custom-fields",
		Short:  "Define and manage custom fields on tickets and customers",
		Hidden: true,
	}

	cmd.AddCommand(newCustomFieldsCreateCmd(flags))
	cmd.AddCommand(newCustomFieldsGetCmd(flags))
	cmd.AddCommand(newCustomFieldsListCmd(flags))
	cmd.AddCommand(newCustomFieldsUpdateCmd(flags))
	cmd.AddCommand(newCustomFieldsUpdateAllCmd(flags))
	return cmd
}
