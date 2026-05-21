// PATCH(crawl-stats: SQLite tables + upserts for crawl-stats poll samples).
// Each poll caps at ~1000 URL samples; unioning across polls is the only way
// to get full URL coverage. The tables here back the `crawl-stats union`
// subcommand. See .printing-press-patches.json patch id: crawl-stats and the
// discovery report under manuscripts/google-search-console/amend-2026-05-21T1402.

package store

import (
	"context"
	"errors"
	"time"
)

// CrawlStatsSampleRow is one row in crawl_stats_samples.
type CrawlStatsSampleRow struct {
	SiteURL       string
	SampleURL     string
	FileType      string    // empty when not from a by-type poll
	ResponseCode  int       // 0 when not observed
	GooglebotType string    // empty when not from a by-googlebot poll
	FetchedAt     time.Time // when GSC says the sample was crawled
	SizeBytes     int64
	ResponseMs    int
	PollAt        time.Time // wall-clock time of this poll
	RawJSON       string    // best-effort raw shard (for forensics)
}

// CrawlStatsTotalsRow is one row in crawl_stats_totals.
type CrawlStatsTotalsRow struct {
	SiteURL           string
	FilterDim         string // "" (overview), "file_type", "response", "googlebot_type", "purpose"
	FilterCode        int
	PollAt            time.Time
	CrawlRequests     int64
	DownloadSizeBytes int64
	AvgResponseMs     int64
}

// CrawlStatsTimeSeriesRow is one row in crawl_stats_time_series.
type CrawlStatsTimeSeriesRow struct {
	SiteURL       string
	FilterDim     string
	FilterCode    int
	Date          string // YYYY-MM-DD
	CrawlRequests int64
	PollAt        time.Time
}

// migrateCrawlStats creates the three crawl-stats tables. Called from the
// main migrate() loop. Separate function to keep migrate() readable.
func (s *Store) migrateCrawlStats(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS crawl_stats_samples (
			site_url TEXT NOT NULL,
			sample_url TEXT NOT NULL,
			file_type TEXT NOT NULL DEFAULT '',
			response_code INTEGER NOT NULL DEFAULT 0,
			googlebot_type TEXT NOT NULL DEFAULT '',
			fetched_at TEXT NOT NULL DEFAULT '',
			size_bytes INTEGER NOT NULL DEFAULT 0,
			response_ms INTEGER NOT NULL DEFAULT 0,
			poll_at TEXT NOT NULL,
			raw_json TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (site_url, sample_url, fetched_at, poll_at)
		)`,
		`CREATE INDEX IF NOT EXISTS crawl_stats_samples_site ON crawl_stats_samples(site_url, poll_at)`,
		`CREATE INDEX IF NOT EXISTS crawl_stats_samples_filters ON crawl_stats_samples(site_url, file_type, response_code, googlebot_type)`,

		`CREATE TABLE IF NOT EXISTS crawl_stats_totals (
			site_url TEXT NOT NULL,
			filter_dim TEXT NOT NULL DEFAULT '',
			filter_code INTEGER NOT NULL DEFAULT 0,
			poll_at TEXT NOT NULL,
			crawl_requests INTEGER NOT NULL DEFAULT 0,
			download_size_bytes INTEGER NOT NULL DEFAULT 0,
			avg_response_ms INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (site_url, filter_dim, filter_code, poll_at)
		)`,
		`CREATE INDEX IF NOT EXISTS crawl_stats_totals_site ON crawl_stats_totals(site_url, poll_at)`,

		`CREATE TABLE IF NOT EXISTS crawl_stats_time_series (
			site_url TEXT NOT NULL,
			filter_dim TEXT NOT NULL DEFAULT '',
			filter_code INTEGER NOT NULL DEFAULT 0,
			date TEXT NOT NULL,
			crawl_requests INTEGER NOT NULL DEFAULT 0,
			poll_at TEXT NOT NULL,
			PRIMARY KEY (site_url, filter_dim, filter_code, date, poll_at)
		)`,
		`CREATE INDEX IF NOT EXISTS crawl_stats_time_series_site ON crawl_stats_time_series(site_url, poll_at)`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

// UpsertCrawlStatsSamples bulk-inserts a batch of sample rows in one
// transaction. Conflicts on the natural primary key are no-ops (first-write-
// wins) — a re-poll on the same (site, sample_url, fetched_at, poll_at)
// shouldn't happen in practice but we don't want a dupe to error the call.
func (s *Store) UpsertCrawlStatsSamples(ctx context.Context, rows []CrawlStatsSampleRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO crawl_stats_samples (
			site_url, sample_url, file_type, response_code, googlebot_type,
			fetched_at, size_bytes, response_ms, poll_at, raw_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_url, sample_url, fetched_at, poll_at) DO NOTHING`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if r.SiteURL == "" || r.SampleURL == "" {
			continue
		}
		fetchedAt := ""
		if !r.FetchedAt.IsZero() {
			fetchedAt = r.FetchedAt.Format(time.RFC3339)
		}
		pollAt := r.PollAt
		if pollAt.IsZero() {
			pollAt = time.Now().UTC()
		}
		if _, err := stmt.ExecContext(ctx,
			r.SiteURL, r.SampleURL, r.FileType, r.ResponseCode, r.GooglebotType,
			fetchedAt, r.SizeBytes, r.ResponseMs, pollAt.Format(time.RFC3339), r.RawJSON,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// UpsertCrawlStatsTotals writes one totals row per poll.
func (s *Store) UpsertCrawlStatsTotals(ctx context.Context, row CrawlStatsTotalsRow) error {
	if row.SiteURL == "" {
		return errors.New("UpsertCrawlStatsTotals: SiteURL required")
	}
	if row.PollAt.IsZero() {
		row.PollAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO crawl_stats_totals (
			site_url, filter_dim, filter_code, poll_at,
			crawl_requests, download_size_bytes, avg_response_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_url, filter_dim, filter_code, poll_at) DO UPDATE SET
			crawl_requests = excluded.crawl_requests,
			download_size_bytes = excluded.download_size_bytes,
			avg_response_ms = excluded.avg_response_ms`,
		row.SiteURL, row.FilterDim, row.FilterCode, row.PollAt.Format(time.RFC3339),
		row.CrawlRequests, row.DownloadSizeBytes, row.AvgResponseMs)
	return err
}

// UpsertCrawlStatsTimeSeries writes a batch of time-series points in one tx.
func (s *Store) UpsertCrawlStatsTimeSeries(ctx context.Context, rows []CrawlStatsTimeSeriesRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO crawl_stats_time_series (
			site_url, filter_dim, filter_code, date, crawl_requests, poll_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_url, filter_dim, filter_code, date, poll_at) DO UPDATE SET
			crawl_requests = excluded.crawl_requests`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if r.SiteURL == "" || r.Date == "" {
			continue
		}
		pollAt := r.PollAt
		if pollAt.IsZero() {
			pollAt = time.Now().UTC()
		}
		if _, err := stmt.ExecContext(ctx,
			r.SiteURL, r.FilterDim, r.FilterCode, r.Date, r.CrawlRequests,
			pollAt.Format(time.RFC3339),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// QueryCrawlStatsSamplesUnion returns the deduplicated union of all sample
// URLs ever polled for a site, optionally filtered by dimension. Unioning
// across polls is the entire reason the SQLite layer exists for crawl-stats
// (each individual poll caps at ~1000 URLs).
func (s *Store) QueryCrawlStatsSamplesUnion(ctx context.Context, siteURL, fileType, googlebotType string, responseCode int, limit int) ([]CrawlStatsSampleRow, error) {
	if siteURL == "" {
		return nil, errors.New("QueryCrawlStatsSamplesUnion: SiteURL required")
	}
	q := `SELECT site_url, sample_url, file_type, response_code, googlebot_type,
		  fetched_at, size_bytes, response_ms, poll_at, raw_json
	      FROM crawl_stats_samples
	      WHERE site_url = ?`
	args := []any{siteURL}
	if fileType != "" {
		q += ` AND file_type = ?`
		args = append(args, fileType)
	}
	if googlebotType != "" {
		q += ` AND googlebot_type = ?`
		args = append(args, googlebotType)
	}
	if responseCode > 0 {
		q += ` AND response_code = ?`
		args = append(args, responseCode)
	}
	q += ` GROUP BY sample_url ORDER BY MAX(poll_at) DESC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CrawlStatsSampleRow{}
	for rows.Next() {
		var r CrawlStatsSampleRow
		var fetchedAtStr, pollAtStr string
		if err := rows.Scan(
			&r.SiteURL, &r.SampleURL, &r.FileType, &r.ResponseCode, &r.GooglebotType,
			&fetchedAtStr, &r.SizeBytes, &r.ResponseMs, &pollAtStr, &r.RawJSON,
		); err != nil {
			return nil, err
		}
		if fetchedAtStr != "" {
			if t, err := time.Parse(time.RFC3339, fetchedAtStr); err == nil {
				r.FetchedAt = t
			}
		}
		if pollAtStr != "" {
			if t, err := time.Parse(time.RFC3339, pollAtStr); err == nil {
				r.PollAt = t
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountCrawlStatsSamples returns the number of unique sample URLs for a site.
func (s *Store) CountCrawlStatsSamples(ctx context.Context, siteURL string) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT sample_url) FROM crawl_stats_samples WHERE site_url = ?`,
		siteURL).Scan(&n)
	return n, err
}
