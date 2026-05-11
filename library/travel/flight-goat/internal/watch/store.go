// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package watch

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SchemaVersion is the on-disk schema version this binary writes. It's
// stamped into PRAGMA user_version on fresh DBs and checked on open so a
// future schema change is detectable.
//
// v1 → v2: added departure_time, fare_brand, exclude_basic for fare-class
// safety. Migration is forward-only: existing rows get the columns with
// NULL/0 defaults, which match the pre-v2 behavior (no time match,
// no brand surfaced, exclude-basic OFF).
const SchemaVersion = 2

// Store wraps the SQLite DB that backs the watch list. It owns its own
// connection pool — intentionally separate from the generator-owned
// internal/store package to avoid coupling watches to migrations on the
// generated FlightAware tables.
type Store struct {
	db   *sql.DB
	path string
}

// DefaultDBPath returns the file path where watches are persisted. It
// honors FLIGHT_GOAT_WATCH_DB (used by tests and by users who want a
// non-standard location), falling back to ~/.local/share/flight-goat-pp-cli/watches.db.
func DefaultDBPath() string {
	if v := os.Getenv("FLIGHT_GOAT_WATCH_DB"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "flight-goat-pp-cli", "watches.db")
}

// Open creates or opens the watches SQLite file and runs the schema
// migration. The directory tree is created if it doesn't exist.
func Open(ctx context.Context, dbPath string) (*Store, error) {
	if dbPath == "" {
		dbPath = DefaultDBPath()
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating watch db directory: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("opening watch db: %w", err)
	}
	db.SetMaxOpenConns(2)
	s := &Store{db: db, path: dbPath}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating watch db: %w", err)
	}
	return s, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error { return s.db.Close() }

// Path returns the on-disk DB path. Useful for `doctor` and tests.
func (s *Store) Path() string { return s.path }

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS watches (
			id                  TEXT PRIMARY KEY,
			origin              TEXT NOT NULL,
			destination         TEXT NOT NULL,
			departure_date      TEXT NOT NULL,
			departure_time      TEXT NOT NULL DEFAULT '',
			airline             TEXT NOT NULL,
			flight_number       TEXT NOT NULL,
			cabin               TEXT NOT NULL DEFAULT '',
			fare_brand          TEXT NOT NULL DEFAULT '',
			exclude_basic       INTEGER NOT NULL DEFAULT 0,
			passengers          INTEGER NOT NULL DEFAULT 1,
			original_price      REAL NOT NULL,
			threshold           REAL NOT NULL,
			currency            TEXT NOT NULL DEFAULT 'USD',
			notify              TEXT NOT NULL DEFAULT '',
			booking_ref         TEXT NOT NULL DEFAULT '',
			notes               TEXT NOT NULL DEFAULT '',
			status              TEXT NOT NULL DEFAULT 'active',
			created_at          TEXT NOT NULL,
			last_checked_at     TEXT,
			last_seen_price     REAL,
			last_alerted_price  REAL
		);
		CREATE INDEX IF NOT EXISTS idx_watches_status ON watches(status);
		CREATE INDEX IF NOT EXISTS idx_watches_route_date ON watches(origin, destination, departure_date);
	`)
	if err != nil {
		return fmt.Errorf("creating watches table: %w", err)
	}
	// v1 → v2 forward migration: add the columns to any existing DB.
	// SQLite errors on duplicate ALTER, so we probe table_info first.
	for _, col := range []struct{ name, decl string }{
		{"departure_time", "TEXT NOT NULL DEFAULT ''"},
		{"fare_brand", "TEXT NOT NULL DEFAULT ''"},
		{"exclude_basic", "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := s.ensureColumn(ctx, col.name, col.decl); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, SchemaVersion)); err != nil {
		return fmt.Errorf("stamping schema version: %w", err)
	}
	return nil
}

// ensureColumn adds a column to the watches table if it doesn't already
// exist. Used to forward-migrate v1 databases to v2 without losing data.
func (s *Store) ensureColumn(ctx context.Context, name, decl string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(watches)`)
	if err != nil {
		return fmt.Errorf("table_info: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var n, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &n, &typ, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info: %w", err)
		}
		if n == name {
			return nil
		}
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE watches ADD COLUMN %s %s`, name, decl)); err != nil {
		return fmt.Errorf("add column %s: %w", name, err)
	}
	return nil
}

// NewWatchID mints a stable, opaque identifier for a new watch. The
// `watch_` prefix is part of the public JSON contract so users can grep
// for IDs in their logs.
func NewWatchID() string {
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	return "watch_" + hex.EncodeToString(buf[:])
}

// Insert persists a fully-validated Watch. Watch.ID is filled if empty,
// and Watch.CreatedAt is set if zero. The ID is returned.
func (s *Store) Insert(ctx context.Context, w *Watch) (string, error) {
	if err := w.Validate(); err != nil {
		return "", err
	}
	if w.ID == "" {
		w.ID = NewWatchID()
	}
	if w.CreatedAt.IsZero() {
		w.CreatedAt = time.Now().UTC()
	}
	excludeBasic := 0
	if w.ExcludeBasic {
		excludeBasic = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO watches (
			id, origin, destination, departure_date, departure_time,
			airline, flight_number, cabin, fare_brand, exclude_basic,
			passengers, original_price, threshold, currency, notify,
			booking_ref, notes, status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		w.ID, w.Origin, w.Destination, w.DepartureDate, w.DepartureTime,
		w.Airline, w.FlightNumber, w.Cabin, w.FareBrand, excludeBasic,
		w.Passengers, w.OriginalPrice, w.Threshold, w.Currency, w.Notify,
		w.BookingRef, w.Notes, w.Status, w.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return "", fmt.Errorf("watch %q already exists", w.ID)
		}
		return "", fmt.Errorf("insert watch: %w", err)
	}
	return w.ID, nil
}

// Get returns the Watch matching id, or ErrNotFound if it doesn't exist.
func (s *Store) Get(ctx context.Context, id string) (*Watch, error) {
	rows, err := s.query(ctx, `WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	return rows[0], nil
}

// List returns watches in a stable order (newest first). Pass empty
// `status` for all rows; otherwise only the matching status is returned.
func (s *Store) List(ctx context.Context, status string) ([]*Watch, error) {
	if status == "" {
		return s.query(ctx, `ORDER BY created_at DESC`)
	}
	return s.query(ctx, `WHERE status = ? ORDER BY created_at DESC`, status)
}

// Delete removes a watch. Returns ErrNotFound if no row matched.
func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM watches WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete watch: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordCheck updates the live-price tracking fields after a check. If
// alerted is true, last_alerted_price is also updated to the foundPrice.
func (s *Store) RecordCheck(ctx context.Context, id string, checkedAt time.Time, foundPrice *float64, alerted bool) error {
	stmt := `UPDATE watches SET last_checked_at = ?, last_seen_price = ?`
	args := []any{checkedAt.UTC().Format(time.RFC3339Nano), nullableFloat(foundPrice)}
	if alerted && foundPrice != nil {
		stmt += `, last_alerted_price = ?`
		args = append(args, *foundPrice)
	}
	stmt += ` WHERE id = ?`
	args = append(args, id)
	res, err := s.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("record check: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func nullableFloat(f *float64) any {
	if f == nil {
		return nil
	}
	return *f
}

// ErrNotFound is returned when an ID lookup misses.
var ErrNotFound = errors.New("watch not found")

// query is the shared row-loader for all Watch reads.
func (s *Store) query(ctx context.Context, tail string, args ...any) ([]*Watch, error) {
	q := `SELECT id, origin, destination, departure_date, departure_time,
		airline, flight_number, cabin, fare_brand, exclude_basic,
		passengers, original_price, threshold, currency, notify,
		booking_ref, notes, status, created_at, last_checked_at,
		last_seen_price, last_alerted_price FROM watches ` + tail
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query watches: %w", err)
	}
	defer rows.Close()
	var out []*Watch
	for rows.Next() {
		w := &Watch{}
		var createdAt string
		var checkedAt sql.NullString
		var lastSeen, lastAlerted sql.NullFloat64
		var excludeBasic int
		if err := rows.Scan(
			&w.ID, &w.Origin, &w.Destination, &w.DepartureDate, &w.DepartureTime,
			&w.Airline, &w.FlightNumber, &w.Cabin, &w.FareBrand, &excludeBasic,
			&w.Passengers, &w.OriginalPrice, &w.Threshold, &w.Currency, &w.Notify,
			&w.BookingRef, &w.Notes, &w.Status, &createdAt, &checkedAt,
			&lastSeen, &lastAlerted,
		); err != nil {
			return nil, fmt.Errorf("scan watch: %w", err)
		}
		w.ExcludeBasic = excludeBasic != 0
		w.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		if checkedAt.Valid {
			if t, err := time.Parse(time.RFC3339Nano, checkedAt.String); err == nil {
				w.LastCheckedAt = &t
			}
		}
		if lastSeen.Valid {
			v := lastSeen.Float64
			w.LastSeenPrice = &v
		}
		if lastAlerted.Valid {
			v := lastAlerted.Float64
			w.LastAlertedPrice = &v
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
