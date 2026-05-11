// Package store backs the time-series and snapshot novel features
// (decay, cannibalize, coverage-drift, opportunity, etc.) with a local
// SQLite database. The schema is intentionally narrow: enough to express
// the cross-time joins that the live Search Console API cannot.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// StoreSchemaVersion is the schema version this binary expects on the
// SQLite store. Persisted via SQLite's PRAGMA user_version and checked on
// every Open so a binary upgrade against an incompatible on-disk schema
// fails loudly rather than corrupting reads.
//
// Bump this constant whenever schemaStatements gains or alters a table in
// a way that older binaries cannot read; users will get a clear error
// directing them to reset the store.
const StoreSchemaVersion = 1

// Store wraps a *sql.DB plus the resolved path for diagnostics.
type Store struct {
	db   *sql.DB
	Path string
}

// DefaultPath returns the conventional database location.
// Honors GOOGLE_SEARCH_CONSOLE_DB_PATH for tests and overrides.
func DefaultPath() string {
	if env := os.Getenv("GOOGLE_SEARCH_CONSOLE_DB_PATH"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "google-search-console-pp-cli", "store.sqlite")
}

// Open opens (creating if necessary) the SQLite database at path and
// applies the latest schema. Safe to call from any command.
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating store dir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("opening store at %s: %w", path, err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db, Path: path}
	if err := s.migrate(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := s.checkSchemaVersion(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// checkSchemaVersion reads SQLite's user_version pragma and compares it to
// the binary's StoreSchemaVersion. Fresh databases (user_version=0) are
// stamped with the current version; mismatched versions fail with a
// recovery hint rather than letting reads return wrong shapes.
func (s *Store) checkSchemaVersion(ctx context.Context) error {
	var v int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("reading user_version: %w", err)
	}
	if v == 0 {
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", StoreSchemaVersion)); err != nil {
			return fmt.Errorf("setting user_version: %w", err)
		}
		return nil
	}
	if v != StoreSchemaVersion {
		return fmt.Errorf("store schema v%d on disk but binary expects v%d; remove %s and re-sync", v, StoreSchemaVersion, s.Path)
	}
	return nil
}

// DB returns the underlying *sql.DB for direct queries.
func (s *Store) DB() *sql.DB { return s.db }

// Close releases the connection.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	for _, stmt := range schemaStatements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrating: %w (stmt: %.80s)", err, stmt)
		}
	}
	return nil
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS search_analytics_rows (
		site_url TEXT NOT NULL,
		search_type TEXT NOT NULL DEFAULT 'web',
		date TEXT NOT NULL,
		query TEXT NOT NULL DEFAULT '',
		page TEXT NOT NULL DEFAULT '',
		country TEXT NOT NULL DEFAULT '',
		device TEXT NOT NULL DEFAULT '',
		search_appearance TEXT NOT NULL DEFAULT '',
		clicks REAL NOT NULL DEFAULT 0,
		impressions REAL NOT NULL DEFAULT 0,
		ctr REAL NOT NULL DEFAULT 0,
		position REAL NOT NULL DEFAULT 0,
		ingested_at TEXT NOT NULL,
		PRIMARY KEY (site_url, search_type, date, query, page, country, device, search_appearance)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_sar_site_date ON search_analytics_rows(site_url, date)`,
	`CREATE INDEX IF NOT EXISTS idx_sar_query ON search_analytics_rows(site_url, query)`,
	`CREATE INDEX IF NOT EXISTS idx_sar_page ON search_analytics_rows(site_url, page)`,

	`CREATE TABLE IF NOT EXISTS sites_snapshots (
		snapshot_at TEXT NOT NULL,
		site_url TEXT NOT NULL,
		permission_level TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (snapshot_at, site_url)
	)`,

	`CREATE TABLE IF NOT EXISTS sitemaps_snapshots (
		snapshot_at TEXT NOT NULL,
		site_url TEXT NOT NULL,
		feed_path TEXT NOT NULL,
		last_submitted TEXT NOT NULL DEFAULT '',
		last_downloaded TEXT NOT NULL DEFAULT '',
		is_pending INTEGER NOT NULL DEFAULT 0,
		is_sitemaps_index INTEGER NOT NULL DEFAULT 0,
		errors INTEGER NOT NULL DEFAULT 0,
		warnings INTEGER NOT NULL DEFAULT 0,
		contents TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (snapshot_at, site_url, feed_path)
	)`,

	`CREATE TABLE IF NOT EXISTS url_inspections (
		inspected_at TEXT NOT NULL,
		site_url TEXT NOT NULL,
		page_url TEXT NOT NULL,
		coverage_state TEXT NOT NULL DEFAULT '',
		google_canonical TEXT NOT NULL DEFAULT '',
		user_canonical TEXT NOT NULL DEFAULT '',
		last_crawl_time TEXT NOT NULL DEFAULT '',
		page_fetch_state TEXT NOT NULL DEFAULT '',
		indexing_state TEXT NOT NULL DEFAULT '',
		robots_txt_state TEXT NOT NULL DEFAULT '',
		mobile_verdict TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (inspected_at, site_url, page_url)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_inspections_site_page ON url_inspections(site_url, page_url, inspected_at DESC)`,

	`CREATE TABLE IF NOT EXISTS sync_runs (
		started_at TEXT NOT NULL,
		finished_at TEXT NOT NULL DEFAULT '',
		site_url TEXT NOT NULL DEFAULT '',
		kind TEXT NOT NULL,
		rows INTEGER NOT NULL DEFAULT 0,
		notes TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (started_at, site_url, kind)
	)`,

	// Annotations: agent-owned notebook. Backs annotate / triage-state /
	// any future watch-rule features. target_type discriminates the entity
	// (page | query | site | triage). expires_at is nullable; populated for
	// snoozes so list queries can transparently filter out expired entries.
	`CREATE TABLE IF NOT EXISTS annotations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		target_type TEXT NOT NULL,
		target TEXT NOT NULL,
		note TEXT NOT NULL DEFAULT '',
		tags TEXT NOT NULL DEFAULT '',
		expires_at TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_annotations_target ON annotations(target_type, target)`,
	`CREATE INDEX IF NOT EXISTS idx_annotations_tags ON annotations(tags)`,
}

// Annotation is the in-memory shape of an agent-owned note attached to a
// page, query, site, or triage item. tags is the raw stored string —
// callers split on comma when filtering.
type Annotation struct {
	ID         int64
	TargetType string
	Target     string
	Note       string
	Tags       string
	ExpiresAt  string
	CreatedAt  string
	UpdatedAt  string
}

// AddAnnotation inserts a new annotation and returns its assigned id.
// Existing annotations on the same (target_type, target) are not merged
// — callers wanting upsert semantics should ListAnnotations + Remove
// before insert. expires defaults to empty (never expires).
func (s *Store) AddAnnotation(ctx context.Context, targetType, target, note, tags, expires string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var expiresArg any
	if expires != "" {
		expiresArg = expires
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO annotations(target_type, target, note, tags, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		targetType, target, note, tags, expiresArg, now, now)
	if err != nil {
		return 0, fmt.Errorf("inserting annotation: %w", err)
	}
	return res.LastInsertId()
}

// ListAnnotations returns annotations filtered by target_type (empty =
// any), target prefix-or-exact (empty = any), and tag substring (empty
// = any). Expired entries are excluded unless includeExpired is true.
func (s *Store) ListAnnotations(ctx context.Context, targetType, target, tagFilter string, includeExpired bool) ([]Annotation, error) {
	q := `SELECT id, target_type, target, note, tags,
	             COALESCE(expires_at, ''), created_at, updated_at
	      FROM annotations WHERE 1=1`
	args := []any{}
	if targetType != "" {
		q += " AND target_type = ?"
		args = append(args, targetType)
	}
	if target != "" {
		q += " AND target = ?"
		args = append(args, target)
	}
	if tagFilter != "" {
		q += " AND tags LIKE ?"
		args = append(args, "%"+tagFilter+"%")
	}
	if !includeExpired {
		q += " AND (expires_at IS NULL OR expires_at = '' OR expires_at > ?)"
		args = append(args, time.Now().UTC().Format(time.RFC3339))
	}
	q += " ORDER BY id DESC"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying annotations: %w", err)
	}
	defer rows.Close()
	out := []Annotation{}
	for rows.Next() {
		var a Annotation
		if err := rows.Scan(&a.ID, &a.TargetType, &a.Target, &a.Note, &a.Tags,
			&a.ExpiresAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning annotation: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating annotations: %w", err)
	}
	return out, nil
}

// RemoveAnnotation deletes by id. Returns the number of rows affected
// (0 when the id doesn't exist).
func (s *Store) RemoveAnnotation(ctx context.Context, id int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM annotations WHERE id = ?`, id)
	if err != nil {
		return 0, fmt.Errorf("removing annotation: %w", err)
	}
	return res.RowsAffected()
}

// AnalyticsRow is the in-memory row shape for batch insert.
type AnalyticsRow struct {
	SiteURL          string
	SearchType       string
	Date             string
	Query            string
	Page             string
	Country          string
	Device           string
	SearchAppearance string
	Clicks           float64
	Impressions      float64
	CTR              float64
	Position         float64
}

// UpsertAnalytics replaces rows for the given primary-key tuples.
func (s *Store) UpsertAnalytics(ctx context.Context, rows []AnalyticsRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO search_analytics_rows
		(site_url, search_type, date, query, page, country, device, search_appearance,
		 clicks, impressions, ctr, position, ingested_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	written := 0
	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.SiteURL, r.SearchType, r.Date, r.Query, r.Page,
			r.Country, r.Device, r.SearchAppearance, r.Clicks, r.Impressions, r.CTR, r.Position, now); err != nil {
			return written, err
		}
		written++
	}
	return written, tx.Commit()
}

// SnapshotSites records the site list at the given timestamp.
func (s *Store) SnapshotSites(ctx context.Context, snapshotAt string, sites []SiteRow) error {
	if len(sites) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, st := range sites {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO sites_snapshots(snapshot_at, site_url, permission_level) VALUES (?, ?, ?)`,
			snapshotAt, st.SiteURL, st.PermissionLevel); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SiteRow is a normalized site list entry for snapshotting.
type SiteRow struct {
	SiteURL         string
	PermissionLevel string
}

// SnapshotSitemaps records sitemap status for a site at a timestamp.
func (s *Store) SnapshotSitemaps(ctx context.Context, snapshotAt string, smaps []SitemapRow) error {
	if len(smaps) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, sm := range smaps {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO sitemaps_snapshots
			(snapshot_at, site_url, feed_path, last_submitted, last_downloaded, is_pending, is_sitemaps_index, errors, warnings, contents)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			snapshotAt, sm.SiteURL, sm.FeedPath, sm.LastSubmitted, sm.LastDownloaded,
			boolToInt(sm.IsPending), boolToInt(sm.IsSitemapsIndex), sm.Errors, sm.Warnings, sm.Contents); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SitemapRow is a flattened sitemap entry for snapshotting.
type SitemapRow struct {
	SiteURL         string
	FeedPath        string
	LastSubmitted   string
	LastDownloaded  string
	IsPending       bool
	IsSitemapsIndex bool
	Errors          int64
	Warnings        int64
	Contents        string // semicolon-joined "type:submitted/indexed" tuples
}

// SaveURLInspection writes one inspection observation.
func (s *Store) SaveURLInspection(ctx context.Context, r URLInspectionRow) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO url_inspections
		(inspected_at, site_url, page_url, coverage_state, google_canonical, user_canonical,
		 last_crawl_time, page_fetch_state, indexing_state, robots_txt_state, mobile_verdict)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.InspectedAt, r.SiteURL, r.PageURL, r.CoverageState, r.GoogleCanonical, r.UserCanonical,
		r.LastCrawlTime, r.PageFetchState, r.IndexingState, r.RobotsTxtState, r.MobileVerdict)
	return err
}

// URLInspectionRow is the persisted shape of one inspection.
type URLInspectionRow struct {
	InspectedAt     string
	SiteURL         string
	PageURL         string
	CoverageState   string
	GoogleCanonical string
	UserCanonical   string
	LastCrawlTime   string
	PageFetchState  string
	IndexingState   string
	RobotsTxtState  string
	MobileVerdict   string
}

// RecordSyncRun appends a sync metadata row.
func (s *Store) RecordSyncRun(ctx context.Context, startedAt, finishedAt, siteURL, kind string, rows int64, notes string) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO sync_runs
		(started_at, finished_at, site_url, kind, rows, notes) VALUES (?, ?, ?, ?, ?, ?)`,
		startedAt, finishedAt, siteURL, kind, rows, notes)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
