// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// ScopedVectorRow identifies the exact local mirror version that was checked
// against the live API before a prune operation.
type ScopedVectorRow struct {
	StorageID    string
	BareID       string
	ExpectedData string
}

// DeleteScopedVectorRows removes only confirmed-absent rows from one index and
// namespace. A concurrently changed local row aborts the whole transaction.
func (s *Store) DeleteScopedVectorRows(ctx context.Context, indexName, namespace string, rows []ScopedVectorRow) (int, error) {
	if indexName == "" {
		return 0, fmt.Errorf("index name is required for scoped vector cleanup")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	deleted := 0
	for _, row := range rows {
		if row.StorageID == "" || row.BareID == "" || row.ExpectedData == "" {
			return 0, fmt.Errorf("incomplete scoped vector cleanup identity")
		}
		var current string
		err := tx.QueryRowContext(ctx, `SELECT data FROM resources WHERE resource_type = 'vectors' AND id = ?`, row.StorageID).Scan(&current)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return 0, err
		}
		if current != row.ExpectedData {
			return 0, fmt.Errorf("local vector %q changed during prune; run prune again", row.BareID)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(current), &obj); err != nil {
			return 0, fmt.Errorf("local vector %q has malformed provenance: %w", row.BareID, err)
		}
		storedID, _ := obj["id"].(string)
		storedIndex, _ := obj["index_name"].(string)
		if storedIndex == "" {
			storedIndex, _ = obj["indexName"].(string)
		}
		if storedIndex == "" {
			storedIndex, _ = obj["index"].(string)
		}
		storedNamespace, hasNamespace := obj["namespace"].(string)
		if storedID != row.BareID || storedIndex != indexName || !hasNamespace || storedNamespace != namespace {
			return 0, fmt.Errorf("local vector %q no longer matches the requested index and namespace", row.BareID)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM vectors WHERE id = ?`, row.StorageID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid = ?`, ftsRowID("vectors", row.StorageID)); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM resources WHERE resource_type = 'vectors' AND id = ?`, row.StorageID); err != nil {
			return 0, err
		}
		deleted++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return deleted, nil
}
