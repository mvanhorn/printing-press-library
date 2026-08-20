// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newPhoneCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "phone",
		Short:  "Voice calls, call events, and recorded audio",
		Hidden: true,
	}

	cmd.AddCommand(newPhoneCallEventsGetCmd(flags))
	cmd.AddCommand(newPhoneCallEventsListCmd(flags))
	cmd.AddCommand(newPhoneCallRecordingsDeleteCmd(flags))
	cmd.AddCommand(newPhoneCallRecordingsGetCmd(flags))
	cmd.AddCommand(newPhoneCallRecordingsListCmd(flags))
	cmd.AddCommand(newPhoneCallsGetCmd(flags))
	cmd.AddCommand(newPhoneCallsListCmd(flags))
	return cmd
}
