// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

// Package store: snapshots.go adds a specialized deal_snapshots table for
// Slickdeals deal observations over time. Unlike the generic resources table
// (one row per resource keyed by id), deal_snapshots is append-only and keyed
// by (deal_id, captured_at) — every poll of a deal creates a new row so we
// can compute thumb velocity, top-stores aggregates, and freshness windows.
//
// The schema is created lazily on first write via EnsureSnapshotsSchema so
// existing Store consumers don't pay a migration cost on Open. Idempotent
// CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS statements make
// concurrent first-use calls safe.
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// DealSnapshot is one observation of a Slickdeals deal at a point in time.
// Append-only: every poll creates a new row, even if the deal_id repeats.
type DealSnapshot struct {
	ID         int64     `json:"id,omitempty"`
	DealID     string    `json:"deal_id"`
	CapturedAt time.Time `json:"captured_at"`
	Price      float64   `json:"price,omitempty"`
	ListPrice  float64   `json:"list_price,omitempty"`
	Thumbs     int       `json:"thumbs"`
	Comments   int       `json:"comments,omitempty"`
	Views      int       `json:"views,omitempty"`
	IsExpired  bool      `json:"is_expired,omitempty"`
	Merchant   string    `json:"merchant,omitempty"`
	Category   string    `json:"category,omitempty"`
	Title      string    `json:"title"`
	Link       string    `json:"link"`
	Raw        string    `json:"-"` // full original JSON; not surfaced
}

// DealFilter is the input to QueryDeals. All fields optional; non-zero fields
// AND-combined into the WHERE clause. Latest dedupes by deal_id post-filter.
type DealFilter struct {
	Store     string    // exact merchant match (case-insensitive)
	Category  string    // exact category match (case-insensitive)
	Since     time.Time // captured_at >= Since (zero value = no filter)
	Until     time.Time // captured_at < Until
	MinThumbs int       // thumbs >= MinThumbs (0 = no filter)
	DealID    string    // exact deal_id
	Limit     int       // 0 = no limit
	Latest    bool      // dedupe by deal_id taking max(captured_at)
}

// StoreStats is one row of the TopStores aggregation.
type StoreStats struct {
	Merchant  string    `json:"merchant"`
	DealCount int       `json:"deal_count"`
	AvgThumbs float64   `json:"avg_thumbs"`
	MaxThumbs int       `json:"max_thumbs"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// VelocityPoint is one observation in a thumbs-over-time series for a deal.
// Delta is thumbs - prev.thumbs (0 on the first point).
type VelocityPoint struct {
	CapturedAt time.Time `json:"captured_at"`
	Thumbs     int       `json:"thumbs"`
	Delta      int       `json:"delta"`
}

// snapshotsSchemaOnce gates schema creation: cheap idempotent gate so callers
// (Insert, QueryDeals, TopStores, ThumbsVelocity) can call EnsureSnapshotsSchema
// at the top of every method without paying for the round-trip after the first.
var snapshotsSchemaOnce sync.Map // map[*Store]*sync.Once

func (s *Store) ensureSchemaOnce() *sync.Once {
	v, _ := snapshotsSchemaOnce.LoadOrStore(s, &sync.Once{})
	return v.(*sync.Once)
}

// EnsureSnapshotsSchema creates the deal_snapshots table + indices if missing.
// Idempotent. Called automatically by mutating and reading methods on first
// use; safe to call directly if a caller wants to materialize the schema before
// querying (e.g. tests asserting table shape).
func (s *Store) EnsureSnapshotsSchema() error {
	var firstErr error
	s.ensureSchemaOnce().Do(func() {
		firstErr = s.createSnapshotsSchema()
	})
	return firstErr
}

func (s *Store) createSnapshotsSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS deal_snapshots (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			deal_id      TEXT NOT NULL,
			captured_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			price        REAL,
			list_price   REAL,
			thumbs       INTEGER,
			comments     INTEGER,
			views        INTEGER,
			is_expired   INTEGER DEFAULT 0,
			merchant     TEXT,
			category     TEXT,
			title        TEXT,
			link         TEXT,
			raw          TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_deal     ON deal_snapshots(deal_id, captured_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_merchant ON deal_snapshots(merchant, captured_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_captured ON deal_snapshots(captured_at DESC)`,
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("creating snapshots schema: %w", err)
		}
	}
	return nil
}

// InsertSnapshot writes a snapshot row. ID is set on return. If snap.CapturedAt
// is the zero value, time.Now() is substituted so callers can omit it.
func (s *Store) InsertSnapshot(snap *DealSnapshot) error {
	if snap == nil {
		return fmt.Errorf("nil snapshot")
	}
	if err := s.EnsureSnapshotsSchema(); err != nil {
		return err
	}
	if snap.CapturedAt.IsZero() {
		snap.CapturedAt = time.Now().UTC()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(
		`INSERT INTO deal_snapshots
		 (deal_id, captured_at, price, list_price, thumbs, comments, views,
		  is_expired, merchant, category, title, link, raw)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snap.DealID,
		snap.CapturedAt.UTC().Format(time.RFC3339Nano),
		snap.Price,
		snap.ListPrice,
		snap.Thumbs,
		snap.Comments,
		snap.Views,
		boolToInt(snap.IsExpired),
		snap.Merchant,
		snap.Category,
		snap.Title,
		snap.Link,
		snap.Raw,
	)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	snap.ID = id
	return nil
}

// QueryDeals returns snapshots matching filter. When filter.Latest, dedupes
// by deal_id keeping max(captured_at). Limit applied after dedupe.
func (s *Store) QueryDeals(filter DealFilter) ([]DealSnapshot, error) {
	if err := s.EnsureSnapshotsSchema(); err != nil {
		return nil, err
	}

	var (
		clauses []string
		args    []any
	)
	if filter.Store != "" {
		clauses = append(clauses, "LOWER(merchant) = LOWER(?)")
		args = append(args, filter.Store)
	}
	if filter.Category != "" {
		clauses = append(clauses, "LOWER(category) = LOWER(?)")
		args = append(args, filter.Category)
	}
	if !filter.Since.IsZero() {
		clauses = append(clauses, "captured_at >= ?")
		args = append(args, filter.Since.UTC().Format(time.RFC3339Nano))
	}
	if !filter.Until.IsZero() {
		clauses = append(clauses, "captured_at < ?")
		args = append(args, filter.Until.UTC().Format(time.RFC3339Nano))
	}
	if filter.MinThumbs > 0 {
		clauses = append(clauses, "thumbs >= ?")
		args = append(args, filter.MinThumbs)
	}
	if filter.DealID != "" {
		clauses = append(clauses, "deal_id = ?")
		args = append(args, filter.DealID)
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	// For the non-Latest path the WHERE clauses can be applied at the SQL
	// level and we're done. For the Latest path, we previously ran a correlated
	// subquery that computed MAX(captured_at) GLOBALLY per deal_id, which
	// silently dropped filtered-merchant rows whose latest snapshot was
	// captured under a DIFFERENT merchant (Greptile #3 in PR #481). The fix:
	// fetch all filtered rows ordered by captured_at DESC, then dedupe in Go.
	// This is the documented alternative in the Greptile feedback and avoids
	// growing the SQL with mirrored WHERE clauses that would have to stay in
	// sync with the outer filter forever.
	query := `SELECT id, deal_id, captured_at, price, list_price, thumbs,
	                comments, views, is_expired, merchant, category, title, link, raw
	         FROM deal_snapshots` + where + `
	         ORDER BY captured_at DESC`

	// Limit only applies to the non-Latest path at the SQL level; for Latest
	// we need to dedupe BEFORE limiting so we don't truncate to a smaller
	// subset that excludes deals whose latest match is further down the
	// captured_at ordering.
	if filter.Limit > 0 && !filter.Latest {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query deals: %w", err)
	}
	defer rows.Close()

	all, err := scanSnapshots(rows)
	if err != nil {
		return nil, err
	}

	if !filter.Latest {
		return all, nil
	}

	// Dedupe by deal_id keeping the first occurrence (which is the most
	// recent thanks to ORDER BY captured_at DESC at the SQL level). Apply
	// limit after dedupe.
	seen := make(map[string]bool, len(all))
	deduped := make([]DealSnapshot, 0, len(all))
	for _, s := range all {
		if seen[s.DealID] {
			continue
		}
		seen[s.DealID] = true
		deduped = append(deduped, s)
		if filter.Limit > 0 && len(deduped) >= filter.Limit {
			break
		}
	}
	return deduped, nil
}

// QuerySnapshotsSince returns snapshots with captured_at >= cutoff, deduped to
// latest-per-deal-id. limit==0 means no limit. Convenience wrapper for the
// watch+digest engineer's freshness window.
func (s *Store) QuerySnapshotsSince(cutoff time.Time, limit int) ([]DealSnapshot, error) {
	return s.QueryDeals(DealFilter{
		Since:  cutoff,
		Limit:  limit,
		Latest: true,
	})
}

// TopStores aggregates by merchant over the time window ending now. Window of
// 0 means "all time". limit==0 returns every merchant.
func (s *Store) TopStores(window time.Duration, limit int) ([]StoreStats, error) {
	if err := s.EnsureSnapshotsSchema(); err != nil {
		return nil, err
	}
	var (
		where string
		args  []any
	)
	if window > 0 {
		where = " WHERE captured_at >= ? AND merchant IS NOT NULL AND merchant != ''"
		args = append(args, time.Now().Add(-window).UTC().Format(time.RFC3339Nano))
	} else {
		where = " WHERE merchant IS NOT NULL AND merchant != ''"
	}
	query := `SELECT merchant,
	                 COUNT(DISTINCT deal_id) AS deal_count,
	                 AVG(thumbs)             AS avg_thumbs,
	                 MAX(thumbs)             AS max_thumbs,
	                 MIN(captured_at)        AS first_seen,
	                 MAX(captured_at)        AS last_seen
	         FROM deal_snapshots` + where + `
	         GROUP BY merchant
	         ORDER BY deal_count DESC, max_thumbs DESC`
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("top stores: %w", err)
	}
	defer rows.Close()

	var out []StoreStats
	for rows.Next() {
		var (
			st           StoreStats
			avg          sql.NullFloat64
			maxT         sql.NullInt64
			cnt          sql.NullInt64
			firstSeenStr sql.NullString
			lastSeenStr  sql.NullString
		)
		if err := rows.Scan(&st.Merchant, &cnt, &avg, &maxT, &firstSeenStr, &lastSeenStr); err != nil {
			return nil, fmt.Errorf("scan top stores: %w", err)
		}
		if cnt.Valid {
			st.DealCount = int(cnt.Int64)
		}
		if avg.Valid {
			st.AvgThumbs = avg.Float64
		}
		if maxT.Valid {
			st.MaxThumbs = int(maxT.Int64)
		}
		if firstSeenStr.Valid {
			st.FirstSeen = parseSnapshotTime(firstSeenStr.String)
		}
		if lastSeenStr.Valid {
			st.LastSeen = parseSnapshotTime(lastSeenStr.String)
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// ThumbsVelocity returns the time-series of thumbs observations for a single
// deal, ordered by captured_at ascending. Each point's Delta is thumbs minus
// the previous observation's thumbs; the first point has Delta=0.
// Empty result when no snapshots exist for the deal — not an error.
func (s *Store) ThumbsVelocity(dealID string) ([]VelocityPoint, error) {
	if err := s.EnsureSnapshotsSchema(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT captured_at, thumbs FROM deal_snapshots
		 WHERE deal_id = ?
		 ORDER BY captured_at ASC`,
		dealID,
	)
	if err != nil {
		return nil, fmt.Errorf("thumbs velocity: %w", err)
	}
	defer rows.Close()

	var (
		points []VelocityPoint
		prev   int
		first  = true
	)
	for rows.Next() {
		var (
			capturedAtStr string
			thumbs        sql.NullInt64
		)
		if err := rows.Scan(&capturedAtStr, &thumbs); err != nil {
			return nil, fmt.Errorf("scan velocity: %w", err)
		}
		t := parseSnapshotTime(capturedAtStr)
		curThumbs := 0
		if thumbs.Valid {
			curThumbs = int(thumbs.Int64)
		}
		delta := 0
		if !first {
			delta = curThumbs - prev
		}
		points = append(points, VelocityPoint{
			CapturedAt: t,
			Thumbs:     curThumbs,
			Delta:      delta,
		})
		prev = curThumbs
		first = false
	}
	return points, rows.Err()
}

// scanSnapshots reads rows produced by the canonical SELECT used in
// QueryDeals. Centralized so any new query that returns the same column
// order can reuse it.
func scanSnapshots(rows *sql.Rows) ([]DealSnapshot, error) {
	var out []DealSnapshot
	for rows.Next() {
		var (
			snap          DealSnapshot
			capturedAtStr string
			price         sql.NullFloat64
			listPrice     sql.NullFloat64
			thumbs        sql.NullInt64
			comments      sql.NullInt64
			views         sql.NullInt64
			isExpired     sql.NullInt64
			merchant      sql.NullString
			category      sql.NullString
			title         sql.NullString
			link          sql.NullString
			raw           sql.NullString
		)
		if err := rows.Scan(
			&snap.ID,
			&snap.DealID,
			&capturedAtStr,
			&price,
			&listPrice,
			&thumbs,
			&comments,
			&views,
			&isExpired,
			&merchant,
			&category,
			&title,
			&link,
			&raw,
		); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		snap.CapturedAt = parseSnapshotTime(capturedAtStr)
		if price.Valid {
			snap.Price = price.Float64
		}
		if listPrice.Valid {
			snap.ListPrice = listPrice.Float64
		}
		if thumbs.Valid {
			snap.Thumbs = int(thumbs.Int64)
		}
		if comments.Valid {
			snap.Comments = int(comments.Int64)
		}
		if views.Valid {
			snap.Views = int(views.Int64)
		}
		if isExpired.Valid {
			snap.IsExpired = isExpired.Int64 != 0
		}
		if merchant.Valid {
			snap.Merchant = merchant.String
		}
		if category.Valid {
			snap.Category = category.String
		}
		if title.Valid {
			snap.Title = title.String
		}
		if link.Valid {
			snap.Link = link.String
		}
		if raw.Valid {
			snap.Raw = raw.String
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// parseSnapshotTime tries the formats SQLite returns for our TIMESTAMP
// columns. modernc.org/sqlite round-trips RFC3339Nano cleanly when we bind
// time.Time-derived strings; CURRENT_TIMESTAMP defaults emit
// "YYYY-MM-DD HH:MM:SS" form, which we also tolerate.
func parseSnapshotTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
