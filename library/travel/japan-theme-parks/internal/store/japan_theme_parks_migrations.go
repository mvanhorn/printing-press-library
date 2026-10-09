// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/cliutil"
)

// WaitRow is one stored Queue-Times wait observation.
type WaitRow struct {
	ParkID          int
	RideID          int
	RideName        string
	Land            string
	IsOpen          bool
	WaitMinutes     int
	SourceUpdatedAt time.Time
	FetchedAt       time.Time
}

// The wait_snapshots table is created by migrateExtras on every writable open.
const waitSnapshotsTableDDL = `CREATE TABLE IF NOT EXISTS wait_snapshots (
	park_id INTEGER NOT NULL,
	ride_id INTEGER NOT NULL,
	ride_name TEXT NOT NULL,
	land TEXT NOT NULL DEFAULT '',
	is_open INTEGER NOT NULL,
	wait_minutes INTEGER NOT NULL,
	source_updated_at TEXT NOT NULL,
	fetched_at TEXT NOT NULL,
	PRIMARY KEY (ride_id, source_updated_at)
)`

const waitSnapshotsIndexDDL = `CREATE INDEX IF NOT EXISTS idx_wait_snapshots_park_time ON wait_snapshots(park_id, source_updated_at)`

// InsertCount is the per-park tally of one InsertWaitRows call.
type InsertCount struct {
	Inserted int // new rows
	Skipped  int // duplicates of (ride_id, source_updated_at)
	Invalid  int // rows without a source update time; never stored
}

// InsertWaitRows appends rows, skipping duplicates of (ride_id,
// source_updated_at). It returns per-park inserted and skipped counts.
func (s *Store) InsertWaitRows(ctx context.Context, rows []WaitRow) (map[int]InsertCount, error) {
	counts := map[int]InsertCount{}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO wait_snapshots
		(park_id, ride_id, ride_name, land, is_open, wait_minutes, source_updated_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("prepare: %w", err)
	}
	for _, r := range rows {
		if r.SourceUpdatedAt.IsZero() || r.FetchedAt.IsZero() {
			c := counts[r.ParkID]
			c.Invalid++
			counts[r.ParkID] = c
			continue
		}
		open := 0
		if r.IsOpen {
			open = 1
		}
		res, err := stmt.ExecContext(ctx, r.ParkID, r.RideID, r.RideName, r.Land, open, r.WaitMinutes,
			r.SourceUpdatedAt.UTC().Format(time.RFC3339), r.FetchedAt.UTC().Format(time.RFC3339))
		if err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return nil, fmt.Errorf("insert ride %d: %w", r.RideID, err)
		}
		c := counts[r.ParkID]
		if n, _ := res.RowsAffected(); n > 0 {
			c.Inserted++
		} else {
			c.Skipped++
		}
		counts[r.ParkID] = c
	}
	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return counts, nil
}

// WaitQuery selects stored observations for one park. Weekday and hour are
// in Asia/Tokyo (UTC+9, no DST). Zero values mean "no filter".
type WaitQuery struct {
	ParkID   int
	Weekdays []int  // 0 = Sunday ... 6 = Saturday
	HourFrom int    // inclusive, used when HourSet
	HourTo   int    // inclusive, used when HourSet
	HourSet  bool   // filter on HourFrom..HourTo
	RideLike string // case-insensitive substring of ride_name
}

// WaitSpan summarizes all stored rows for one park.
type WaitSpan struct {
	Rows  int
	First time.Time
	Last  time.Time
}

const jstModifier = "+9 hours"

func (s *Store) hasWaitSnapshots(ctx context.Context) (bool, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	return tableExists(ctx, conn, "wait_snapshots")
}

// WaitHistorySpan returns the row count and the first and last source time
// for a park. A database without the table returns an empty span.
func (s *Store) WaitHistorySpan(ctx context.Context, parkID int) (WaitSpan, error) {
	ok, err := s.hasWaitSnapshots(ctx)
	if err != nil || !ok {
		return WaitSpan{}, err
	}
	var n int
	var first, last sql.NullString
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*), MIN(source_updated_at), MAX(source_updated_at) FROM wait_snapshots WHERE park_id = ?`, parkID).Scan(&n, &first, &last)
	if err != nil {
		return WaitSpan{}, fmt.Errorf("query wait_snapshots span: %w", err)
	}
	return WaitSpan{Rows: n, First: cliutil.ParseStoredTime(first.String), Last: cliutil.ParseStoredTime(last.String)}, nil
}

// QueryWaitRows reads the matching observations, oldest first, with the
// weekday, hour and ride filters applied in SQL. Rows whose stored time does
// not parse are skipped and counted. FetchedAt is not read.
func (s *Store) QueryWaitRows(ctx context.Context, q WaitQuery) ([]WaitRow, int, error) {
	ok, err := s.hasWaitSnapshots(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return []WaitRow{}, 0, nil
	}
	// The query text is static; every filter is a bound parameter and an
	// unused filter is switched off by its own parameter. The weekday set
	// is passed as ",0,6," and matched against ",<weekday>,".
	weekdays := ""
	if len(q.Weekdays) > 0 {
		parts := make([]string, len(q.Weekdays))
		for i, d := range q.Weekdays {
			parts[i] = strconv.Itoa(d)
		}
		weekdays = "," + strings.Join(parts, ",") + ","
	}
	hourSet := 0
	if q.HourSet {
		hourSet = 1
	}
	ride := ""
	if q.RideLike != "" {
		ride = "%" + likeEscaper.Replace(q.RideLike) + "%"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT park_id, ride_id, ride_name, land, is_open, wait_minutes, source_updated_at
		FROM wait_snapshots
		WHERE park_id = ?1
		  AND (?2 = '' OR instr(?2, ',' || strftime('%w', source_updated_at, ?3) || ',') > 0)
		  AND (?4 = 0 OR CAST(strftime('%H', source_updated_at, ?3) AS INTEGER) BETWEEN ?5 AND ?6)
		  AND (?7 = '' OR ride_name LIKE ?7 ESCAPE '\')
		ORDER BY source_updated_at`,
		q.ParkID, weekdays, jstModifier, hourSet, q.HourFrom, q.HourTo, ride)
	if err != nil {
		return nil, 0, fmt.Errorf("query wait_snapshots: %w", err)
	}
	defer rows.Close()
	out := make([]WaitRow, 0)
	bad := 0
	for rows.Next() {
		var r WaitRow
		var open int
		var src string
		if err := rows.Scan(&r.ParkID, &r.RideID, &r.RideName, &r.Land, &open, &r.WaitMinutes, &src); err != nil {
			return nil, 0, fmt.Errorf("scan wait_snapshots: %w", err)
		}
		r.IsOpen = open == 1
		if r.SourceUpdatedAt = cliutil.ParseStoredTime(src); r.SourceUpdatedAt.IsZero() {
			bad++
			continue
		}
		out = append(out, r)
	}
	return out, bad, rows.Err()
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

// RejectNewerSchema returns an error when PRAGMA user_version is above
// StoreSchemaVersion. A read-only open does not migrate and does not run the
// version check that a writable open runs, so history readers call this
// before they trust the wait_snapshots layout.
func (s *Store) RejectNewerSchema() error {
	v, err := s.SchemaVersion()
	if err != nil {
		return err
	}
	if v > StoreSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d; upgrade the CLI binary or open an older database", v, StoreSchemaVersion)
	}
	return nil
}

// ParkHistory is the stored-row summary for one park.
type ParkHistory struct {
	ParkID int
	WaitSpan
}

// WaitHistoryByPark returns the row count and the first and last source time
// for each park that has stored rows, ordered by park id. A database without
// the table returns an empty slice.
func (s *Store) WaitHistoryByPark(ctx context.Context) ([]ParkHistory, error) {
	ok, err := s.hasWaitSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []ParkHistory{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT park_id, COUNT(*), MIN(source_updated_at), MAX(source_updated_at) FROM wait_snapshots GROUP BY park_id ORDER BY park_id`)
	if err != nil {
		return nil, fmt.Errorf("query wait_snapshots summary: %w", err)
	}
	defer rows.Close()
	out := make([]ParkHistory, 0)
	for rows.Next() {
		var p ParkHistory
		var first, last sql.NullString
		if err := rows.Scan(&p.ParkID, &p.Rows, &first, &last); err != nil {
			return nil, fmt.Errorf("scan wait_snapshots summary: %w", err)
		}
		p.First, p.Last = cliutil.ParseStoredTime(first.String), cliutil.ParseStoredTime(last.String)
		out = append(out, p)
	}
	return out, rows.Err()
}
