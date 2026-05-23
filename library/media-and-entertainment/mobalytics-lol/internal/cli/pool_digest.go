// Copyright 2026 quantumglitch. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	moba "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/internal/mobalytics"
)

// newPoolDigestCmd composites one champion-build fetch per slug and
// returns: current tier, WR, top-1 counter, top-1 synergy.
func newPoolDigestCmd(flags *rootFlags) *cobra.Command {
	var pool string
	cmd := &cobra.Command{
		Use:         "pool-digest",
		Short:       "Composite per-champion: current tier, top-1 counter, top-1 synergy.",
		Example:     `  mobalytics-lol-pp-cli pool-digest --pool jinx,kaisa,ezreal,ashe`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if pool == "" {
				return fmt.Errorf("--pool is required (CSV of champion slugs)")
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			type row struct {
				Slug         string  `json:"slug"`
				Tier         string  `json:"tier"`
				WinRate      float64 `json:"winRate"`
				PickRate     float64 `json:"pickRate"`
				TopCounter   string  `json:"topCounter,omitempty"`
				TopCounterWR float64 `json:"topCounterWR,omitempty"`
				TopSynergy   string  `json:"topSynergy,omitempty"`
				TopSynergyWR float64 `json:"topSynergyWR,omitempty"`
			}
			out := []row{}
			for _, slug := range splitCSV(pool) {
				buildHTML, err := client.Fetch(moba.ChampionPath(slug, "build"))
				if err != nil {
					return fmt.Errorf("fetch %s build: %w", slug, err)
				}
				stats := moba.ParseChampionStats(buildHTML, slug)
				syn := moba.ParseSynergies(buildHTML, slug)
				sort.SliceStable(syn, func(i, j int) bool { return syn[i].WinRate > syn[j].WinRate })
				// Soft-fail on counter fetch so one slow / rate-limited
				// counter page doesn't abort the entire digest. Build
				// data is still fatal above (no build = no row); counter
				// data is enrichment.
				counterHTML, cErr := client.Fetch(moba.ChampionPath(slug, "counters"))
				if cErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  warning: failed to fetch counters for %s: %v\n", slug, cErr)
				}
				counters := moba.ParseCounters(counterHTML, slug)
				moba.SortCountersByDelta(counters, true)
				r := row{Slug: slug, Tier: stats.Tier, WinRate: stats.WinRate, PickRate: stats.PickRate}
				if len(counters) > 0 {
					r.TopCounter = counters[0].OpponentSlug
					r.TopCounterWR = counters[0].WinRate
				}
				if len(syn) > 0 {
					r.TopSynergy = syn[0].PartnerSlug
					r.TopSynergyWR = syn[0].WinRate
				}
				out = append(out, r)
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&pool, "pool", "", "CSV of champion slugs (required).")
	return cmd
}
