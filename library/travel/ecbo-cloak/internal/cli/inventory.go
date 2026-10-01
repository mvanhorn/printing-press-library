// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelInventoryCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "inventory",
		Short:       "Work with inventory",
		Example:     "  ecbo-cloak-pp-cli inventory list",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelInventoryListCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelInventoryRefreshCmd(flags))
	return cmd
}
