// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #3 — backlink delta since last sync.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newBacklinkNewCmd(flags *rootFlags) *cobra.Command {
	var since string
	var targetType string
	var limit int

	cmd := &cobra.Command{
		Use:         "new [domain]",
		Short:       "Show backlinks and referring domains first seen in the last --since window.",
		Long:        "new filters the local backlink store to rows whose first_seen timestamp lies in the requested time window. Run 'semrush-pp-cli sync --resource backlink' first.",
		Example:     "  semrush-pp-cli backlink new mysite.com --since 7d --target-type root_domain",
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

			if !hintIfUnsynced(cmd, db, "backlink") {
				hintIfStale(cmd, db, "backlink", flags.maxAge)
			}

			domain := args[0]
			window, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-window)

			// first_seen may be stored as ISO timestamp string or epoch
			// millis; handle both by selecting both column forms and
			// post-filtering in Go.
			rows, err := db.DB().QueryContext(ctx,
				`SELECT resource_type,
				        COALESCE(json_extract(data, '$.source_url'), json_extract(data, '$.domain'), json_extract(data, '$.Dn'), '') AS source,
				        COALESCE(json_extract(data, '$.target'), json_extract(data, '$.Tg'), '') AS target,
				        COALESCE(json_extract(data, '$.target_type'), json_extract(data, '$.Tt'), '') AS tt,
				        COALESCE(json_extract(data, '$.first_seen'), json_extract(data, '$.Fs'), '') AS first_seen,
				        COALESCE(json_extract(data, '$.domain_ascore'), json_extract(data, '$.As'), 0) AS ascore,
				        data
				 FROM resources
				 WHERE resource_type IN ('backlink', 'backlink_referring_domains', 'referring_domain')
				   AND (json_extract(data, '$.target') = ? OR json_extract(data, '$.Tg') = ? OR json_extract(data, '$.domain') = ?)`,
				domain, domain, domain)
			if err != nil {
				return fmt.Errorf("query backlinks: %w", err)
			}
			defer rows.Close()

			type newLink struct {
				ResourceType string  `json:"resource_type"`
				Source       string  `json:"source"`
				Target       string  `json:"target"`
				TargetType   string  `json:"target_type"`
				FirstSeen    string  `json:"first_seen"`
				Ascore       float64 `json:"ascore"`
			}
			var hits []newLink
			for rows.Next() {
				var l newLink
				var data string
				if err := rows.Scan(&l.ResourceType, &l.Source, &l.Target, &l.TargetType, &l.FirstSeen, &l.Ascore, &data); err != nil {
					return fmt.Errorf("scan backlink row: %w", err)
				}
				if targetType != "" && l.TargetType != "" && !strings.EqualFold(l.TargetType, targetType) {
					continue
				}
				if l.FirstSeen == "" {
					continue
				}
				t, ok := parseFlexibleTime(l.FirstSeen)
				if !ok {
					continue
				}
				if t.Before(cutoff) {
					continue
				}
				hits = append(hits, l)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate backlink rows: %w", err)
			}
			// Deterministic top-N. The query has no ORDER BY, so SQLite
			// returns rows in rowid order; without this sort, --limit
			// would pick whichever 50 links happened to be inserted
			// first, not the most interesting. Sort by Ascore desc
			// (authority first), tiebreak by FirstSeen desc (newest
			// first), final tiebreak by Source for stability.
			sort.SliceStable(hits, func(i, j int) bool {
				if hits[i].Ascore != hits[j].Ascore {
					return hits[i].Ascore > hits[j].Ascore
				}
				if hits[i].FirstSeen != hits[j].FirstSeen {
					return hits[i].FirstSeen > hits[j].FirstSeen
				}
				return hits[i].Source < hits[j].Source
			})
			totalHitCount := len(hits)
			truncated := false
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
				truncated = true
			}

			out := map[string]any{
				"domain":          domain,
				"since":           since,
				"target_type":     targetType,
				"cutoff":          cutoff.Format(time.RFC3339),
				"hit_count":       totalHitCount,
				"hit_count_shown": len(hits),
				"truncated":       truncated,
				"new_links":       hits,
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Window to inspect (e.g. 1d, 7d, 30d)")
	cmd.Flags().StringVar(&targetType, "target-type", "root_domain", "url | domain | root_domain — match only this Semrush target_type")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum hits to return (0 disables)")
	return cmd
}

// parseFlexibleTime accepts ISO8601, RFC3339, "YYYY-MM-DD", and integer
// epoch (seconds or millis) strings.
func parseFlexibleTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	// Try epoch
	if n, err := parseIntStrict(s); err == nil {
		if n > 1e12 {
			return time.UnixMilli(n), true
		}
		return time.Unix(n, 0), true
	}
	return time.Time{}, false
}

func parseIntStrict(s string) (int64, error) {
	var n int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not an integer: %s", s)
		}
		n = n*10 + int64(r-'0')
	}
	return n, nil
}
