// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
//
// practice is passage's local, personal layer — the part that's yours and
// compounds: your reflections (the journal), your shelf, and your reading log.
// It rides on the generated store's SQLite database via its own tables.
package practice

import (
	"context"
	"database/sql"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/passage/internal/store"
)

type Practice struct {
	st *store.Store
	db *sql.DB
}

func Open(ctx context.Context, dbPath string) (*Practice, error) {
	st, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	p := &Practice{st: st, db: st.DB()}
	if err := p.migrate(ctx); err != nil {
		_ = st.Close()
		return nil, err
	}
	return p, nil
}

func (p *Practice) Close() error { return p.st.Close() }

func (p *Practice) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS reflections(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			gutenberg_id INTEGER, title TEXT, note TEXT, mood INTEGER, created_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS shelf(
			work_key TEXT PRIMARY KEY, title TEXT, author TEXT, status TEXT, added_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS reading_log(
			work_key TEXT PRIMARY KEY, title TEXT, status TEXT, rating INTEGER,
			started_at TEXT, finished_at TEXT)`,
	}
	for _, s := range stmts {
		if _, err := p.db.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// ---- reflections (the journal) ----

type Reflection struct {
	ID          int    `json:"id"`
	GutenbergID int    `json:"gutenberg_id"`
	Title       string `json:"title"`
	Note        string `json:"note"`
	Mood        int    `json:"mood"`
	CreatedAt   string `json:"created_at"`
}

func (p *Practice) AddReflection(ctx context.Context, gid int, title, note string, mood int) error {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO reflections(gutenberg_id,title,note,mood,created_at) VALUES(?,?,?,?,?)`,
		gid, title, note, mood, now())
	return err
}

func (p *Practice) ListReflections(ctx context.Context, limit int) ([]Reflection, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := p.db.QueryContext(ctx,
		`SELECT id,gutenberg_id,title,note,mood,created_at FROM reflections ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reflection
	for rows.Next() {
		var r Reflection
		if err := rows.Scan(&r.ID, &r.GutenbergID, &r.Title, &r.Note, &r.Mood, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecentSitIDs returns the set of gutenberg_ids sat with in the last `days` days
// (for today's anti-repeat).
func (p *Practice) RecentSitIDs(ctx context.Context, days int) (map[int]bool, error) {
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)
	rows, err := p.db.QueryContext(ctx,
		`SELECT DISTINCT gutenberg_id FROM reflections WHERE created_at >= ?`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[int]bool{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		seen[id] = true
	}
	return seen, rows.Err()
}

// ---- shelf ----

type ShelfItem struct {
	WorkKey string `json:"work_key"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	Status  string `json:"status"`
	AddedAt string `json:"added_at"`
}

func (p *Practice) AddShelf(ctx context.Context, key, title, author, status string) error {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO shelf(work_key,title,author,status,added_at) VALUES(?,?,?,?,?)
		 ON CONFLICT(work_key) DO UPDATE SET status=excluded.status, title=CASE WHEN excluded.title<>'' THEN excluded.title ELSE shelf.title END, author=CASE WHEN excluded.author<>'' THEN excluded.author ELSE shelf.author END`,
		key, title, author, status, now())
	return err
}

func (p *Practice) ListShelf(ctx context.Context, status string) ([]ShelfItem, error) {
	q := `SELECT work_key,title,author,status,added_at FROM shelf`
	args := []any{}
	if status != "" {
		q += ` WHERE status=?`
		args = append(args, status)
	}
	q += ` ORDER BY added_at ASC`
	rows, err := p.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShelfItem
	for rows.Next() {
		var s ShelfItem
		if err := rows.Scan(&s.WorkKey, &s.Title, &s.Author, &s.Status, &s.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ---- reading log ----

type LogItem struct {
	WorkKey    string `json:"work_key"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Rating     int    `json:"rating"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
}

func (p *Practice) LogRead(ctx context.Context, key, title, status string, rating int) error {
	finished := ""
	if status == "read" {
		finished = now()
	}
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO reading_log(work_key,title,status,rating,started_at,finished_at)
		 VALUES(?,?,?,?,?,?)
		 ON CONFLICT(work_key) DO UPDATE SET status=excluded.status, title=CASE WHEN excluded.title<>'' THEN excluded.title ELSE reading_log.title END,
		   rating=CASE WHEN excluded.rating>0 THEN excluded.rating ELSE reading_log.rating END,
		   finished_at=CASE WHEN excluded.status='read' THEN excluded.finished_at ELSE '' END`,
		key, title, status, rating, now(), finished)
	return err
}

type Stats struct {
	Want        int     `json:"want"`
	Reading     int     `json:"reading"`
	Read        int     `json:"read"`
	AvgRating   float64 `json:"avg_rating"`
	Reflections int     `json:"reflections"`
}

func (p *Practice) Stats(ctx context.Context) (Stats, error) {
	var s Stats
	rows, err := p.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM reading_log GROUP BY status`)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			rows.Close()
			return s, err
		}
		switch st {
		case "want":
			s.Want = n
		case "reading":
			s.Reading = n
		case "read":
			s.Read = n
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return s, err
	}
	rows.Close()
	var avg sql.NullFloat64
	_ = p.db.QueryRowContext(ctx, `SELECT AVG(rating) FROM reading_log WHERE rating>0`).Scan(&avg)
	if avg.Valid {
		s.AvgRating = avg.Float64
	}
	_ = p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reflections`).Scan(&s.Reflections)
	return s, nil
}
