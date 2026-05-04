package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// WatchlistEntry is one row in the movie-goat watchlist. ID is the SQLite
// rowid; AddedAt is stored as RFC3339 text so the column survives sqlite's
// loose datetime affinity.
type WatchlistEntry struct {
	ID      int64
	TMDBID  int
	Kind    string // "movie" or "tv"
	Title   string
	AddedAt time.Time
}

// WatchlistAdd inserts a new watchlist entry. UNIQUE(kind, tmdb_id) ensures
// duplicate adds surface as a structured error rather than silently shadowing
// an earlier row.
func (s *Store) WatchlistAdd(ctx context.Context, e WatchlistEntry) error {
	if e.Kind != "movie" && e.Kind != "tv" {
		return fmt.Errorf("watchlist: kind must be \"movie\" or \"tv\", got %q", e.Kind)
	}
	if e.TMDBID <= 0 {
		return fmt.Errorf("watchlist: tmdb_id must be positive, got %d", e.TMDBID)
	}
	added := e.AddedAt
	if added.IsZero() {
		added = time.Now().UTC()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO movie_goat_watchlist (tmdb_id, kind, title, added_at) VALUES (?, ?, ?, ?)`,
		e.TMDBID, e.Kind, e.Title, added.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("watchlist add: %w", err)
	}
	return nil
}

// WatchlistRemove deletes the row matching (kind, tmdb_id). Returns nil even
// when no row matches — callers can check WatchlistContains beforehand if they
// need to distinguish missing vs. removed.
func (s *Store) WatchlistRemove(ctx context.Context, kind string, tmdbID int) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM movie_goat_watchlist WHERE kind = ? AND tmdb_id = ?`,
		kind, tmdbID)
	if err != nil {
		return fmt.Errorf("watchlist remove: %w", err)
	}
	return nil
}

// WatchlistList returns every entry sorted by added_at ascending so the
// human-readable list is stable across runs.
func (s *Store) WatchlistList(ctx context.Context) ([]WatchlistEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tmdb_id, kind, title, added_at FROM movie_goat_watchlist ORDER BY added_at ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("watchlist list: %w", err)
	}
	defer rows.Close()

	var out []WatchlistEntry
	for rows.Next() {
		var e WatchlistEntry
		var addedAt string
		if err := rows.Scan(&e.ID, &e.TMDBID, &e.Kind, &e.Title, &addedAt); err != nil {
			return nil, fmt.Errorf("watchlist scan: %w", err)
		}
		if t, perr := time.Parse(time.RFC3339, addedAt); perr == nil {
			e.AddedAt = t
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// WatchlistContains reports whether (kind, tmdb_id) is present.
func (s *Store) WatchlistContains(ctx context.Context, kind string, tmdbID int) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM movie_goat_watchlist WHERE kind = ? AND tmdb_id = ? LIMIT 1`,
		kind, tmdbID).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("watchlist contains: %w", err)
	}
	return true, nil
}
