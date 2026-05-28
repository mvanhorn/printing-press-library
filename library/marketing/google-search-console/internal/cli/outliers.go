// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence — T8 outliers.

package cli

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/spf13/cobra"
)

func newOutliersCmd(flags *rootFlags) *cobra.Command {
	var (
		metric  string
		topN    int
		sigma   float64
		minImps int
	)

	cmd := &cobra.Command{
		Use:         "outliers <site>",
		Short:       "Queries or pages with click-through rates that deviate from the observed CTR-by-position curve",
		Long:        "Computes mean+stddev CTR per integer-position bucket from your local corpus, then flags rows whose CTR is more than --sigma standard deviations from the bucket mean. Title-tag and snippet-rewrite candidates an agent can act on directly: high impressions, low CTR for their position.",
		Example:     "  google-search-console-pp-cli outliers sc-domain:example.com --metric ctr --sigma 2 --top 50 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			site := args[0]
			if metric != "ctr" {
				return usageErr(fmt.Errorf("only --metric ctr supported (got %q)", metric))
			}
			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			// Step 1: bucket stats from aggregated (query, page) -- per-row CTR
			// over 1-day, 1-query, 1-page slices is too noisy (single click on a
			// single impression flips CTR to 1.0 / 0.0). Aggregate to query+page
			// granularity first so the bucket mean+stddev reflect realistic
			// rather than splat distributions, then z-score the same aggregates.
			bucketRows, err := s.DB().QueryContext(ctx, `
WITH agg AS (
  SELECT query, page,
         CAST(AVG(position) AS INTEGER) AS bucket,
         CASE WHEN SUM(impressions) > 0 THEN SUM(clicks) / SUM(impressions) ELSE 0 END AS qctr,
         SUM(impressions) AS imps
  FROM search_analytics_rows
  WHERE site_url = ? AND query != ''
  GROUP BY query, page
  HAVING SUM(impressions) >= ?
)
SELECT bucket,
       AVG(qctr) AS mean,
       AVG(qctr * qctr) AS mean_sq,
       COUNT(*) AS n
FROM agg
GROUP BY bucket
HAVING COUNT(*) >= 3`, site, minImps)
			if err != nil {
				return apiErr(err)
			}
			defer bucketRows.Close()
			type bucket struct{ mean, stddev float64 }
			buckets := map[int]bucket{}
			for bucketRows.Next() {
				var b int
				var mean, meanSq float64
				var n int64
				if err := bucketRows.Scan(&b, &mean, &meanSq, &n); err != nil {
					return err
				}
				variance := meanSq - mean*mean
				if variance < 0 {
					variance = 0
				}
				buckets[b] = bucket{mean: mean, stddev: math.Sqrt(variance)}
			}
			if err := bucketRows.Err(); err != nil {
				return err
			}

			// Step 2: scan aggregated rows, compute z-score per (query, page).
			rows, err := s.DB().QueryContext(ctx, `
SELECT query, page,
       CAST(AVG(position) AS INTEGER) AS pos_bucket,
       AVG(position) AS avg_position,
       CASE WHEN SUM(impressions) > 0 THEN SUM(clicks) / SUM(impressions) ELSE 0 END AS qctr,
       SUM(impressions) AS imps
FROM search_analytics_rows
WHERE site_url = ? AND query != ''
GROUP BY query, page
HAVING SUM(impressions) >= ?`, site, minImps)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type out struct {
				m map[string]any
				z float64
			}
			var hits []out
			for rows.Next() {
				var q, p string
				var posBucket int
				var avgPos, ctr, imps float64
				if err := rows.Scan(&q, &p, &posBucket, &avgPos, &ctr, &imps); err != nil {
					return err
				}
				b, ok := buckets[posBucket]
				if !ok || b.stddev == 0 {
					continue
				}
				z := (ctr - b.mean) / b.stddev
				if math.Abs(z) >= sigma {
					hits = append(hits, out{
						m: map[string]any{
							"query":        q,
							"page":         p,
							"position":     avgPos,
							"ctr":          ctr,
							"expected_ctr": b.mean,
							"z_score":      z,
							"impressions":  imps,
						},
						z: math.Abs(z),
					})
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}
			sort.Slice(hits, func(i, j int) bool { return hits[i].z > hits[j].z })
			if len(hits) > topN {
				hits = hits[:topN]
			}
			final := make([]map[string]any, len(hits))
			for i, h := range hits {
				final[i] = h.m
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
	cmd.Flags().StringVar(&metric, "metric", "ctr", "Metric to scan (only 'ctr' supported)")
	cmd.Flags().IntVar(&topN, "top", 50, "Max outliers to return")
	cmd.Flags().Float64Var(&sigma, "sigma", 2.0, "Sigma threshold (|z| >= sigma flagged)")
	cmd.Flags().IntVar(&minImps, "min-imps", 50, "Minimum impressions per row to consider")
	return cmd
}
