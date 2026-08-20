// Copyright 2026 Nathan Kettles and contributors. Licensed under Apache-2.0. See LICENSE.

// Package cliutil provides shared helpers used across generated CLI commands.
package cliutil

import (
	"fmt"
	"io"
	"os"
	"time"
)

// DefaultStaleThreshold is the default duration after which locally-cached
// data is considered stale. Configurable via NYLAS_PP_CLI_STALE_AFTER.
const DefaultStaleThreshold = 5 * time.Minute

// StaleThreshold returns the configured staleness threshold. It reads
// NYLAS_PP_CLI_STALE_AFTER first (e.g. "30m", "2h"), then falls back to
// DefaultStaleThreshold. Parse errors on the env var are silently ignored
// and the default is used.
func StaleThreshold() time.Duration {
	if v := os.Getenv("NYLAS_PP_CLI_STALE_AFTER"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return DefaultStaleThreshold
}

// FreshnessResult describes the age of cached data relative to a threshold.
type FreshnessResult struct {
	// LastSyncedAt is when the cache was last written. Zero means never synced.
	LastSyncedAt time.Time
	// Age is time.Since(LastSyncedAt). Zero when never synced.
	Age time.Duration
	// Threshold is the configured staleness threshold.
	Threshold time.Duration
	// Stale is true when Age > Threshold or the cache was never synced.
	Stale bool
}

// Banner returns a human-readable freshness line suitable for printing to
// stderr on TTY sessions. Returns an empty string when data is fresh.
//
//	data as of 3 minutes ago (stale; threshold 5m0s)
func (r FreshnessResult) Banner() string {
	if r.LastSyncedAt.IsZero() {
		return "warning: no local data; run 'nylas-pp-cli sync' to populate the cache"
	}
	age := r.Age.Round(time.Second)
	if !r.Stale {
		return ""
	}
	return fmt.Sprintf("warning: data as of %s ago (stale; threshold %s) — run 'nylas-pp-cli sync' to refresh",
		age, r.Threshold)
}

// EnsureFresh checks whether lastSyncedAt is within threshold of now.
// When lastSyncedAt is zero the cache is considered permanently stale.
// w receives a warning banner when the result is stale; pass nil to
// suppress output. The return value always describes the freshness state
// so callers can branch on Stale independently of banner output.
func EnsureFresh(w io.Writer, lastSyncedAt time.Time, threshold time.Duration) FreshnessResult {
	r := FreshnessResult{
		LastSyncedAt: lastSyncedAt,
		Threshold:    threshold,
	}
	if lastSyncedAt.IsZero() {
		r.Stale = true
		if w != nil {
			fmt.Fprintln(w, r.Banner())
		}
		return r
	}
	r.Age = time.Since(lastSyncedAt)
	r.Stale = r.Age > threshold
	if r.Stale && w != nil {
		fmt.Fprintln(w, r.Banner())
	}
	return r
}
