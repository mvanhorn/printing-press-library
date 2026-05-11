// Copyright 2026 james-frewin. Licensed under Apache-2.0. See LICENSE.

// Stats summarizes locally-cached search-analytics rows for a site into a
// single rollup. Intended for agents that want "what's in my store?" at a
// glance without writing custom SQL. Runs entirely against the local
// SQLite store; if the store is empty, callers are directed to sync.
package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Summary stats over the local store for a site",
		Long: strings.TrimSpace(`
Aggregates locally-cached search-analytics rows for a site into a single
report: total clicks, total impressions, average CTR, average position,
distinct queries/pages, and observed date range. All numbers come from
the local SQLite store — run sync first if the store is empty.
`),
		Example:     "  google-search-console-pp-cli stats --site sc-domain:example.com --window 28d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireSite(cf.site); err != nil {
				return err
			}
			s, err := openStoreFromFlag(cmd.Context(), cf.db)
			if err != nil {
				return err
			}
			defer s.Close()
			ok, err := requireStoreData(s, cf.site)
			if err != nil {
				return err
			}
			if !ok {
				return emitEmpty(cmd, flags, fmt.Sprintf("no data for site %s", cf.site))
			}
			days := parseWindow(cf.window, 28)
			start, end := dateRange(days)

			var (
				totalClicks      float64
				totalImpressions float64
				avgPosition      float64
				distinctQueries  int64
				distinctPages    int64
				firstDate        sql.NullString
				lastDate         sql.NullString
			)
			row := s.DB().QueryRowContext(cmd.Context(), `
				SELECT
				  COALESCE(SUM(clicks), 0),
				  COALESCE(SUM(impressions), 0),
				  COALESCE(AVG(position), 0),
				  COUNT(DISTINCT query),
				  COUNT(DISTINCT page),
				  MIN(date),
				  MAX(date)
				FROM search_analytics_rows
				WHERE site_url = ? AND date BETWEEN ? AND ?`,
				cf.site, start, end)
			if err := row.Scan(&totalClicks, &totalImpressions, &avgPosition,
				&distinctQueries, &distinctPages, &firstDate, &lastDate); err != nil {
				return apiErr(err)
			}

			ctr := 0.0
			if totalImpressions > 0 {
				ctr = totalClicks / totalImpressions
			}
			dateRangeStr := ""
			if firstDate.Valid && lastDate.Valid {
				dateRangeStr = fmt.Sprintf("%s..%s", firstDate.String, lastDate.String)
			}

			return emit(cmd, flags, map[string]any{
				"site":              cf.site,
				"window_days":       days,
				"date_range":        dateRangeStr,
				"total_clicks":      totalClicks,
				"total_impressions": totalImpressions,
				"avg_ctr":           ctr,
				"avg_position":      avgPosition,
				"distinct_queries":  distinctQueries,
				"distinct_pages":    distinctPages,
			})
		},
	}
	cmd.Flags().StringVar(&cf.site, "site", "", "Site URL (e.g. sc-domain:example.com). Required.")
	cmd.Flags().StringVar(&cf.window, "window", "28d", "Window: Nd, Nw, or Nm.")
	cmd.Flags().StringVar(&cf.db, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	return cmd
}
