// BLS-specific store tables for the novel-feature commands. The generator
// emits a generic SQLite scaffold (resources, series, surveys, FTS); these
// tables hold the curated catalog, release calendar, and observation cache
// that power the transcendence features.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/bls/internal/blsdata"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/bls/internal/store"
)

// migrateBLSTables creates the BLS-specific tables and FTS indexes if they
// don't exist. Safe to call on every command invocation. Returns nil if the
// store was opened read-only and the tables are already present.
func migrateBLSTables(ctx context.Context, db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS series_catalog (
			id TEXT PRIMARY KEY,
			title TEXT,
			survey TEXT,
			area TEXT,
			item TEXT,
			units TEXT,
			adjust TEXT,
			data JSON,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS series_catalog_fts USING fts5(
			id, title, survey, area, item, content='series_catalog', content_rowid='rowid', tokenize='porter unicode61'
		)`,
		`CREATE TRIGGER IF NOT EXISTS series_catalog_ai AFTER INSERT ON series_catalog BEGIN
			INSERT INTO series_catalog_fts(rowid, id, title, survey, area, item)
			VALUES (new.rowid, new.id, new.title, new.survey, new.area, new.item);
		END`,
		`CREATE TRIGGER IF NOT EXISTS series_catalog_au AFTER UPDATE ON series_catalog BEGIN
			INSERT INTO series_catalog_fts(series_catalog_fts, rowid, id, title, survey, area, item)
			VALUES('delete', old.rowid, old.id, old.title, old.survey, old.area, old.item);
			INSERT INTO series_catalog_fts(rowid, id, title, survey, area, item)
			VALUES (new.rowid, new.id, new.title, new.survey, new.area, new.item);
		END`,
		`CREATE TABLE IF NOT EXISTS bls_releases (
			id TEXT PRIMARY KEY,
			release_date DATETIME,
			release_time TEXT,
			survey TEXT,
			title TEXT,
			url TEXT,
			period TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_bls_releases_date ON bls_releases(release_date)`,
		`CREATE INDEX IF NOT EXISTS idx_bls_releases_survey ON bls_releases(survey)`,
		`CREATE TABLE IF NOT EXISTS observations (
			series_id TEXT,
			year INTEGER,
			period TEXT,
			period_name TEXT,
			value REAL,
			footnotes TEXT,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (series_id, year, period)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_observations_series ON observations(series_id, year DESC, period DESC)`,
	}
	for _, stmt := range stmts {
		if _, err := db.DB().ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate bls tables: %w", err)
		}
	}
	return nil
}

// seedBLSCatalog populates series_catalog from the embedded curated catalog
// when the table is empty. Returns the number of rows inserted.
func seedBLSCatalog(ctx context.Context, db *store.Store) (int, error) {
	var count int
	if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM series_catalog`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count series_catalog: %w", err)
	}
	if count > 0 {
		return 0, nil
	}
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO series_catalog (id, title, survey, area, item, units, adjust, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()
	inserted := 0
	for _, e := range blsdata.Catalog() {
		raw, _ := json.Marshal(e)
		if _, err := stmt.ExecContext(ctx, e.ID, e.Title, e.Survey, e.Area, e.Item, e.Units, e.Adjust, raw); err != nil {
			return inserted, err
		}
		inserted++
	}
	if err := tx.Commit(); err != nil {
		return inserted, err
	}
	return inserted, nil
}

// seedBLSReleases populates bls_releases from the embedded calendar when
// the table is empty. Returns the number of rows inserted.
func seedBLSReleases(ctx context.Context, db *store.Store) (int, error) {
	var count int
	if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM bls_releases`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count bls_releases: %w", err)
	}
	if count > 0 {
		return 0, nil
	}
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO bls_releases (id, release_date, release_time, survey, title, url, period) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()
	inserted := 0
	for _, e := range blsdata.ReleaseCalendar() {
		id := fmt.Sprintf("%s-%s", e.Survey, e.Date.Format("2006-01-02"))
		if _, err := stmt.ExecContext(ctx, id, e.Date.UTC(), e.Time, e.Survey, e.Title, e.URL, e.Period); err != nil {
			return inserted, err
		}
		inserted++
	}
	if err := tx.Commit(); err != nil {
		return inserted, err
	}
	return inserted, nil
}

// openBLSStore opens the local SQLite store at the canonical path and seeds
// it on first use. Returns the open store; caller must Close.
func openBLSStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("bls-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := migrateBLSTables(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := seedBLSCatalog(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := seedBLSReleases(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// cacheObservations stores observation rows from a `series get|batch` response
// in the local observations table. Used by series get/batch and by extremum
// to enable historical-extremum queries against cached data.
func cacheObservations(ctx context.Context, db *sql.DB, seriesID string, observations []ObservationRow) error {
	if len(observations) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO observations (series_id, year, period, period_name, value, footnotes) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, o := range observations {
		_, _ = stmt.ExecContext(ctx, seriesID, o.Year, o.Period, o.PeriodName, o.Value, o.Footnotes)
	}
	return tx.Commit()
}

// ObservationRow is a minimal parsed observation suitable for caching and
// for extremum queries.
type ObservationRow struct {
	Year       int
	Period     string
	PeriodName string
	Value      float64
	Footnotes  string
}

// injectRegistrationKey adds the BLS registration key from the loaded
// config to a POST body. BLS's quirk: when posting to /timeseries/data/
// the API only honors the registration key when it's in the JSON body —
// the query-string placement (the default for api_key + in:query auth)
// gets accepted by the gateway but BLS treats the request as
// unauthenticated, capping it to the 10-year/3-series unauth tier and
// silently dropping the annualaverage/calculations/catalog/aspects flags.
// Adding the key here in addition to the query placement means
// registered-tier features work end-to-end without changing the generic
// client. Safe no-op when the user has no key.
func injectRegistrationKey(body map[string]any) map[string]any {
	if body == nil {
		body = map[string]any{}
	}
	if _, exists := body["registrationkey"]; exists {
		return body
	}
	if k := strings.TrimSpace(os.Getenv("BLS_API_KEY")); k != "" {
		body["registrationkey"] = k
	}
	return body
}
