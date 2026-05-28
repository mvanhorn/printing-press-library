// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-05-20: value-compare) — Thin wrapper over the shared
// internal/cache template. The wrapper adds mod-time-aware reads so the
// caller can detect "expired but readable" entries and use them as the
// stale-fallback in the Lookup chain.

package valuation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheTTL is the freshness window for on-disk valuations. TPG updates
// monthly; 30 days matches that cadence with one day of slack.
const CacheTTL = 30 * 24 * time.Hour

// ValuationRecord is what gets persisted to disk for each program.
type ValuationRecord struct {
	Program   Program   `json:"program"`
	CPPCents  float64   `json:"cpp_cents"`
	SourceURL string    `json:"source_url"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Cache is a per-program file-backed store. Unlike the shared cache
// template, Get returns the record alongside its mod-time and a fresh
// flag — the Lookup chain needs to distinguish "stale but present"
// from "absent" to drive its fallback behavior.
type Cache struct {
	Dir string
	TTL time.Duration
}

// NewCache constructs a Cache rooted at dir (created on first Set).
func NewCache(dir string) *Cache {
	return &Cache{Dir: dir, TTL: CacheTTL}
}

// path returns the per-program cache file. One file per program slug;
// human-readable names are intentional — easier to debug than sha256
// digests for a registry that will only ever hold a handful of keys.
func (c *Cache) path(p Program) string {
	return filepath.Join(c.Dir, string(p)+".json")
}

// Get returns (record, modTime, fresh, ok). ok is true when a file
// exists and decoded cleanly; fresh is true when within TTL.
func (c *Cache) Get(p Program) (ValuationRecord, time.Time, bool, bool) {
	path := c.path(p)
	info, err := os.Stat(path)
	if err != nil {
		return ValuationRecord{}, time.Time{}, false, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ValuationRecord{}, info.ModTime(), false, false
	}
	var rec ValuationRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return ValuationRecord{}, info.ModTime(), false, false
	}
	fresh := time.Since(info.ModTime()) <= c.TTL
	return rec, info.ModTime(), fresh, true
}

// Set persists a record for the given program. Errors are returned
// rather than swallowed (unlike the shared template), so a corrupted
// cache dir surfaces visibly instead of silently re-fetching forever.
func (c *Cache) Set(p Program, rec ValuationRecord) error {
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return fmt.Errorf("mkdir cache dir: %w", err)
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}
	tmp := c.path(p) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}
	if err := os.Rename(tmp, c.path(p)); err != nil {
		return fmt.Errorf("rename cache file: %w", err)
	}
	return nil
}

// DefaultCacheDir returns the canonical on-disk location for valuation
// caches: ~/.cache/alaska-airlines-pp-cli/valuations/.
func DefaultCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("could not resolve user home dir for valuation cache")
	}
	return filepath.Join(home, ".cache", "alaska-airlines-pp-cli", "valuations"), nil
}
