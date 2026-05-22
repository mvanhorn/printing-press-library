// Copyright 2026 justinwfu. Licensed under Apache-2.0.

// art-goat extension to the generated store: unified `works` and `sits`
// tables driving the contemplative spine.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const worksAndSitsSchema = `
CREATE TABLE IF NOT EXISTS works (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    title TEXT,
    creator TEXT,
    creator_canonical TEXT,
    date_text TEXT,
    date_start INTEGER,
    date_end INTEGER,
    medium TEXT,
    classification TEXT,
    period TEXT,
    culture_region TEXT,
    description TEXT,
    image_url TEXT,
    thumbnail_url TEXT,
    license TEXT,
    source_url TEXT,
    raw_json TEXT,
    synced_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_works_source ON works(source);
CREATE INDEX IF NOT EXISTS idx_works_creator ON works(creator_canonical);
CREATE INDEX IF NOT EXISTS idx_works_date_start ON works(date_start);
CREATE INDEX IF NOT EXISTS idx_works_culture ON works(culture_region);
CREATE INDEX IF NOT EXISTS idx_works_medium ON works(medium);

-- Manually managed FTS5 table (no content= coupling so DELETE works).
CREATE VIRTUAL TABLE IF NOT EXISTS works_fts USING fts5(
    work_id UNINDEXED,
    title, creator, description, medium, period
);

CREATE TABLE IF NOT EXISTS sits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    work_id TEXT,
    duration_seconds INTEGER,
    prompt TEXT,
    reflection TEXT,
    mood INTEGER,
    tags TEXT,
    mode TEXT
);

CREATE INDEX IF NOT EXISTS idx_sits_started ON sits(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_sits_work ON sits(work_id);

-- Manually managed FTS5 table.
CREATE VIRTUAL TABLE IF NOT EXISTS sits_fts USING fts5(
    sit_id UNINDEXED,
    reflection, prompt, tags
);

-- today_picks caches the deterministic daily pick keyed by (pick_date,
-- mode) so repeated 'today' invocations on the same calendar day return
-- the same work, why, and prompt. The pick_date is the LOCAL date the
-- pick was made (contemplative practice is felt in local time). Mode
-- is the empty string for default rotation, or e.g. 'bridge-from-last'
-- for moded picks — distinct modes get distinct daily caches so a user
-- switching policies doesn't get stuck with the first-mode pick.
CREATE TABLE IF NOT EXISTS today_picks (
    pick_date TEXT NOT NULL,
    mode TEXT NOT NULL DEFAULT '',
    work_id TEXT NOT NULL,
    why TEXT NOT NULL,
    prompt TEXT NOT NULL,
    chosen_at TIMESTAMP NOT NULL,
    PRIMARY KEY (pick_date, mode)
);
`

// EnsureArtGoatTables idempotently creates the works + sits schema and
// their FTS5 indexes. Safe to call on every CLI invocation; the IF NOT
// EXISTS guards make it a no-op once provisioned.
//
// Also performs a one-time schema migration: if a legacy FTS5 table
// exists without the work_id/sit_id columns this version expects, drop
// and recreate it. The FTS tables are derived from works/sits so dropping
// them is safe; they get rebuilt on the next upsert/insert.
func (s *Store) EnsureArtGoatTables(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := s.db.ExecContext(ctx, worksAndSitsSchema); err != nil {
		return fmt.Errorf("ensure art-goat tables: %w", err)
	}
	// Migrate legacy FTS5 schemas. Old shape used content='works'/'sits'
	// which couples to the parent table by rowid and doesn't expose a
	// work_id/sit_id column. Detect by probing for the column.
	if !s.columnExists(ctx, "works_fts", "work_id") {
		if _, err := s.db.ExecContext(ctx, `DROP TABLE IF EXISTS works_fts`); err != nil {
			return fmt.Errorf("drop legacy works_fts: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, `
CREATE VIRTUAL TABLE works_fts USING fts5(
    work_id UNINDEXED,
    title, creator, description, medium, period
)`); err != nil {
			return fmt.Errorf("recreate works_fts: %w", err)
		}
		// Backfill from existing works rows.
		_, _ = s.db.ExecContext(ctx, `
INSERT INTO works_fts (work_id, title, creator, description, medium, period)
SELECT id, title, creator, description, medium, period FROM works`)
	}
	if !s.columnExists(ctx, "sits_fts", "sit_id") {
		if _, err := s.db.ExecContext(ctx, `DROP TABLE IF EXISTS sits_fts`); err != nil {
			return fmt.Errorf("drop legacy sits_fts: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, `
CREATE VIRTUAL TABLE sits_fts USING fts5(
    sit_id UNINDEXED,
    reflection, prompt, tags
)`); err != nil {
			return fmt.Errorf("recreate sits_fts: %w", err)
		}
		// Backfill from existing sits rows.
		_, _ = s.db.ExecContext(ctx, `
INSERT INTO sits_fts (sit_id, reflection, prompt, tags)
SELECT id, reflection, prompt, tags FROM sits`)
	}
	return nil
}

// columnExists returns true when the named column appears in the named
// table's pragma table_info output. Used to detect legacy FTS5 shapes
// before this version's columns were added.
func (s *Store) columnExists(ctx context.Context, table, column string) bool {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info("%s")`, table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

// Work mirrors source.Work for store-internal use (avoids cross-package
// dependency from store → source). Conversion happens in the upsert call
// site.
type Work struct {
	ID               string
	Source           string
	SourceID         string
	Title            string
	Creator          string
	CreatorCanonical string
	DateText         string
	DateStart        int
	DateEnd          int
	Medium           string
	Classification   string
	Period           string
	CultureRegion    string
	Description      string
	ImageURL         string
	ThumbnailURL     string
	License          string
	SourceURL        string
	RawJSON          string
	SyncedAt         time.Time
}

// UpsertWork inserts or replaces a single work and refreshes its FTS row.
// Uses a transaction so the works row and FTS index can't get out of sync.
func (s *Store) UpsertWork(ctx context.Context, w Work) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin upsert tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
INSERT OR REPLACE INTO works (
    id, source, source_id, title, creator, creator_canonical,
    date_text, date_start, date_end, medium, classification, period,
    culture_region, description, image_url, thumbnail_url, license,
    source_url, raw_json, synced_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
`,
		w.ID, w.Source, w.SourceID, w.Title, w.Creator, w.CreatorCanonical,
		w.DateText, w.DateStart, w.DateEnd, w.Medium, w.Classification, w.Period,
		w.CultureRegion, w.Description, w.ImageURL, w.ThumbnailURL, w.License,
		w.SourceURL, w.RawJSON, w.SyncedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert works: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM works_fts WHERE work_id = ?`, w.ID); err != nil {
		return fmt.Errorf("delete works_fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO works_fts (work_id, title, creator, description, medium, period)
VALUES (?, ?, ?, ?, ?, ?)`,
		w.ID, w.Title, w.Creator, w.Description, w.Medium, w.Period,
	); err != nil {
		return fmt.Errorf("insert works_fts: %w", err)
	}
	return tx.Commit()
}

// UpsertWorksBatch inserts/replaces many works in one transaction. Faster
// than calling UpsertWork in a loop for sync paths that produce ~5k rows.
func (s *Store) UpsertWorksBatch(ctx context.Context, works []Work) (int, error) {
	if len(works) == 0 {
		return 0, nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin batch tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
INSERT OR REPLACE INTO works (
    id, source, source_id, title, creator, creator_canonical,
    date_text, date_start, date_end, medium, classification, period,
    culture_region, description, image_url, thumbnail_url, license,
    source_url, raw_json, synced_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
`)
	if err != nil {
		return 0, fmt.Errorf("prepare works upsert: %w", err)
	}
	defer stmt.Close()

	ftsDel, err := tx.PrepareContext(ctx, `DELETE FROM works_fts WHERE work_id = ?`)
	if err != nil {
		return 0, fmt.Errorf("prepare fts delete: %w", err)
	}
	defer ftsDel.Close()

	ftsIns, err := tx.PrepareContext(ctx, `
INSERT INTO works_fts (work_id, title, creator, description, medium, period)
VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare fts insert: %w", err)
	}
	defer ftsIns.Close()

	count := 0
	for _, w := range works {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if _, err := stmt.ExecContext(ctx,
			w.ID, w.Source, w.SourceID, w.Title, w.Creator, w.CreatorCanonical,
			w.DateText, w.DateStart, w.DateEnd, w.Medium, w.Classification, w.Period,
			w.CultureRegion, w.Description, w.ImageURL, w.ThumbnailURL, w.License,
			w.SourceURL, w.RawJSON, w.SyncedAt,
		); err != nil {
			return count, fmt.Errorf("upsert work %s: %w", w.ID, err)
		}
		if _, err := ftsDel.ExecContext(ctx, w.ID); err != nil {
			return count, fmt.Errorf("fts delete %s: %w", w.ID, err)
		}
		if _, err := ftsIns.ExecContext(ctx, w.ID, w.Title, w.Creator, w.Description, w.Medium, w.Period); err != nil {
			return count, fmt.Errorf("fts insert %s: %w", w.ID, err)
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit batch: %w", err)
	}
	return count, nil
}

// GetWork fetches one work by ID, or nil if not present.
func (s *Store) GetWork(ctx context.Context, id string) (*Work, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, source, source_id, title, creator, creator_canonical,
       date_text, date_start, date_end, medium, classification, period,
       culture_region, description, image_url, thumbnail_url, license,
       source_url, raw_json, synced_at
FROM works WHERE id = ?`, id)
	w := &Work{}
	err := row.Scan(
		&w.ID, &w.Source, &w.SourceID, &w.Title, &w.Creator, &w.CreatorCanonical,
		&w.DateText, &w.DateStart, &w.DateEnd, &w.Medium, &w.Classification, &w.Period,
		&w.CultureRegion, &w.Description, &w.ImageURL, &w.ThumbnailURL, &w.License,
		&w.SourceURL, &w.RawJSON, &w.SyncedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get work %s: %w", id, err)
	}
	return w, nil
}

// RandomWork picks one work, optionally restricted by source. Returns nil
// if the works table is empty (or the source filter matches nothing).
// Avoid pieces in excludeIDs (used by today's anti-repeat logic).
func (s *Store) RandomWork(ctx context.Context, sources []string, excludeIDs []string) (*Work, error) {
	var where []string
	var args []any
	if len(sources) > 0 {
		placeholders := strings.Repeat("?,", len(sources))
		placeholders = strings.TrimRight(placeholders, ",")
		where = append(where, "source IN ("+placeholders+")")
		for _, s := range sources {
			args = append(args, s)
		}
	}
	if len(excludeIDs) > 0 {
		placeholders := strings.Repeat("?,", len(excludeIDs))
		placeholders = strings.TrimRight(placeholders, ",")
		where = append(where, "id NOT IN ("+placeholders+")")
		for _, id := range excludeIDs {
			args = append(args, id)
		}
	}
	q := "SELECT id FROM works"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY RANDOM() LIMIT 1"

	var id string
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("random work: %w", err)
	}
	return s.GetWork(ctx, id)
}

// WorkCounts returns a per-source count of synced works.
func (s *Store) WorkCounts(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT source, COUNT(*) FROM works GROUP BY source ORDER BY source`)
	if err != nil {
		return nil, fmt.Errorf("work counts: %w", err)
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var src string
		var n int
		if err := rows.Scan(&src, &n); err != nil {
			return nil, err
		}
		out[src] = n
	}
	return out, rows.Err()
}

// SearchWorks performs an FTS5 query over works_fts and returns hits up
// to limit, ordered by rank. Empty query returns recent works by
// synced_at as a sensible default.
func (s *Store) SearchWorks(ctx context.Context, query string, limit int) ([]Work, error) {
	if limit <= 0 {
		limit = 20
	}
	if strings.TrimSpace(query) == "" {
		return s.recentWorks(ctx, limit)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT w.id, w.source, w.source_id, w.title, w.creator, w.creator_canonical,
       w.date_text, w.date_start, w.date_end, w.medium, w.classification, w.period,
       w.culture_region, w.description, w.image_url, w.thumbnail_url, w.license,
       w.source_url, w.raw_json, w.synced_at
FROM works_fts JOIN works w ON w.id = works_fts.work_id
WHERE works_fts MATCH ?
ORDER BY rank
LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search works: %w", err)
	}
	defer rows.Close()
	return scanWorks(rows)
}

func (s *Store) recentWorks(ctx context.Context, limit int) ([]Work, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, source, source_id, title, creator, creator_canonical,
       date_text, date_start, date_end, medium, classification, period,
       culture_region, description, image_url, thumbnail_url, license,
       source_url, raw_json, synced_at
FROM works ORDER BY synced_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent works: %w", err)
	}
	defer rows.Close()
	return scanWorks(rows)
}

func scanWorks(rows *sql.Rows) ([]Work, error) {
	var out []Work
	for rows.Next() {
		w := Work{}
		if err := rows.Scan(
			&w.ID, &w.Source, &w.SourceID, &w.Title, &w.Creator, &w.CreatorCanonical,
			&w.DateText, &w.DateStart, &w.DateEnd, &w.Medium, &w.Classification, &w.Period,
			&w.CultureRegion, &w.Description, &w.ImageURL, &w.ThumbnailURL, &w.License,
			&w.SourceURL, &w.RawJSON, &w.SyncedAt,
		); err != nil {
			return out, fmt.Errorf("scan work: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// WorksByCreator returns works whose creator_canonical contains substr
// (case-insensitive substring match). When source is non-empty, results
// are restricted to that source. Ordered chronologically by date_start
// ascending so a creator query reads as a career arc.
func (s *Store) WorksByCreator(ctx context.Context, substr, source string, limit int) ([]Work, error) {
	if limit <= 0 {
		limit = 20
	}
	pattern := "%" + strings.ToLower(strings.TrimSpace(substr)) + "%"
	args := []any{pattern}
	q := `
SELECT id, source, source_id, title, creator, creator_canonical,
       date_text, date_start, date_end, medium, classification, period,
       culture_region, description, image_url, thumbnail_url, license,
       source_url, raw_json, synced_at
FROM works
WHERE LOWER(creator_canonical) LIKE ?`
	if strings.TrimSpace(source) != "" {
		q += ` AND source = ?`
		args = append(args, source)
	}
	q += ` ORDER BY date_start ASC, id ASC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("works by creator: %w", err)
	}
	defer rows.Close()
	return scanWorks(rows)
}

// WorksByStructuredSimilarity returns works that share at least one of
// the given dimensions (medium, culture region, canonical creator) with
// a seed work. Used as a fallback by `similar` when the FTS5 query
// returns nothing. The seed itself is excluded via excludeID. Ordered
// chronologically by date_start so the result reads as related context.
func (s *Store) WorksByStructuredSimilarity(ctx context.Context, medium, region, creatorCanonical, excludeID string, limit int) ([]Work, error) {
	if limit <= 0 {
		limit = 10
	}
	var clauses []string
	var args []any
	if strings.TrimSpace(medium) != "" {
		clauses = append(clauses, "LOWER(medium) = ?")
		args = append(args, strings.ToLower(medium))
	}
	if strings.TrimSpace(region) != "" {
		clauses = append(clauses, "LOWER(culture_region) = ?")
		args = append(args, strings.ToLower(region))
	}
	if strings.TrimSpace(creatorCanonical) != "" {
		clauses = append(clauses, "LOWER(creator_canonical) = ?")
		args = append(args, strings.ToLower(creatorCanonical))
	}
	if len(clauses) == 0 {
		return nil, nil
	}
	q := `
SELECT id, source, source_id, title, creator, creator_canonical,
       date_text, date_start, date_end, medium, classification, period,
       culture_region, description, image_url, thumbnail_url, license,
       source_url, raw_json, synced_at
FROM works
WHERE (` + strings.Join(clauses, " OR ") + `)`
	if strings.TrimSpace(excludeID) != "" {
		q += ` AND id <> ?`
		args = append(args, excludeID)
	}
	q += ` ORDER BY date_start ASC, id ASC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("structured similarity: %w", err)
	}
	defer rows.Close()
	return scanWorks(rows)
}

// BrowseFilter captures the optional, narrow filters supported by the
// `browse` command. Empty fields mean "no constraint on this axis".
// Substring fields (Medium, Region) match via LIKE %term%.
type BrowseFilter struct {
	Source string
	Medium string
	Region string
	Limit  int
	Offset int
}

// BrowseWorks paginates the unified works table with optional filters,
// ordered deterministically by id. Used by the `browse` command to give
// agents and humans a stable cursor over the local corpus.
func (s *Store) BrowseWorks(ctx context.Context, f BrowseFilter) ([]Work, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	var where []string
	var args []any
	if strings.TrimSpace(f.Source) != "" {
		where = append(where, "source = ?")
		args = append(args, f.Source)
	}
	if strings.TrimSpace(f.Medium) != "" {
		where = append(where, "LOWER(medium) LIKE ?")
		args = append(args, "%"+strings.ToLower(f.Medium)+"%")
	}
	if strings.TrimSpace(f.Region) != "" {
		where = append(where, "LOWER(culture_region) LIKE ?")
		args = append(args, "%"+strings.ToLower(f.Region)+"%")
	}

	q := `
SELECT id, source, source_id, title, creator, creator_canonical,
       date_text, date_start, date_end, medium, classification, period,
       culture_region, description, image_url, thumbnail_url, license,
       source_url, raw_json, synced_at
FROM works`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY id LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("browse works: %w", err)
	}
	defer rows.Close()
	return scanWorks(rows)
}

// CompactAndReindex runs SQLite VACUUM to reclaim free pages and rebuilds
// the works_fts and sits_fts indexes by deleting their contents and
// re-inserting from the parent tables. Returns the total number of FTS
// rows rebuilt (works + sits) so callers can confirm the index is healthy
// after the run.
//
// VACUUM cannot run inside a transaction, and it acquires a brief
// exclusive lock on the database — callers should invoke this from an
// admin-style command and ensure no concurrent sit is in flight.
func (s *Store) CompactAndReindex(ctx context.Context) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Rebuild works_fts.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM works_fts`); err != nil {
		return 0, fmt.Errorf("clear works_fts: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO works_fts (work_id, title, creator, description, medium, period)
SELECT id, title, creator, description, medium, period FROM works`)
	if err != nil {
		return 0, fmt.Errorf("rebuild works_fts: %w", err)
	}
	worksRows, _ := res.RowsAffected()

	// Rebuild sits_fts.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sits_fts`); err != nil {
		return int(worksRows), fmt.Errorf("clear sits_fts: %w", err)
	}
	res2, err := s.db.ExecContext(ctx, `
INSERT INTO sits_fts (sit_id, reflection, prompt, tags)
SELECT id, reflection, prompt, tags FROM sits`)
	if err != nil {
		return int(worksRows), fmt.Errorf("rebuild sits_fts: %w", err)
	}
	sitsRows, _ := res2.RowsAffected()

	// VACUUM must run outside any open transaction. Our autocommit Exec
	// path satisfies that here.
	if _, err := s.db.ExecContext(ctx, `VACUUM`); err != nil {
		return int(worksRows + sitsRows), fmt.Errorf("vacuum: %w", err)
	}
	return int(worksRows + sitsRows), nil
}
