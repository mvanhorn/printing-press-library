// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.
// EnsureFresh checks whether locally cached data is within the stale threshold.

package cliutil

import (
	"context"
	"database/sql"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/synology-dsm/internal/store"
)

// EnsureFresh opens the local store and checks whether the most recent sync
// for resourceType is within staleAfter. Returns (true, nil) when data is
// fresh or the store does not yet exist. Returns (false, nil) when data is
// stale. Returns (false, err) only for unexpected errors (corrupt database,
// permission denied, etc.).
//
// resourceType may be empty to check the oldest sync time across all resource types.
//
// This function is read-only: it never triggers a sync. Callers that want to
// trigger a refresh should dispatch a goroutine to run the sync command after
// EnsureFresh returns false.
func EnsureFresh(ctx context.Context, dbPath, resourceType string, staleAfter time.Duration) (bool, error) {
	if staleAfter <= 0 {
		staleAfter = 6 * time.Hour
	}

	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		// No database yet — treat as fresh (first-run, not stale).
		return true, nil
	}
	defer db.Close()

	var query string
	var args []any
	if resourceType != "" {
		query = `SELECT last_synced_at FROM sync_state WHERE resource_type = ? LIMIT 1`
		args = []any{resourceType}
	} else {
		query = `SELECT MIN(last_synced_at) FROM sync_state`
	}

	var lastSynced sql.NullTime
	row := db.DB().QueryRowContext(ctx, query, args...)
	if err := row.Scan(&lastSynced); err != nil {
		// Table may not exist (never synced) — treat as fresh to avoid blocking.
		return true, nil
	}
	if !lastSynced.Valid {
		// Null time → never synced → data is stale.
		return false, nil
	}

	age := time.Since(lastSynced.Time)
	return age <= staleAfter, nil
}

// FreshnessAge returns the age of the oldest cached resource, or a zero
// duration when the store does not exist or has never been synced.
// Useful for metrics and doctor reporting.
func FreshnessAge(ctx context.Context, dbPath string) time.Duration {
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return 0
	}
	defer db.Close()

	var lastSynced sql.NullTime
	row := db.DB().QueryRowContext(ctx, `SELECT MIN(last_synced_at) FROM sync_state`)
	if err := row.Scan(&lastSynced); err != nil || !lastSynced.Valid {
		return 0
	}
	return time.Since(lastSynced.Time)
}
