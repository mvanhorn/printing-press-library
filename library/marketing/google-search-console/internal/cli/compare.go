// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence — T3 compare.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var (
		periodFlag string
		vsFlag     string
		dimFlag    string
		topN       int
	)

	cmd := &cobra.Command{
		Use:         "compare <site>",
		Short:       "Period-over-period delta on clicks, impressions, CTR, and position for any dimension",
		Long:        "Computes [today-period, today] vs [today-2*period, today-period] from the local store and joins the two windows on the dimension key. Emits Δclicks, Δimpressions, ΔCTR (percentage points), Δposition. Sorted by abs(Δclicks) descending.",
		Example:     "  google-search-console-pp-cli compare sc-domain:example.com --period 28d --vs prev-period --dim query --top 50 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			site := args[0]
			period, err := parseLast(periodFlag)
			if err != nil {
				return usageErr(err)
			}
			if vsFlag != "prev-period" {
				return usageErr(fmt.Errorf("only --vs prev-period is supported"))
			}
			if dimFlag != "query" && dimFlag != "page" && dimFlag != "country" && dimFlag != "device" {
				return usageErr(fmt.Errorf("--dim must be one of: query, page, country, device"))
			}

			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			now := time.Now().UTC()
			currStart := now.Add(-period).Format("2006-01-02")
			currEnd := now.Format("2006-01-02")
			prevStart := now.Add(-2 * period).Format("2006-01-02")
			prevEnd := currStart

			curr, err := aggregateForCompare(ctx, s.DB(), site, dimFlag, currStart, currEnd)
			if err != nil {
				return err
			}
			prev, err := aggregateForCompare(ctx, s.DB(), site, dimFlag, prevStart, prevEnd)
			if err != nil {
				return err
			}

			// Join on dim key
			out := []map[string]any{}
			seen := map[string]bool{}
			handle := func(key string) {
				if seen[key] {
					return
				}
				seen[key] = true
				c := curr[key]
				p := prev[key]
				deltaCtrPp := 0.0
				if c.imps > 0 || p.imps > 0 {
					deltaCtrPp = (safeRatio(c.clicks, c.imps) - safeRatio(p.clicks, p.imps)) * 100
				}
				out = append(out, map[string]any{
					"key":                 key,
					"clicks_current":      c.clicks,
					"clicks_prev":         p.clicks,
					"delta_clicks":        c.clicks - p.clicks,
					"impressions_current": c.imps,
					"impressions_prev":    p.imps,
					"delta_impressions":   c.imps - p.imps,
					"delta_ctr_pp":        deltaCtrPp,
					"delta_position":      safeAvg(c.posSum, c.posCount) - safeAvg(p.posSum, p.posCount),
				})
			}
			for k := range curr {
				handle(k)
			}
			for k := range prev {
				handle(k)
			}

			sort.Slice(out, func(i, j int) bool {
				return math.Abs(out[i]["delta_clicks"].(float64)) > math.Abs(out[j]["delta_clicks"].(float64))
			})
			if len(out) > topN {
				out = out[:topN]
			}
			if len(out) == 0 {
				count, _ := s.CountSearchAnalyticsRows(ctx, site)
				if count == 0 {
					return emptyStoreErr(site)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"site":          site,
				"current_start": currStart,
				"current_end":   currEnd,
				"prev_start":    prevStart,
				"prev_end":      prevEnd,
				"rows":          out,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&periodFlag, "period", "28d", "Window length per side (e.g. 7d, 28d, 90d)")
	cmd.Flags().StringVar(&vsFlag, "vs", "prev-period", "Comparison anchor (only 'prev-period' supported today)")
	cmd.Flags().StringVar(&dimFlag, "dim", "query", "Dimension to group by (query, page, country, device)")
	cmd.Flags().IntVar(&topN, "top", 50, "Max rows to return after sort")
	return cmd
}

type compareAgg struct {
	clicks   float64
	imps     float64
	posSum   float64
	posCount int64
}

func aggregateForCompare(ctx context.Context, db *sql.DB, site, dim, startDate, endDate string) (map[string]compareAgg, error) {
	if !validDim(dim) {
		return nil, fmt.Errorf("invalid dim %q", dim)
	}
	q := fmt.Sprintf(`
SELECT %s AS key,
       COALESCE(SUM(clicks),0),
       COALESCE(SUM(impressions),0),
       COALESCE(SUM(position),0),
       COUNT(*)
FROM search_analytics_rows
WHERE site_url = ?
  AND date BETWEEN ? AND ?
  AND %s != ''
GROUP BY %s`, dim, dim, dim)
	rows, err := db.QueryContext(ctx, q, site, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]compareAgg{}
	for rows.Next() {
		var key string
		var clicks, imps, posSum float64
		var count int64
		if err := rows.Scan(&key, &clicks, &imps, &posSum, &count); err != nil {
			return nil, err
		}
		out[key] = compareAgg{clicks: clicks, imps: imps, posSum: posSum, posCount: count}
	}
	return out, rows.Err()
}

// validDim guards against SQL injection on the dim column.
func validDim(d string) bool {
	switch d {
	case "query", "page", "country", "device", "search_appearance", "date", "search_type":
		return true
	}
	return false
}

func safeRatio(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
func safeAvg(sum float64, n int64) float64 {
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}
