// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelIdentityCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "identity",
		Short:       "Security identity history: symbol, name and ISIN changes over time",
		Example:     "  cdc-pakistan-pp-cli identity ledger --symbol LOTCHEM --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelIdentityLedgerCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelIdentityResolveCmd(flags))
	return cmd
}
