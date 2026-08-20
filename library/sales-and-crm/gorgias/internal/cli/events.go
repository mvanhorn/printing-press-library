// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newEventsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "events",
		Short:  "Audit log of who-changed-what across tickets, customers, settings",
		Hidden: true,
	}

	cmd.AddCommand(newEventsGetCmd(flags))
	cmd.AddCommand(newEventsListCmd(flags))
	return cmd
}
