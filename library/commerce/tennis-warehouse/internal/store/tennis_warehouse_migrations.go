package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

// twMigrationsOnce ensures the typed tennis-warehouse schema is created exactly once
// per Store handle. The generic resources table already exists; these tables are
// additional, parallel to it, and survive --force regen because this file is
// hand-authored beside the emitted store.go (per the printing-press convention).
var twMigrationsOnce sync.Map // *Store -> bool

// EnsureTennisWarehouseSchema creates the typed tennis-warehouse tables
// (racquets, used_models, used_units, price_snapshots, watchlist) and the
// FTS indexes if they don't already exist. Idempotent. Callers in novel
// commands should invoke this lazily before any typed-table query.
func (s *Store) EnsureTennisWarehouseSchema(ctx context.Context) error {
	if _, ok := twMigrationsOnce.Load(s); ok {
		return nil
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS racquets (
			sku            TEXT PRIMARY KEY,
			brand          TEXT NOT NULL,
			model          TEXT NOT NULL,
			price          REAL,
			msrp           REAL,
			url            TEXT,
			image_url      TEXT,
			head_size_in2  REAL,
			strung_weight  REAL,
			unstrung_oz    REAL,
			balance        TEXT,
			swingweight    INTEGER,
			stiffness      INTEGER,
			beam_width     TEXT,
			string_pattern TEXT,
			length_in      REAL,
			composition    TEXT,
			power_level    TEXT,
			stroke_style   TEXT,
			status         TEXT,
			description    TEXT,
			last_seen_at   TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_racquets_brand    ON racquets(brand)`,
		`CREATE INDEX IF NOT EXISTS idx_racquets_headsize ON racquets(head_size_in2)`,
		`CREATE INDEX IF NOT EXISTS idx_racquets_pattern  ON racquets(string_pattern)`,
		`CREATE INDEX IF NOT EXISTS idx_racquets_price    ON racquets(price)`,

		`CREATE TABLE IF NOT EXISTS used_models (
			pcode          TEXT PRIMARY KEY,
			brand          TEXT NOT NULL,
			model          TEXT NOT NULL,
			url            TEXT,
			image_url      TEXT,
			price_low      REAL,
			price_high     REAL,
			msrp           REAL,
			head_size_in2  REAL,
			strung_weight  REAL,
			unstrung_oz    REAL,
			balance        TEXT,
			swingweight    INTEGER,
			stiffness      INTEGER,
			beam_width     TEXT,
			string_pattern TEXT,
			length_in      REAL,
			composition    TEXT,
			power_level    TEXT,
			stroke_style   TEXT,
			unit_count     INTEGER,
			first_seen_at  TEXT NOT NULL,
			last_seen_at   TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_used_models_brand     ON used_models(brand)`,
		`CREATE INDEX IF NOT EXISTS idx_used_models_headsize  ON used_models(head_size_in2)`,
		`CREATE INDEX IF NOT EXISTS idx_used_models_pricelow  ON used_models(price_low)`,
		`CREATE INDEX IF NOT EXISTS idx_used_models_first_seen ON used_models(first_seen_at)`,

		`CREATE TABLE IF NOT EXISTS used_units (
			stock_code    TEXT PRIMARY KEY,
			pcode         TEXT NOT NULL,
			grade         TEXT NOT NULL,
			grip_size     TEXT,
			price         REAL NOT NULL,
			notes         TEXT,
			first_seen_at TEXT NOT NULL,
			last_seen_at  TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_used_units_pcode   ON used_units(pcode)`,
		`CREATE INDEX IF NOT EXISTS idx_used_units_grade   ON used_units(grade)`,
		`CREATE INDEX IF NOT EXISTS idx_used_units_grip    ON used_units(grip_size)`,
		`CREATE INDEX IF NOT EXISTS idx_used_units_seen    ON used_units(first_seen_at)`,

		`CREATE TABLE IF NOT EXISTS price_snapshots (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			kind        TEXT NOT NULL,   -- 'used_unit' | 'used_model' | 'racquet'
			ref         TEXT NOT NULL,   -- stock_code, pcode, or sku
			price       REAL NOT NULL,
			captured_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_ref_time ON price_snapshots(kind, ref, captured_at)`,

		`CREATE TABLE IF NOT EXISTS watchlist (
			pcode      TEXT PRIMARY KEY,
			label      TEXT,
			added_at   TEXT NOT NULL
		)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS racquets_fts USING fts5(
			sku UNINDEXED,
			brand,
			model,
			composition,
			tokenize='porter unicode61'
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS used_models_fts USING fts5(
			pcode UNINDEXED,
			brand,
			model,
			composition,
			tokenize='porter unicode61'
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("tennis-warehouse migration: %w\nstmt: %s", err, stmt)
		}
	}
	twMigrationsOnce.Store(s, true)
	return nil
}

// (DB() is already exported by the generated store.go — no need to redeclare.)
var _ = sql.LevelDefault
