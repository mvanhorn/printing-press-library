// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

func newSummaryCmd(flags *rootFlags) *cobra.Command {
	var bedsMin float64

	cmd := &cobra.Command{
		Use:   "summary [region-slug-or-id]",
		Short: "One-shot region snapshot: counts, medians, and trend headline.",
		Long: `Reads the local homes table for the given region (filtered by --beds-min)
and computes counts (active, pending, sold), medians (list price, sold
price, DOM, $/sqft), and a percent-with-price-drops figure. Also pulls a
trend snapshot via aggregate-trends.`,
		Example: `  redfin-pp-cli summary 30772 --json
  redfin-pp-cli summary "city/30772/TX/Austin" --beds-min 3 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"region_id":   0,
						"region_type": 6,
					}, flags)
				}
				return cmd.Help()
			}
			id, typ, err := parseRegionSlug(args[0])
			if err != nil {
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"region_id":   0,
						"region_type": 6,
					}, flags)
				}
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"region_id":   id,
					"region_type": typ,
				}, flags)
			}
			s, err := openRedfinStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			listings, err := listingsFromHomesTable(cmd.Context(), s.DB(), id, typ)
			if err != nil {
				return err
			}
			activeCount, pendingCount, soldCount := 0, 0, 0
			var listPrices, soldPrices, dom, ppsqft []float64
			withDrops := 0
			for _, l := range listings {
				if bedsMin > 0 && l.Beds < bedsMin {
					continue
				}
				switch strings.ToLower(l.Status) {
				case "active":
					activeCount++
					if l.Price > 0 {
						listPrices = append(listPrices, float64(l.Price))
					}
				case "pending":
					pendingCount++
				case "sold":
					soldCount++
					if l.Price > 0 {
						soldPrices = append(soldPrices, float64(l.Price))
					}
				}
				if l.DOM > 0 {
					dom = append(dom, float64(l.DOM))
				}
				if l.Price > 0 && l.Sqft > 0 {
					ppsqft = append(ppsqft, float64(l.Price)/float64(l.Sqft))
				}
				for _, e := range l.PriceHistory {
					if strings.Contains(strings.ToLower(e.Event), "price") && strings.Contains(strings.ToLower(e.Event), "chang") {
						withDrops++
						break
					}
				}
			}
			out := map[string]any{
				"region_id":      id,
				"region_type":    typ,
				"active_count":   activeCount,
				"pending_count":  pendingCount,
				"sold_count":     soldCount,
				"median_list":    median(listPrices),
				"median_sold":    median(soldPrices),
				"median_dom":     median(dom),
				"median_ppsqft":  median(ppsqft),
				"with_drops_pct": pctOfTotal(withDrops, len(listings)),
			}
			// Trend snapshot — best-effort; skip on error.
			if rows, terr := fetchMarketTrends(flags, id, typ, 12, strconv.FormatInt(id, 10)); terr == nil {
				out["trend_snapshot"] = trendSnapshot(rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().Float64Var(&bedsMin, "beds-min", 0, "Minimum bedroom filter")
	return cmd
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	mid := len(s) / 2
	if len(s)%2 == 0 {
		return (s[mid-1] + s[mid]) / 2
	}
	return s[mid]
}

func pctOfTotal(num, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(num) / float64(total) * 100
}

// trendSnapshot collapses a long-format trend table into a compact map so
// summary's headline doesn't carry the full month-by-month series.
func trendSnapshot(rows []redfin.RegionTrendPoint) map[string]float64 {
	out := map[string]float64{}
	bestMonth := map[string]string{}
	for _, r := range rows {
		if r.Month > bestMonth[r.Metric] {
			bestMonth[r.Metric] = r.Month
			out[r.Metric] = r.Value
		}
	}
	return out
}
