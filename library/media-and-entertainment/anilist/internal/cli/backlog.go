// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Command group for personal backlog workflows.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelBacklogCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "backlog",
		Short:       "backlog subcommands: pick",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelBacklogPickCmd(flags))
	return cmd
}
