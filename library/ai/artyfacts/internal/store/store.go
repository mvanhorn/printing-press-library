// Copyright 2026 bernard-maltais. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection for the artyfacts local cache.
type DB struct {
	db   *sql.DB
	path string
}

// DefaultPath returns the canonical SQLite database path.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "artyfacts-pp-cli", "store.db")
}

// Open opens (or creates) the store at path and runs schema migrations.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating store directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		db.Close()
		return nil, err
	}
	s := &DB{db: db, path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *DB) Close() error { return s.db.Close() }

// Path returns the database file path.
func (s *DB) Path() string { return s.path }

func (s *DB) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS artifacts (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL DEFAULT '',
    artifact_type TEXT NOT NULL DEFAULT '',
    summary     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT '',
    visibility  TEXT NOT NULL DEFAULT '',
    retention   TEXT NOT NULL DEFAULT '',
    tags        TEXT NOT NULL DEFAULT '[]',
    parent_id   TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT '',
    updated_at  TEXT NOT NULL DEFAULT '',
    raw         TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS sections (
    id          TEXT NOT NULL,
    artifact_id TEXT NOT NULL,
    heading     TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL DEFAULT '',
    body        TEXT NOT NULL DEFAULT '',
    position    INTEGER NOT NULL DEFAULT 0,
    agent_name  TEXT NOT NULL DEFAULT '',
    is_streaming INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT '',
    updated_at  TEXT NOT NULL DEFAULT '',
    raw         TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY (artifact_id, id),
    FOREIGN KEY (artifact_id) REFERENCES artifacts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS org_context (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    data        TEXT NOT NULL DEFAULT '{}',
    synced_at   TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS sync_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    synced_at   TEXT NOT NULL,
    artifacts   INTEGER NOT NULL DEFAULT 0,
    sections    INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    mode        TEXT NOT NULL DEFAULT 'full'
);

CREATE VIRTUAL TABLE IF NOT EXISTS artifacts_fts USING fts5(
    id UNINDEXED,
    title,
    summary,
    content=artifacts,
    content_rowid=rowid
);

CREATE VIRTUAL TABLE IF NOT EXISTS sections_fts USING fts5(
    artifact_id UNINDEXED,
    id UNINDEXED,
    heading,
    body,
    content=sections,
    content_rowid=rowid
);

CREATE INDEX IF NOT EXISTS idx_artifacts_parent   ON artifacts(parent_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_type     ON artifacts(artifact_type);
CREATE INDEX IF NOT EXISTS idx_artifacts_status   ON artifacts(status);
CREATE INDEX IF NOT EXISTS idx_artifacts_updated  ON artifacts(updated_at);
CREATE INDEX IF NOT EXISTS idx_sections_artifact  ON sections(artifact_id);
CREATE INDEX IF NOT EXISTS idx_sections_agent     ON sections(agent_name);
`)
	return err
}

// UpsertArtifact inserts or replaces an artifact record.
func (s *DB) UpsertArtifact(a map[string]any) error {
	id, _ := a["id"].(string)
	if id == "" {
		return fmt.Errorf("artifact missing id")
	}
	raw, _ := json.Marshal(a)

	tags, _ := json.Marshal(sliceOf(a["tags"]))
	_, err := s.db.Exec(`
INSERT INTO artifacts (id, title, artifact_type, summary, status, visibility, retention, tags, parent_id, created_at, updated_at, raw)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    title=excluded.title, artifact_type=excluded.artifact_type, summary=excluded.summary,
    status=excluded.status, visibility=excluded.visibility, retention=excluded.retention,
    tags=excluded.tags, parent_id=excluded.parent_id, created_at=excluded.created_at,
    updated_at=excluded.updated_at, raw=excluded.raw`,
		id,
		strOf(a["title"]),
		firstStr(a["artifact_type"], a["type"]),
		strOf(a["summary"]),
		strOf(a["status"]),
		strOf(a["visibility"]),
		strOf(a["retention"]),
		string(tags),
		strOf(a["parent_id"]),
		strOf(a["created_at"]),
		strOf(a["updated_at"]),
		string(raw),
	)
	if err != nil {
		return err
	}
	// rebuild FTS
	_, err = s.db.Exec(`INSERT INTO artifacts_fts(artifacts_fts) VALUES('rebuild')`)
	return err
}

// UpsertSection inserts or replaces a section record.
func (s *DB) UpsertSection(sec map[string]any, artifactID string) error {
	id, _ := sec["id"].(string)
	if id == "" {
		return fmt.Errorf("section missing id")
	}
	raw, _ := json.Marshal(sec)
	streaming := 0
	if b, ok := sec["is_streaming"].(bool); ok && b {
		streaming = 1
	}
	_, err := s.db.Exec(`
INSERT INTO sections (id, artifact_id, heading, type, body, position, agent_name, is_streaming, created_at, updated_at, raw)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(artifact_id, id) DO UPDATE SET
    heading=excluded.heading, type=excluded.type, body=excluded.body,
    position=excluded.position, agent_name=excluded.agent_name,
    is_streaming=excluded.is_streaming, created_at=excluded.created_at,
    updated_at=excluded.updated_at, raw=excluded.raw`,
		id,
		artifactID,
		strOf(sec["heading"]),
		strOf(sec["type"]),
		strOf(sec["body"]),
		intOf(sec["position"]),
		strOf(sec["agent_name"]),
		streaming,
		strOf(sec["created_at"]),
		strOf(sec["updated_at"]),
		string(raw),
	)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO sections_fts(sections_fts) VALUES('rebuild')`)
	return err
}

// UpsertOrgContext stores the org context.
func (s *DB) UpsertOrgContext(data json.RawMessage) error {
	_, err := s.db.Exec(`
INSERT INTO org_context (id, data, synced_at) VALUES (1, ?, ?)
ON CONFLICT(id) DO UPDATE SET data=excluded.data, synced_at=excluded.synced_at`,
		string(data), time.Now().UTC().Format(time.RFC3339))
	return err
}

// LogSync records a sync run summary.
func (s *DB) LogSync(mode string, artifacts, sections int, duration time.Duration) error {
	_, err := s.db.Exec(`INSERT INTO sync_log (synced_at, artifacts, sections, duration_ms, mode) VALUES (?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), artifacts, sections, duration.Milliseconds(), mode)
	return err
}

// LastSyncTime returns the timestamp of the most recent sync, or zero if none.
func (s *DB) LastSyncTime() (time.Time, error) {
	var ts string
	err := s.db.QueryRow(`SELECT synced_at FROM sync_log ORDER BY id DESC LIMIT 1`).Scan(&ts)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339, ts)
	return t, err
}

// SearchResult holds a FTS match.
type SearchResult struct {
	ArtifactID   string
	ArtifactTitle string
	SectionID    string
	SectionHeading string
	Snippet      string
	Score        float64
}

// Search runs a full-text query across artifact titles/summaries and section bodies.
func (s *DB) Search(query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("empty search query")
	}

	var results []SearchResult

	// Search artifacts first
	rows, err := s.db.Query(`
SELECT a.id, a.title, snippet(artifacts_fts, 1, '[', ']', '...', 10)
FROM artifacts_fts
JOIN artifacts a ON artifacts_fts.id = a.id
WHERE artifacts_fts MATCH ?
ORDER BY rank
LIMIT ?`, q, limit)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var r SearchResult
			if err := rows.Scan(&r.ArtifactID, &r.ArtifactTitle, &r.Snippet); err == nil {
				results = append(results, r)
			}
		}
	}

	// Search sections
	rows2, err := s.db.Query(`
SELECT s.artifact_id, a.title, s.id, s.heading, snippet(sections_fts, 3, '[', ']', '...', 10)
FROM sections_fts
JOIN sections s ON sections_fts.artifact_id = s.artifact_id AND sections_fts.id = s.id
JOIN artifacts a ON s.artifact_id = a.id
WHERE sections_fts MATCH ?
ORDER BY rank
LIMIT ?`, q, limit)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var r SearchResult
			if err := rows2.Scan(&r.ArtifactID, &r.ArtifactTitle, &r.SectionID, &r.SectionHeading, &r.Snippet); err == nil {
				results = append(results, r)
			}
		}
	}

	return results, nil
}

// ArtifactRow is a simplified artifact from the local store.
type ArtifactRow struct {
	ID           string
	Title        string
	ArtifactType string
	Summary      string
	Status       string
	ParentID     string
	UpdatedAt    string
}

// ListArtifacts returns artifacts matching the given filters.
func (s *DB) ListArtifacts(artifactType, parentID, status string, rootOnly bool) ([]ArtifactRow, error) {
	q := `SELECT id, title, artifact_type, summary, status, parent_id, updated_at FROM artifacts WHERE 1=1`
	var args []any
	if artifactType != "" {
		q += ` AND artifact_type = ?`
		args = append(args, artifactType)
	}
	if parentID != "" {
		q += ` AND parent_id = ?`
		args = append(args, parentID)
	}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	if rootOnly {
		q += ` AND (parent_id = '' OR parent_id IS NULL)`
	}
	q += ` ORDER BY updated_at DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ArtifactRow
	for rows.Next() {
		var r ArtifactRow
		if err := rows.Scan(&r.ID, &r.Title, &r.ArtifactType, &r.Summary, &r.Status, &r.ParentID, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetArtifact returns the raw JSON for a single artifact by ID.
func (s *DB) GetArtifact(id string) (json.RawMessage, error) {
	var raw string
	err := s.db.QueryRow(`SELECT raw FROM artifacts WHERE id = ?`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("artifact %s not found in local store (run sync first)", id)
	}
	return json.RawMessage(raw), err
}

// StaleArtifacts returns artifacts that were updated remotely after their local record.
// remoteUpdates is a map of artifact_id -> remote updated_at string.
func (s *DB) StaleArtifacts(remoteUpdates map[string]string) ([]StaleRow, error) {
	var out []StaleRow
	for id, remoteUpdatedAt := range remoteUpdates {
		var localUpdatedAt string
		err := s.db.QueryRow(`SELECT updated_at FROM artifacts WHERE id = ?`, id).Scan(&localUpdatedAt)
		if err == sql.ErrNoRows {
			out = append(out, StaleRow{ID: id, RemoteUpdatedAt: remoteUpdatedAt, Reason: "not-synced"})
			continue
		}
		if err != nil {
			continue
		}
		if remoteUpdatedAt > localUpdatedAt {
			out = append(out, StaleRow{ID: id, LocalUpdatedAt: localUpdatedAt, RemoteUpdatedAt: remoteUpdatedAt, Reason: "remote-newer"})
		}
	}
	return out, nil
}

// StaleRow is a single stale artifact entry.
type StaleRow struct {
	ID              string
	LocalUpdatedAt  string
	RemoteUpdatedAt string
	Reason          string
}

// AgentStats holds contribution stats for a single agent.
type AgentStats struct {
	AgentName     string
	SectionCount  int
	ArtifactCount int
	LastActive    string
}

// AgentContributions returns section and artifact counts grouped by agent_name.
func (s *DB) AgentContributions() ([]AgentStats, error) {
	rows, err := s.db.Query(`
SELECT s.agent_name,
       COUNT(s.id) AS section_count,
       COUNT(DISTINCT s.artifact_id) AS artifact_count,
       MAX(s.updated_at) AS last_active
FROM sections s
WHERE s.agent_name != ''
GROUP BY s.agent_name
ORDER BY section_count DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentStats
	for rows.Next() {
		var a AgentStats
		if err := rows.Scan(&a.AgentName, &a.SectionCount, &a.ArtifactCount, &a.LastActive); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// TypeStats returns artifact counts grouped by type.
func (s *DB) TypeStats() ([]struct{ Type, Count string }, error) {
	rows, err := s.db.Query(`SELECT artifact_type, COUNT(*) FROM artifacts GROUP BY artifact_type ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct{ Type, Count string }
	for rows.Next() {
		var r struct{ Type, Count string }
		if err := rows.Scan(&r.Type, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// StatusStats returns artifact counts grouped by status.
func (s *DB) StatusStats() ([]struct{ Status, Count string }, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM artifacts GROUP BY status ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct{ Status, Count string }
	for rows.Next() {
		var r struct{ Status, Count string }
		if err := rows.Scan(&r.Status, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateArtifactStatus updates the status field for a single artifact (local store only).
func (s *DB) UpdateArtifactStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE artifacts SET status = ? WHERE id = ?`, status, id)
	return err
}

// ListByStatus returns all artifact IDs matching a type and optional status filter.
func (s *DB) ListByStatus(artifactType, status string) ([]string, error) {
	q := `SELECT id FROM artifacts WHERE 1=1`
	var args []any
	if artifactType != "" {
		q += ` AND artifact_type = ?`
		args = append(args, artifactType)
	}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ChildArtifacts returns all artifacts whose parent_id matches parentID.
func (s *DB) ChildArtifacts(parentID string) ([]ArtifactRow, error) {
	return s.ListArtifacts("", parentID, "", false)
}

// ArtifactCount returns the total number of artifacts in the store.
func (s *DB) ArtifactCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM artifacts`).Scan(&n)
	return n, err
}

// SectionCount returns the total number of sections in the store.
func (s *DB) SectionCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sections`).Scan(&n)
	return n, err
}

// helper: string field from map
func strOf(v any) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// helper: first non-empty string field
func firstStr(vals ...any) string {
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// helper: int field from map
func intOf(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

// helper: convert any to []any
func sliceOf(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}
