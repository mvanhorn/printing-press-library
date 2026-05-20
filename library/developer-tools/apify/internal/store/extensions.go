// Package store extensions: tables that don't map to a spec endpoint but
// power the novel features (novelty diffing, cross-Actor FTS, cost ledger,
// presets, workflow history). All extension tables are prefixed `pp_`
// to keep them distinct from the spec-derived schema.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// EnsureExtensions creates the pp_* tables and FTS5 indexes. Safe to call
// on every Open — CREATE IF NOT EXISTS is idempotent.
func (s *Store) EnsureExtensions(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS pp_dataset_items (
			hash TEXT PRIMARY KEY,
			source_actor TEXT NOT NULL,
			run_id TEXT,
			dataset_id TEXT,
			url TEXT,
			title TEXT,
			body TEXT,
			author TEXT,
			published_at TEXT,
			engagement_score INTEGER,
			fetched_at TEXT NOT NULL,
			raw_json TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS pp_dataset_items_actor_idx ON pp_dataset_items(source_actor, fetched_at DESC)`,
		`CREATE INDEX IF NOT EXISTS pp_dataset_items_run_idx ON pp_dataset_items(run_id)`,
		`CREATE INDEX IF NOT EXISTS pp_dataset_items_published_idx ON pp_dataset_items(published_at DESC)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS pp_dataset_items_fts USING fts5(
			hash UNINDEXED, url, title, body, author, source_actor,
			content='pp_dataset_items', content_rowid='rowid'
		)`,
		// keep FTS in sync via triggers
		`CREATE TRIGGER IF NOT EXISTS pp_dataset_items_ai AFTER INSERT ON pp_dataset_items BEGIN
			INSERT INTO pp_dataset_items_fts(rowid, hash, url, title, body, author, source_actor)
			VALUES (new.rowid, new.hash, new.url, new.title, new.body, new.author, new.source_actor);
		END`,
		`CREATE TRIGGER IF NOT EXISTS pp_dataset_items_ad AFTER DELETE ON pp_dataset_items BEGIN
			DELETE FROM pp_dataset_items_fts WHERE rowid = old.rowid;
		END`,
		// UPDATE trigger: an FTS5 external-content table needs all three of
		// INSERT/UPDATE/DELETE maintained. UpsertNormalizedItem uses
		// INSERT OR REPLACE today (DELETE+INSERT, covered above), but a direct
		// UPDATE on pp_dataset_items would otherwise leave the index stale.
		`CREATE TRIGGER IF NOT EXISTS pp_dataset_items_au AFTER UPDATE ON pp_dataset_items BEGIN
			DELETE FROM pp_dataset_items_fts WHERE rowid = old.rowid;
			INSERT INTO pp_dataset_items_fts(rowid, hash, url, title, body, author, source_actor)
			VALUES (new.rowid, new.hash, new.url, new.title, new.body, new.author, new.source_actor);
		END`,
		`CREATE TABLE IF NOT EXISTS pp_actor_run_history (
			run_id TEXT PRIMARY KEY,
			actor_id TEXT NOT NULL,
			actor_name TEXT,
			status TEXT,
			compute_units REAL DEFAULT 0,
			memory_mbytes INTEGER DEFAULT 0,
			duration_secs REAL DEFAULT 0,
			dataset_id TEXT,
			started_at TEXT,
			finished_at TEXT,
			input_json TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS pp_run_history_actor_idx ON pp_actor_run_history(actor_id, started_at DESC)`,
		`CREATE TABLE IF NOT EXISTS pp_presets (
			name TEXT NOT NULL,
			actor_id TEXT NOT NULL,
			input_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			created_from_run TEXT,
			PRIMARY KEY (name, actor_id)
		)`,
		`CREATE TABLE IF NOT EXISTS pp_workflow_runs (
			id TEXT PRIMARY KEY,
			workflow_name TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			status TEXT NOT NULL,
			result_json TEXT
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("creating extension table: %w (stmt: %s)", err, firstLine(stmt))
		}
	}
	return nil
}

// UpsertNormalizedItem inserts or updates a normalized dataset item.
// Returns true if the hash was new (item is novel).
func (s *Store) UpsertNormalizedItem(ctx context.Context,
	hash, sourceActor, runID, datasetID,
	url, title, body, author, publishedAt string,
	engagementScore int64, fetchedAt time.Time, rawJSON []byte) (bool, error) {
	// Check existence first to report novelty
	var existing string
	err := s.db.QueryRowContext(ctx,
		`SELECT hash FROM pp_dataset_items WHERE hash = ?`, hash).Scan(&existing)
	isNew := err == sql.ErrNoRows
	if err != nil && !isNew {
		return false, err
	}
	// Upsert (INSERT OR REPLACE keeps the latest fetched_at)
	_, err = s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO pp_dataset_items
		(hash, source_actor, run_id, dataset_id, url, title, body, author, published_at, engagement_score, fetched_at, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, hash, sourceActor, runID, datasetID, url, title, body, author, publishedAt,
		engagementScore, fetchedAt.UTC().Format(time.RFC3339), string(rawJSON))
	if err != nil {
		return false, err
	}
	return isNew, nil
}

// HashesSeen returns the subset of given hashes that already exist in
// pp_dataset_items. Used by --only-new to filter incoming batches.
func (s *Store) HashesSeen(ctx context.Context, hashes []string) (map[string]bool, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	q := `SELECT hash FROM pp_dataset_items WHERE hash IN (`
	args := make([]any, len(hashes))
	for i, h := range hashes {
		if i > 0 {
			q += ","
		}
		q += "?"
		args[i] = h
	}
	q += ")"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := make(map[string]bool, len(hashes))
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		seen[h] = true
	}
	return seen, rows.Err()
}

// RecordActorRun captures the run's cost-relevant numbers after it completes.
// Caller passes whatever fields are known; zeros are fine for missing data.
func (s *Store) RecordActorRun(ctx context.Context,
	runID, actorID, actorName, status string,
	cu float64, memoryMbytes int, durationSecs float64,
	datasetID string, startedAt, finishedAt time.Time, inputJSON []byte) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO pp_actor_run_history
		(run_id, actor_id, actor_name, status, compute_units, memory_mbytes, duration_secs, dataset_id, started_at, finished_at, input_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, runID, actorID, actorName, status, cu, memoryMbytes, durationSecs,
		datasetID, timeStrOrEmpty(startedAt), timeStrOrEmpty(finishedAt), string(inputJSON))
	return err
}

// LoadActorRunHistory returns prior runs for the given actor (or all when
// actorID is empty). Sorted by started_at descending.
func (s *Store) LoadActorRunHistory(ctx context.Context, actorID string, limit int) (
	[]ActorRunRecord, error) {
	q := `SELECT run_id, actor_id, actor_name, status, compute_units, memory_mbytes,
	             duration_secs, dataset_id, started_at, finished_at
	      FROM pp_actor_run_history`
	args := []any{}
	if actorID != "" {
		// Callers pass the human-readable Actor slug (e.g. apidojo/twitter-
		// scraper-lite), which is stored in actor_name. actor_id holds the
		// opaque Apify-internal ID, which callers never have on hand.
		q += ` WHERE actor_name = ?`
		args = append(args, actorID)
	}
	q += ` ORDER BY started_at DESC`
	if limit > 0 {
		q += fmt.Sprintf(` LIMIT %d`, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActorRunRecord
	for rows.Next() {
		var r ActorRunRecord
		var started, finished string
		if err := rows.Scan(&r.RunID, &r.ActorID, &r.ActorName, &r.Status,
			&r.ComputeUnits, &r.MemoryMbytes, &r.DurationSecs, &r.DatasetID,
			&started, &finished); err != nil {
			return nil, err
		}
		r.StartedAt = parseStoreTime(started)
		r.FinishedAt = parseStoreTime(finished)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ActorRunRecord is the row shape returned by LoadActorRunHistory.
type ActorRunRecord struct {
	RunID        string
	ActorID      string
	ActorName    string
	Status       string
	ComputeUnits float64
	MemoryMbytes int
	DurationSecs float64
	DatasetID    string
	StartedAt    time.Time
	FinishedAt   time.Time
}

// SavePreset records a named input preset for an Actor.
func (s *Store) SavePreset(ctx context.Context, name, actorID, fromRun string, inputJSON []byte) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO pp_presets (name, actor_id, input_json, created_at, created_from_run)
		VALUES (?, ?, ?, ?, ?)
	`, name, actorID, string(inputJSON), time.Now().UTC().Format(time.RFC3339), fromRun)
	return err
}

// LoadPreset returns the input JSON for a saved preset, or empty if not found.
func (s *Store) LoadPreset(ctx context.Context, name, actorID string) ([]byte, error) {
	var in string
	err := s.db.QueryRowContext(ctx,
		`SELECT input_json FROM pp_presets WHERE name = ? AND actor_id = ?`,
		name, actorID).Scan(&in)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []byte(in), nil
}

// --- helpers ---

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

func timeStrOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func parseStoreTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
