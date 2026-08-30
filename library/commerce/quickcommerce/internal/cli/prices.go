// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
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
		Short:       "Decision-ready comparisons",
		Example:     "  quickcommerce-pp-cli prices value --query milk --location 12.9021,77.6639 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelPricesValueCmd(flags))
	return cmd
}
