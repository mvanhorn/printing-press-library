// Copyright 2026 riteshtiwari. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"context"
	"time"
)

// FreshnessChecker can report whether a local data store is stale.
type FreshnessChecker interface {
	// GetLastSyncedAt returns the RFC3339 timestamp of the last sync for the
	// named resource, or an empty string if the resource has never been synced.
	GetLastSyncedAt(resourceType string) string
}

// EnsureFresh returns true if all of the given resources were synced within
// staleAfter. It returns false when any resource is missing or older than the
// threshold, signalling that a sync is recommended before trusting cached data.
// ctx is reserved for future cancellation support.
func EnsureFresh(ctx context.Context, store FreshnessChecker, staleAfter time.Duration, resources ...string) bool {
	if store == nil {
		return false
	}
	cutoff := time.Now().Add(-staleAfter)
	for _, r := range resources {
		ts := store.GetLastSyncedAt(r)
		if ts == "" {
			return false
		}
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			return false
		}
		if t.Before(cutoff) {
			return false
		}
	}
	return true
}
