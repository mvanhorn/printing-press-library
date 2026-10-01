// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelChangesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "changes",
		Short:       "See what changed in the budget from the sync change log",
		Example:     "  actual-budget-pp-cli changes since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelChangesSinceCmd(flags))
	return cmd
}
