// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelStatsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Append-only history of CDC's CDS aggregate statistics",
		Example:     "  cdc-pakistan-pp-cli stats history --metric sub_accounts_individual --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelStatsHistoryCmd(flags))
	return cmd
}
