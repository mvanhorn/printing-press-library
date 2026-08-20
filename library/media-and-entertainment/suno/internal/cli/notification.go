// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNotificationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "notification",
		Short:  "Your notifications",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newNotificationListCmd(flags))
	cmd.AddCommand(newNotificationBadgeCountCmd(flags))
	return cmd
}
