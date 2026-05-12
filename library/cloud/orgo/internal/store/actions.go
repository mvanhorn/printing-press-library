// Hand-authored: actions ledger for orgo-pp-cli novel features.
// Created during Phase 3 build; survives regen because it lives in its own file.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const actionsSchema = `
CREATE TABLE IF NOT EXISTS actions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts TEXT NOT NULL,
    computer_id TEXT NOT NULL,
    workspace_id TEXT,
    kind TEXT NOT NULL,
    command TEXT,
    args_json TEXT,
    output_snippet TEXT,
    exit_code INTEGER,
    duration_ms INTEGER,
    status_code INTEGER
);
CREATE INDEX IF NOT EXISTS idx_actions_ts ON actions(ts);
CREATE INDEX IF NOT EXISTS idx_actions_computer ON actions(computer_id);
CREATE INDEX IF NOT EXISTS idx_actions_kind ON actions(kind);
CREATE VIRTUAL TABLE IF NOT EXISTS actions_fts USING fts5(
    computer_id, kind, command, args_json, output_snippet,
    content='actions', content_rowid='id'
);
CREATE TRIGGER IF NOT EXISTS actions_ai AFTER INSERT ON actions BEGIN
    INSERT INTO actions_fts(rowid, computer_id, kind, command, args_json, output_snippet)
    VALUES (new.id, new.computer_id, new.kind, COALESCE(new.command,''), COALESCE(new.args_json,''), COALESCE(new.output_snippet,''));
END;
CREATE TRIGGER IF NOT EXISTS actions_ad AFTER DELETE ON actions BEGIN
    INSERT INTO actions_fts(actions_fts, rowid, computer_id, kind, command, args_json, output_snippet)
    VALUES('delete', old.id, old.computer_id, old.kind, COALESCE(old.command,''), COALESCE(old.args_json,''), COALESCE(old.output_snippet,''));
END;
`

func (s *Store) ensureActionsSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, actionsSchema)
	return err
}

type Action struct {
	ID            int64
	Timestamp     time.Time
	ComputerID    string
	WorkspaceID   string
	Kind          string
	Command       string
	ArgsJSON      string
	OutputSnippet string
	ExitCode      sql.NullInt64
	DurationMS    sql.NullInt64
	StatusCode    sql.NullInt64
}

func (s *Store) LogAction(ctx context.Context, a Action) error {
	if err := s.ensureActionsSchema(ctx); err != nil {
		return fmt.Errorf("ensuring actions schema: %w", err)
	}
	if a.Timestamp.IsZero() {
		a.Timestamp = time.Now().UTC()
	}
	if len(a.OutputSnippet) > 4096 {
		a.OutputSnippet = a.OutputSnippet[:4096] + "...(truncated)"
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO actions (ts, computer_id, workspace_id, kind, command, args_json, output_snippet, exit_code, duration_ms, status_code)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Timestamp.Format(time.RFC3339Nano),
		a.ComputerID, a.WorkspaceID, a.Kind, a.Command, a.ArgsJSON, a.OutputSnippet,
		nullableInt(a.ExitCode), nullableInt(a.DurationMS), nullableInt(a.StatusCode))
	return err
}

func nullableInt(n sql.NullInt64) any {
	if !n.Valid {
		return nil
	}
	return n.Int64
}

type ActionFilter struct {
	WorkspaceID string
	ComputerID  string
	Kinds       []string
	Since       time.Time
	Until       time.Time
	Search      string // FTS5 query
	Limit       int
}

func (s *Store) QueryActions(ctx context.Context, f ActionFilter) ([]Action, error) {
	if err := s.ensureActionsSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensuring actions schema: %w", err)
	}
	var (
		clauses []string
		args    []any
	)

	if f.Search != "" {
		// FTS5 path: join with the virtual table.
		clauses = append(clauses, "id IN (SELECT rowid FROM actions_fts WHERE actions_fts MATCH ?)")
		args = append(args, f.Search)
	}
	if f.ComputerID != "" {
		clauses = append(clauses, "computer_id = ?")
		args = append(args, f.ComputerID)
	}
	if f.WorkspaceID != "" {
		clauses = append(clauses, "(workspace_id = ? OR workspace_id IS NULL)")
		args = append(args, f.WorkspaceID)
	}
	if len(f.Kinds) > 0 {
		placeholders := make([]string, len(f.Kinds))
		for i, k := range f.Kinds {
			placeholders[i] = "?"
			args = append(args, k)
		}
		clauses = append(clauses, "kind IN ("+strings.Join(placeholders, ",")+")")
	}
	if !f.Since.IsZero() {
		clauses = append(clauses, "ts >= ?")
		args = append(args, f.Since.UTC().Format(time.RFC3339Nano))
	}
	if !f.Until.IsZero() {
		clauses = append(clauses, "ts <= ?")
		args = append(args, f.Until.UTC().Format(time.RFC3339Nano))
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 500
	}

	q := `SELECT id, ts, computer_id, COALESCE(workspace_id,''), kind,
	       COALESCE(command,''), COALESCE(args_json,''), COALESCE(output_snippet,''),
	       exit_code, duration_ms, status_code
	      FROM actions`
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY ts DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Action
	for rows.Next() {
		var a Action
		var tsStr string
		if err := rows.Scan(&a.ID, &tsStr, &a.ComputerID, &a.WorkspaceID, &a.Kind,
			&a.Command, &a.ArgsJSON, &a.OutputSnippet,
			&a.ExitCode, &a.DurationMS, &a.StatusCode); err != nil {
			return nil, err
		}
		a.Timestamp, _ = time.Parse(time.RFC3339Nano, tsStr)
		out = append(out, a)
	}
	return out, rows.Err()
}

// LastActionTimes returns the most recent action timestamp for each computer
// that has at least one action. Used by `idle`, `oversized`, `cost`.
func (s *Store) LastActionTimes(ctx context.Context) (map[string]time.Time, error) {
	if err := s.ensureActionsSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensuring actions schema: %w", err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT computer_id, MAX(ts) FROM actions GROUP BY computer_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]time.Time{}
	for rows.Next() {
		var id, tsStr string
		if err := rows.Scan(&id, &tsStr); err != nil {
			return nil, err
		}
		t, _ := time.Parse(time.RFC3339Nano, tsStr)
		m[id] = t
	}
	return m, rows.Err()
}

// EncodeActionArgs serialises a small map of CLI args for the actions log.
// Returns "" on failure rather than blocking the action call.
func EncodeActionArgs(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}
