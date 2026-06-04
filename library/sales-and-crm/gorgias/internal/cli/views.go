// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newViewsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "views",
		Short:  "Saved Gorgias inbox views (named filters used by agents)",
		Hidden: true,
	}

	cmd.AddCommand(newViewsCreateCmd(flags))
	cmd.AddCommand(newViewsDeleteCmd(flags))
	cmd.AddCommand(newViewsGetCmd(flags))
	cmd.AddCommand(newViewsItemsListCmd(flags))
	cmd.AddCommand(newViewsItemsUpdateCmd(flags))
	cmd.AddCommand(newViewsListCmd(flags))
	cmd.AddCommand(newViewsUpdateCmd(flags))
	return cmd
}
