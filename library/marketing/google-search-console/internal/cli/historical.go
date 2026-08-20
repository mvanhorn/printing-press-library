// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence — T7 historical.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newHistoricalCmd(flags *rootFlags) *cobra.Command {
	var (
		startDate string
		endDate   string
		dim       string
		topN      int
	)

	cmd := &cobra.Command{
		Use:         "historical <site>",
		Short:       "Search analytics for date ranges older than the API's 16-month rolling window",
		Long:        "Pure SELECT against the local store — works for any date range you've previously synced, including dates predating the GSC API's 16-month limit. Pairs naturally with `sync` runs that captured your earliest data when it was still in the window.",
		Example:     "  google-search-console-pp-cli historical sc-domain:example.com --start 2023-01-01 --end 2023-12-31 --dim query --top 100 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			site := args[0]
			if startDate == "" || endDate == "" {
				return usageErr(fmt.Errorf("--start and --end are required (YYYY-MM-DD)"))
			}
			if !validDim(dim) {
				return usageErr(fmt.Errorf("invalid --dim %q", dim))
			}
			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			q := fmt.Sprintf(`
SELECT %s AS dim_value,
       COALESCE(SUM(clicks),0) AS clicks,
       COALESCE(SUM(impressions),0) AS impressions,
       CASE WHEN SUM(impressions) > 0 THEN SUM(clicks)/SUM(impressions) ELSE 0 END AS ctr,
       COALESCE(AVG(position),0) AS avg_position
FROM search_analytics_rows
WHERE site_url = ?
  AND date BETWEEN ? AND ?
  AND %s != ''
GROUP BY %s
ORDER BY SUM(clicks) DESC
LIMIT ?`, dim, dim, dim)
			rows, err := s.DB().QueryContext(ctx, q, site, startDate, endDate, topN)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var dimVal string
				var clicks, imps, ctr, pos float64
				if err := rows.Scan(&dimVal, &clicks, &imps, &ctr, &pos); err != nil {
					return err
				}
				out = append(out, map[string]any{
					"dim_value":    dimVal,
					"clicks":       clicks,
					"impressions":  imps,
					"ctr":          ctr,
					"avg_position": pos,
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(out) == 0 {
				count, _ := s.CountSearchAnalyticsRows(ctx, site)
				if count == 0 {
					return emptyStoreErr(site)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&startDate, "start", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end", "", "End date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dim, "dim", "query", "Dimension to group by (query, page, country, device)")
	cmd.Flags().IntVar(&topN, "top", 1000, "Max rows to return")
	return cmd
}
