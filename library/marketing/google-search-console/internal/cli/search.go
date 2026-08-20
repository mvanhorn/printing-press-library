// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence layer.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		siteURL string
		topN    int
	)

	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Full-text search over query and page text in the local store (FTS5 with LIKE fallback)",
		Long:        "Searches `search_analytics_rows.query` and `search_analytics_rows.page` for matching rows in the local SQLite store. Uses FTS5 when available, otherwise falls back to case-insensitive LIKE. Read-only.",
		Example:     "  google-search-console-pp-cli search \"battery life\" --site sc-domain:example.com --top 25 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			term := strings.TrimSpace(strings.Join(args, " "))
			if term == "" {
				return usageErr(fmt.Errorf("search term required"))
			}

			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			db := s.DB()
			// LIKE is the default and the only path with no FTS-rebuild dependency.
			// We previously gated on s.HasFTS5() but the contentless FTS index built
			// in Open() requires explicit rebuild triggers that aren't emitted yet,
			// so MATCH consistently returned zero. LIKE is fast enough on the indexed
			// (site_url, query) and (site_url, page) columns and produces honest
			// substring results that match user expectations.
			like := "%" + term + "%"
			q := `SELECT site_url, date, query, page, clicks, impressions, ctr, position
                      FROM search_analytics_rows
                      WHERE (query LIKE ? OR page LIKE ?)`
			params := []any{like, like}
			if siteURL != "" {
				q += ` AND site_url = ?`
				params = append(params, siteURL)
			}
			q += ` ORDER BY impressions DESC LIMIT ?`
			params = append(params, topN)
			rows, err := db.QueryContext(ctx, q, params...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			out := []map[string]any{}
			for rows.Next() {
				var siteURLv, date, query, page string
				var clicks, impressions, ctr, position float64
				if err := rows.Scan(&siteURLv, &date, &query, &page, &clicks, &impressions, &ctr, &position); err != nil {
					return err
				}
				out = append(out, map[string]any{
					"site_url":    siteURLv,
					"date":        date,
					"query":       query,
					"page":        page,
					"clicks":      clicks,
					"impressions": impressions,
					"ctr":         ctr,
					"position":    position,
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&siteURL, "site", "", "Limit search to one property")
	cmd.Flags().IntVar(&topN, "top", 50, "Max results to return")

	return cmd
}
