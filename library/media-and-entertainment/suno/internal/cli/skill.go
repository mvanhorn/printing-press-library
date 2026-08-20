// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newSkillCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "skill",
		Short:  "Install the Suno CLI as a coding-agent skill",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newSkillInstallCmd(flags))
	return cmd
}
