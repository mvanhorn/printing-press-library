// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Hunt commands build privacy-safe taxon checklists.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelHuntCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "hunt",
		Short:       "hunt subcommands: create",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelHuntCreateCmd(flags))
	return cmd
}
