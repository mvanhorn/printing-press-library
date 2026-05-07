// Package goatstore is the multi-source persistence layer that sits on top
// of the generated SQLite store. The generated `places` table tracks
// Nominatim API responses; goatstore.GoatPlaces is the fused multi-source
// view every transcendence command queries.
//
// Design: this package opens its own *sql.DB against the same SQLite file
// the generated store uses, but writes to its own `goat_*` tables so the
// generator-emitted schema is not touched. CREATE TABLE IF NOT EXISTS
// runs idempotently on every Open.
package goatstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DefaultPath returns the SQLite path the goatstore shares with the
// generated `store` package. Both call OpenDB with the same dbPath.
func DefaultPath(cliName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", cliName+".db")
	}
	return filepath.Join(home, ".local", "share", cliName, "store.db")
}

// Store is the goatstore handle.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens the goatstore at dbPath, creating the directory and tables
// if needed.
func Open(ctx context.Context, dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating db dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	db.SetMaxOpenConns(2)
	s := &Store{db: db, path: dbPath}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// DB returns the underlying database for advanced queries.
func (s *Store) DB() *sql.DB { return s.db }

// Close releases the connection.
func (s *Store) Close() error { return s.db.Close() }

// Path returns the on-disk SQLite file path.
func (s *Store) Path() string { return s.path }

// migrate creates the goat_* tables if they don't exist.
func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS goat_places (
			id TEXT PRIMARY KEY,
			source TEXT NOT NULL,
			intent TEXT NOT NULL,
			name TEXT NOT NULL,
			name_local TEXT,
			lat REAL,
			lng REAL,
			address TEXT,
			country TEXT,
			region TEXT,
			city_slug TEXT,
			trust REAL DEFAULT 0,
			why_special TEXT,
			data JSON,
			discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_goat_places_source ON goat_places(source)`,
		`CREATE INDEX IF NOT EXISTS idx_goat_places_intent ON goat_places(intent)`,
		`CREATE INDEX IF NOT EXISTS idx_goat_places_city ON goat_places(city_slug)`,
		`CREATE INDEX IF NOT EXISTS idx_goat_places_country ON goat_places(country)`,
		`CREATE INDEX IF NOT EXISTS idx_goat_places_latlng ON goat_places(lat, lng)`,

		`CREATE TABLE IF NOT EXISTS goat_cities (
			slug TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			country TEXT NOT NULL,
			lat REAL,
			lng REAL,
			last_synced_at DATETIME,
			data JSON
		)`,

		`CREATE TABLE IF NOT EXISTS goat_reddit_threads (
			id TEXT PRIMARY KEY,
			subreddit TEXT NOT NULL,
			title TEXT,
			url TEXT,
			permalink TEXT,
			score INTEGER DEFAULT 0,
			num_comments INTEGER DEFAULT 0,
			body TEXT,
			place_mentions TEXT,
			city_slug TEXT,
			discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_goat_reddit_subreddit ON goat_reddit_threads(subreddit)`,
		`CREATE INDEX IF NOT EXISTS idx_goat_reddit_city ON goat_reddit_threads(city_slug)`,

		`CREATE TABLE IF NOT EXISTS goat_routes_cache (
			id TEXT PRIMARY KEY,
			from_lat REAL,
			from_lng REAL,
			to_lat REAL,
			to_lng REAL,
			distance_m REAL,
			duration_s REAL,
			polyline TEXT,
			cached_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS goat_sync_state (
			source TEXT,
			city_slug TEXT,
			last_cursor TEXT,
			last_synced_at DATETIME,
			row_count INTEGER DEFAULT 0,
			PRIMARY KEY (source, city_slug)
		)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS goat_places_fts USING fts5(
			id UNINDEXED, name, name_local, why_special, address,
			tokenize='porter unicode61'
		)`,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %q: %w", firstLine(stmt), err)
		}
	}
	return tx.Commit()
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i > 0 {
		return s[:i]
	}
	return s
}

// Place is a row in goat_places. Nullable fields use sql.NullString.
type Place struct {
	ID         string
	Source     string
	Intent     string
	Name       string
	NameLocal  string
	Lat        float64
	Lng        float64
	Address    string
	Country    string
	Region     string
	CitySlug   string
	Trust      float64
	WhySpecial string
	Data       map[string]any
	Updated    time.Time
}

// UpsertPlace inserts or updates a goat_places row keyed on ID.
func (s *Store) UpsertPlace(ctx context.Context, p Place) error {
	dataJSON := []byte("{}")
	if p.Data != nil {
		var err error
		dataJSON, err = json.Marshal(p.Data)
		if err != nil {
			return fmt.Errorf("marshal data: %w", err)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO goat_places
		(id, source, intent, name, name_local, lat, lng, address, country, region, city_slug, trust, why_special, data, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			name_local = excluded.name_local,
			lat = excluded.lat,
			lng = excluded.lng,
			address = excluded.address,
			country = excluded.country,
			region = excluded.region,
			city_slug = excluded.city_slug,
			trust = excluded.trust,
			why_special = excluded.why_special,
			data = excluded.data,
			updated_at = CURRENT_TIMESTAMP`,
		p.ID, p.Source, p.Intent, p.Name, p.NameLocal, p.Lat, p.Lng, p.Address, p.Country, p.Region, p.CitySlug, p.Trust, p.WhySpecial, string(dataJSON),
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("upsert place: %w", err)
	}
	// Mirror into FTS.
	_, err = tx.ExecContext(ctx, `INSERT INTO goat_places_fts (id, name, name_local, why_special, address)
		VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.NameLocal, p.WhySpecial, p.Address)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("fts insert: %w", err)
	}
	return tx.Commit()
}

// QueryRadius returns places near (lat,lng) within rough meters using a
// bounding-box pre-filter. Caller should refine with proper haversine
// downstream when ranking.
func (s *Store) QueryRadius(ctx context.Context, lat, lng, meters float64, intent string) ([]Place, error) {
	deg := meters / 111_000.0
	q := `SELECT id, source, intent, name, name_local, lat, lng, address, country, region, city_slug, trust, why_special
		FROM goat_places
		WHERE lat BETWEEN ? AND ? AND lng BETWEEN ? AND ?`
	args := []any{lat - deg, lat + deg, lng - deg, lng + deg}
	if intent != "" {
		q += ` AND intent = ?`
		args = append(args, intent)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Place
	for rows.Next() {
		var p Place
		if err := rows.Scan(&p.ID, &p.Source, &p.Intent, &p.Name, &p.NameLocal, &p.Lat, &p.Lng, &p.Address, &p.Country, &p.Region, &p.CitySlug, &p.Trust, &p.WhySpecial); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SaveSync records a (source, city_slug) sync run.
func (s *Store) SaveSync(ctx context.Context, source, citySlug, cursor string, rowCount int) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO goat_sync_state (source, city_slug, last_cursor, last_synced_at, row_count)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(source, city_slug) DO UPDATE SET
			last_cursor = excluded.last_cursor,
			last_synced_at = excluded.last_synced_at,
			row_count = excluded.row_count`,
		source, citySlug, cursor, rowCount)
	return err
}

// CoverageRow captures the per-source coverage for `coverage` command.
type CoverageRow struct {
	Source       string
	RowCount     int
	LastSyncedAt sql.NullString
}

// Coverage returns per-source row counts + last-sync ages for a city.
func (s *Store) Coverage(ctx context.Context, citySlug string) ([]CoverageRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.source, COALESCE(s.row_count, 0), s.last_synced_at
		FROM goat_sync_state s
		WHERE s.city_slug = ? OR s.city_slug = ''
		ORDER BY s.source`, citySlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CoverageRow
	for rows.Next() {
		var r CoverageRow
		if err := rows.Scan(&r.Source, &r.RowCount, &r.LastSyncedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RedditThread is a row in goat_reddit_threads.
type RedditThread struct {
	ID            string
	Subreddit     string
	Title         string
	URL           string
	Permalink     string
	Score         int
	NumComments   int
	Body          string
	PlaceMentions string
	CitySlug      string
}

// UpsertRedditThread stores a thread keyed on ID.
func (s *Store) UpsertRedditThread(ctx context.Context, t RedditThread) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO goat_reddit_threads
		(id, subreddit, title, url, permalink, score, num_comments, body, place_mentions, city_slug)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			score = excluded.score,
			num_comments = excluded.num_comments,
			body = excluded.body,
			place_mentions = excluded.place_mentions`,
		t.ID, t.Subreddit, t.Title, t.URL, t.Permalink, t.Score, t.NumComments, t.Body, t.PlaceMentions, t.CitySlug)
	return err
}

// QuotesForPlace finds reddit threads that mention any of the names.
func (s *Store) QuotesForPlace(ctx context.Context, names []string) ([]RedditThread, error) {
	if len(names) == 0 {
		return nil, nil
	}
	clauses := make([]string, 0, len(names))
	args := make([]any, 0, len(names)*2)
	for _, n := range names {
		clauses = append(clauses, "(body LIKE ? OR title LIKE ?)")
		args = append(args, "%"+n+"%", "%"+n+"%")
	}
	q := `SELECT id, subreddit, title, url, permalink, score, num_comments, body, place_mentions, city_slug
		FROM goat_reddit_threads
		WHERE ` + strings.Join(clauses, " OR ") + `
		ORDER BY score DESC LIMIT 20`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RedditThread
	for rows.Next() {
		var t RedditThread
		if err := rows.Scan(&t.ID, &t.Subreddit, &t.Title, &t.URL, &t.Permalink, &t.Score, &t.NumComments, &t.Body, &t.PlaceMentions, &t.CitySlug); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CacheRoute stores an OSRM walking route between two points.
func (s *Store) CacheRoute(ctx context.Context, fromLat, fromLng, toLat, toLng, distance, duration float64, polyline string) error {
	id := fmt.Sprintf("%.5f,%.5f→%.5f,%.5f", fromLat, fromLng, toLat, toLng)
	_, err := s.db.ExecContext(ctx, `INSERT INTO goat_routes_cache
		(id, from_lat, from_lng, to_lat, to_lng, distance_m, duration_s, polyline, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			distance_m = excluded.distance_m,
			duration_s = excluded.duration_s,
			polyline = excluded.polyline,
			cached_at = CURRENT_TIMESTAMP`,
		id, fromLat, fromLng, toLat, toLng, distance, duration, polyline)
	return err
}

// LookupRoute returns a cached route distance/duration if present.
func (s *Store) LookupRoute(ctx context.Context, fromLat, fromLng, toLat, toLng float64) (distance, duration float64, polyline string, found bool, err error) {
	id := fmt.Sprintf("%.5f,%.5f→%.5f,%.5f", fromLat, fromLng, toLat, toLng)
	row := s.db.QueryRowContext(ctx, `SELECT distance_m, duration_s, polyline FROM goat_routes_cache WHERE id = ?`, id)
	err = row.Scan(&distance, &duration, &polyline)
	if err == sql.ErrNoRows {
		return 0, 0, "", false, nil
	}
	if err != nil {
		return 0, 0, "", false, err
	}
	return distance, duration, polyline, true, nil
}
