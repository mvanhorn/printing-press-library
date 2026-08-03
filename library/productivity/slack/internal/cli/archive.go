// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelArchiveCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "archive",
		Short:       "archive subcommands: coverage",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelArchiveCoverageCmd(flags))
	cmd.AddCommand(newNovelArchiveRecallCmd(flags))
	cmd.AddCommand(newNovelArchiveSyncCmd(flags))
	return cmd
}
