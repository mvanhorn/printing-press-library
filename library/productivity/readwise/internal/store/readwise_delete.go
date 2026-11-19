package store

import (
	"context"
	"fmt"
	"time"
)

// DeleteReadwiseHighlight removes an export tombstone from both mirrors. The
// typed-table DELETE trigger updates highlights_fts in the same transaction.
func (s *Store) DeleteReadwiseHighlight(ctx context.Context, id string) (int, error) {
	if id == "" {
		return 0, fmt.Errorf("deleted highlight missing id")
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM highlights WHERE id = ?`, id)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid = ?`, ftsRowID("highlights", id)); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM resources WHERE resource_type = 'highlights' AND id = ?`, id); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return int(n), nil
}

// ResetReadwiseSyncState deliberately clears the watermark before a full walk.
func (s *Store) ResetReadwiseSyncState(resource string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.Exec(`INSERT INTO sync_state(resource_type,last_cursor,last_synced_at,last_attempt_complete) VALUES (?,'',NULL,0) ON CONFLICT(resource_type) DO UPDATE SET last_cursor='',last_synced_at=NULL,last_attempt_complete=0,scope_key=''`, resource)
	return err
}

// SaveReadwiseSyncProgress preserves the run filter alongside its cursor.
func (s *Store) SaveReadwiseSyncProgress(resource, cursor string, count int, since string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.Exec(`INSERT INTO sync_state(resource_type,last_cursor,total_count,scope_key,last_attempt_complete) VALUES (?,?,?,?,0) ON CONFLICT(resource_type) DO UPDATE SET last_cursor=excluded.last_cursor,total_count=excluded.total_count,scope_key=excluded.scope_key,last_attempt_complete=0`, resource, cursor, count, since)
	return err
}
func (s *Store) ReadwiseSyncSince(resource string) (string, error) {
	var since string
	err := s.db.QueryRow(`SELECT scope_key FROM sync_state WHERE resource_type=?`, resource).Scan(&since)
	return since, err
}

// PruneReadwiseDocuments reconciles a complete unfiltered Reader snapshot.
// Descendants of removed documents are removed in the same transaction.
func (s *Store) PruneReadwiseDocuments(ctx context.Context, seen map[string]bool, pruneChildren bool) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,data FROM resources WHERE resource_type='documents'`)
	if err != nil {
		return err
	}
	parents := map[string]string{}
	removed := map[string]bool{}
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			_ = rows.Close()
			return err
		}
		obj, err := DecodeJSONObject([]byte(data))
		if err != nil {
			_ = rows.Close()
			return err
		}
		parent := ResourceIDString(obj["parent_id"])
		parents[id] = parent
		if (parent == "" || pruneChildren) && !seen[id] {
			removed[id] = true
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for changed := true; changed; {
		changed = false
		for id, parent := range parents {
			if removed[parent] && !removed[id] {
				removed[id] = true
				changed = true
			}
		}
	}
	for id := range removed {
		if _, err = tx.ExecContext(ctx, `DELETE FROM documents WHERE id=?`, id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid=?`, ftsRowID("documents", id)); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM resources WHERE resource_type='documents' AND id=?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteReadwiseTombstones applies a page's book and highlight erasures atomically.
func (s *Store) DeleteReadwiseTombstones(ctx context.Context, books, highlights []string) (int, error) {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	total := 0
	for _, bookID := range books {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM resources WHERE resource_type='highlights' AND CAST(json_extract(data,'$.book_id') AS TEXT)=?`, bookID)
		if err != nil {
			return 0, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return 0, err
			}
			highlights = append(highlights, id)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return 0, err
		}
	}
	// Constant per-table statements: table names are never assembled from strings.
	deleteByID := map[string]string{
		"books":      `DELETE FROM books WHERE id=?`,
		"highlights": `DELETE FROM highlights WHERE id=?`,
	}
	for resource, ids := range map[string][]string{"books": books, "highlights": highlights} {
		for _, id := range ids {
			if id == "" {
				return 0, fmt.Errorf("deleted %s missing id", resource)
			}
			res, err := tx.ExecContext(ctx, deleteByID[resource], id)
			if err != nil {
				return 0, err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return 0, err
			}
			total += int(n)
			if _, err = tx.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid=?`, ftsRowID(resource, id)); err != nil {
				return 0, err
			}
			if _, err = tx.ExecContext(ctx, `DELETE FROM resources WHERE resource_type=? AND id=?`, resource, id); err != nil {
				return 0, err
			}
		}
	}
	return total, tx.Commit()
}
func (s *Store) DeleteReadwiseBook(ctx context.Context, id string) (int, error) {
	return s.DeleteReadwiseTombstones(ctx, []string{id}, nil)
}

// ReadwiseChainStart retains the first leg's start across capped resumes.
func (s *Store) ReadwiseChainStart(resource string, resume bool, now time.Time) (time.Time, error) {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if resume {
		var saved string
		if err := s.db.QueryRow(`SELECT readwise_chain_started FROM sync_state WHERE resource_type=?`, resource).Scan(&saved); err != nil {
			return time.Time{}, err
		}
		if at, err := time.Parse(time.RFC3339Nano, saved); err == nil {
			return at, nil
		}
	}
	_, err := s.db.Exec(`INSERT INTO sync_state(resource_type,readwise_chain_started) VALUES (?,?) ON CONFLICT(resource_type) DO UPDATE SET readwise_chain_started=excluded.readwise_chain_started`, resource, now.UTC().Format(time.RFC3339Nano))
	return now, err
}
