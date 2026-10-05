// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Store tables are prefixed uj_ so they never collide with generated tables.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS uj_postings (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	team TEXT,
	sub_team TEXT,
	country_iso3 TEXT,
	posted_raw TEXT,
	posted_on TEXT,
	is_floor INTEGER NOT NULL DEFAULT 0,
	work_pattern TEXT,
	contract_type TEXT,
	source TEXT NOT NULL,
	data TEXT NOT NULL,
	first_seen TEXT NOT NULL,
	last_seen TEXT NOT NULL,
	closed_on TEXT
);
CREATE INDEX IF NOT EXISTS uj_postings_open ON uj_postings(closed_on);
CREATE TABLE IF NOT EXISTS uj_posting_locations (
	posting_id TEXT NOT NULL,
	idx INTEGER NOT NULL,
	country_iso3 TEXT,
	country TEXT,
	city TEXT,
	PRIMARY KEY (posting_id, idx)
);
CREATE TABLE IF NOT EXISTS uj_sync_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	started_at TEXT NOT NULL,
	finished_at TEXT NOT NULL,
	source TEXT NOT NULL,
	scope TEXT NOT NULL,
	total INTEGER NOT NULL,
	unique_count INTEGER NOT NULL,
	complete INTEGER NOT NULL,
	scan_cap_hit INTEGER NOT NULL,
	closed_marked INTEGER NOT NULL,
	note TEXT
);
CREATE TABLE IF NOT EXISTS uj_saved_searches (
	name TEXT PRIMARY KEY,
	filters TEXT NOT NULL,
	created_at TEXT NOT NULL,
	baseline_at TEXT,
	last_advanced_at TEXT,
	last_checked_at TEXT
);
CREATE TABLE IF NOT EXISTS uj_saved_search_members (
	name TEXT NOT NULL,
	posting_id TEXT NOT NULL,
	title TEXT,
	first_seen TEXT NOT NULL,
	last_seen TEXT NOT NULL,
	removed_on TEXT,
	PRIMARY KEY (name, posting_id)
);
CREATE TABLE IF NOT EXISTS uj_facets (
	fetched_at TEXT PRIMARY KEY,
	data TEXT NOT NULL
);
`

// EnsureSchema creates the uj_ tables when missing.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	for _, stmt := range strings.Split(schemaSQL, ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("creating uber-jobs tables: %w", err)
		}
	}
	return nil
}

// SyncRun records one read applied to the store.
type SyncRun struct {
	ID           int64  `json:"id"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
	Source       string `json:"source"`
	Scope        string `json:"scope"`
	Total        int    `json:"total"`
	UniqueCount  int    `json:"unique_count"`
	Complete     bool   `json:"complete"`
	ScanCapHit   bool   `json:"scan_cap_hit"`
	ClosedMarked int    `json:"closed_marked"`
	Note         string `json:"note,omitempty"`
}

// ApplyOptions tunes ApplyRead.
type ApplyOptions struct {
	// PreserveExisting keeps stored rows' data and only refreshes last_seen,
	// so an Oracle fallback read (no descriptions) never degrades the store.
	PreserveExisting bool
}

// ApplyStats counts what ApplyRead changed.
type ApplyStats struct {
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
}

// ApplyRead upserts postings from one read. Closures are marked only when the
// read covered the whole corpus (scope "all") and was complete and uncapped;
// a scoped or partial read never closes anything.
func ApplyRead(ctx context.Context, db *sql.DB, postings []Posting, run SyncRun, now time.Time, opts ApplyOptions) (SyncRun, ApplyStats, error) {
	var stats ApplyStats
	stamp := now.UTC().Format(time.RFC3339)
	existing, err := existingIDs(ctx, db)
	if err != nil {
		return run, stats, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return run, stats, err
	}
	defer func() { _ = tx.Rollback() }()
	for _, p := range postings {
		if existing[p.ID] {
			stats.Updated++
			if opts.PreserveExisting {
				if _, err := tx.ExecContext(ctx, `UPDATE uj_postings SET last_seen = ?, closed_on = NULL WHERE id = ?`, stamp, p.ID); err != nil {
					return run, stats, err
				}
				continue
			}
		} else {
			stats.Inserted++
		}
		data, err := json.Marshal(p)
		if err != nil {
			return run, stats, err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO uj_postings (id, title, team, sub_team, country_iso3, posted_raw, posted_on, is_floor, work_pattern, contract_type, source, data, first_seen, last_seen, closed_on)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
ON CONFLICT(id) DO UPDATE SET title=excluded.title, team=excluded.team, sub_team=excluded.sub_team,
	country_iso3=excluded.country_iso3, posted_raw=excluded.posted_raw, posted_on=excluded.posted_on,
	is_floor=excluded.is_floor, work_pattern=excluded.work_pattern, contract_type=excluded.contract_type,
	source=excluded.source, data=excluded.data, last_seen=excluded.last_seen, closed_on=NULL`,
			p.ID, p.Title, p.JobCategory, p.SubTeam, p.CountryCode, p.PostedRaw, p.PostedOn, boolInt(p.PostedDateIsFloor),
			p.WorkPattern, p.ContractType, p.Source, string(data), stamp, stamp); err != nil {
			return run, stats, fmt.Errorf("upserting posting %s: %w", p.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM uj_posting_locations WHERE posting_id = ?`, p.ID); err != nil {
			return run, stats, err
		}
		for i, l := range p.Locations {
			if _, err := tx.ExecContext(ctx, `INSERT INTO uj_posting_locations (posting_id, idx, country_iso3, country, city) VALUES (?, ?, ?, ?, ?)`,
				p.ID, i, l.CountryCode, l.Country, l.City); err != nil {
				return run, stats, err
			}
		}
	}
	closed := 0
	// An empty full read never closes anything: the site always lists
	// postings, so zero rows is a bad read even when it claims complete.
	if run.Scope == "all" && run.Complete && !run.ScanCapHit && len(postings) > 0 {
		ids := make([]string, 0, len(postings))
		for _, p := range postings {
			ids = append(ids, p.ID)
		}
		res, err := tx.ExecContext(ctx, `UPDATE uj_postings SET closed_on = ? WHERE closed_on IS NULL AND id NOT IN (SELECT value FROM json_each(?))`, stamp, mustJSON(ids))
		if err != nil {
			return run, stats, fmt.Errorf("marking closed postings: %w", err)
		}
		n, _ := res.RowsAffected()
		closed = int(n)
	}
	run.ClosedMarked = closed
	run.FinishedAt = stamp
	r, err := tx.ExecContext(ctx, `INSERT INTO uj_sync_runs (started_at, finished_at, source, scope, total, unique_count, complete, scan_cap_hit, closed_marked, note) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.StartedAt, run.FinishedAt, run.Source, run.Scope, run.Total, run.UniqueCount, boolInt(run.Complete), boolInt(run.ScanCapHit), run.ClosedMarked, run.Note)
	if err != nil {
		return run, stats, err
	}
	run.ID, _ = r.LastInsertId()
	if err := tx.Commit(); err != nil {
		return run, stats, err
	}
	return run, stats, nil
}

func existingIDs(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT id FROM uj_postings`)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// LastFullSync returns the latest complete, uncapped, whole-corpus read.
func LastFullSync(ctx context.Context, db *sql.DB) (*SyncRun, error) {
	row := db.QueryRowContext(ctx, `SELECT id, started_at, finished_at, source, scope, total, unique_count, complete, scan_cap_hit, closed_marked, COALESCE(note, '')
FROM uj_sync_runs WHERE scope = 'all' AND complete = 1 AND scan_cap_hit = 0 ORDER BY id DESC LIMIT 1`)
	var r SyncRun
	var complete, capped int
	if err := row.Scan(&r.ID, &r.StartedAt, &r.FinishedAt, &r.Source, &r.Scope, &r.Total, &r.UniqueCount, &complete, &capped, &r.ClosedMarked, &r.Note); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	r.Complete, r.ScanCapHit = complete == 1, capped == 1
	return &r, nil
}

// SyncRunsAfter counts the sync runs recorded after the run with id: keyword,
// partial or fallback reads that changed the store after that run.
func SyncRunsAfter(ctx context.Context, db *sql.DB, id int64) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM uj_sync_runs WHERE id > ?`, id).Scan(&n)
	return n, err
}

// FirstSyncAt returns when the store first saw any complete full read.
func FirstSyncAt(ctx context.Context, db *sql.DB) (time.Time, bool) {
	var s sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT MIN(started_at) FROM uj_sync_runs WHERE scope = 'all' AND complete = 1`).Scan(&s); err != nil || !s.Valid {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s.String)
	return t, err == nil
}

// StoredPosting is a posting with its local history.
type StoredPosting struct {
	Posting
	FirstSeen string  `json:"first_seen"`
	LastSeen  string  `json:"last_seen"`
	ClosedOn  *string `json:"closed_on"`
}

// LoadPostings reads postings from the store; openOnly skips closed rows.
// Rows are drained before returning so callers can issue follow-up queries.
func LoadPostings(ctx context.Context, db *sql.DB, openOnly bool) ([]StoredPosting, error) {
	q := `SELECT data, first_seen, last_seen, closed_on FROM uj_postings`
	if openOnly {
		q += ` WHERE closed_on IS NULL`
	}
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]StoredPosting, 0)
	for rows.Next() {
		var data, first, last string
		var closed sql.NullString
		if err := rows.Scan(&data, &first, &last, &closed); err != nil {
			_ = rows.Close()
			return nil, err
		}
		var sp StoredPosting
		if err := json.Unmarshal([]byte(data), &sp.Posting); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("decoding stored posting: %w", err)
		}
		sp.FirstSeen, sp.LastSeen = first, last
		if closed.Valid {
			c := closed.String
			sp.ClosedOn = &c
		}
		out = append(out, sp)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// CountPostings returns the number of stored postings (empty-store check by
// row count, never by file existence).
func CountPostings(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM uj_postings`).Scan(&n)
	return n, err
}

// SaveFacets stores a facets snapshot.
func SaveFacets(ctx context.Context, db *sql.DB, f *Facets, now time.Time) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT OR REPLACE INTO uj_facets (fetched_at, data) VALUES (?, ?)`, now.UTC().Format(time.RFC3339), string(data))
	return err
}

// LatestFacets returns the newest facets snapshot.
func LatestFacets(ctx context.Context, db *sql.DB) (*Facets, string, error) {
	var at, data string
	err := db.QueryRowContext(ctx, `SELECT fetched_at, data FROM uj_facets ORDER BY fetched_at DESC LIMIT 1`).Scan(&at, &data)
	if err == sql.ErrNoRows {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	var f Facets
	if err := json.Unmarshal([]byte(data), &f); err != nil {
		return nil, "", err
	}
	return &f, at, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
