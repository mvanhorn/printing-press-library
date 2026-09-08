// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelPortfolioCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "portfolio",
		Short:       "Work with portfolio",
		Example:     "  keyword-planner-pp-cli portfolio coverage keyword-planner portfolio coverage latest --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelPortfolioCoverageCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPortfolioDiffCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPortfolioIntegrityCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPortfolioSafeStatsCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPortfolioTraceCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPortfolioVariantsCmd(flags))
	return cmd
}
