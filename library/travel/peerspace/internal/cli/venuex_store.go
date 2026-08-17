// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
)

const peerspaceCLIName = "peerspace-pp-cli"

// resolveNovelDBPath returns the explicit --db path or the canonical default.
func resolveNovelDBPath(dbPath string) string {
	if dbPath != "" {
		return dbPath
	}
	return defaultDBPath(peerspaceCLIName)
}

// openNovelStoreRO opens the local SQLite store read-only.
// When the file is missing it returns (nil, nil) so callers can emit empty
// JSON without failing (agent-friendly cold start).
func openNovelStoreRO(ctx context.Context, dbPath string) (*store.Store, error) {
	dbPath = resolveNovelDBPath(dbPath)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return store.OpenReadOnlyContext(ctx, dbPath)
}

// openNovelStoreRW opens the store for snapshot writes (shortlist delta/drift).
// Missing file returns (nil, nil).
func openNovelStoreRW(ctx context.Context, dbPath string) (*store.Store, error) {
	dbPath = resolveNovelDBPath(dbPath)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return store.OpenWithContext(ctx, dbPath)
}

func missingDBHint(dbPath string) {
	fmt.Fprintf(os.Stderr, "local store not found at %s — run `peerspace-pp-cli sync` first\n", resolveNovelDBPath(dbPath))
}

// loadListings reads venue/listing/search rows from the resources table and
// expands search envelopes into individual listings.
func loadListings(ctx context.Context, s *store.Store) ([]venuex.Listing, error) {
	if s == nil {
		return make([]venuex.Listing, 0), nil
	}
	rows, err := s.DB().QueryContext(ctx, `
SELECT id, data FROM resources
WHERE resource_type IN ('venues','listings','search','venues_list')
`)
	if err != nil {
		return nil, err
	}
	type pair struct {
		id   string
		data []byte
	}
	// Drain-first: close rows before any follow-up work.
	pairs := make([]pair, 0)
	for rows.Next() {
		var id sql.NullString
		var data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if !data.Valid || data.String == "" {
			continue
		}
		pairs = append(pairs, pair{id: id.String, data: []byte(data.String)})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	out := make([]venuex.Listing, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, venuex.ExpandResourceData(p.id, p.data)...)
	}
	return venuex.DedupeByID(out), nil
}

// loadFavoriteIDs extracts favorite listing ids from projects/fav_board style rows.
func loadFavoriteIDs(ctx context.Context, s *store.Store) ([]string, error) {
	if s == nil {
		return make([]string, 0), nil
	}
	rows, err := s.DB().QueryContext(ctx, `
SELECT id, data FROM resources
WHERE resource_type IN ('projects','fav_board','favorites','attachments','projects_fav_board')
`)
	if err != nil {
		return nil, err
	}
	pairs := make([][]byte, 0)
	for rows.Next() {
		var id sql.NullString
		var data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if data.Valid && data.String != "" {
			pairs = append(pairs, []byte(data.String))
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	ids := make([]string, 0)
	for _, data := range pairs {
		ids = append(ids, venuex.ExtractFavoriteIDs(data)...)
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

// loadProjects returns raw project resource rows (id + data JSON).
func loadProjects(ctx context.Context, s *store.Store) ([]map[string]any, error) {
	if s == nil {
		return make([]map[string]any, 0), nil
	}
	rows, err := s.DB().QueryContext(ctx, `
SELECT id, data FROM resources
WHERE resource_type IN ('projects','project','projects_details')
`)
	if err != nil {
		return nil, err
	}
	type pair struct {
		id   string
		data string
	}
	pairs := make([]pair, 0)
	for rows.Next() {
		var id, data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			_ = rows.Close()
			return nil, err
		}
		pairs = append(pairs, pair{id: id.String, data: data.String})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	out := make([]map[string]any, 0, len(pairs))
	for _, p := range pairs {
		row := map[string]any{"id": p.id}
		if p.data != "" {
			var m map[string]any
			if err := json.Unmarshal([]byte(p.data), &m); err == nil {
				row["data"] = m
			} else {
				row["data"] = p.data
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// loadCoverage reports resource_type counts and last_synced_at when available.
func loadCoverage(ctx context.Context, s *store.Store) ([]map[string]any, error) {
	if s == nil {
		return make([]map[string]any, 0), nil
	}
	rows, err := s.DB().QueryContext(ctx, `
SELECT resource_type, COUNT(*) AS n FROM resources GROUP BY resource_type ORDER BY resource_type
`)
	if err != nil {
		return nil, err
	}
	type countRow struct {
		rt string
		n  int
	}
	counts := make([]countRow, 0)
	for rows.Next() {
		var rt sql.NullString
		var n sql.NullInt64
		if err := rows.Scan(&rt, &n); err != nil {
			_ = rows.Close()
			return nil, err
		}
		counts = append(counts, countRow{rt: rt.String, n: int(n.Int64)})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	synced := map[string]string{}
	srows, err := s.DB().QueryContext(ctx, `SELECT resource_type, last_synced_at FROM sync_state`)
	if err == nil {
		for srows.Next() {
			var rt, ts sql.NullString
			if err := srows.Scan(&rt, &ts); err != nil {
				continue
			}
			if rt.Valid {
				synced[rt.String] = ts.String
			}
		}
		_ = srows.Close()
	}

	out := make([]map[string]any, 0, len(counts))
	seen := map[string]struct{}{}
	for _, c := range counts {
		row := map[string]any{
			"resource_type": c.rt,
			"count":         c.n,
		}
		if ts, ok := synced[c.rt]; ok && ts != "" {
			row["last_synced"] = ts
		}
		out = append(out, row)
		seen[c.rt] = struct{}{}
	}
	for rt, ts := range synced {
		if _, ok := seen[rt]; ok {
			continue
		}
		out = append(out, map[string]any{
			"resource_type": rt,
			"count":         0,
			"last_synced":   ts,
		})
	}
	return out, nil
}

// findListingByID looks up a single listing id across listing resource types.
func findListingByID(ctx context.Context, s *store.Store, id string) (venuex.Listing, json.RawMessage, bool, error) {
	if s == nil || id == "" {
		return venuex.Listing{}, nil, false, nil
	}
	rows, err := s.DB().QueryContext(ctx, `
SELECT id, data FROM resources
WHERE resource_type IN ('venues','listings','search','venues_list')
  AND (id = ? OR data LIKE ?)
`, id, "%"+id+"%")
	if err != nil {
		return venuex.Listing{}, nil, false, err
	}
	type pair struct {
		id   string
		data []byte
	}
	pairs := make([]pair, 0)
	for rows.Next() {
		var rid, data sql.NullString
		if err := rows.Scan(&rid, &data); err != nil {
			_ = rows.Close()
			return venuex.Listing{}, nil, false, err
		}
		if data.Valid {
			pairs = append(pairs, pair{id: rid.String, data: []byte(data.String)})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return venuex.Listing{}, nil, false, err
	}
	_ = rows.Close()

	for _, p := range pairs {
		for _, l := range venuex.ExpandResourceData(p.id, p.data) {
			if l.ID == id {
				raw := l.Raw
				if len(raw) == 0 {
					raw = p.data
				}
				return l, json.RawMessage(raw), true, nil
			}
		}
	}
	return venuex.Listing{}, nil, false, nil
}

// listingToRow converts a listing to a compact table/JSON row.
func listingToRow(l venuex.Listing) map[string]any {
	return map[string]any{
		"id":           l.ID,
		"title":        l.Title,
		"city":         l.City,
		"neighborhood": l.Neighborhood,
		"guests":       l.Guests,
		"price_hourly": l.PriceHourly,
		"currency":     l.Currency,
		"instant_book": l.InstantBook,
		"review_stars": l.ReviewStars,
		"review_count": l.ReviewCount,
		"space_type":   l.SpaceType,
		"format_fit":   l.FormatFit,
		"host_id":      l.HostID,
		"host_name":    l.HostName,
		"space_id":     l.SpaceID,
		"amenities":    l.Amenities,
		"about":        firstNonEmptyLocal(l.About, l.Description),
		"rules":        l.Rules,
		"parking":      l.Parking,
		"included":     l.Included,
		"cleaning":     l.Cleaning,
		"cancellation": l.Cancellation,
		"hydrated":     l.Hydrated,
		"sqft":         l.Sqft,
	}
}

func firstNonEmptyLocal(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
