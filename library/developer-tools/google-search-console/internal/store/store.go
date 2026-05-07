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
	return s, nil
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
