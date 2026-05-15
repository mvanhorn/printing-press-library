package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database for offline access to synced API data.
type Store struct {
	db *sql.DB
}

// DefaultPath returns the default store path under the user's home directory.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "booking-com-demand-pp-cli", "store.db")
}

// Open opens (or creates) a SQLite store at the given path.
func Open(path string) (*Store, error) {
	return OpenWithContext(context.Background(), path)
}

// OpenWithContext opens a SQLite store respecting the given context.
func OpenWithContext(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating store directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging store: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating store: %w", err)
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for raw SQL access.
func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS accommodations (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS reviews (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS review_scores (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS availability (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS locations_cities (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS locations_districts (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS locations_landmarks (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS accommodations_fts USING fts5(
			name, description, content='accommodations', content_rowid='rowid'
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS reviews_fts USING fts5(
			text, content='reviews', content_rowid='rowid'
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("executing %q: %w", stmt[:40], err)
		}
	}
	return nil
}

// Upsert inserts or replaces a row in the given table.
func (s *Store) Upsert(table, id string, data json.RawMessage) error {
	q := fmt.Sprintf(`INSERT OR REPLACE INTO %s (id, data, synced_at) VALUES (?, ?, ?)`, table)
	_, err := s.db.Exec(q, id, string(data), time.Now().UTC())
	return err
}

// UpsertAccommodation upserts into the accommodations table and updates FTS.
func (s *Store) UpsertAccommodation(id string, data json.RawMessage) error {
	if err := s.Upsert("accommodations", id, data); err != nil {
		return err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	name, _ := obj["name"].(string)
	desc, _ := obj["description"].(string)
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO accommodations_fts(rowid, name, description) VALUES ((SELECT rowid FROM accommodations WHERE id = ?), ?, ?)`,
		id, name, desc,
	)
	return err
}

// UpsertReview upserts into the reviews table and updates FTS.
func (s *Store) UpsertReview(id string, data json.RawMessage) error {
	if err := s.Upsert("reviews", id, data); err != nil {
		return err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	text, _ := obj["text"].(string)
	if text == "" {
		text, _ = obj["content"].(string)
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO reviews_fts(rowid, text) VALUES ((SELECT rowid FROM reviews WHERE id = ?), ?)`,
		id, text,
	)
	return err
}

// Search performs an FTS5 search across accommodations and reviews.
func (s *Store) Search(query string) ([]map[string]any, error) {
	var results []map[string]any

	// Search accommodations
	rows, err := s.db.Query(
		`SELECT a.id, a.data FROM accommodations a
		 JOIN accommodations_fts f ON a.rowid = f.rowid
		 WHERE accommodations_fts MATCH ?
		 LIMIT 50`, query,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, data string
			if err := rows.Scan(&id, &data); err != nil {
				continue
			}
			var obj map[string]any
			if json.Unmarshal([]byte(data), &obj) == nil {
				obj["_source"] = "accommodations"
				obj["_id"] = id
				results = append(results, obj)
			}
		}
	}

	// Search reviews
	rows2, err := s.db.Query(
		`SELECT r.id, r.data FROM reviews r
		 JOIN reviews_fts f ON r.rowid = f.rowid
		 WHERE reviews_fts MATCH ?
		 LIMIT 50`, query,
	)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var id, data string
			if err := rows2.Scan(&id, &data); err != nil {
				continue
			}
			var obj map[string]any
			if json.Unmarshal([]byte(data), &obj) == nil {
				obj["_source"] = "reviews"
				obj["_id"] = id
				results = append(results, obj)
			}
		}
	}

	return results, nil
}

// SQL executes a raw SQL query and returns the results as a slice of maps.
func (s *Store) SQL(query string) ([]map[string]any, error) {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}
	return results, nil
}

// Count returns the number of rows in the given table.
func (s *Store) Count(table string) (int, error) {
	var count int
	err := s.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
	return count, err
}

// LastSyncTime returns the most recent synced_at timestamp for a table.
func (s *Store) LastSyncTime(table string) (time.Time, error) {
	var ts sql.NullTime
	err := s.db.QueryRow(fmt.Sprintf("SELECT MAX(synced_at) FROM %s", table)).Scan(&ts)
	if err != nil || !ts.Valid {
		return time.Time{}, err
	}
	return ts.Time, nil
}
