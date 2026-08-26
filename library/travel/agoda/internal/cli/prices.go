// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelPricesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "prices",
		Short:       "Price trends and cheapest-date sweeps across a window",
		Example:     "  agoda-pp-cli prices cheapest Tokyo --window 2026-10-01..2026-11-30 --nights 3 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelPricesCheapestCmd(flags))
	return cmd
}
