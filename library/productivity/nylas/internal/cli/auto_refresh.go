// Copyright 2026 Nathan Kettles and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/nylas/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/nylas/internal/store"
)

// autoRefreshIfStale is called by local-mirror commands (since, search, sql,
// gravity, response-time, export) before they serve data from the local SQLite
// cache. It queries sync_state for the most-recently-written resource and
// compares the age to the configured staleness threshold.
//
// When the cache is stale the function:
//  1. Prints a "data as of X (stale)" warning to stderr (TTY only; suppressed
//     when stdout is piped so agent/JSON flows stay clean).
//  2. Returns the FreshnessResult so callers can decide whether to abort.
//
// It does not block or trigger a background sync; triggering a sync from inside
// a read command would change the database while the caller is iterating rows.
// Use the --stale-after flag or NYLAS_PP_CLI_STALE_AFTER env var to tune the
// threshold (default: 5 minutes).
func autoRefreshIfStale(ctx context.Context, dbPath string, stderr io.Writer) cliutil.FreshnessResult {
	if dbPath == "" {
		dbPath = defaultDBPath("nylas-pp-cli")
	}
	threshold := cliutil.StaleThreshold()

	// If the database doesn't exist yet, report stale so the caller can print
	// a hint. Don't attempt to open — OpenReadOnly on a missing file errors.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		zero := time.Time{}
		return cliutil.EnsureFresh(stderr, zero, threshold)
	}

	s, err := store.OpenReadOnly(dbPath)
	if err != nil {
		// Can't read store — treat as stale but don't propagate the error;
		// the calling command's own Open will produce a better message.
		return cliutil.FreshnessResult{Stale: true, Threshold: threshold}
	}
	defer s.Close()

	// Find the most-recently-synced resource timestamp across all tracked tables.
	var lastSynced time.Time
	row := s.DB().QueryRowContext(ctx, `SELECT MAX(last_synced_at) FROM sync_state WHERE last_synced_at IS NOT NULL`)
	var ts *time.Time
	if err := row.Scan(&ts); err == nil && ts != nil {
		lastSynced = *ts
	}

	// Suppress banner when stdout is not a terminal (agent/JSON flows).
	var w io.Writer
	if isTerminal(os.Stdout) {
		w = stderr
	}
	return cliutil.EnsureFresh(w, lastSynced, threshold)
}

// warnIfStaleFlag returns a --stale-after flag description for commands that
// want to expose threshold tuning to the user. The returned default shows the
// runtime default so help text stays accurate even if the env var is set.
func warnIfStaleFlag() string {
	return fmt.Sprintf("consider cache stale after this duration (default %s; overrides NYLAS_PP_CLI_STALE_AFTER)", cliutil.DefaultStaleThreshold)
}
