// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Command group for personal progress workflows.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelProgressCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "progress",
		Short:       "progress subcommands: catch-up, check-in",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelProgressCatchUpCmd(flags))
	cmd.AddCommand(newNovelProgressCheckInCmd(flags))
	return cmd
}
