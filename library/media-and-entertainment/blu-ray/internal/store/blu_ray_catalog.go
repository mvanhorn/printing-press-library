package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (s *Store) LockedExec(ctx context.Context, stmt string, args ...any) (sql.Result, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.DB().ExecContext(ctx, stmt, args...)
}

func (s *Store) LockedExecTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

func (s *Store) MigrateBluRayCatalog() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS releases_catalog (
			id INTEGER PRIMARY KEY,
			kind TEXT NOT NULL,
			slug TEXT NOT NULL,
			title_normalized TEXT,
			country TEXT,
			year_hint INTEGER,
			lastmod TEXT,
			fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS releases_catalog_kind ON releases_catalog(kind)`,
		`CREATE INDEX IF NOT EXISTS releases_catalog_country ON releases_catalog(country)`,
		`CREATE INDEX IF NOT EXISTS releases_catalog_year ON releases_catalog(year_hint)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS releases_fts USING fts5(
			title_normalized, slug, distributor UNINDEXED, country UNINDEXED, kind UNINDEXED, year UNINDEXED,
			content='releases_catalog', content_rowid='id'
		)`,
		`CREATE TRIGGER IF NOT EXISTS releases_catalog_ai AFTER INSERT ON releases_catalog BEGIN
			INSERT INTO releases_fts(rowid, title_normalized, slug, distributor, country, kind, year)
			VALUES (new.id, new.title_normalized, new.slug, '', new.country, new.kind, CAST(new.year_hint AS TEXT));
		END`,
		`CREATE TRIGGER IF NOT EXISTS releases_catalog_ad AFTER DELETE ON releases_catalog BEGIN
			INSERT INTO releases_fts(releases_fts, rowid, title_normalized, slug, distributor, country, kind, year)
			VALUES('delete', old.id, old.title_normalized, old.slug, '', old.country, old.kind, CAST(old.year_hint AS TEXT));
		END`,
		`CREATE TRIGGER IF NOT EXISTS releases_catalog_au AFTER UPDATE ON releases_catalog BEGIN
			INSERT INTO releases_fts(releases_fts, rowid, title_normalized, slug, distributor, country, kind, year)
			VALUES('delete', old.id, old.title_normalized, old.slug, '', old.country, old.kind, CAST(old.year_hint AS TEXT));
			INSERT INTO releases_fts(rowid, title_normalized, slug, distributor, country, kind, year)
			VALUES (new.id, new.title_normalized, new.slug, '', new.country, new.kind, CAST(new.year_hint AS TEXT));
		END`,
		`CREATE TABLE IF NOT EXISTS news_catalog (
			id INTEGER PRIMARY KEY,
			url TEXT NOT NULL,
			title TEXT,
			publication_date TEXT,
			fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS watchlist (
			release_id INTEGER PRIMARY KEY,
			target_price REAL,
			added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			low_seen REAL,
			alerted_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS price_history (
			release_id INTEGER NOT NULL,
			retailer_id INTEGER NOT NULL,
			observed_at TIMESTAMP NOT NULL,
			price REAL NOT NULL,
			PRIMARY KEY (release_id, retailer_id, observed_at)
		)`,
		`DROP INDEX IF EXISTS price_history_release`,
		`CREATE INDEX IF NOT EXISTS price_history_release_retailer_observed ON price_history(release_id, retailer_id, observed_at)`,
		`CREATE TABLE IF NOT EXISTS sitemap_snapshot (
			taken_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			sitemap_name TEXT NOT NULL,
			url_count INTEGER NOT NULL,
			url_set_hash TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS upc_index (
			upc TEXT PRIMARY KEY,
			release_id INTEGER NOT NULL
		)`,
	}
	for _, stmt := range migrations {
		if _, err := s.DB().Exec(stmt); err != nil {
			return fmt.Errorf("migrating Blu-ray catalog schema: %w", err)
		}
	}
	return nil
}

type CatalogRow struct {
	ID              int
	Kind            string
	Slug            string
	TitleNormalized string
	Country         string
	YearHint        int
	Lastmod         string
}

type CatalogSearchOpts struct {
	Query   string
	Format  string
	Year    string
	Country string
	Limit   int
}

type NewsRow struct {
	ID              int
	URL             string
	Title           string
	PublicationDate string
}

type WatchlistRow struct {
	ReleaseID   int
	TargetPrice float64
	AddedAt     string
	LowSeen     sql.NullFloat64
	AlertedAt   sql.NullString
	Title       sql.NullString
	CurrentLow  sql.NullFloat64
}

type PriceObservation struct {
	ReleaseID  int
	RetailerID int
	ObservedAt string
	Price      float64
}

type SitemapSnapshot struct {
	TakenAt     string
	SitemapName string
	URLCount    int
	URLSetHash  string
}

type CatalogStats struct {
	TotalRows  int
	RowsByKind map[string]int
}

func (s *Store) SearchCatalog(ctx context.Context, opts CatalogSearchOpts) ([]CatalogRow, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 25
	}
	var rows *sql.Rows
	var err error
	if strings.TrimSpace(opts.Query) == "" {
		where, args := catalogFilters(opts.Format, opts.Year, opts.Country)
		args = append(args, limit)
		rows, err = s.DB().QueryContext(ctx, `SELECT id, kind, slug, title_normalized, country, COALESCE(year_hint, 0), lastmod
			FROM releases_catalog`+where+` ORDER BY id LIMIT ?`, args...)
	} else {
		where, args := catalogJoinFilters(opts.Format, opts.Year, opts.Country)
		args = append([]any{opts.Query}, args...)
		args = append(args, limit)
		rows, err = s.DB().QueryContext(ctx, `SELECT c.id, c.kind, c.slug, c.title_normalized, c.country, COALESCE(c.year_hint, 0), c.lastmod
			FROM releases_fts f JOIN releases_catalog c ON c.id = f.rowid
			WHERE releases_fts MATCH ?`+strings.Replace(where, " WHERE ", " AND ", 1)+`
			ORDER BY rank LIMIT ?`, args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCatalogRows(rows)
}

func (s *Store) ListCatalog(ctx context.Context, kind string, limit int) ([]CatalogRow, error) {
	if limit <= 0 {
		limit = 1000000
	}
	return s.SearchCatalog(ctx, CatalogSearchOpts{Format: kind, Limit: limit})
}

func catalogFilters(format, year, country string) (string, []any) {
	return catalogFiltersWithPrefix("", format, year, country)
}

func catalogJoinFilters(format, year, country string) (string, []any) {
	return catalogFiltersWithPrefix("c.", format, year, country)
}

func catalogFiltersWithPrefix(prefix, format, year, country string) (string, []any) {
	var where []string
	var args []any
	if format != "" {
		where = append(where, prefix+"kind = ?")
		args = append(args, format)
	}
	if year != "" {
		where = append(where, prefix+"year_hint = ?")
		args = append(args, year)
	}
	if country != "" {
		where = append(where, prefix+"country = ?")
		args = append(args, country)
	}
	if len(where) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

func scanCatalogRows(rows *sql.Rows) ([]CatalogRow, error) {
	var out []CatalogRow
	for rows.Next() {
		var r CatalogRow
		var title, country, lastmod sql.NullString
		if err := rows.Scan(&r.ID, &r.Kind, &r.Slug, &title, &country, &r.YearHint, &lastmod); err != nil {
			return nil, err
		}
		r.TitleNormalized = nullString(title)
		r.Country = nullString(country)
		r.Lastmod = nullString(lastmod)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetRelease(ctx context.Context, id int) (CatalogRow, bool, error) {
	rows, err := s.DB().QueryContext(ctx, `SELECT id, kind, slug, title_normalized, country, COALESCE(year_hint, 0), lastmod
		FROM releases_catalog WHERE id = ?`, id)
	if err != nil {
		return CatalogRow{}, false, err
	}
	defer rows.Close()
	out, err := scanCatalogRows(rows)
	if err != nil {
		return CatalogRow{}, false, err
	}
	if len(out) == 0 {
		return CatalogRow{}, false, nil
	}
	return out[0], true, nil
}

func (s *Store) UpsertCatalogRows(ctx context.Context, rows []CatalogRow) error {
	if len(rows) == 0 {
		return nil
	}
	return s.LockedExecTx(ctx, func(tx *sql.Tx) error {
		for _, r := range rows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO releases_catalog(id, kind, slug, title_normalized, country, year_hint, lastmod)
				VALUES(?, ?, ?, ?, ?, NULLIF(?, 0), ?)
				ON CONFLICT(id) DO UPDATE SET kind=excluded.kind, slug=excluded.slug, title_normalized=excluded.title_normalized, country=excluded.country, year_hint=excluded.year_hint, lastmod=excluded.lastmod`,
				r.ID, r.Kind, r.Slug, r.TitleNormalized, r.Country, r.YearHint, r.Lastmod); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) UpsertNewsRows(ctx context.Context, rows []NewsRow) error {
	if len(rows) == 0 {
		return nil
	}
	return s.LockedExecTx(ctx, func(tx *sql.Tx) error {
		for _, r := range rows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO news_catalog(id, url, title, publication_date)
				VALUES(?, ?, ?, ?)
				ON CONFLICT(id) DO UPDATE SET url=excluded.url, title=excluded.title, publication_date=excluded.publication_date`,
				r.ID, r.URL, r.Title, r.PublicationDate); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) RecordSitemapSnapshot(ctx context.Context, name string, urlCount int, hash string) error {
	_, err := s.LockedExec(ctx, `INSERT INTO sitemap_snapshot(taken_at, sitemap_name, url_count, url_set_hash) VALUES(CURRENT_TIMESTAMP, ?, ?, ?)`, name, urlCount, hash)
	return err
}

func (s *Store) ListSitemapSnapshots(ctx context.Context, sitemapNameLike string, sinceISO string) ([]SitemapSnapshot, error) {
	query := `SELECT taken_at, sitemap_name, url_count, url_set_hash FROM sitemap_snapshot`
	var where []string
	var args []any
	if sitemapNameLike != "" {
		where = append(where, `sitemap_name LIKE ? ESCAPE '\'`)
		args = append(args, sitemapNameLike)
	}
	if sinceISO != "" {
		where = append(where, "taken_at >= ?")
		args = append(args, sinceISO)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY taken_at ASC"
	rows, err := s.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SitemapSnapshot
	for rows.Next() {
		var r SitemapSnapshot
		if err := rows.Scan(&r.TakenAt, &r.SitemapName, &r.URLCount, &r.URLSetHash); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListWatchlist(ctx context.Context) ([]WatchlistRow, error) {
	rows, err := s.DB().QueryContext(ctx, `SELECT w.release_id, COALESCE(w.target_price, 0), w.added_at, w.low_seen, w.alerted_at,
		c.title_normalized, (SELECT MIN(price) FROM price_history p WHERE p.release_id=w.release_id)
		FROM watchlist w LEFT JOIN releases_catalog c ON c.id=w.release_id ORDER BY w.added_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchlistRow
	for rows.Next() {
		var r WatchlistRow
		if err := rows.Scan(&r.ReleaseID, &r.TargetPrice, &r.AddedAt, &r.LowSeen, &r.AlertedAt, &r.Title, &r.CurrentLow); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) AddToWatchlist(ctx context.Context, releaseID int, targetPrice float64) error {
	_, err := s.LockedExec(ctx, `INSERT INTO watchlist(release_id, target_price, added_at)
		VALUES(?, NULLIF(?, 0), CURRENT_TIMESTAMP)
		ON CONFLICT(release_id) DO UPDATE SET target_price=excluded.target_price`, releaseID, targetPrice)
	return err
}

func (s *Store) RemoveFromWatchlist(ctx context.Context, releaseID int) (int64, error) {
	res, err := s.LockedExec(ctx, `DELETE FROM watchlist WHERE release_id=?`, releaseID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) MarkWatchlistAlerted(ctx context.Context, releaseID int, low float64) error {
	_, err := s.LockedExec(ctx, `UPDATE watchlist SET
		low_seen = CASE WHEN low_seen IS NULL OR ? < low_seen THEN ? ELSE low_seen END,
		alerted_at = CURRENT_TIMESTAMP
		WHERE release_id=?`, low, low, releaseID)
	return err
}

func (s *Store) UpdateWatchlistLow(ctx context.Context, releaseID int, low float64) error {
	_, err := s.LockedExec(ctx, `UPDATE watchlist SET low_seen = CASE WHEN low_seen IS NULL OR ? < low_seen THEN ? ELSE low_seen END WHERE release_id=?`, low, low, releaseID)
	return err
}

func (s *Store) RecordPrice(ctx context.Context, p PriceObservation) error {
	_, err := s.LockedExec(ctx, `INSERT OR REPLACE INTO price_history(release_id, retailer_id, observed_at, price) VALUES(?, ?, ?, ?)`, p.ReleaseID, p.RetailerID, p.ObservedAt, p.Price)
	return err
}

func (s *Store) GetPriceHistory(ctx context.Context, releaseID int, retailerID int) ([]PriceObservation, error) {
	query := `SELECT release_id, retailer_id, observed_at, price FROM price_history WHERE release_id=?`
	args := []any{releaseID}
	if retailerID > 0 {
		query += ` AND retailer_id=?`
		args = append(args, retailerID)
	}
	query += ` ORDER BY observed_at`
	rows, err := s.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PriceObservation
	for rows.Next() {
		var r PriceObservation
		if err := rows.Scan(&r.ReleaseID, &r.RetailerID, &r.ObservedAt, &r.Price); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ResolveUPC(ctx context.Context, upc string) (releaseID int, ok bool, err error) {
	err = s.DB().QueryRowContext(ctx, `SELECT release_id FROM upc_index WHERE upc=?`, upc).Scan(&releaseID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return releaseID, true, nil
}

func (s *Store) CatalogStats(ctx context.Context) (CatalogStats, error) {
	stats := CatalogStats{RowsByKind: map[string]int{}}
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM releases_catalog`).Scan(&stats.TotalRows); err != nil {
		return stats, err
	}
	rows, err := s.DB().QueryContext(ctx, `SELECT kind, COUNT(*) FROM releases_catalog GROUP BY kind ORDER BY kind`)
	if err != nil {
		return stats, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			return stats, err
		}
		stats.RowsByKind[kind] = count
	}
	return stats, rows.Err()
}

func nullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}
