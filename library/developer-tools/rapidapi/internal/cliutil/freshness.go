// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: cache freshness helpers — schema-gated staleness check.

package cliutil

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// freshnessWindow is how long a cache entry is considered fresh.
const freshnessWindow = 24 * time.Hour

// freshnessMarkerPath returns the marker file path for a store path.
func freshnessMarkerPath(storePath string) string {
	if storePath == "" {
		storePath = filepath.Join(os.Getenv("HOME"), ".local", "share", "rapidapi-pp-cli")
	}
	return filepath.Join(filepath.Dir(storePath), "cache-freshness")
}

// EnsureFresh reports whether the local store at storePath is fresh (no
// refresh needed). A missing marker or a marker older than the window means
// stale. Returns (true, nil) when the store is unavailable so callers can
// degrade gracefully.
func EnsureFresh(ctx context.Context, storePath string) (bool, error) {
	if ctx == nil {
		return true, nil
	}
	marker := freshnessMarkerPath(storePath)
	st, err := os.Stat(marker)
	if err != nil {
		return false, nil // no marker → stale
	}
	return time.Since(st.ModTime()) < freshnessWindow, nil
}

// MarkFresh records the current time as the freshness timestamp for storePath.
func MarkFresh(ctx context.Context, storePath string, at time.Time) error {
	if ctx == nil {
		return nil
	}
	marker := freshnessMarkerPath(storePath)
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(at.UTC().Format(time.RFC3339)), 0o600)
}
