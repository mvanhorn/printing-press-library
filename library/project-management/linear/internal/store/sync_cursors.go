package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Per-resource sync cursor state.
//
// The sync_state table has carried last_synced_at and total_count since the
// first schema, but only the issues fetcher ever wrote a row and it wrote an
// empty cursor. Everything downstream that asks "how old is the local copy of
// X" (localProvenance, hintIfStale, doctor's cache report) therefore saw a
// zero timestamp for six of the seven synced resources and stayed silent.
//
// This file gives sync one place to read and write that state:
//
//	SyncCursorState  reads last_synced_at back as a real time.Time
//	RecordSyncPass   writes it for one resource, at a caller-chosen instant
//	CountRows        reports how many rows the local table actually holds
//
// RecordSyncPass takes the timestamp from the caller rather than using
// CURRENT_TIMESTAMP on purpose. An incremental sync must record the instant
// the fetch STARTED, not the instant it finished: anything mutated upstream
// while the crawl was in flight has to fall inside the next window, otherwise
// it is skipped forever.

// sqliteDateTimeLayout is the shape SQLite's CURRENT_TIMESTAMP writes, in UTC.
// RecordSyncPass writes the same layout so rows written here and rows written
// by the older UpdateSyncCursor path stay indistinguishable to every reader,
// including GetSyncState's driver-level DATETIME scan.
const sqliteDateTimeLayout = "2006-01-02 15:04:05"

// syncTimeLayouts are the shapes last_synced_at has been observed in: the
// SQLite CURRENT_TIMESTAMP form, and the RFC3339 forms a driver may write when
// a time.Time is bound directly. Parsed in order, first match wins.
var syncTimeLayouts = []string{
	sqliteDateTimeLayout,
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05.999999999-07:00",
	time.RFC3339Nano,
	time.RFC3339,
}

// SyncCursorState is one sync_state row, decoded. HasSynced is false when the
// resource has never been synced or the stored timestamp is unparseable, and
// callers MUST treat that as "no window, fetch everything".
type SyncCursorState struct {
	ResourceType string
	Cursor       string
	LastSyncedAt time.Time
	HasSynced    bool
	TotalCount   int
}

// SyncCursorState reads the sync_state row for resourceType. A missing row is
// not an error: it returns a zero state with HasSynced false.
func (s *Store) SyncCursorState(resourceType string) (SyncCursorState, error) {
	out := SyncCursorState{ResourceType: resourceType}
	var cursor, synced sql.NullString
	var count sql.NullInt64
	err := s.db.QueryRow(
		`SELECT last_cursor, last_synced_at, total_count FROM sync_state WHERE resource_type = ?`,
		resourceType,
	).Scan(&cursor, &synced, &count)
	if err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if cursor.Valid {
		out.Cursor = cursor.String
	}
	if count.Valid {
		out.TotalCount = int(count.Int64)
	}
	if synced.Valid && synced.String != "" {
		if ts, ok := parseSyncTime(synced.String); ok {
			out.LastSyncedAt = ts
			out.HasSynced = true
		}
	}
	return out, nil
}

// parseSyncTime decodes a stored last_synced_at. A layout without a zone is
// read as UTC, which is what SQLite's CURRENT_TIMESTAMP produces.
func parseSyncTime(raw string) (time.Time, bool) {
	for _, layout := range syncTimeLayouts {
		if ts, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return ts.UTC(), true
		}
	}
	return time.Time{}, false
}

// RecordSyncPass stamps last_synced_at and total_count for one resource.
//
// syncedAt should be the instant the fetch began. last_cursor is deliberately
// left alone on update: this CLI pages every resource to exhaustion within a
// single run and has no use for a resumable page cursor, so overwriting it
// with an empty string would only destroy information a future resumable
// fetcher might store there.
func (s *Store) RecordSyncPass(resourceType string, syncedAt time.Time, totalCount int) error {
	if resourceType == "" {
		return fmt.Errorf("recording sync pass: empty resource type")
	}
	if syncedAt.IsZero() {
		syncedAt = time.Now()
	}
	stamp := syncedAt.UTC().Format(sqliteDateTimeLayout)
	_, err := s.db.Exec(
		`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count)
		 VALUES (?, '', ?, ?)
		 ON CONFLICT(resource_type) DO UPDATE SET
		 last_synced_at = excluded.last_synced_at,
		 total_count = excluded.total_count`,
		resourceType, stamp, totalCount,
	)
	if err != nil {
		return fmt.Errorf("recording sync pass for %s: %w", resourceType, err)
	}
	return nil
}

// CountRows reports how many rows table currently holds. Only tables on the
// reconcile allowlist are accepted, because the name reaches SQL through
// string concatenation.
//
// An incremental pass fetches only what changed, so its fetch count says
// nothing about how much data the store holds. This is what gets written into
// total_count instead, which keeps hintIfUnsynced from announcing an empty
// store after a quiet incremental run returned zero changed rows.
func (s *Store) CountRows(table string) (int, error) {
	if !prunableTables[table] {
		return 0, fmt.Errorf("table %q is not countable", table)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM "` + table + `"`).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting %s: %w", table, err)
	}
	return n, nil
}
