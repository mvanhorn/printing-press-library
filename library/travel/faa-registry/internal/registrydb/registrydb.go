// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

// Package registrydb owns the offline copy of the FAA Releasable Aircraft
// Database: schema, the daily-zip downloader, a header-driven CSV importer
// (resilient to the FAA's occasional column additions), and the typed queries
// behind the CLI's offline commands.
package registrydb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite handle holding the registry tables.
type DB struct {
	db *sql.DB
}

// Open opens (creating if needed) the registry database at path.
func Open(ctx context.Context, path string) (*DB, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	d := &DB{db: db}
	if err := d.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

// Close closes the database.
func (d *DB) Close() error { return d.db.Close() }

// SQL returns the underlying handle for ad-hoc queries.
func (d *DB) SQL() *sql.DB { return d.db }

const schema = `
CREATE TABLE IF NOT EXISTS faa_master (
	n_number TEXT PRIMARY KEY,
	serial_number TEXT, mfr_mdl_code TEXT, eng_mfr_mdl TEXT, year_mfr TEXT,
	type_registrant TEXT, name TEXT, street TEXT, street2 TEXT, city TEXT,
	state TEXT, zip_code TEXT, region TEXT, county TEXT, country TEXT,
	last_action_date TEXT, cert_issue_date TEXT, certification TEXT,
	type_aircraft TEXT, type_engine TEXT, status_code TEXT,
	mode_s_code TEXT, fract_owner TEXT, air_worth_date TEXT,
	other_name_1 TEXT, other_name_2 TEXT, other_name_3 TEXT,
	other_name_4 TEXT, other_name_5 TEXT,
	expiration_date TEXT, unique_id TEXT, kit_mfr TEXT, kit_model TEXT,
	mode_s_code_hex TEXT
);
CREATE INDEX IF NOT EXISTS idx_master_hex ON faa_master(mode_s_code_hex);
CREATE INDEX IF NOT EXISTS idx_master_name ON faa_master(name);
CREATE INDEX IF NOT EXISTS idx_master_mfr ON faa_master(mfr_mdl_code);
CREATE INDEX IF NOT EXISTS idx_master_state ON faa_master(state);
CREATE INDEX IF NOT EXISTS idx_master_expiration ON faa_master(expiration_date);
CREATE INDEX IF NOT EXISTS idx_master_serial ON faa_master(serial_number);

CREATE TABLE IF NOT EXISTS faa_dereg (
	n_number TEXT, serial_number TEXT, mfr_mdl_code TEXT, status_code TEXT,
	name TEXT, street_mail TEXT, street2_mail TEXT, city_mail TEXT,
	state_abbrev_mail TEXT, zip_code_mail TEXT, eng_mfr_mdl TEXT, year_mfr TEXT,
	certification TEXT, region TEXT, county_mail TEXT, country_mail TEXT,
	air_worth_date TEXT, cancel_date TEXT, mode_s_code TEXT, indicator_group TEXT,
	exp_country TEXT, last_act_date TEXT, cert_issue_date TEXT,
	other_name_1 TEXT, other_name_2 TEXT, other_name_3 TEXT,
	other_name_4 TEXT, other_name_5 TEXT,
	kit_mfr TEXT, kit_model TEXT, mode_s_code_hex TEXT
);
CREATE INDEX IF NOT EXISTS idx_dereg_n ON faa_dereg(n_number);
CREATE INDEX IF NOT EXISTS idx_dereg_serial ON faa_dereg(serial_number);
CREATE INDEX IF NOT EXISTS idx_dereg_name ON faa_dereg(name);

CREATE TABLE IF NOT EXISTS faa_reserved (
	n_number TEXT PRIMARY KEY,
	registrant TEXT, street TEXT, street2 TEXT, city TEXT, state TEXT,
	zip_code TEXT, rsv_date TEXT, tr TEXT, exp_date TEXT,
	n_num_chg TEXT, purge_date TEXT
);
CREATE INDEX IF NOT EXISTS idx_reserved_registrant ON faa_reserved(registrant);

CREATE TABLE IF NOT EXISTS faa_acftref (
	code TEXT PRIMARY KEY,
	mfr TEXT, model TEXT, type_acft TEXT, type_eng TEXT, ac_cat TEXT,
	build_cert_ind TEXT, no_eng TEXT, no_seats TEXT, ac_weight TEXT,
	speed TEXT, tc_data_sheet TEXT, tc_data_holder TEXT
);
CREATE INDEX IF NOT EXISTS idx_acftref_mfr ON faa_acftref(mfr);
CREATE INDEX IF NOT EXISTS idx_acftref_model ON faa_acftref(model);

CREATE TABLE IF NOT EXISTS faa_engine (
	code TEXT PRIMARY KEY,
	mfr TEXT, model TEXT, type TEXT, horsepower TEXT, thrust TEXT
);

CREATE TABLE IF NOT EXISTS faa_meta (
	key TEXT PRIMARY KEY,
	value TEXT
);

CREATE TABLE IF NOT EXISTS faa_watches (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kind TEXT NOT NULL,   -- 'owner' or 'tail'
	value TEXT NOT NULL,
	added_at TEXT NOT NULL,
	UNIQUE(kind, value)
);

CREATE TABLE IF NOT EXISTS faa_watch_state (
	watch_id INTEGER NOT NULL,
	n_number TEXT NOT NULL,
	row_hash TEXT NOT NULL,
	PRIMARY KEY (watch_id, n_number)
);

CREATE VIRTUAL TABLE IF NOT EXISTS faa_master_fts USING fts5(
	n_number, name, other_names, model, mfr
);
`

func (d *DB) migrate(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, schema)
	return err
}

// Meta returns a value from the sync-metadata table ("" when unset).
func (d *DB) Meta(ctx context.Context, key string) (string, error) {
	var v string
	err := d.db.QueryRowContext(ctx, `SELECT value FROM faa_meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// SetMeta stores a sync-metadata value.
func (d *DB) SetMeta(ctx context.Context, key, value string) error {
	_, err := d.db.ExecContext(ctx, `INSERT INTO faa_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// Synced reports whether a full import completed successfully. It gates on the
// "sync_complete" marker (set only after every required table AND the search
// index import in one run) rather than on row counts, so a torn or aborted
// sync — missing archive file, a failed table import, or a failed FTS rebuild
// that leaves base tables replaced but the index stale — reports NOT synced.
// Offline commands then tell the user to re-sync instead of serving
// inconsistent joins.
func (d *DB) Synced(ctx context.Context) (bool, error) {
	v, err := d.Meta(ctx, "sync_complete")
	if err != nil {
		return false, err
	}
	return v != "", nil
}

// ErrNotSynced is returned by offline queries when sync has not run yet.
var ErrNotSynced = fmt.Errorf("local registry database is empty — run `faa-registry-pp-cli sync` first")

func (d *DB) requireSynced(ctx context.Context) error {
	ok, err := d.Synced(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotSynced
	}
	return nil
}

// NormalizeTail strips the leading N and uppercases, matching the registry's
// stored key format (MASTER stores "101DQ" for N101DQ).
func NormalizeTail(tail string) string {
	t := strings.ToUpper(strings.TrimSpace(tail))
	t = strings.TrimPrefix(t, "N")
	return t
}
