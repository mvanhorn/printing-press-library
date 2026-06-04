// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence — T5 roll-up.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newRollUpCmd(flags *rootFlags) *cobra.Command {
	var (
		metric  string
		topN    int
		groupBy string
		last    string
	)

	cmd := &cobra.Command{
		Use:         "roll-up",
		Short:       "Aggregate top queries or pages across every verified property in one command",
		Long:        "Single SQL aggregate across all (site_url, query|page, ...) partitions in the local store. Lets agency workflows surface portfolio-wide top performers without writing per-site loops.",
		Example:     "  google-search-console-pp-cli roll-up --metric clicks --group-by query --top 50 --last 28d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if !validDim(groupBy) || (groupBy != "query" && groupBy != "page" && groupBy != "country" && groupBy != "device") {
				return usageErr(fmt.Errorf("--group-by must be one of: query, page, country, device"))
			}
			if metric != "clicks" && metric != "impressions" {
				return usageErr(fmt.Errorf("--metric must be clicks or impressions"))
			}
			dur, err := parseLast(last)
			if err != nil {
				return usageErr(err)
			}
			startDate, endDate := dateRange(dur)

			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			q := fmt.Sprintf(`
SELECT %s AS dim_value,
       COALESCE(SUM(%s),0) AS total,
       COUNT(DISTINCT site_url) AS sites_contributing,
       COALESCE(SUM(clicks),0) AS total_clicks,
       COALESCE(SUM(impressions),0) AS total_impressions
FROM search_analytics_rows
WHERE date BETWEEN ? AND ?
  AND %s != ''
GROUP BY %s
ORDER BY total DESC
LIMIT ?`, groupBy, metric, groupBy, groupBy)
			rows, err := s.DB().QueryContext(ctx, q, startDate, endDate, topN)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var dim string
				var total, clicks, imps float64
				var sites int64
				if err := rows.Scan(&dim, &total, &sites, &clicks, &imps); err != nil {
					return err
				}
				out = append(out, map[string]any{
					"dim_value":          dim,
					"total":              total,
					"total_clicks":       clicks,
					"total_impressions":  imps,
					"sites_contributing": sites,
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(out) == 0 {
				count, _ := s.CountSearchAnalyticsRows(ctx, "")
				if count == 0 {
					return emptyStoreErr("")
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&metric, "metric", "clicks", "Metric to rank by (clicks or impressions)")
	cmd.Flags().IntVar(&topN, "top", 50, "Max results to return")
	cmd.Flags().StringVar(&groupBy, "group-by", "query", "Dimension to roll up on (query, page, country, device)")
	cmd.Flags().StringVar(&last, "last", "28d", "How far back to scan (e.g. 7d, 28d, 90d)")
	return cmd
}
