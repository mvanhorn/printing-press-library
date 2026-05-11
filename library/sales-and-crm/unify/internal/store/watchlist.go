package store

import (
	"context"
	"time"
)

// WatchEntry is one row of the watchlist: a key/value the user wants
// refreshed on every sync. Records have no LIST endpoint, so the watchlist
// is the explicit-ID cursor sync needs.
type WatchEntry struct {
	ObjectName string `json:"object_name"`
	MatchKey   string `json:"match_key"`
	MatchValue string `json:"match_value"`
	AddedAt    int64  `json:"added_at"`
}

// AddWatch inserts a watchlist entry (idempotent — re-adding refreshes
// added_at).
func (s *Store) AddWatch(ctx context.Context, e WatchEntry) error {
	if e.AddedAt == 0 {
		e.AddedAt = time.Now().Unix()
	}
	_, err := s.DB.ExecContext(ctx,
		`INSERT OR REPLACE INTO watchlist (object_name, match_key, match_value, added_at) VALUES (?, ?, ?, ?)`,
		e.ObjectName, e.MatchKey, e.MatchValue, e.AddedAt)
	return err
}

// RemoveWatch deletes a watchlist entry by composite key. Returns the row
// count that was removed (0 if nothing matched).
func (s *Store) RemoveWatch(ctx context.Context, objectName, matchKey, matchValue string) (int64, error) {
	res, err := s.DB.ExecContext(ctx,
		`DELETE FROM watchlist WHERE object_name = ? AND match_key = ? AND match_value = ?`,
		objectName, matchKey, matchValue)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ListWatch returns every watchlist entry, optionally filtered by object
// name (pass "" for all).
func (s *Store) ListWatch(ctx context.Context, objectName string) ([]WatchEntry, error) {
	q := `SELECT object_name, match_key, match_value, added_at FROM watchlist`
	args := []any{}
	if objectName != "" {
		q += ` WHERE object_name = ?`
		args = append(args, objectName)
	}
	q += ` ORDER BY object_name, match_key, match_value`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchEntry
	for rows.Next() {
		var e WatchEntry
		if err := rows.Scan(&e.ObjectName, &e.MatchKey, &e.MatchValue, &e.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
