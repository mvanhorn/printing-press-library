// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence — T2 cannibalization.

package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
)

func newCannibalizationCmd(flags *rootFlags) *cobra.Command {
	var (
		minImps          int
		topN             int
		includeSingleton bool
	)

	cmd := &cobra.Command{
		Use:         "cannibalization <site>",
		Short:       "Find queries where multiple pages compete, ranked by combined impressions",
		Long:        "Reads the local store and groups by query, surfacing queries the API answers across more than one page (the keyword-cannibalization audit the GSC web UI doesn't offer). Use --include-singletons to also list queries that resolve to exactly one page (useful for sanity-check coverage).",
		Example:     "  google-search-console-pp-cli cannibalization sc-domain:example.com --min-imps 50 --top 25 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			site := args[0]

			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			db := s.DB()
			having := `HAVING COUNT(DISTINCT page) > 1 AND SUM(impressions) >= ?`
			if includeSingleton {
				having = `HAVING COUNT(DISTINCT page) > 0 AND SUM(impressions) >= ?`
			}
			query := `
SELECT query,
       GROUP_CONCAT(DISTINCT page) AS pages,
       COALESCE(SUM(impressions),0) AS imps,
       COALESCE(SUM(clicks),0) AS clicks,
       COUNT(DISTINCT page) AS page_count
FROM search_analytics_rows
WHERE site_url = ?
  AND query != ''
  AND page  != ''
GROUP BY query
` + having + `
ORDER BY SUM(impressions) DESC LIMIT ?`

			rows, err := db.QueryContext(ctx, query, site, minImps, topN)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			out := []map[string]any{}
			for rows.Next() {
				var q, pages string
				var imps, clicks float64
				var pageCount int
				if err := rows.Scan(&q, &pages, &imps, &clicks, &pageCount); err != nil {
					return err
				}
				pageList := []string{}
				if pages != "" {
					pageList = strings.Split(pages, ",")
				}
				out = append(out, map[string]any{
					"query":             q,
					"pages":             pageList,
					"page_count":        pageCount,
					"total_impressions": imps,
					"total_clicks":      clicks,
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
	cmd.Flags().IntVar(&minImps, "min-imps", 50, "Minimum total impressions per query")
	cmd.Flags().IntVar(&topN, "top", 50, "Max queries to return")
	cmd.Flags().BoolVar(&includeSingleton, "include-singletons", false, "Include queries that resolve to exactly one page")

	return cmd
}
