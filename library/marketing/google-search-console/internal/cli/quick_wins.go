// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence — T1 quick-wins.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newQuickWinsCmd(flags *rootFlags) *cobra.Command {
	var (
		positionRange string
		minImps       int
		minCTR        float64
		topN          int
	)

	cmd := &cobra.Command{
		Use:         "quick-wins <site>",
		Short:       "Surface page-2 queries with high impressions and low CTR — page-2-to-page-1 candidates ranked by upside",
		Long:        "Computes from the local SQLite store: queries averaging position 8-20 with at least N impressions, ranked by impressions descending. Optionally filters out rows whose CTR is already above --min-ctr (already converting). Reads only — run `sync --site <url> --last 90d` first.",
		Example:     "  google-search-console-pp-cli quick-wins sc-domain:example.com --position 8-20 --min-imps 100 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			site := args[0]
			pMin, pMax, err := parseRange(positionRange)
			if err != nil {
				return usageErr(err)
			}

			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			db := s.DB()
			query := `
SELECT query,
       COALESCE(SUM(impressions), 0) AS imps,
       COALESCE(SUM(clicks), 0)      AS clicks,
       CASE WHEN SUM(impressions) > 0
            THEN SUM(clicks) / SUM(impressions)
            ELSE 0 END               AS avg_ctr,
       COALESCE(AVG(position), 0)    AS avg_position
FROM search_analytics_rows
WHERE site_url = ?
  AND query != ''
  AND position BETWEEN ? AND ?
GROUP BY query
HAVING SUM(impressions) >= ?`
			argsList := []any{site, pMin, pMax, minImps}
			if minCTR > 0 {
				query += ` AND (SUM(clicks) / SUM(impressions)) <= ?`
				argsList = append(argsList, minCTR)
			}
			query += ` ORDER BY SUM(impressions) DESC LIMIT ?`
			argsList = append(argsList, topN)

			rows, err := db.QueryContext(ctx, query, argsList...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			out := []map[string]any{}
			for rows.Next() {
				var q string
				var imps, clicks, ctr, pos float64
				if err := rows.Scan(&q, &imps, &clicks, &ctr, &pos); err != nil {
					return err
				}
				out = append(out, map[string]any{
					"query":        q,
					"impressions":  imps,
					"clicks":       clicks,
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
	cmd.Flags().StringVar(&positionRange, "position", "8-20", "Position range to consider (e.g. 8-20)")
	cmd.Flags().IntVar(&minImps, "min-imps", 100, "Minimum impressions per query")
	cmd.Flags().Float64Var(&minCTR, "min-ctr", 0, "Skip queries already above this CTR (0 = no filter)")
	cmd.Flags().IntVar(&topN, "top", 50, "Max queries to return")

	return cmd
}

// parseRange parses "8-20" into (8, 20). Single-number "12" yields (12, 12).
func parseRange(s string) (float64, float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, fmt.Errorf("empty range")
	}
	if !strings.Contains(s, "-") {
		v, err := strconv.ParseFloat(s, 64)
		return v, v, err
	}
	parts := strings.SplitN(s, "-", 2)
	a, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, err
	}
	b, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, err
	}
	return a, b, nil
}
