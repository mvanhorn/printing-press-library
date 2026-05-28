// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #11 — cannibalization detector.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newCannibalizationCmd(flags *rootFlags) *cobra.Command {
	var database string
	var limit int

	cmd := &cobra.Command{
		Use:         "cannibalization [domain]",
		Short:       "Phrases where multiple URLs on the same domain rank — a classic SEO content-overlap signal.",
		Long:        "cannibalization groups organic keyword positions by phrase and surfaces phrases where the same domain ranks for two or more URLs. Run 'semrush-pp-cli sync --resource keyword' first.",
		Example:     "  semrush-pp-cli cannibalization mysite.com --limit 50",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openNovelStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			recordBalanceSnapshotForCmd(ctx, db, flags, cmd.CommandPath(), cmd.ErrOrStderr())

			if !hintIfUnsynced(cmd, db, "keyword") {
				hintIfStale(cmd, db, "keyword", flags.maxAge)
			}

			domain := args[0]
			// ORDER BY synced_at DESC so the dedup loop below (which keeps
			// the FIRST row encountered per (phrase, URL)) retains the
			// most-recent position, not the oldest. Without this, a user
			// who syncs weekly would see positions from their first-ever
			// sync rather than the current state.
			rows, err := db.DB().QueryContext(ctx,
				`SELECT COALESCE(json_extract(data, '$.Ph'), json_extract(data, '$.phrase'), '') AS phrase,
				        COALESCE(json_extract(data, '$.Ur'), json_extract(data, '$.url'), '') AS url,
				        COALESCE(json_extract(data, '$.Po'), json_extract(data, '$.position'), 0) AS position
				 FROM resources
				 WHERE resource_type IN ('keyword', 'domain_keywords')
				   AND (json_extract(data, '$.domain') = ? OR json_extract(data, '$.Dn') = ?)
				   AND (? = '' OR json_extract(data, '$.database') = ? OR json_extract(data, '$.database') IS NULL)
				 ORDER BY synced_at DESC`,
				domain, domain, database, database)
			if err != nil {
				return fmt.Errorf("query keyword positions: %w", err)
			}
			defer rows.Close()

			type entry struct {
				URL      string  `json:"url"`
				Position float64 `json:"position"`
			}
			byPhrase := map[string][]entry{}
			urlsSeen := map[string]map[string]bool{}
			for rows.Next() {
				var phrase, url string
				var position float64
				if err := rows.Scan(&phrase, &url, &position); err != nil {
					return fmt.Errorf("scan keyword row: %w", err)
				}
				phrase = strings.TrimSpace(phrase)
				url = strings.TrimSpace(url)
				if phrase == "" || url == "" {
					continue
				}
				if urlsSeen[phrase] == nil {
					urlsSeen[phrase] = map[string]bool{}
				}
				if urlsSeen[phrase][url] {
					continue
				}
				urlsSeen[phrase][url] = true
				byPhrase[phrase] = append(byPhrase[phrase], entry{URL: url, Position: position})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate keyword rows: %w", err)
			}

			type hit struct {
				Phrase   string  `json:"phrase"`
				URLCount int     `json:"url_count"`
				URLs     []entry `json:"urls"`
			}
			var hits []hit
			for phrase, entries := range byPhrase {
				if len(entries) < 2 {
					continue
				}
				hits = append(hits, hit{Phrase: phrase, URLCount: len(entries), URLs: entries})
			}
			// Sort by URLCount desc, then phrase asc
			sort.SliceStable(hits, func(i, j int) bool {
				if hits[i].URLCount != hits[j].URLCount {
					return hits[i].URLCount > hits[j].URLCount
				}
				return hits[i].Phrase < hits[j].Phrase
			})
			totalHitCount := len(hits)
			truncated := false
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
				truncated = true
			}

			out := map[string]any{
				"domain":          domain,
				"database":        database,
				"hit_count":       totalHitCount,
				"hit_count_shown": len(hits),
				"truncated":       truncated,
				"hits":            hits,
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&database, "database", "us", "Semrush database/country to filter on; empty matches all")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum cannibalization hits to return (0 disables)")
	return cmd
}
