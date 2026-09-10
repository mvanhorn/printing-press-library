// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared live-fetch and local-store helpers for the hand-written MyAnimeList commands.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/malhtml"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/store"
)

// malURL builds a site path. MyAnimeList ignores the slug segment but requires
// a third path segment for sub-routes, so sub-resource paths use a literal "_"
// placeholder: /anime/1/stats serves the base detail page, /anime/1/_/stats
// serves the statistics page.
func malURL(kind string, id int, sub string) string {
	if sub == "" {
		return fmt.Sprintf("/%s/%d", kind, id)
	}
	return fmt.Sprintf("/%s/%d/_/%s", kind, id, sub)
}

// malGet fetches one path through the generated client (which applies the
// descriptive User-Agent, the configured timeout, the response cache, and the
// per-source rate limiter).
//
// MyAnimeList answers JSON endpoints with JSON and everything else with
// server-rendered HTML, so the generated client needs to be told which shape to
// expect; without the opt-in header it rejects a page with "expected JSON, API
// returned HTML instead of JSON".
func malGet(ctx context.Context, flags *rootFlags, path string, params map[string]string, html bool) (string, error) {
	c, err := flags.newClient()
	if err != nil {
		return "", err
	}
	var data json.RawMessage
	if html {
		data, err = c.GetWithHeaders(ctx, path, params, map[string]string{"X-Printing-Press-HTML-Response": "true"})
	} else {
		data, err = c.Get(ctx, path, params)
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// malDetail fetches and parses a title detail page.
func malDetail(ctx context.Context, flags *rootFlags, kind string, id int) (*malhtml.Detail, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return malDetailWith(ctx, c, kind, id)
}

// malDetailWith is malDetail for callers that already hold a client.
func malDetailWith(ctx context.Context, c *client.Client, kind string, id int) (*malhtml.Detail, error) {
	data, err := c.GetWithHeaders(ctx, malURL(kind, id, ""), nil, map[string]string{"X-Printing-Press-HTML-Response": "true"})
	if err != nil {
		return nil, fmt.Errorf("fetching %s %d: %w", kind, id, err)
	}
	return malhtml.ParseDetail(kind, id, string(data))
}

// malStats fetches and parses a title statistics page.
func malStats(ctx context.Context, flags *rootFlags, kind string, id int) (*malhtml.Stats, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return malStatsWith(ctx, c, kind, id)
}

// malStatsWith is malStats for callers that already hold a client.
func malStatsWith(ctx context.Context, c *client.Client, kind string, id int) (*malhtml.Stats, error) {
	data, err := c.GetWithHeaders(ctx, malURL(kind, id, "stats"), nil, map[string]string{"X-Printing-Press-HTML-Response": "true"})
	if err != nil {
		return nil, fmt.Errorf("fetching %s %d stats: %w", kind, id, err)
	}
	return malhtml.ParseStats(kind, id, string(data))
}

// malEpisodes fetches and parses a title's episode table.
func malEpisodes(ctx context.Context, flags *rootFlags, id int) ([]malhtml.Episode, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return malEpisodesWith(ctx, c, id)
}

// malEpisodesWith is malEpisodes for callers that already hold a client.
func malEpisodesWith(ctx context.Context, c *client.Client, id int) ([]malhtml.Episode, error) {
	data, err := c.GetWithHeaders(ctx, malURL("anime", id, "episode"), nil, map[string]string{"X-Printing-Press-HTML-Response": "true"})
	if err != nil {
		return nil, fmt.Errorf("fetching anime %d episodes: %w", id, err)
	}
	return malhtml.ParseEpisodes(id, string(data))
}

// malIntArg parses a positive integer positional argument.
func malIntArg(arg, name string) (int, error) {
	n := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(arg), "%d", &n); err != nil || n <= 0 {
		return 0, usageErr(fmt.Errorf("%s must be a positive MyAnimeList id, got %q", name, arg))
	}
	return n, nil
}

// --- local store -----------------------------------------------------------

const malSnapshotDDL = `
CREATE TABLE IF NOT EXISTS mal_snapshots (
  kind TEXT NOT NULL,
  id INTEGER NOT NULL,
  captured_at TEXT NOT NULL,
  title TEXT,
  score REAL,
  members INTEGER,
  favorites INTEGER,
  rank INTEGER,
  popularity INTEGER,
  episodes INTEGER,
  PRIMARY KEY (kind, id, captured_at)
)`

const malLibraryDDL = `
CREATE TABLE IF NOT EXISTS mal_library (
  kind TEXT NOT NULL,
  id INTEGER NOT NULL,
  title TEXT,
  status TEXT NOT NULL,
  progress INTEGER NOT NULL DEFAULT 0,
  total INTEGER NOT NULL DEFAULT 0,
  score INTEGER NOT NULL DEFAULT 0,
  notes TEXT,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (kind, id)
)`

// malOpenStore opens the local SQLite mirror and ensures this CLI's own tables
// exist. The tables are namespaced with a mal_ prefix so they cannot collide
// with generator-owned tables.
func malOpenStore(ctx context.Context, dbPath string) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	for _, ddl := range []string{malSnapshotDDL, malLibraryDDL} {
		if _, err := db.DB().ExecContext(ctx, ddl); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("creating local tables: %w", err)
		}
	}
	return db, nil
}

// malDBPath resolves the database path for hand-written commands.
func malDBPath(flags *rootFlags, override string) string {
	if override != "" {
		return override
	}
	return defaultDBPath("myanimelist-pp-cli")
}

// malRecordSnapshot stores one row per capture time. Snapshots are the only way
// to answer "did the score move?", because MyAnimeList publishes no history.
func malRecordSnapshot(ctx context.Context, db *store.Store, d *malhtml.Detail) error {
	if d == nil || d.ID == 0 {
		return fmt.Errorf("nothing to record")
	}
	_, err := db.DB().ExecContext(ctx, `
		INSERT OR REPLACE INTO mal_snapshots
		  (kind, id, captured_at, title, score, members, favorites, rank, popularity, episodes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Kind, d.ID, time.Now().UTC().Format(time.RFC3339), d.Title,
		d.Score, d.Members, d.Favorites, d.Rank, d.Popularity, d.Episodes)
	if err != nil {
		return fmt.Errorf("recording snapshot: %w", err)
	}
	return nil
}

// malLibraryEntry is one row of the local (credential-free) watch library.
type malLibraryEntry struct {
	Kind      string `json:"kind"`
	ID        int    `json:"id"`
	Title     string `json:"title,omitempty"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Total     int    `json:"total,omitempty"`
	Score     int    `json:"score,omitempty"`
	Notes     string `json:"notes,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

var malStatuses = []string{"watching", "completed", "on-hold", "dropped", "plan-to-watch"}

func malValidStatus(s string) bool {
	for _, v := range malStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// malLoadLibrary drains the library table into a slice before any follow-up
// query: SQLite uses a single connection, so no query may run while a *sql.Rows
// is open.
func malLoadLibrary(ctx context.Context, db *store.Store, kind string) ([]malLibraryEntry, error) {
	query := `SELECT kind, id, COALESCE(title,''), status, progress, total, score, COALESCE(notes,''), updated_at FROM mal_library`
	args := []any{}
	if kind != "" {
		query += ` WHERE kind = ?`
		args = append(args, kind)
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("reading local library: %w", err)
	}
	out := make([]malLibraryEntry, 0, 16)
	for rows.Next() {
		var e malLibraryEntry
		if err := rows.Scan(&e.Kind, &e.ID, &e.Title, &e.Status, &e.Progress, &e.Total, &e.Score, &e.Notes, &e.UpdatedAt); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning library row: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("reading local library: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing library rows: %w", err)
	}
	return out, nil
}

// malLibraryIDs returns the set of "kind:id" keys already tracked locally.
func malLibraryIDs(ctx context.Context, db *store.Store) (map[string]bool, error) {
	entries, err := malLoadLibrary(ctx, db, "")
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(entries))
	for _, e := range entries {
		set[fmt.Sprintf("%s:%d", e.Kind, e.ID)] = true
	}
	return set, nil
}

// malWriteLibrary upserts one library row.
func malWriteLibrary(ctx context.Context, db *store.Store, e *malLibraryEntry) error {
	if e.UpdatedAt == "" {
		e.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := db.DB().ExecContext(ctx, `
		INSERT INTO mal_library (kind, id, title, status, progress, total, score, notes, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(kind, id) DO UPDATE SET
		  title = COALESCE(NULLIF(excluded.title,''), mal_library.title),
		  status = excluded.status,
		  progress = excluded.progress,
		  total = COALESCE(NULLIF(excluded.total,0), mal_library.total),
		  score = COALESCE(NULLIF(excluded.score,0), mal_library.score),
		  notes = COALESCE(NULLIF(excluded.notes,''), mal_library.notes),
		  updated_at = excluded.updated_at`,
		e.Kind, e.ID, e.Title, e.Status, e.Progress, e.Total, e.Score, e.Notes, e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("writing local library row: %w", err)
	}
	return nil
}

// malStoreExists reports whether the SQLite file exists yet.
func malStoreExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
