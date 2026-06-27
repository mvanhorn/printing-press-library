// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelSourcesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "sources",
		Short:       "Inspect and sync the wired sources (list, sync)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelSourcesListCmd(flags))
	cmd.AddCommand(newSourcesSyncCmd(flags))
	return cmd
}
