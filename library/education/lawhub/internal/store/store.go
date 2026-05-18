package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const Schema = `
CREATE TABLE IF NOT EXISTS tests (
  id TEXT PRIMARY KEY,
  name TEXT,
  type TEXT,
  available INTEGER,
  source_url TEXT,
  synced_at TEXT
);
CREATE TABLE IF NOT EXISTS attempts (
  id TEXT PRIMARY KEY,
  test_id TEXT,
  test_name TEXT,
  mode TEXT,
  started_at TEXT,
  completed_at TEXT,
  scaled_score INTEGER,
  raw_score INTEGER,
  total_questions INTEGER,
  correct_count INTEGER,
  source_url TEXT,
  synced_at TEXT
);
CREATE TABLE IF NOT EXISTS sections (
  id TEXT PRIMARY KEY,
  attempt_id TEXT,
  section_index INTEGER,
  section_type TEXT,
  correct_count INTEGER,
  total_questions INTEGER,
  time_limit_seconds INTEGER,
  time_spent_seconds INTEGER,
  FOREIGN KEY(attempt_id) REFERENCES attempts(id)
);
CREATE TABLE IF NOT EXISTS questions (
  id TEXT PRIMARY KEY,
  attempt_id TEXT,
  section_id TEXT,
  section_index INTEGER,
  question_number INTEGER,
  question_type TEXT,
  chosen_answer TEXT,
  correct_answer TEXT,
  is_correct INTEGER,
  time_spent_seconds INTEGER,
  flagged INTEGER,
  eliminated_count INTEGER,
  review_note TEXT,
  source_url TEXT,
  answered INTEGER,
  answer_state_json TEXT,
  difficulty INTEGER,
  FOREIGN KEY(attempt_id) REFERENCES attempts(id),
  FOREIGN KEY(section_id) REFERENCES sections(id)
);
CREATE TABLE IF NOT EXISTS sync_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  synced_at TEXT,
  source TEXT,
  item_count INTEGER,
  status TEXT,
  message TEXT
);
`

func Open(dataDir string) (*sql.DB, string, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, "", err
	}
	dbPath := filepath.Join(dataDir, "lawhub.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, "", err
	}
	if _, err := db.Exec(Schema); err != nil {
		db.Close()
		return nil, "", err
	}
	for _, c := range []struct{ column, decl string }{
		{column: "answered", decl: "INTEGER"},
		{column: "answer_state_json", decl: "TEXT"},
		{column: "difficulty", decl: "INTEGER"},
	} {
		if err := ensureColumn(db, "questions", c.column, c.decl); err != nil {
			db.Close()
			return nil, "", err
		}
	}
	return db, dbPath, nil
}

func ensureColumn(db *sql.DB, table, column, decl string) error {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, table))
	if err != nil {
		return fmt.Errorf("table_info %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info %s: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table_info %s: %w", table, err)
	}
	if _, err := db.Exec(fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, table, column, decl)); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

func Count(db *sql.DB, table string) (int, error) {
	queries := map[string]string{
		"tests":     "SELECT COUNT(*) FROM tests",
		"attempts":  "SELECT COUNT(*) FROM attempts",
		"sections":  "SELECT COUNT(*) FROM sections",
		"questions": "SELECT COUNT(*) FROM questions",
		"sync_log":  "SELECT COUNT(*) FROM sync_log",
	}
	q, ok := queries[table]
	if !ok {
		return 0, fmt.Errorf("unsupported count table: %s", table)
	}
	var c int
	if err := db.QueryRow(q).Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}

func NullString(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}
func NullInt(ni sql.NullInt64) any {
	if ni.Valid {
		return ni.Int64
	}
	return nil
}
func NullFloat(nf sql.NullFloat64) any {
	if nf.Valid {
		return nf.Float64
	}
	return nil
}
