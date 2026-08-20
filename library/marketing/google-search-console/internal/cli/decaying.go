// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence — T10 decaying.

package cli

import (
	"context"
	"math"
	"sort"

	"github.com/spf13/cobra"
)

func newDecayingCmd(flags *rootFlags) *cobra.Command {
	var (
		windowFlag string
		minImps    int
		topN       int
	)

	cmd := &cobra.Command{
		Use:         "decaying <site>",
		Short:       "Pages with monotonic click decline over a rolling window — the content-refresh queue",
		Long:        "Computes a least-squares slope on per-page weekly clicks across the window, ranking pages with negative slope by abs(slope) × total impressions. The content marketers' refresh queue: agent picks update candidates with concrete supporting numbers.",
		Example:     "  google-search-console-pp-cli decaying sc-domain:example.com --window 90d --min-imps 500 --top 50 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			site := args[0]
			dur, err := parseLast(windowFlag)
			if err != nil {
				return usageErr(err)
			}
			startDate, _ := dateRange(dur)

			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			// Bucket per page per ISO week
			rows, err := s.DB().QueryContext(ctx, `
SELECT page,
       strftime('%Y-%W', date) AS week,
       COALESCE(SUM(clicks),0) AS clicks,
       COALESCE(SUM(impressions),0) AS impressions
FROM search_analytics_rows
WHERE site_url = ?
  AND date >= ?
  AND page != ''
GROUP BY page, week
ORDER BY page, week`, site, startDate)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			byPage := map[string][]weekly{}
			pageImps := map[string]float64{}
			for rows.Next() {
				var page, week string
				var clicks, imps float64
				if err := rows.Scan(&page, &week, &clicks, &imps); err != nil {
					return err
				}
				_ = week
				byPage[page] = append(byPage[page], weekly{clicks: clicks, imps: imps})
				pageImps[page] += imps
			}
			if err := rows.Err(); err != nil {
				return err
			}

			type result struct {
				page        string
				slope       float64
				totalImps   float64
				weeks       int
				totalClicks float64
			}
			out := []result{}
			for page, series := range byPage {
				if len(series) < 3 {
					continue
				}
				if pageImps[page] < float64(minImps) {
					continue
				}
				slope := linearSlope(series)
				if slope >= 0 {
					continue
				}
				totalClicks := 0.0
				for _, w := range series {
					totalClicks += w.clicks
				}
				out = append(out, result{
					page:        page,
					slope:       slope,
					totalImps:   pageImps[page],
					weeks:       len(series),
					totalClicks: totalClicks,
				})
			}
			sort.Slice(out, func(i, j int) bool {
				return math.Abs(out[i].slope)*out[i].totalImps > math.Abs(out[j].slope)*out[j].totalImps
			})
			if len(out) > topN {
				out = out[:topN]
			}
			final := make([]map[string]any, len(out))
			for i, r := range out {
				final[i] = map[string]any{
					"page":              r.page,
					"slope":             r.slope,
					"total_impressions": r.totalImps,
					"total_clicks":      r.totalClicks,
					"weeks_observed":    r.weeks,
				}
			}
			if len(final) == 0 {
				count, _ := s.CountSearchAnalyticsRows(ctx, site)
				if count == 0 {
					return emptyStoreErr(site)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), final, flags)
		},
	}
	cmd.Flags().StringVar(&windowFlag, "window", "90d", "Rolling window length")
	cmd.Flags().IntVar(&minImps, "min-imps", 500, "Minimum total impressions for a page to qualify")
	cmd.Flags().IntVar(&topN, "top", 50, "Max pages to return")
	return cmd
}

// weekly is a one-week (clicks, impressions) pair for a single page.
type weekly struct {
	clicks float64
	imps   float64
}

// linearSlope returns the least-squares slope of the click-count series. The
// X-axis is week index (0..N-1).
func linearSlope(series []weekly) float64 {
	n := float64(len(series))
	var sumX, sumY, sumXY, sumXX float64
	for i, w := range series {
		x := float64(i)
		y := w.clicks
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}
