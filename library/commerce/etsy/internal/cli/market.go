// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelMarketCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "market",
		Short: "Work with market",
		// etsy-ext: MCP descriptions retain envelope and projection guidance.
		Long:        "Market commands emit `{results, notes}`; `market tags` payload is `results.tags` / `results.title_words`; `market trend` payload is under `results` (e.g. `results.new_listings`). --select paths are relative to the envelope even under --agent (e.g. --agent --select results.tags); the output then appears at results.results.tags. Don't mix anchored (results.x) and unanchored (x) paths in one --select. demand listings_total is null for not_fetched terms. Demand states include no_results (fetched with no matches) and not_fetched (resume needed). Shop shop_name is nullable when unknown. --csv and --plain are refused for the nested market reports (tags, trend); use --json or --agent.",
		Example:     "  etsy-pp-cli market demand --window 12mo --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelMarketDemandCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelMarketGapsCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelMarketPriceCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelMarketShopsCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelMarketSnapshotCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelMarketTagsCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelMarketTrendCmd(flags))
	return cmd
}
