// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.
// Cache auto-refresh helpers: automatically re-sync stale local data before
// read commands run, so agents always get fresh results without explicit sync calls.

package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/synology-router/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-router/internal/store"
)

// autoRefreshIfStale checks whether the local store is stale and, if so,
// warns the user to run sync. SRM resources have per-resource stale thresholds
// (traffic: 15s, devices: 30s, wan: 60s, config: 5m, dns: 10m, system: 1h).
//
// resourceType filters which resource to check; empty means check all resources.
// Returns nil if the store does not exist yet (first run), or if a refresh is
// not needed.
func autoRefreshIfStale(ctx context.Context, w io.Writer, dbPath, resourceType string, staleAfter time.Duration) error {
	if staleAfter == 0 {
		staleAfter = cliutil.SRMStaleAfter(resourceType)
	}
	if dbPath == "" {
		dbPath = defaultDBPath("synology-router-pp-cli")
	}

	fresh, err := cliutil.EnsureFresh(ctx, dbPath, resourceType, staleAfter)
	if err != nil {
		// Non-fatal: if we can't check freshness, proceed with potentially stale data.
		return nil
	}

	if !fresh {
		fmt.Fprintf(w, "  ℹ  Cache stale (>%s for %q). Run 'synology-router-pp-cli sync' to refresh.\n", staleAfter, resourceType)
	}
	return nil
}

// autoRefreshStoreResource checks staleness for a single resource type and
// returns whether the cached data can be trusted (true = fresh or unknown).
// This is a lightweight version that does not write to any output — intended
// for use in commands that fall back to live API when cache is stale.
func autoRefreshStoreResource(ctx context.Context, dbPath, resourceType string) bool {
	if dbPath == "" {
		dbPath = defaultDBPath("synology-router-pp-cli")
	}
	fresh, err := cliutil.EnsureFresh(ctx, dbPath, resourceType, cliutil.SRMStaleAfter(resourceType))
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
		dbPath = defaultDBPath("synology-router-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'synology-router-pp-cli sync' first", err)
	}
	count, err := db.Count(resourceType)
	if err != nil || count == 0 {
		db.Close()
		return nil, fmt.Errorf("no local data for %q.\nRun 'synology-router-pp-cli sync' to hydrate the cache", resourceType)
	}
	return db, nil
}
