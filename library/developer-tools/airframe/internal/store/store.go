// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

// Package store provides local SQLite persistence for airframe-pp-cli.
// Uses modernc.org/sqlite (pure Go, no CGO) for zero-dependency cross-compilation.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// StoreSchemaVersion is the on-disk schema version this binary understands.
// It is stamped into SQLite's PRAGMA user_version on fresh databases and
// checked on every open. Bump this whenever a migration changes table shape
// so an older binary refuses to open a newer database rather than silently
// producing wrong results against a schema it cannot read.
const StoreSchemaVersion = 1

// DefaultDBPath returns the XDG-compliant on-disk path for the airframe store.
func DefaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "airframe-pp-cli", "data.db")
}

type Store struct {
	db      *sql.DB
	writeMu sync.Mutex
	path    string
}

func Open(dbPath string) (*Store, error) {
	return OpenWithContext(context.Background(), dbPath)
}

func OpenWithContext(ctx context.Context, dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY&_mmap_size=268435456")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	db.SetMaxOpenConns(2)

	s := &Store{db: db, path: dbPath}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return s, nil
}

func OpenReadOnly(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY&_mmap_size=268435456")
	if err != nil {
		return nil, fmt.Errorf("opening database (read-only): %w", err)
	}
	db.SetMaxOpenConns(2)

	var current int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&current); err != nil {
		db.Close()
		return nil, fmt.Errorf("reading schema version: %w", err)
	}
	if current > StoreSchemaVersion {
		db.Close()
		return nil, fmt.Errorf("database schema version %d is newer than supported version %d; upgrade the CLI binary", current, StoreSchemaVersion)
	}

	return &Store{db: db, path: dbPath}, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) DB() *sql.DB  { return s.db }
func (s *Store) Path() string { return s.path }
func (s *Store) Lock()        { s.writeMu.Lock() }
func (s *Store) Unlock()      { s.writeMu.Unlock() }

// SchemaVersion returns the user_version stamp from the open database.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var v int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

func (s *Store) migrate(ctx context.Context) error {
	var current int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	if current > StoreSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d; upgrade the CLI binary", current, StoreSchemaVersion)
	}
	// PATCH: skip the write transaction when the schema is already current.
	// The DDL is all `CREATE … IF NOT EXISTS`, so re-running it is correct —
	// but it still acquires the SQLite write lock and stamps user_version on
	// every Open, blocking concurrent readers under WAL contention.
	if current == StoreSchemaVersion {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range schemaV1 {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration failed: %w\nstatement: %s", err, stmt)
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, StoreSchemaVersion)); err != nil {
		return fmt.Errorf("stamping user_version: %w", err)
	}
	return tx.Commit()
}

// schemaV1 is the initial schema. Bumping StoreSchemaVersion requires a new
// migration slice that runs only when current < the new version.
var schemaV1 = []string{
	// FAA aircraft registry (one row per N-number, slim Core columns)
	`CREATE TABLE IF NOT EXISTS aircraft (
		registration         TEXT PRIMARY KEY,
		serial_number        TEXT,
		make_model_code      TEXT,
		engine_code          TEXT,
		year_mfr             INTEGER,
		type_registrant      TEXT,
		type_aircraft        TEXT,
		type_engine          TEXT,
		status_code          TEXT,
		cert_issue_date      TEXT,
		last_action_date     TEXT,
		airworthiness_date   TEXT,
		expiration_date      TEXT,
		mode_s_code_hex      TEXT,
		owner_name           TEXT,
		owner_street         TEXT,
		owner_city           TEXT,
		owner_state          TEXT,
		owner_zip            TEXT,
		owner_country        TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_aircraft_make_model    ON aircraft(make_model_code)`,
	`CREATE INDEX IF NOT EXISTS idx_aircraft_mode_s        ON aircraft(mode_s_code_hex)`,
	`CREATE INDEX IF NOT EXISTS idx_aircraft_owner_name_ci ON aircraft(owner_name COLLATE NOCASE)`,

	// FAA make/model reference (ACFTREF) — MFR+MDL+SERIES code is the join key
	`CREATE TABLE IF NOT EXISTS make_model (
		code                   TEXT PRIMARY KEY,
		manufacturer           TEXT NOT NULL,
		model                  TEXT NOT NULL,
		aircraft_type          TEXT,
		engine_type            TEXT,
		category               TEXT,
		builder_certification  TEXT,
		number_engines         INTEGER,
		number_seats           INTEGER,
		weight_class           TEXT,
		cruising_speed         INTEGER
	)`,
	`CREATE INDEX IF NOT EXISTS idx_make_model_mfr_ci  ON make_model(manufacturer COLLATE NOCASE)`,
	`CREATE INDEX IF NOT EXISTS idx_make_model_full_ci ON make_model(manufacturer COLLATE NOCASE, model COLLATE NOCASE)`,

	// FAA engine reference
	`CREATE TABLE IF NOT EXISTS engine (
		code          TEXT PRIMARY KEY,
		manufacturer  TEXT,
		model         TEXT,
		engine_type   TEXT,
		horsepower    INTEGER,
		thrust        INTEGER
	)`,

	// FAA deregistration history (populated only with --include-dereg).
	// cancel_status_code is the FAA STATUS-CODE at time of cancellation;
	// the official meanings (e.g. R = revoked, X = expired) are documented
	// in ardata.pdf inside the FAA zip.
	`CREATE TABLE IF NOT EXISTS dereg (
		registration         TEXT NOT NULL,
		cancel_date          TEXT NOT NULL,
		cancel_status_code   TEXT,
		make_model_code      TEXT,
		prev_owner           TEXT,
		cert_issue_date      TEXT,
		last_action_date     TEXT,
		PRIMARY KEY (registration, cancel_date)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_dereg_registration ON dereg(registration)`,

	// NTSB events
	`CREATE TABLE IF NOT EXISTS events (
		event_id          TEXT PRIMARY KEY,
		event_type        TEXT,
		event_date        TEXT NOT NULL,
		event_city        TEXT,
		event_state       TEXT,
		event_country     TEXT,
		latitude          REAL,
		longitude         REAL,
		highest_injury    TEXT,
		total_fatal       INTEGER,
		total_serious     INTEGER,
		total_minor       INTEGER,
		total_uninjured   INTEGER,
		weather           TEXT,
		light_condition   TEXT,
		phase_of_flight   TEXT,
		ntsb_report_no    TEXT,
		probable_cause    TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_events_event_date  ON events(event_date)`,
	`CREATE INDEX IF NOT EXISTS idx_events_state_date  ON events(event_state, event_date)`,

	// NTSB aircraft-per-event (multi-aircraft events keyed by aircraft_idx)
	`CREATE TABLE IF NOT EXISTS event_aircraft (
		event_id          TEXT NOT NULL,
		aircraft_idx      INTEGER NOT NULL,
		registration      TEXT,
		make_model_code   TEXT,
		damage            TEXT,
		operator_name     TEXT,
		far_part          TEXT,
		flight_phase      TEXT,
		PRIMARY KEY (event_id, aircraft_idx),
		FOREIGN KEY (event_id) REFERENCES events(event_id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_event_aircraft_registration ON event_aircraft(registration)`,
	`CREATE INDEX IF NOT EXISTS idx_event_aircraft_event_id     ON event_aircraft(event_id)`,
	`CREATE INDEX IF NOT EXISTS idx_event_aircraft_make_model   ON event_aircraft(make_model_code)`,

	// NTSB narratives. summary is the truncated Core-profile text; full_zstd is
	// populated only when --full-narratives is passed at sync time.
	`CREATE TABLE IF NOT EXISTS narratives (
		event_id   TEXT PRIMARY KEY,
		summary    TEXT,
		full_zstd  BLOB,
		FOREIGN KEY (event_id) REFERENCES events(event_id) ON DELETE CASCADE
	)`,

	// Per-source sync metadata. Single-row-per-source identified by `source`
	// (faa_master | ntsb_avall | ntsb_pre1982). schema_profile records which
	// optional tiers were synced so doctor can flag stale-profile mismatches.
	`CREATE TABLE IF NOT EXISTS sync_meta (
		source            TEXT PRIMARY KEY,
		source_url        TEXT NOT NULL,
		last_modified     TEXT,
		source_etag       TEXT,
		last_synced_at    TEXT,
		row_count         INTEGER,
		bytes_downloaded  INTEGER,
		schema_profile    TEXT
	)`,
}

// SyncMetaRow is the read shape for a single sync_meta row.
type SyncMetaRow struct {
	Source          string
	SourceURL       string
	LastModified    string
	SourceETag      string
	LastSyncedAt    string
	RowCount        int64
	BytesDownloaded int64
	SchemaProfile   string
}

// ListSyncMeta returns every row in sync_meta. Used by doctor.
func (s *Store) ListSyncMeta(ctx context.Context) ([]SyncMetaRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source, source_url, COALESCE(last_modified,''), COALESCE(source_etag,''),
		       COALESCE(last_synced_at,''), COALESCE(row_count,0), COALESCE(bytes_downloaded,0),
		       COALESCE(schema_profile,'')
		FROM sync_meta
		ORDER BY source
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SyncMetaRow
	for rows.Next() {
		var r SyncMetaRow
		if err := rows.Scan(&r.Source, &r.SourceURL, &r.LastModified, &r.SourceETag,
			&r.LastSyncedAt, &r.RowCount, &r.BytesDownloaded, &r.SchemaProfile); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
