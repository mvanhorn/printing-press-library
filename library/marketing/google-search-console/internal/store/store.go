// Package store provides the local SQLite cache that powers the offline
// transcendence commands (quick-wins, cannibalization, compare, cliff,
// roll-up, coverage-drift, historical, outliers, sitemap-watch, decaying,
// new-queries). Pure-Go via modernc.org/sqlite (no cgo).
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store is the offline-first SQLite cache used by sync, search, sql, and the
// transcendence commands.
type Store struct {
	db   *sql.DB
	path string
	fts  bool
}

// Site is a verified Search Console property.
type Site struct {
	SiteURL         string
	PermissionLevel string
}

// SitemapRow captures one (site, feedpath) sitemap snapshot.
type SitemapRow struct {
	SiteURL         string
	Feedpath        string
	Type            string
	IsPending       bool
	IsSitemapsIndex bool
	LastSubmitted   string
	LastDownloaded  string
	Errors          int64
	Warnings        int64
	ContentsJSON    string
	SnapshotAt      time.Time
}

// SearchAnalyticsRow is one row of the search-analytics workhorse table.
type SearchAnalyticsRow struct {
	SiteURL          string
	Date             string
	Query            string
	Page             string
	Country          string
	Device           string
	SearchAppearance string
	SearchType       string
	Clicks           float64
	Impressions      float64
	CTR              float64
	Position         float64
}

// URLInspectionRow is a single URL-inspection snapshot.
type URLInspectionRow struct {
	SiteURL         string
	InspectionURL   string
	SnapshotAt      time.Time
	Verdict         string
	CoverageState   string
	RobotsTxtState  string
	IndexingState   string
	PageFetchState  string
	LastCrawlTime   string
	GoogleCanonical string
	UserCanonical   string
	CrawledAs       string
	RawJSON         string
}

// DefaultDBPath returns the canonical user-cache location for the store.
func DefaultDBPath() string {
	if h, err := os.UserCacheDir(); err == nil {
		return filepath.Join(h, "google-search-console-pp-cli", "store.db")
	}
	return filepath.Join(os.TempDir(), "google-search-console-pp-cli-store.db")
}

// Open opens the SQLite store. An empty path uses DefaultDBPath().
// The directory is created if it doesn't exist.
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		path = DefaultDBPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("ensure store dir: %w", err)
	}

	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db, path: path}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB exposes the raw *sql.DB for hand-rolled queries (sql/search/transcendence).
func (s *Store) DB() *sql.DB {
	return s.db
}

// HasFTS5 reports whether FTS5 is available on this build of modernc.org/sqlite.
func (s *Store) HasFTS5() bool {
	return s.fts
}

// Path returns the on-disk file path.
func (s *Store) Path() string {
	return s.path
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sites (
			site_url TEXT PRIMARY KEY,
			permission_level TEXT,
			synced_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS sitemaps (
			site_url TEXT NOT NULL,
			feedpath TEXT NOT NULL,
			type TEXT,
			is_pending INTEGER,
			is_sitemaps_index INTEGER,
			last_submitted TEXT,
			last_downloaded TEXT,
			errors INTEGER,
			warnings INTEGER,
			contents_json TEXT,
			snapshot_at TEXT NOT NULL,
			PRIMARY KEY (site_url, feedpath, snapshot_at)
		)`,
		`CREATE INDEX IF NOT EXISTS sitemaps_site_snapshot ON sitemaps(site_url, snapshot_at)`,

		`CREATE TABLE IF NOT EXISTS search_analytics_rows (
			site_url TEXT NOT NULL,
			date TEXT NOT NULL,
			query TEXT NOT NULL DEFAULT '',
			page TEXT NOT NULL DEFAULT '',
			country TEXT NOT NULL DEFAULT '',
			device TEXT NOT NULL DEFAULT '',
			search_appearance TEXT NOT NULL DEFAULT '',
			search_type TEXT NOT NULL DEFAULT 'WEB',
			clicks REAL,
			impressions REAL,
			ctr REAL,
			position REAL,
			PRIMARY KEY (site_url, date, query, page, country, device, search_appearance, search_type)
		)`,
		`CREATE INDEX IF NOT EXISTS sar_site_date ON search_analytics_rows(site_url, date)`,
		`CREATE INDEX IF NOT EXISTS sar_site_query ON search_analytics_rows(site_url, query)`,
		`CREATE INDEX IF NOT EXISTS sar_site_page ON search_analytics_rows(site_url, page)`,

		`CREATE TABLE IF NOT EXISTS url_inspections (
			site_url TEXT NOT NULL,
			inspection_url TEXT NOT NULL,
			snapshot_at TEXT NOT NULL,
			verdict TEXT,
			coverage_state TEXT,
			robots_txt_state TEXT,
			indexing_state TEXT,
			page_fetch_state TEXT,
			last_crawl_time TEXT,
			google_canonical TEXT,
			user_canonical TEXT,
			crawled_as TEXT,
			raw_json TEXT,
			PRIMARY KEY (site_url, inspection_url, snapshot_at)
		)`,
		`CREATE INDEX IF NOT EXISTS url_inspections_site_url ON url_inspections(site_url, inspection_url)`,
	}

	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("migrate: %w (stmt: %.80s...)", err, q)
		}
	}

	// FTS5 is best-effort: try to create the contentless FTS index. Some
	// modernc.org/sqlite builds don't ship FTS5; if that's the case, the
	// search command falls back to LIKE.
	ftsStmt := `CREATE VIRTUAL TABLE IF NOT EXISTS sar_fts USING fts5(
		site_url UNINDEXED,
		date UNINDEXED,
		query,
		page,
		content='search_analytics_rows',
		content_rowid='rowid'
	)`
	if _, err := s.db.ExecContext(ctx, ftsStmt); err == nil {
		s.fts = true
	}
	return nil
}

// UpsertSite inserts or replaces a site row.
func (s *Store) UpsertSite(ctx context.Context, site Site) error {
	if site.SiteURL == "" {
		return errors.New("UpsertSite: empty SiteURL")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sites (site_url, permission_level, synced_at)
		VALUES (?, ?, ?)
		ON CONFLICT(site_url) DO UPDATE SET
			permission_level = excluded.permission_level,
			synced_at = excluded.synced_at`,
		site.SiteURL, site.PermissionLevel, time.Now().UTC().Format(time.RFC3339))
	return err
}

// UpsertSitemap inserts or replaces a sitemap snapshot row.
func (s *Store) UpsertSitemap(ctx context.Context, row SitemapRow) error {
	if row.SiteURL == "" || row.Feedpath == "" {
		return errors.New("UpsertSitemap: SiteURL and Feedpath required")
	}
	if row.SnapshotAt.IsZero() {
		row.SnapshotAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sitemaps (
			site_url, feedpath, type, is_pending, is_sitemaps_index,
			last_submitted, last_downloaded, errors, warnings,
			contents_json, snapshot_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_url, feedpath, snapshot_at) DO UPDATE SET
			type = excluded.type,
			is_pending = excluded.is_pending,
			is_sitemaps_index = excluded.is_sitemaps_index,
			last_submitted = excluded.last_submitted,
			last_downloaded = excluded.last_downloaded,
			errors = excluded.errors,
			warnings = excluded.warnings,
			contents_json = excluded.contents_json`,
		row.SiteURL, row.Feedpath, row.Type, row.IsPending, row.IsSitemapsIndex,
		row.LastSubmitted, row.LastDownloaded, row.Errors, row.Warnings,
		row.ContentsJSON, row.SnapshotAt.Format(time.RFC3339))
	return err
}

// UpsertSearchAnalyticsRow inserts or replaces one analytics row.
func (s *Store) UpsertSearchAnalyticsRow(ctx context.Context, row SearchAnalyticsRow) error {
	if row.SiteURL == "" || row.Date == "" {
		return errors.New("UpsertSearchAnalyticsRow: SiteURL and Date required")
	}
	if row.SearchType == "" {
		row.SearchType = "WEB"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO search_analytics_rows (
			site_url, date, query, page, country, device,
			search_appearance, search_type,
			clicks, impressions, ctr, position
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_url, date, query, page, country, device, search_appearance, search_type) DO UPDATE SET
			clicks = excluded.clicks,
			impressions = excluded.impressions,
			ctr = excluded.ctr,
			position = excluded.position`,
		row.SiteURL, row.Date, row.Query, row.Page, row.Country, row.Device,
		row.SearchAppearance, row.SearchType,
		row.Clicks, row.Impressions, row.CTR, row.Position)
	return err
}

// CountSearchAnalyticsRows returns the number of rows currently stored for a site.
// The transcendence commands use this as an empty-store guardrail before running
// queries that would otherwise return zero rows for unhelpful reasons.
func (s *Store) CountSearchAnalyticsRows(ctx context.Context, siteURL string) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM search_analytics_rows WHERE site_url = ?`, siteURL).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// BulkUpsertSearchAnalyticsRows is a batch helper that wraps a transaction.
func (s *Store) BulkUpsertSearchAnalyticsRows(ctx context.Context, rows []SearchAnalyticsRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO search_analytics_rows (
			site_url, date, query, page, country, device,
			search_appearance, search_type,
			clicks, impressions, ctr, position
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_url, date, query, page, country, device, search_appearance, search_type) DO UPDATE SET
			clicks = excluded.clicks,
			impressions = excluded.impressions,
			ctr = excluded.ctr,
			position = excluded.position`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, r := range rows {
		if r.SiteURL == "" || r.Date == "" {
			continue
		}
		if r.SearchType == "" {
			r.SearchType = "WEB"
		}
		if _, err := stmt.ExecContext(ctx,
			r.SiteURL, r.Date, r.Query, r.Page, r.Country, r.Device,
			r.SearchAppearance, r.SearchType,
			r.Clicks, r.Impressions, r.CTR, r.Position); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// UpsertUrlInspection inserts or replaces a URL inspection snapshot.
func (s *Store) UpsertUrlInspection(ctx context.Context, row URLInspectionRow) error {
	if row.SiteURL == "" || row.InspectionURL == "" {
		return errors.New("UpsertUrlInspection: SiteURL and InspectionURL required")
	}
	if row.SnapshotAt.IsZero() {
		row.SnapshotAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO url_inspections (
			site_url, inspection_url, snapshot_at, verdict,
			coverage_state, robots_txt_state, indexing_state,
			page_fetch_state, last_crawl_time, google_canonical,
			user_canonical, crawled_as, raw_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_url, inspection_url, snapshot_at) DO UPDATE SET
			verdict = excluded.verdict,
			coverage_state = excluded.coverage_state,
			robots_txt_state = excluded.robots_txt_state,
			indexing_state = excluded.indexing_state,
			page_fetch_state = excluded.page_fetch_state,
			last_crawl_time = excluded.last_crawl_time,
			google_canonical = excluded.google_canonical,
			user_canonical = excluded.user_canonical,
			crawled_as = excluded.crawled_as,
			raw_json = excluded.raw_json`,
		row.SiteURL, row.InspectionURL, row.SnapshotAt.Format(time.RFC3339),
		row.Verdict, row.CoverageState, row.RobotsTxtState, row.IndexingState,
		row.PageFetchState, row.LastCrawlTime, row.GoogleCanonical,
		row.UserCanonical, row.CrawledAs, row.RawJSON)
	return err
}

// LastSyncedDate returns the most-recent date present in search_analytics_rows
// for the given site, or an empty string when no rows exist.
func (s *Store) LastSyncedDate(ctx context.Context, siteURL string) (string, error) {
	var d sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT MAX(date) FROM search_analytics_rows WHERE site_url = ?`, siteURL).Scan(&d)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(d.String), nil
}
