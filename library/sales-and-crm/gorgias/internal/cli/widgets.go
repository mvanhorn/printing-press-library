// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newWidgetsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "widgets",
		Short:  "Configure on-site chat/contact widget instances",
		Hidden: true,
	}

	cmd.AddCommand(newWidgetsCreateCmd(flags))
	cmd.AddCommand(newWidgetsDeleteCmd(flags))
	cmd.AddCommand(newWidgetsGetCmd(flags))
	cmd.AddCommand(newWidgetsListCmd(flags))
	cmd.AddCommand(newWidgetsUpdateCmd(flags))
	return cmd
}
