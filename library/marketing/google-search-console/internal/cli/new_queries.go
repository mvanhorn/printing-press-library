// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence — T11 new-queries.

package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newNewQueriesCmd(flags *rootFlags) *cobra.Command {
	var (
		since   string
		minImps int
		topN    int
	)

	cmd := &cobra.Command{
		Use:         "new-queries <site>",
		Short:       "Queries that started showing up with impressions in the last N days but didn't exist before",
		Long:        "Local set difference: queries with impressions in the last --since window NOT present in any prior window. Surfaces emerging demand for content-marketing workflows. Requires retained history before the window starts.",
		Example:     "  google-search-console-pp-cli new-queries sc-domain:example.com --since 28d --min-imps 50 --top 100 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			site := args[0]
			dur, err := parseLast(since)
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

			rows, err := s.DB().QueryContext(ctx, `
SELECT query,
       SUM(impressions) AS imps,
       SUM(clicks) AS clicks,
       MIN(date) AS first_seen
FROM search_analytics_rows
WHERE site_url = ?
  AND date >= ?
  AND query != ''
GROUP BY query
HAVING SUM(impressions) >= ?
   AND query NOT IN (
       SELECT query FROM search_analytics_rows
       WHERE site_url = ? AND date < ? AND query != ''
   )
ORDER BY SUM(impressions) DESC
LIMIT ?`, site, startDate, minImps, site, startDate, topN)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var q, firstSeen string
				var imps, clicks float64
				if err := rows.Scan(&q, &imps, &clicks, &firstSeen); err != nil {
					return err
				}
				out = append(out, map[string]any{
					"query":              q,
					"recent_impressions": imps,
					"recent_clicks":      clicks,
					"first_seen_date":    firstSeen,
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
	cmd.Flags().StringVar(&since, "since", "28d", "Recent window length")
	cmd.Flags().IntVar(&minImps, "min-imps", 50, "Minimum impressions in the recent window")
	cmd.Flags().IntVar(&topN, "top", 100, "Max queries to return")
	return cmd
}
