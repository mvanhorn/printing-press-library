// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

// Package store is the on-disk SQLite mirror of the user's Peloton history.
//
// The schema is small and relational on purpose: workouts → rides → songs,
// with a join table for ride playlists. We do NOT use the generator's
// generic "resources(id, type, data JSON)" bag — Peloton has three real
// entities, they have stable shapes, and FTS5 over title/instructor/artist
// is the only thing search ever needs to do. A typed schema produces
// honest indexes and lets `peloton search` ship one query, not three.
//
// Driver: modernc.org/sqlite (pure Go, no CGO) so `go install` works on
// stock toolchains. WAL mode + busy_timeout cover the single-writer / many-
// reader pattern this CLI uses; one CLI invocation, one writer.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/client"
)

// SchemaVersion is stamped into PRAGMA user_version on every Open. Bump when
// table shape changes; older binaries will refuse to open a newer DB rather
// than silently producing wrong results against a schema they can't read.
const SchemaVersion = 1

// Store is the handle. Cheap to hold for the duration of a CLI invocation;
// callers should Close() before exiting so WAL gets checkpointed.
type Store struct {
	db      *sql.DB
	writeMu sync.Mutex // serializes writes; reads run concurrently against WAL
	path    string
}

// DefaultPath returns ~/.local/share/peloton-pp-cli/peloton.db. Sticks with
// XDG conventions — the config TOML lives under ~/.config/, the database
// under ~/.local/share/, so a `chmod -R 0` of one doesn't break the other.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "peloton-pp-cli", "peloton.db"), nil
}

// Open creates or opens the SQLite store at dbPath. Empty path means
// DefaultPath(). The pragmas mirror what the rest of the catalog uses for
// modernc.org/sqlite v1.37.0 — WAL + 5s busy timeout is the well-trodden
// shape for "one CLI process, one writer, occasional read concurrency."
func Open(ctx context.Context, dbPath string) (*Store, error) {
	if dbPath == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		dbPath = p
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	db.SetMaxOpenConns(2)

	s := &Store{db: db, path: dbPath}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	return s, nil
}

// Close closes the underlying *sql.DB, which checkpoints WAL on the way out.
func (s *Store) Close() error { return s.db.Close() }

// Path returns the on-disk path of the database file.
func (s *Store) Path() string { return s.path }

// DB exposes the underlying handle for ad-hoc queries (mostly the MCP server
// reaches for this so it can shape its own JSON without re-wrapping every
// query). Callers must not Close it.
func (s *Store) DB() *sql.DB { return s.db }

// migrate runs the schema once per Open. The migrations are idempotent
// (CREATE TABLE IF NOT EXISTS) so re-running is free; user_version stamping
// is the version gate.
func (s *Store) migrate(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquiring migration connection: %w", err)
	}
	defer conn.Close()

	var current int
	if err := conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	if current > SchemaVersion {
		return fmt.Errorf("database schema_version %d is newer than this binary's %d — upgrade peloton-pp-cli or open an older db", current, SchemaVersion)
	}

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS workouts (
			id                 TEXT PRIMARY KEY,
			ride_id            TEXT,
			workout_date       TEXT NOT NULL,
			workout_time       TEXT NOT NULL,
			fitness_discipline TEXT,
			title              TEXT,
			instructor         TEXT,
			duration_seconds   INTEGER,
			total_output_kj    REAL,
			calories           INTEGER,
			avg_heart_rate     INTEGER,
			max_heart_rate     INTEGER,
			synced_at          DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workouts_date ON workouts(workout_date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_workouts_ride ON workouts(ride_id)`,
		`CREATE TABLE IF NOT EXISTS rides (
			id        TEXT PRIMARY KEY,
			title     TEXT,
			duration  INTEGER,
			fetched_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS songs (
			id        TEXT PRIMARY KEY,
			title     TEXT,
			album     TEXT,
			artists   TEXT, -- JSON array of artist names
			genres    TEXT  -- JSON array of genre names
		)`,
		`CREATE TABLE IF NOT EXISTS ride_songs (
			ride_id            TEXT NOT NULL,
			song_id            TEXT NOT NULL,
			idx                INTEGER NOT NULL,
			liked              INTEGER NOT NULL DEFAULT 0,
			start_time_offset  INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (ride_id, song_id, idx),
			FOREIGN KEY (ride_id) REFERENCES rides(id),
			FOREIGN KEY (song_id) REFERENCES songs(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ride_songs_song ON ride_songs(song_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ride_songs_liked ON ride_songs(liked) WHERE liked = 1`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS workouts_fts USING fts5(
			id UNINDEXED, title, instructor, content='workouts', content_rowid='rowid', tokenize='porter unicode61'
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS songs_fts USING fts5(
			id UNINDEXED, title, artists, album, content='songs', content_rowid='rowid', tokenize='porter unicode61'
		)`,
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("migration: %s\n%w", firstLine(m), err)
		}
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, SchemaVersion)); err != nil {
		return fmt.Errorf("stamping user_version: %w", err)
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// UpsertWorkouts batches a slice of client.Workouts in one transaction. Returns
// the number of NEW (previously-unseen) workout ids, since `peloton sync`
// reports that as "new". Existing rows are updated in place.
func (s *Store) UpsertWorkouts(ctx context.Context, ws []client.Workout) (newRows int, err error) {
	if len(ws) == 0 {
		return 0, nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // committed below; rollback on error

	check, err := tx.PrepareContext(ctx, `SELECT 1 FROM workouts WHERE id = ?`)
	if err != nil {
		return 0, fmt.Errorf("prepare check: %w", err)
	}
	defer check.Close()

	insertWorkout, err := tx.PrepareContext(ctx, `
		INSERT INTO workouts(id, ride_id, workout_date, workout_time, fitness_discipline, title, instructor,
		                    duration_seconds, total_output_kj, calories, avg_heart_rate, max_heart_rate, synced_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			ride_id            = excluded.ride_id,
			workout_date       = excluded.workout_date,
			workout_time       = excluded.workout_time,
			fitness_discipline = excluded.fitness_discipline,
			title              = excluded.title,
			instructor         = excluded.instructor,
			duration_seconds   = excluded.duration_seconds,
			total_output_kj    = excluded.total_output_kj,
			calories           = excluded.calories,
			avg_heart_rate     = excluded.avg_heart_rate,
			max_heart_rate     = excluded.max_heart_rate,
			synced_at          = excluded.synced_at
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare upsert: %w", err)
	}
	defer insertWorkout.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, w := range ws {
		var existed int
		if err := check.QueryRowContext(ctx, w.ID).Scan(&existed); err == sql.ErrNoRows {
			newRows++
		} else if err != nil {
			return 0, fmt.Errorf("check %s: %w", w.ID, err)
		}
		if _, err := insertWorkout.ExecContext(ctx,
			w.ID, nullIfEmpty(w.RideID), w.WorkoutDate, w.WorkoutTime, w.FitnessDiscipline,
			w.Title, w.Instructor, w.DurationSeconds, w.TotalOutputKJ, w.Calories,
			w.AvgHeartRate, w.MaxHeartRate, now,
		); err != nil {
			return 0, fmt.Errorf("upsert workout %s: %w", w.ID, err)
		}
	}
	if err := s.rebuildWorkoutsFTS(ctx, tx); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return newRows, nil
}

// UpsertRideDetails writes one ride + its playlist + its songs in a single tx.
// Idempotent: re-running on the same ride leaves song/ride rows unchanged
// except `fetched_at`. ride_songs is rebuilt for that ride to capture liked-
// flag changes (the user can like/unlike a song after the fact).
func (s *Store) UpsertRideDetails(ctx context.Context, rd client.RideDetails) error {
	if rd.RideID == "" {
		return fmt.Errorf("UpsertRideDetails: empty ride id")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO rides(id, title, duration, fetched_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title      = excluded.title,
			duration   = excluded.duration,
			fetched_at = excluded.fetched_at
	`, rd.RideID, rd.Title, rd.Duration, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("upsert ride: %w", err)
	}

	// Replace this ride's songs wholesale — the playlist for a given ride is
	// stable, but liked-flags can change between sync runs.
	if _, err := tx.ExecContext(ctx, `DELETE FROM ride_songs WHERE ride_id = ?`, rd.RideID); err != nil {
		return fmt.Errorf("clear ride_songs: %w", err)
	}

	upSong, err := tx.PrepareContext(ctx, `
		INSERT INTO songs(id, title, album, artists, genres)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title   = excluded.title,
			album   = excluded.album,
			artists = excluded.artists,
			genres  = excluded.genres
	`)
	if err != nil {
		return fmt.Errorf("prepare song upsert: %w", err)
	}
	defer upSong.Close()
	upJoin, err := tx.PrepareContext(ctx, `
		INSERT INTO ride_songs(ride_id, song_id, idx, liked, start_time_offset)
		VALUES(?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare ride_songs insert: %w", err)
	}
	defer upJoin.Close()

	for _, sg := range rd.Songs {
		artistsJSON, _ := json.Marshal(sg.Artists)
		genresJSON, _ := json.Marshal(sg.Genres)
		if _, err := upSong.ExecContext(ctx, sg.ID, sg.Title, sg.Album, string(artistsJSON), string(genresJSON)); err != nil {
			return fmt.Errorf("upsert song %s: %w", sg.ID, err)
		}
		liked := 0
		if sg.Liked {
			liked = 1
		}
		if _, err := upJoin.ExecContext(ctx, rd.RideID, sg.ID, sg.Index, liked, sg.StartTimeOffset); err != nil {
			return fmt.Errorf("insert ride_songs %s/%s: %w", rd.RideID, sg.ID, err)
		}
	}
	if err := s.rebuildSongsFTS(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// rebuildWorkoutsFTS / rebuildSongsFTS use FTS5's "rebuild" command. It's
// O(rows) but simple — for a personal Peloton history (low thousands of
// workouts at most), the cost is sub-second on the dev mac. Cleaner than
// hand-maintaining triggers + rowid bookkeeping for an external content
// table this small.
func (s *Store) rebuildWorkoutsFTS(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO workouts_fts(workouts_fts) VALUES('rebuild')`); err != nil {
		return fmt.Errorf("rebuild workouts_fts: %w", err)
	}
	return nil
}

func (s *Store) rebuildSongsFTS(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO songs_fts(songs_fts) VALUES('rebuild')`); err != nil {
		return fmt.Errorf("rebuild songs_fts: %w", err)
	}
	return nil
}

// KnownWorkoutIDs returns the set of workout ids in the store. Sync uses this
// to feed client.ListWorkouts' early-stop heuristic — the first page where
// every id is already known means the rest of the feed is older still and
// there's nothing new to fetch.
func (s *Store) KnownWorkoutIDs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM workouts`)
	if err != nil {
		return nil, fmt.Errorf("scan workouts.id: %w", err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

// RideIDsMissingDetails returns workout.ride_id values that have a workout
// row but no entry in rides. Sync uses this to backfill ride details
// incrementally — only workouts whose ride hasn't been hydrated yet.
func (s *Store) RideIDsMissingDetails(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT w.ride_id FROM workouts w
		LEFT JOIN rides r ON r.id = w.ride_id
		WHERE w.ride_id IS NOT NULL AND w.ride_id <> '' AND r.id IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("query missing ride ids: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Counts returns the row counts the `peloton sync` summary needs.
type Counts struct {
	Workouts  int `json:"workouts"`
	Rides     int `json:"rides"`
	Songs     int `json:"songs"`
	RideSongs int `json:"ride_songs"`
}

func (s *Store) Counts(ctx context.Context) (Counts, error) {
	var c Counts
	for _, q := range []struct {
		dst *int
		sql string
	}{
		{&c.Workouts, `SELECT COUNT(*) FROM workouts`},
		{&c.Rides, `SELECT COUNT(*) FROM rides`},
		{&c.Songs, `SELECT COUNT(*) FROM songs`},
		{&c.RideSongs, `SELECT COUNT(*) FROM ride_songs`},
	} {
		if err := s.db.QueryRowContext(ctx, q.sql).Scan(q.dst); err != nil {
			return c, fmt.Errorf("count: %w", err)
		}
	}
	return c, nil
}

// SetSyncedAt records the wall-clock of a successful sync so `peloton me`
// (and the MCP server) can surface "last synced" without a separate config.
func (s *Store) SetSyncedAt(ctx context.Context, key string, t time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO meta(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, t.UTC().Format(time.RFC3339))
	return err
}

// GetSyncedAt returns the timestamp set by SetSyncedAt, or zero if never set.
func (s *Store) GetSyncedAt(ctx context.Context, key string) (time.Time, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, v)
}

// SearchHit is the union shape returned by Search — kind tells the caller
// whether they're looking at a workout or a song.
type SearchHit struct {
	Kind    string  `json:"kind"` // "workout" | "song"
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Subtitle string `json:"subtitle,omitempty"` // instructor (workout) | artists/album (song)
	Date    string  `json:"date,omitempty"`     // workout_date for workouts
	Score   float64 `json:"score"`              // FTS5 bm25 score (lower is better)
}

// Search runs FTS5 against workouts (title + instructor) and songs (title +
// artists + album), interleaves the results by score, and returns up to
// limit hits. The query string is passed verbatim to FTS5 — callers can use
// quotes for phrases, NEAR(), prefix*, etc.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 20
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	wRows, err := s.db.QueryContext(ctx, `
		SELECT w.id, w.title, w.instructor, w.workout_date, bm25(workouts_fts)
		FROM workouts_fts JOIN workouts w ON w.rowid = workouts_fts.rowid
		WHERE workouts_fts MATCH ?
		ORDER BY bm25(workouts_fts)
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("workouts FTS: %w", err)
	}
	defer wRows.Close()
	var hits []SearchHit
	for wRows.Next() {
		var h SearchHit
		if err := wRows.Scan(&h.ID, &h.Title, &h.Subtitle, &h.Date, &h.Score); err != nil {
			return nil, fmt.Errorf("scan workout hit: %w", err)
		}
		h.Kind = "workout"
		hits = append(hits, h)
	}
	if err := wRows.Err(); err != nil {
		return nil, err
	}

	sRows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.title, s.artists, s.album, bm25(songs_fts)
		FROM songs_fts JOIN songs s ON s.rowid = songs_fts.rowid
		WHERE songs_fts MATCH ?
		ORDER BY bm25(songs_fts)
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("songs FTS: %w", err)
	}
	defer sRows.Close()
	for sRows.Next() {
		var (
			id, title, artistsJSON, album string
			score                         float64
		)
		if err := sRows.Scan(&id, &title, &artistsJSON, &album, &score); err != nil {
			return nil, fmt.Errorf("scan song hit: %w", err)
		}
		var artists []string
		_ = json.Unmarshal([]byte(artistsJSON), &artists)
		sub := strings.Join(artists, ", ")
		if album != "" {
			sub = sub + " — " + album
		}
		hits = append(hits, SearchHit{
			Kind: "song", ID: id, Title: title, Subtitle: sub, Score: score,
		})
	}
	if err := sRows.Err(); err != nil {
		return nil, err
	}

	// Cross-table interleave by bm25 score. FTS5 scores are negative; lower
	// is more relevant. Stable sort preserves intra-kind order.
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].Score < hits[j-1].Score; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
