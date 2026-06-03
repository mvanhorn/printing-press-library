// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newTeamsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "teams",
		Short:  "Agent teams: how tickets are grouped and routed for assignment",
		Hidden: true,
	}

	cmd.AddCommand(newTeamsCreateCmd(flags))
	cmd.AddCommand(newTeamsDeleteCmd(flags))
	cmd.AddCommand(newTeamsGetCmd(flags))
	cmd.AddCommand(newTeamsListCmd(flags))
	cmd.AddCommand(newTeamsUpdateCmd(flags))
	return cmd
}
