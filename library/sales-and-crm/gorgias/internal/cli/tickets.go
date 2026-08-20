// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newTicketsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "tickets",
		Short:  "Read and write Gorgias tickets, messages, and tag assignments",
		Hidden: true,
	}

	cmd.AddCommand(newTicketsCreateCmd(flags))
	cmd.AddCommand(newTicketsCustomFieldsListCmd(flags))
	cmd.AddCommand(newTicketsCustomFieldsSetCmd(flags))
	cmd.AddCommand(newTicketsCustomFieldsSetAllCmd(flags))
	cmd.AddCommand(newTicketsCustomFieldsUnsetCmd(flags))
	cmd.AddCommand(newTicketsDeleteCmd(flags))
	cmd.AddCommand(newTicketsGetCmd(flags))
	cmd.AddCommand(newTicketsListCmd(flags))
	cmd.AddCommand(newTicketsMessagesCreateCmd(flags))
	cmd.AddCommand(newTicketsMessagesDeleteCmd(flags))
	cmd.AddCommand(newTicketsMessagesGetCmd(flags))
	cmd.AddCommand(newTicketsMessagesListCmd(flags))
	cmd.AddCommand(newTicketsMessagesUpdateCmd(flags))
	cmd.AddCommand(newTicketsTagsAddCmd(flags))
	cmd.AddCommand(newTicketsTagsListCmd(flags))
	cmd.AddCommand(newTicketsTagsRemoveCmd(flags))
	cmd.AddCommand(newTicketsTagsReplaceCmd(flags))
	cmd.AddCommand(newTicketsUpdateCmd(flags))
	return cmd
}
