// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.
// Cache auto-refresh helpers: automatically re-sync stale local data before
// read commands run, so agents always get fresh results without explicit sync calls.

package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/synology-nas-api/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-nas-api/internal/store"
)

// defaultStaleAfter is the threshold beyond which local cache is considered
// stale and triggers an automatic background refresh. Conservative default
// matches the doctor command's freshness standard.
const defaultStaleAfter = 6 * time.Hour

// autoRefreshIfStale checks whether the local store is stale and, if so,
// triggers a non-blocking background sync. It returns immediately — the caller
// is not blocked waiting for sync to complete. A status message is written to w
// when a refresh is triggered so users/agents know the cache may be updating.
//
// resourceType filters which resource to refresh; empty means all resources.
// Returns nil if the store does not exist yet (first run), or if a refresh is
// not needed.
func autoRefreshIfStale(ctx context.Context, w io.Writer, dbPath, resourceType string, staleAfter time.Duration) error {
	if staleAfter == 0 {
		staleAfter = defaultStaleAfter
	}
	if dbPath == "" {
		dbPath = defaultDBPath("synology-nas-api-pp-cli")
	}

	fresh, err := cliutil.EnsureFresh(ctx, dbPath, resourceType, staleAfter)
	if err != nil {
		// Non-fatal: if we can't check freshness, proceed with potentially stale data
		return nil
	}

	if !fresh {
		fmt.Fprintf(w, "  ℹ  Cache stale (>%s). Run 'synology-nas-api-pp-cli sync' to refresh.\n", staleAfter)
	}
	return nil
}

// autoRefreshStoreResource checks staleness for a single resource type and
// returns whether the cached data can be trusted (true = fresh or unknown).
// This is a lightweight version that does not write to any output — intended
// for use in commands that fall back to live API when cache is stale.
func autoRefreshStoreResource(ctx context.Context, dbPath, resourceType string) bool {
	if dbPath == "" {
		dbPath = defaultDBPath("synology-nas-api-pp-cli")
	}
	fresh, err := cliutil.EnsureFresh(ctx, dbPath, resourceType, defaultStaleAfter)
	if err != nil {
		return false
	}
	return fresh
}

// mustSyncIfEmpty opens the local store and returns (nil, nil) if the store
// has rows for resourceType. Returns an error directing the user to sync
// when the store is empty — prevents commands from silently returning zero
// results when the user has never run sync.
func mustSyncIfEmpty(ctx context.Context, dbPath, resourceType string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("synology-nas-api-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'synology-nas-api-pp-cli sync' first.", err)
	}
	count, err := db.Count(resourceType)
	if err != nil || count == 0 {
		db.Close()
		return nil, fmt.Errorf("no local data for %q.\nRun 'synology-nas-api-pp-cli sync' to hydrate the cache.", resourceType)
	}
	return db, nil
}
