// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored schema extension. Kept in its own file so `generate --force`
// preserves it; do not fold these statements into the generated store.go
// migration slice.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// reservationMirrorDDL backs the reservation history that Bookclicker only
// renders as HTML. The generated `reservations` table is shaped by the API's
// mutation responses (id/status/price), which carry none of the counterparty
// or list-size context the launch pages show, so scraped rows live here.
const reservationMirrorDDL = `
CREATE TABLE IF NOT EXISTS reservation_mirror (
	id                  TEXT PRIMARY KEY,
	book_id             INTEGER,
	book_title          TEXT,
	list_id             INTEGER,
	list_name           TEXT,
	list_size           INTEGER,
	date                TEXT,
	inv_type            TEXT,
	status              TEXT,
	is_swap             INTEGER,
	price               INTEGER,
	payment_offer       INTEGER,
	swap_reservation_id INTEGER,
	counterparty        TEXT,
	created_at          TEXT,
	seller_accepted_at  TEXT,
	seller_declined_at  TEXT,
	buyer_cancelled_at  TEXT,
	seller_cancelled_at TEXT,
	confirmation_requested_at TEXT,
	source              TEXT,
	first_seen_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
	synced_at           DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

// listRateHistoryDDL records each sync's open/click rate per list so `drift`
// can compare snapshots over time. The `lists` table only ever holds the
// latest values, so decay is invisible without this.
const listRateHistoryDDL = `
CREATE TABLE IF NOT EXISTS list_rate_history (
	list_id       INTEGER NOT NULL,
	observed_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
	open_rate     REAL,
	click_rate    REAL,
	member_count  INTEGER,
	PRIMARY KEY (list_id, observed_at)
);
CREATE INDEX IF NOT EXISTS idx_list_rate_history_list ON list_rate_history(list_id);
`

// reservationMirrorColumns are added after the base CREATE so a database made
// by an earlier build gains them. CREATE TABLE IF NOT EXISTS is a no-op on an
// existing table, so widening the DDL alone would leave old stores missing
// columns and fail the index creation that follows.
var reservationMirrorColumns = [][2]string{
	{"list_id", "INTEGER"},
	{"payment_offer", "INTEGER"},
	{"swap_reservation_id", "INTEGER"},
	{"created_at", "TEXT"},
	{"seller_accepted_at", "TEXT"},
	{"seller_declined_at", "TEXT"},
	{"buyer_cancelled_at", "TEXT"},
	{"seller_cancelled_at", "TEXT"},
	{"confirmation_requested_at", "TEXT"},
}

const reservationMirrorIndexes = `
CREATE INDEX IF NOT EXISTS idx_reservation_mirror_book ON reservation_mirror(book_id);
CREATE INDEX IF NOT EXISTS idx_reservation_mirror_date ON reservation_mirror(date);
CREATE INDEX IF NOT EXISTS idx_reservation_mirror_status ON reservation_mirror(status);
CREATE INDEX IF NOT EXISTS idx_reservation_mirror_list ON reservation_mirror(list_id);
`

// EnsureBookclickerTables creates the hand-owned tables and brings older
// databases up to the current column set. Idempotent; safe to call from any
// command that reads or writes them.
func EnsureBookclickerTables(ctx context.Context, s *Store) error {
	for _, ddl := range []string{reservationMirrorDDL, listRateHistoryDDL} {
		if _, err := s.DB().ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("creating bookclicker tables: %w", err)
		}
	}
	existing, err := reservationMirrorExistingColumns(ctx, s)
	if err != nil {
		return err
	}
	for _, col := range reservationMirrorColumns {
		if _, ok := existing[col[0]]; ok {
			continue
		}
		stmt := fmt.Sprintf("ALTER TABLE reservation_mirror ADD COLUMN %s %s", col[0], col[1])
		if _, err := s.DB().ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("adding reservation_mirror.%s: %w", col[0], err)
		}
	}
	if _, err := s.DB().ExecContext(ctx, reservationMirrorIndexes); err != nil {
		return fmt.Errorf("indexing reservation_mirror: %w", err)
	}
	return nil
}

func reservationMirrorExistingColumns(ctx context.Context, s *Store) (map[string]struct{}, error) {
	rows, err := s.DB().QueryContext(ctx, "PRAGMA table_info(reservation_mirror)")
	if err != nil {
		return nil, fmt.Errorf("reading reservation_mirror schema: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]struct{}{}
	for rows.Next() {
		var (
			cid, notnull, pk int
			name, ctype      string
			dflt             any
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("scanning reservation_mirror schema: %w", err)
		}
		out[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating reservation_mirror schema: %w", err)
	}
	return out, nil
}

// SnapshotListRatesIfStale records a snapshot only when the mirrored lists are
// newer than the most recent observation.
//
// `sync` is generated code, so there is no hook to snapshot from. Instead every
// path that reads rate history calls this first: the snapshot then tracks sync
// activity rather than command invocations, so running `drift` five times in a
// row cannot manufacture five identical data points.
//
// Returns the number of rows written (0 when already current).
func SnapshotListRatesIfStale(ctx context.Context, s *Store) (int, error) {
	if err := EnsureBookclickerTables(ctx, s); err != nil {
		return 0, err
	}
	// datetime() normalizes both sides before comparing. The two columns are
	// written by different layers in different formats — `lists.synced_at` is
	// Go RFC3339 ("2026-08-09T15:53:34Z"), `observed_at` is SQLite
	// CURRENT_TIMESTAMP ("2026-08-09 15:53:34"). Comparing them as raw strings
	// pits ' ' (0x20) against 'T' (0x54) at index 10, so the snapshot always
	// looks older than the sync and the guard never fires.
	var lastObserved, lastSynced sql.NullString
	if err := s.DB().QueryRowContext(ctx,
		`SELECT datetime(MAX(observed_at)) FROM list_rate_history`).Scan(&lastObserved); err != nil {
		return 0, fmt.Errorf("reading last snapshot: %w", err)
	}
	if err := s.DB().QueryRowContext(ctx,
		`SELECT datetime(MAX(synced_at)) FROM lists`).Scan(&lastSynced); err != nil {
		return 0, fmt.Errorf("reading last sync: %w", err)
	}
	if !lastSynced.Valid {
		return 0, nil // nothing mirrored yet
	}
	if lastObserved.Valid && lastObserved.String >= lastSynced.String {
		return 0, nil // already snapshotted this sync
	}
	return SnapshotListRates(ctx, s)
}

// SnapshotListRates appends the current open/click rates for every mirrored
// list, giving `drift` a second data point to compare against.
func SnapshotListRates(ctx context.Context, s *Store) (int, error) {
	if err := EnsureBookclickerTables(ctx, s); err != nil {
		return 0, err
	}
	res, err := s.DB().ExecContext(ctx, `
		INSERT OR IGNORE INTO list_rate_history (list_id, open_rate, click_rate, member_count)
		SELECT CAST(id AS INTEGER),
		       CAST(open_rate AS REAL),
		       CAST(click_rate AS REAL),
		       active_member_count
		FROM lists
		WHERE id IS NOT NULL`)
	if err != nil {
		return 0, fmt.Errorf("snapshotting list rates: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return int(n), nil
}
