package cli

import (
	"github.com/spf13/cobra"
)

func newInboxCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Inbox health analytics — unread count, reply rate, thread depth per mailbox",
	}

	cmd.AddCommand(newInboxHealthCmd(flags))
	return cmd
}
