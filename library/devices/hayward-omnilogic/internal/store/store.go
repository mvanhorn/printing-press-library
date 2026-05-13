// Package store is the local SQLite layer that backs every read-side
// transcendence command (chemistry log, drift, runtime, command log,
// schedule diff, sweep, status). The cloud API returns "now" only; this
// package turns sequential cloud reads into a time-series + audit trail.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schemaVersion = 2

type Store struct {
	DB   *sql.DB
	Path string
}

// Open opens (or creates) the SQLite store at the given path. Empty path
// resolves to the default location under the user's config dir.
func Open(path string) (*Store, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	// Match the auth cache and response cache permissions: user-only.
	// The store holds telemetry history, chemistry readings, equipment
	// topology, the command audit log, and alarm history — at least as
	// sensitive as the response cache, so use 0o700 / 0o600 for parity.
	// PATCH (fix-store-permissions-p1): 0o700 dir + 0o600 file (chmod after open) so local non-owner users cannot dump telemetry, chemistry, equipment topology, command log, or alarm history.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating store dir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	s := &Store{DB: db, Path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating store: %w", err)
	}
	// modernc.org/sqlite creates the main DB file with umask-defaulted
	// permissions (typically 0o644 on macOS/Linux). Clamp to 0o600 to
	// match the cache and auth files. WAL/SHM sidecars only exist after
	// the first write transaction, so we constrain them too when present
	// — chmod returns NotExist silently and that's fine.
	_ = os.Chmod(path, 0o600)
	for _, sidecar := range []string{path + "-wal", path + "-shm"} {
		if err := os.Chmod(sidecar, 0o600); err != nil && !os.IsNotExist(err) {
			// Non-fatal: report but don't fail Open.
			_ = err
		}
	}
	return s, nil
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "hayward-omnilogic-pp-cli", "store.sqlite"), nil
}

func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

func (s *Store) migrate() error {
	if _, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}
	var cur int
	row := s.DB.QueryRow(`SELECT version FROM schema_version LIMIT 1`)
	if err := row.Scan(&cur); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if cur == schemaVersion {
		return nil
	}
	for _, stmt := range migrations {
		if _, err := s.DB.Exec(stmt); err != nil {
			return fmt.Errorf("migration failed: %w\nstmt: %s", err, stmt)
		}
	}
	if cur == 0 {
		_, err := s.DB.Exec(`INSERT INTO schema_version (version) VALUES (?)`, schemaVersion)
		return err
	}
	_, err := s.DB.Exec(`UPDATE schema_version SET version = ?`, schemaVersion)
	return err
}

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS sites (
		msp_system_id INTEGER PRIMARY KEY,
		backyard_name TEXT NOT NULL,
		last_seen_at  TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS bodies_of_water (
		bow_system_id        TEXT NOT NULL,
		site_msp_system_id   INTEGER NOT NULL,
		name                 TEXT NOT NULL,
		type                 TEXT,
		shared_type          TEXT,
		shared_equip_id      TEXT,
		supports_spillover   TEXT,
		last_seen_at         TEXT NOT NULL,
		PRIMARY KEY (site_msp_system_id, bow_system_id)
	)`,
	`CREATE TABLE IF NOT EXISTS equipment (
		equipment_system_id  TEXT NOT NULL,
		site_msp_system_id   INTEGER NOT NULL,
		bow_system_id        TEXT,
		name                 TEXT NOT NULL,
		kind                 TEXT NOT NULL,
		type                 TEXT,
		function             TEXT,
		min_speed            TEXT,
		max_speed            TEXT,
		last_seen_at         TEXT NOT NULL,
		PRIMARY KEY (site_msp_system_id, equipment_system_id)
	)`,
	`CREATE TABLE IF NOT EXISTS msp_config_snapshots (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		site_msp_system_id  INTEGER NOT NULL,
		fetched_at          TEXT NOT NULL,
		raw_xml             TEXT NOT NULL,
		summary_json        TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS msp_snap_site_ts ON msp_config_snapshots(site_msp_system_id, fetched_at DESC)`,
	`CREATE TABLE IF NOT EXISTS telemetry_samples (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		site_msp_system_id  INTEGER NOT NULL,
		bow_system_id       TEXT,
		equipment_system_id TEXT,
		metric              TEXT NOT NULL,
		value_real          REAL,
		value_int           INTEGER,
		value_text          TEXT,
		sampled_at          TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS tele_site_metric_ts ON telemetry_samples(site_msp_system_id, metric, sampled_at)`,
	`CREATE INDEX IF NOT EXISTS tele_equip_ts ON telemetry_samples(equipment_system_id, sampled_at)`,
	`CREATE TABLE IF NOT EXISTS alarms (
		alarm_key            TEXT PRIMARY KEY,
		site_msp_system_id   INTEGER NOT NULL,
		bow_system_id        TEXT,
		equipment_system_id  TEXT,
		code                 TEXT,
		severity             TEXT,
		message              TEXT,
		raw_json             TEXT,
		first_seen           TEXT NOT NULL,
		last_seen            TEXT NOT NULL,
		cleared_at           TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS alarms_site_lastseen ON alarms(site_msp_system_id, last_seen DESC)`,
	`CREATE TABLE IF NOT EXISTS command_log (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		ts           TEXT NOT NULL,
		op           TEXT NOT NULL,
		target       TEXT,
		params_json  TEXT,
		status       TEXT,
		detail       TEXT,
		dry_run      INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS command_log_ts ON command_log(ts DESC)`,
	`CREATE TABLE IF NOT EXISTS auth_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS site_capabilities (
		site_msp_system_id INTEGER PRIMARY KEY,
		has_ph_sensor      INTEGER NOT NULL DEFAULT 1,
		has_orp_sensor     INTEGER NOT NULL DEFAULT 1,
		has_salt_sensor    INTEGER NOT NULL DEFAULT 1,
		temp_needs_flow    INTEGER NOT NULL DEFAULT 0,
		configured_at      TEXT NOT NULL,
		notes              TEXT
	)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS equipment_fts USING fts5(
		name, kind, type, function, content='equipment', content_rowid='rowid'
	)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS alarms_fts USING fts5(
		message, severity, code, content='alarms', content_rowid='rowid'
	)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS command_log_fts USING fts5(
		op, target, detail, content='command_log', content_rowid='rowid'
	)`,
}
