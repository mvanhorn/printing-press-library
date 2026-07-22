// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Shared plumbing for the internal/gtrends-backed novel commands: output
// rendering, deterministic resource ids, and the local-store query helpers
// used by the read-only commands (trends changes/opportunities/seasonality).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/store"
)

// sha256ID derives the deterministic resources.id used by every
// gtrends-backed resource type: hex(sha256(join(parts, "|")))[:16].
func sha256ID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])[:16]
}

// printTypedResult marshals a Go-typed value (built by a hand-written
// gtrends command, not raw endpoint response bytes) through the same
// human-table / --json / --compact / --csv pipeline the generated endpoint
// commands use, tagging the response with the given provenance source.
func printTypedResult(cmd *cobra.Command, flags *rootFlags, v any, source string) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		var items []map[string]any
		if json.Unmarshal(raw, &items) == nil && len(items) > 0 {
			if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
				return err
			}
			if len(items) >= 25 {
				fmt.Fprintf(cmd.ErrOrStderr(), "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
			}
			return nil
		}
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": source})
}

// printLiveResult renders a value fetched fresh from the live Google Trends
// API, tagging provenance as "live".
func printLiveResult(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printTypedResult(cmd, flags, v, "live")
}

// printLocalResult renders a value computed from the local SQLite mirror,
// tagging provenance as "local".
func printLocalResult(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printTypedResult(cmd, flags, v, "local")
}

// noLocalMirrorHint prints the standard hint when a Priority-2 command finds
// no local database at all (no sync/fetch has ever run), then returns an
// empty result via printLocalResult rather than a SQL/open error — the
// printing-press missing-mirror convention.
func noLocalMirrorHint(cmd *cobra.Command, flags *rootFlags, howToPopulate string, empty any) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "hint: no local mirror yet. Run '%s' first.\n", howToPopulate)
	return printLocalResult(cmd, flags, empty)
}

// distinctSyncedAtDesc parses a list of RFC3339 synced_at strings (as
// written by the gtrends wrapper commands), dedupes them, and returns the
// distinct instants sorted most-recent-first. Unparseable entries are
// skipped rather than failing the whole comparison.
func distinctSyncedAtDesc(values []string) []time.Time {
	seen := map[string]bool{}
	times := make([]time.Time, 0, len(values))
	for _, s := range values {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			continue
		}
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i].After(times[j]) })
	return times
}

// queryRelatedTermsForKeyword loads every gt_related_term row for keyword
// from the local store, decoded into gtRelatedTermRecord. Fully drains and
// closes its *sql.Rows before returning, so callers may safely issue a
// follow-up db.Query afterward.
func queryRelatedTermsForKeyword(db *store.Store, keyword string) ([]gtRelatedTermRecord, error) {
	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'gt_related_term' AND json_extract(data, '$.keyword') = ?`,
		keyword,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]gtRelatedTermRecord, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var rec gtRelatedTermRecord
		if err := json.Unmarshal([]byte(data), &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// queryInterestPointsForKeyword loads every gt_interest_point row for
// keyword (any geo) from the local store. See queryRelatedTermsForKeyword
// for the drain-before-return contract.
func queryInterestPointsForKeyword(db *store.Store, keyword string) ([]gtInterestPointRecord, error) {
	return queryInterestPointsForKeywordGeo(db, keyword, "")
}

// queryInterestPointsForKeywordGeo is queryInterestPointsForKeyword scoped
// to a specific geo when geo != "".
func queryInterestPointsForKeywordGeo(db *store.Store, keyword, geo string) ([]gtInterestPointRecord, error) {
	query := `SELECT data FROM resources WHERE resource_type = 'gt_interest_point' AND json_extract(data, '$.keyword') = ?`
	args := []any{keyword}
	if geo != "" {
		query += ` AND json_extract(data, '$.geo') = ?`
		args = append(args, geo)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]gtInterestPointRecord, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var rec gtInterestPointRecord
		if err := json.Unmarshal([]byte(data), &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// filterInterestRowsToLatestScope narrows rows to only those matching the
// full scope (geo, timeframe, category, property, compare_scope) of the
// most-recently-synced row. Every one of those dimensions changes what
// Google Trends returns for the same keyword; without this filter, syncing
// the same keyword under two different scopes (e.g. two categories, or two
// geos) would silently blend both scoped series into one when a local
// command diffs, averages, or ranks the cached history -- the on-disk rows
// now coexist (the cache key includes scope), but the read side must still
// pick one scope to reason about. Defaulting to "whatever was synced most
// recently" matches these commands' existing latest-sync-instant convention.
// Callers that already scope by geo (e.g. queryInterestPointsForKeywordGeo)
// still need this: it additionally narrows by timeframe/category/property/
// compare_scope, which that geo-only query does not filter on.
func filterInterestRowsToLatestScope(rows []gtInterestPointRecord) []gtInterestPointRecord {
	if len(rows) == 0 {
		return rows
	}
	latestIdx := 0
	var latestT time.Time
	for i, r := range rows {
		t, err := time.Parse(time.RFC3339, r.SyncedAt)
		if err != nil {
			continue
		}
		if t.After(latestT) {
			latestT = t
			latestIdx = i
		}
	}
	scope := rows[latestIdx]
	out := make([]gtInterestPointRecord, 0, len(rows))
	for _, r := range rows {
		if r.Geo == scope.Geo && r.Timeframe == scope.Timeframe &&
			r.Category == scope.Category && r.Property == scope.Property && r.CompareScope == scope.CompareScope {
			out = append(out, r)
		}
	}
	return out
}

// filterRelatedRowsToLatestScope is filterInterestRowsToLatestScope's
// counterpart for gt_related_term rows. Geo, timeframe, and category are all
// part of the cache identity (so all three can coexist for the same
// keyword); this must match on all three, not just category, or two
// same-category rows from different geos or timeframes would still be
// compared/diffed together as if they were one snapshot scope.
func filterRelatedRowsToLatestScope(rows []gtRelatedTermRecord) []gtRelatedTermRecord {
	if len(rows) == 0 {
		return rows
	}
	latestIdx := 0
	var latestT time.Time
	for i, r := range rows {
		t, err := time.Parse(time.RFC3339, r.SyncedAt)
		if err != nil {
			continue
		}
		if t.After(latestT) {
			latestT = t
			latestIdx = i
		}
	}
	scope := rows[latestIdx]
	out := make([]gtRelatedTermRecord, 0, len(rows))
	for _, r := range rows {
		if r.Geo == scope.Geo && r.Timeframe == scope.Timeframe && r.Category == scope.Category {
			out = append(out, r)
		}
	}
	return out
}
