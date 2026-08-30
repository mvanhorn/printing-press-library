// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelHistoryCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "history",
		Short:       "Local state that compounds",
		Example:     "  quickcommerce-pp-cli history diff --item 501346 --latest 2 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelHistoryDiffCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHistoryPricesCmd(flags))
	return cmd
}
