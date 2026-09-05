package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// NCCPL observation storage.
//
// The generator emits no resource mirror for this API: every NCCPL data endpoint
// is a POST carrying a required settlement date, so the syncable-resource profile
// (which wants GET list endpoints with no required params) matches nothing. This
// file supplies the mirror by hand.
//
// Shape is deliberately generic rather than 20 typed tables. Every NCCPL payload is
// a flat JSON object belonging to exactly one (resource, date) pair, so one row
// store keyed (resource, date, row_key) serves every consumer -- panel export,
// cross-resource joins, universe reconstruction and per-field change detection --
// without 20 near-identical schemas drifting apart.
//
// Kept in a separate hand-authored file so `generate --force` preserves it.

// NCCPLRow is one upstream record: a stable within-date key and its raw JSON object.
type NCCPLRow struct {
	Key     string
	Payload string
}

// NCCPLObs is a stored observation.
type NCCPLObs struct {
	Resource   string
	Date       string
	Key        string
	Payload    string
	ObservedAt string
}

// NCCPLCoverage is one fetch attempt's outcome for a (resource, date).
//
// RowCount 0 is a real, meaningful value: it records that the date WAS fetched and
// legitimately held no rows. That is what separates "fetched and empty" from
// "never fetched", which a date-set diff alone cannot see.
type NCCPLCoverage struct {
	Resource  string
	Date      string
	RowCount  int
	FetchedAt string
}

const nccplSchemaDDL = `
CREATE TABLE IF NOT EXISTS nccpl_obs (
  resource    TEXT NOT NULL,
  date        TEXT NOT NULL,
  row_key     TEXT NOT NULL,
  payload     TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  PRIMARY KEY (resource, date, row_key)
);
CREATE INDEX IF NOT EXISTS idx_nccpl_obs_date     ON nccpl_obs(date);
CREATE INDEX IF NOT EXISTS idx_nccpl_obs_res_date ON nccpl_obs(resource, date);
CREATE INDEX IF NOT EXISTS idx_nccpl_obs_key      ON nccpl_obs(row_key, date);

CREATE TABLE IF NOT EXISTS nccpl_coverage (
  resource   TEXT NOT NULL,
  date       TEXT NOT NULL,
  row_count  INTEGER NOT NULL,
  fetched_at TEXT NOT NULL,
  PRIMARY KEY (resource, date)
);
CREATE INDEX IF NOT EXISTS idx_nccpl_cov_res ON nccpl_coverage(resource, date);
`

// EnsureNCCPLSchema creates the observation and coverage tables if absent.
func EnsureNCCPLSchema(ctx context.Context, s *Store) error {
	if s == nil || s.DB() == nil {
		return fmt.Errorf("nccpl schema: nil store")
	}
	if _, err := s.DB().ExecContext(ctx, nccplSchemaDDL); err != nil {
		return fmt.Errorf("nccpl schema: %w", err)
	}
	return nil
}

// SaveNCCPLDate writes one (resource, date) fetch result and its coverage entry in a
// single write transaction.
//
// Both halves must land together: a coverage row claiming N rows with no observations
// behind it, or observations with no coverage row, would each make the gap audit lie.
// Rows may be empty -- the coverage entry is still written, recording an
// intentional zero.
func SaveNCCPLDate(ctx context.Context, s *Store, resource, date string, rows []NCCPLRow, observedAt time.Time) error {
	if s == nil || s.DB() == nil {
		return fmt.Errorf("nccpl save: nil store")
	}
	stamp := observedAt.UTC().Format(time.RFC3339)

	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("nccpl save: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	obsStmt, err := tx.PrepareContext(ctx, `
INSERT INTO nccpl_obs (resource, date, row_key, payload, observed_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(resource, date, row_key) DO UPDATE SET
  payload = excluded.payload`)
	if err != nil {
		return fmt.Errorf("nccpl save: prepare obs: %w", err)
	}
	defer func() { _ = obsStmt.Close() }()

	// observed_at is deliberately NOT updated on conflict. It records when this
	// value was FIRST seen, which is what establishes ex-ante availability; a
	// re-sync must not silently move the vintage forward.
	for _, r := range rows {
		if _, err := obsStmt.ExecContext(ctx, resource, date, r.Key, r.Payload, stamp); err != nil {
			return fmt.Errorf("nccpl save: obs %s/%s/%s: %w", resource, date, r.Key, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO nccpl_coverage (resource, date, row_count, fetched_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(resource, date) DO UPDATE SET
  row_count  = excluded.row_count,
  fetched_at = excluded.fetched_at`,
		resource, date, len(rows), stamp); err != nil {
		return fmt.Errorf("nccpl save: coverage %s/%s: %w", resource, date, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("nccpl save: commit: %w", err)
	}
	return nil
}

// NCCPLCoveredDates returns every fetched date for a resource with its row count.
func NCCPLCoveredDates(ctx context.Context, s *Store, resource string) ([]NCCPLCoverage, error) {
	if s == nil || s.DB() == nil {
		return nil, fmt.Errorf("nccpl coverage: nil store")
	}
	rows, err := s.DB().QueryContext(ctx, `
SELECT resource, date, row_count, fetched_at
FROM nccpl_coverage WHERE resource = ? ORDER BY date`, resource)
	if err != nil {
		return nil, fmt.Errorf("nccpl coverage query: %w", err)
	}
	out := make([]NCCPLCoverage, 0)
	for rows.Next() {
		var c NCCPLCoverage
		var cnt sql.NullInt64
		var fetched sql.NullString
		if err := rows.Scan(&c.Resource, &c.Date, &cnt, &fetched); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("nccpl coverage scan: %w", err)
		}
		c.RowCount = int(cnt.Int64)
		c.FetchedAt = fetched.String
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("nccpl coverage iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("nccpl coverage close: %w", err)
	}
	return out, nil
}

// NCCPLObservations returns stored rows for a resource within an inclusive date range.
// Empty from/to bounds are treated as open-ended.
func NCCPLObservations(ctx context.Context, s *Store, resource, from, to string) ([]NCCPLObs, error) {
	if s == nil || s.DB() == nil {
		return nil, fmt.Errorf("nccpl observations: nil store")
	}
	q := `SELECT resource, date, row_key, payload, observed_at FROM nccpl_obs WHERE resource = ?`
	args := []any{resource}
	if from != "" {
		q += ` AND date >= ?`
		args = append(args, from)
	}
	if to != "" {
		q += ` AND date <= ?`
		args = append(args, to)
	}
	q += ` ORDER BY date, row_key`

	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("nccpl observations query: %w", err)
	}
	out := make([]NCCPLObs, 0)
	for rows.Next() {
		var o NCCPLObs
		var payload, observed sql.NullString
		if err := rows.Scan(&o.Resource, &o.Date, &o.Key, &payload, &observed); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("nccpl observations scan: %w", err)
		}
		o.Payload = payload.String
		o.ObservedAt = observed.String
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("nccpl observations iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("nccpl observations close: %w", err)
	}
	return out, nil
}

// NCCPLStoredResources lists resources that have at least one coverage entry.
func NCCPLStoredResources(ctx context.Context, s *Store) ([]string, error) {
	if s == nil || s.DB() == nil {
		return nil, fmt.Errorf("nccpl resources: nil store")
	}
	rows, err := s.DB().QueryContext(ctx,
		`SELECT DISTINCT resource FROM nccpl_coverage ORDER BY resource`)
	if err != nil {
		return nil, fmt.Errorf("nccpl resources query: %w", err)
	}
	out := make([]string, 0)
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("nccpl resources scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("nccpl resources iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("nccpl resources close: %w", err)
	}
	return out, nil
}

// NCCPLSchemaReady reports whether the observation tables exist.
//
// Read-only commands must call this instead of EnsureNCCPLSchema: the read-only
// connection cannot execute DDL, so creating the schema there fails with
// "attempt to write a readonly database" even when the caller only wants to read.
// A false result means nothing has been synced yet, which every read command
// renders as an empty result plus a sync hint rather than an error.
func NCCPLSchemaReady(ctx context.Context, s *Store) (bool, error) {
	if s == nil || s.DB() == nil {
		return false, fmt.Errorf("nccpl schema check: nil store")
	}
	var n int
	err := s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('nccpl_obs','nccpl_coverage')`).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("nccpl schema check: %w", err)
	}
	return n == 2, nil
}
