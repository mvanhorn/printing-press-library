// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.
// Preserved adapter: the curated implementation lives in keyword_views.go so
// regeneration keeps the command behavior in the hand-written extension.

package cli

import "github.com/spf13/cobra"

func newNovelPortfolioSafeStatsCmd(flags *rootFlags) *cobra.Command {
	return newPlannerPortfolioSafeStatsCmd(flags)
}
