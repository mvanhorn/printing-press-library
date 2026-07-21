// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Command group for the personal schedule workflow.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelScheduleCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "schedule",
		Short:       "schedule subcommands: tonight",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelScheduleTonightCmd(flags))
	return cmd
}
