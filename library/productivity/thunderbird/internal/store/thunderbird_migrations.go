package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const tbMboxStateSQL = `CREATE TABLE IF NOT EXISTS tb_mbox_state (
	mbox_path TEXT PRIMARY KEY,
	folder_key TEXT NOT NULL,
	size INTEGER NOT NULL,
	mtime INTEGER NOT NULL,
	last_offset INTEGER NOT NULL,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`

// MboxState is the incremental-ingest checkpoint of one mbox file.
type MboxState struct {
	Path       string
	FolderKey  string
	Size       int64
	MTime      int64
	LastOffset int64
}

// EnsureThunderbirdTables creates the Thunderbird sync-state tables.
func (s *Store) EnsureThunderbirdTables(ctx context.Context) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if _, err := s.db.ExecContext(ctx, tbMboxStateSQL); err != nil {
		return fmt.Errorf("creating tb_mbox_state: %w", err)
	}
	return nil
}

// GetMboxState returns the checkpoint of path; ok is false when absent.
func (s *Store) GetMboxState(path string) (MboxState, bool, error) {
	st := MboxState{Path: path}
	err := s.db.QueryRow(`SELECT folder_key, size, mtime, last_offset FROM tb_mbox_state WHERE mbox_path = ?`, path).
		Scan(&st.FolderKey, &st.Size, &st.MTime, &st.LastOffset)
	if errors.Is(err, sql.ErrNoRows) {
		return st, false, nil
	}
	if err != nil {
		return st, false, err
	}
	return st, true, nil
}

// SaveMboxState upserts a checkpoint.
func (s *Store) SaveMboxState(st MboxState) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.Exec(`INSERT INTO tb_mbox_state (mbox_path, folder_key, size, mtime, last_offset, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(mbox_path) DO UPDATE SET folder_key = excluded.folder_key, size = excluded.size,
		mtime = excluded.mtime, last_offset = excluded.last_offset, updated_at = excluded.updated_at`,
		st.Path, st.FolderKey, st.Size, st.MTime, st.LastOffset)
	return err
}

// ListMboxStates returns every stored checkpoint.
func (s *Store) ListMboxStates() ([]MboxState, error) {
	rows, err := s.db.Query(`SELECT mbox_path, folder_key, size, mtime, last_offset FROM tb_mbox_state ORDER BY mbox_path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MboxState, 0)
	for rows.Next() {
		var st MboxState
		if err := rows.Scan(&st.Path, &st.FolderKey, &st.Size, &st.MTime, &st.LastOffset); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// DeleteMboxState removes the checkpoint of path.
func (s *Store) DeleteMboxState(path string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.Exec(`DELETE FROM tb_mbox_state WHERE mbox_path = ?`, path)
	return err
}

// PruneResourcesNotIn deletes rows of resourceType whose id is not in keep.
func (s *Store) PruneResourcesNotIn(resourceType string, keep map[string]bool) (int, error) {
	ids, err := s.ListIDs(resourceType)
	if err != nil {
		return 0, err
	}
	var doomed []string
	for _, id := range ids {
		if !keep[id] {
			doomed = append(doomed, id)
		}
	}
	return s.DeleteResourceIDs(resourceType, doomed)
}

// DeleteResourceIDs deletes the listed rows of resourceType, including their FTS rows.
func (s *Store) DeleteResourceIDs(resourceType string, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM resources WHERE resource_type = ? AND id = ?`, resourceType, id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM resources_fts WHERE rowid = ?`, ftsRowID(resourceType, id)); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}
