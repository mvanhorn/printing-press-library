// Copyright 2026 grahac and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/botsee/internal/store"
)

// newSitesSummaryCmd aggregates cited sources across every synced site,
// grouped by domain. Cross-site rollup that BotSee's per-site API cannot
// produce; only computable in the local SQLite cache.
//
// Output columns: domain, citation_count, distinct_sites_citing, first_seen.
func newSitesSummaryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var groupBy string
	var limit int

	cmd := &cobra.Command{
		Use:   "sites-summary",
		Short: "Cross-site source rollup: which domains are getting cited across every site you've synced.",
		Long: `Aggregate cited sources across every synced site, grouped by domain.

Returns domain × citation_count × distinct_sites_citing × first_seen.
Useful for agencies (cross-client portfolio view) AND single-users with
multiple brands. Requires that 'botsee-pp-cli sync' has been run first
to populate the local SQLite cache with analysis sources.`,
		Example: "  botsee-pp-cli sites-summary\n" +
			"  botsee-pp-cli sites-summary --agent --select domain,citation_count,distinct_sites_citing\n" +
			"  botsee-pp-cli sites-summary --limit 25 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:novel":      "sites-summary",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("botsee-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, resource_type, data, synced_at
				FROM resources
				WHERE resource_type IN ('sources','analysis_sources')
				ORDER BY synced_at DESC
			`)
			if err != nil {
				return fmt.Errorf("query sources: %w", err)
			}
			defer rows.Close()

			type agg struct {
				Domain        string `json:"domain"`
				CitationCount int    `json:"citation_count"`
				DistinctSites int    `json:"distinct_sites_citing"`
				FirstSeen     string `json:"first_seen"`
				distinctSites map[string]struct{}
			}
			aggs := map[string]*agg{}

			for rows.Next() {
				var id, rtype string
				var data sql.NullString
				var syncedAt sql.NullString
				if err := rows.Scan(&id, &rtype, &data, &syncedAt); err != nil {
					continue
				}
				if !data.Valid {
					continue
				}
				var src map[string]any
				if err := json.Unmarshal([]byte(data.String), &src); err != nil {
					continue
				}
				rawURL := asString(src["url"])
				if rawURL == "" {
					rawURL = asString(src["source_url"])
				}
				domain := extractHost(rawURL)
				if domain == "" {
					continue
				}
				siteUUID := asString(src["site_uuid"])
				if siteUUID == "" {
					siteUUID = asString(src["analysis_uuid"])
				}

				a, ok := aggs[domain]
				if !ok {
					a = &agg{Domain: domain, distinctSites: map[string]struct{}{}}
					aggs[domain] = a
				}
				if mentions := asInt(src["mentions"]); mentions > 0 {
					a.CitationCount += mentions
				} else if cited := asInt(src["citation_count"]); cited > 0 {
					a.CitationCount += cited
				} else {
					a.CitationCount++
				}
				if siteUUID != "" {
					a.distinctSites[siteUUID] = struct{}{}
				}
				if syncedAt.Valid {
					if a.FirstSeen == "" || syncedAt.String < a.FirstSeen {
						a.FirstSeen = syncedAt.String
					}
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating sources: %w", err)
			}

			out := make([]*agg, 0, len(aggs))
			for _, a := range aggs {
				a.DistinctSites = len(a.distinctSites)
				out = append(out, a)
			}
			sort.Slice(out, func(i, j int) bool {
				return out[i].CitationCount > out[j].CitationCount
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			w := cmd.OutOrStdout()
			if flags.asJSON || flags.agent || !isTerminal(w) {
				return printJSONFiltered(w, map[string]any{"domains": out, "group_by": groupBy, "total": len(aggs)}, flags)
			}

			if len(out) == 0 {
				fmt.Fprintln(w, "No sources found locally. Run `botsee-pp-cli sync --resources sources` first.")
				return nil
			}
			fmt.Fprintf(w, "%-44s %12s %12s\n", "DOMAIN", "CITATIONS", "SITES")
			for _, a := range out {
				fmt.Fprintf(w, "%-44s %12d %12d\n", truncate(a.Domain, 44), a.CitationCount, a.DistinctSites)
			}
			fmt.Fprintf(w, "\n%d unique domains across all synced sites.\n", len(aggs))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.botsee-pp-cli/store.db)")
	cmd.Flags().StringVar(&groupBy, "group-by", "domain", "Aggregation key (currently only 'domain' is supported)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Cap result rows (0 = no cap)")
	return cmd
}

// extractHost normalizes a URL or domain string to its bare host.
func extractHost(s string) string {
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	idx := strings.Index(s, "://")
	if idx < 0 {
		return ""
	}
	rest := s[idx+3:]
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}
	rest = strings.ToLower(rest)
	rest = strings.TrimPrefix(rest, "www.")
	if i := strings.Index(rest, ":"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
	case string:
		var i int
		fmt.Sscanf(n, "%d", &i)
		return i
	}
	return 0
}
