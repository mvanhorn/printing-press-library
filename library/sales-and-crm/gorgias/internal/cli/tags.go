// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newTagsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "tags",
		Short:  "Ticket tags — the labels that drive routing rules and reporting",
		Hidden: true,
	}

	cmd.AddCommand(newTagsCreateCmd(flags))
	cmd.AddCommand(newTagsDeleteCmd(flags))
	cmd.AddCommand(newTagsDeleteAllCmd(flags))
	cmd.AddCommand(newTagsGetCmd(flags))
	cmd.AddCommand(newTagsListCmd(flags))
	cmd.AddCommand(newTagsMergeCmd(flags))
	cmd.AddCommand(newTagsUpdateCmd(flags))
	return cmd
}
