// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package opentable

// PATCH: cross-network-source-clients (avail_cache) — see .printing-press-patches.json.
//
// Disk-cache layer for RestaurantsAvailability responses, keyed by
// (restID, date, time, party, forwardMinutes, backwardMinutes). Cache files
// live under <UserCacheDir>/table-reservation-goat-pp-cli/ot-avail/ with
// SHA-256-hashed filenames so user-influenced date/time inputs cannot
// produce path-traversal vectors. Each entry embeds the persisted-query
// Hash and a SchemaVersion integer; reads with mismatch on either are
// treated as cache misses, so a gateway-side query rotation OR a
// CLI-side cache-shape change invalidates entries cleanly.
//
// Modeled on internal/source/auth/chrome.go's akamai cache pattern: atomic
// write-then-rename, mode 0600, best-effort semantics on disk failure.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// availCacheSchemaVersion bumps any time the on-disk shape of availCacheEntry
// changes incompatibly. Bumping invalidates all existing cache files.
const availCacheSchemaVersion = 1

// availCacheTTLDefault is the default cache lifetime. Chosen conservatively
// under OT's documented "~minutes" slot-token lifetime so cached responses
// don't carry stale tokens that would fail at booking.
const availCacheTTLDefault = 3 * time.Minute

// availCacheStaleHardCap is the absolute maximum age a stale-cache fallback
// will return. Beyond this, cached entries are treated as missing.
const availCacheStaleHardCap = 24 * time.Hour

// availCacheTTLMin and availCacheTTLMax bound the TRG_OT_CACHE_TTL override.
// Values outside this range fall back to the default.
const (
	availCacheTTLMin = 1 * time.Minute
	availCacheTTLMax = 24 * time.Hour
)

// availCacheKey identifies one (restID, date, time, party, window) request.
// All fields participate in the cache key — earliest passes a different
// window than watch, and serving one's response to the other returns slots
// outside the caller's window.
type availCacheKey struct {
	RestID          int    `json:"rest_id"`
	Date            string `json:"date"` // YYYY-MM-DD
	Time            string `json:"time"` // HH:MM
	PartySize       int    `json:"party_size"`
	ForwardMinutes  int    `json:"forward_minutes"`
	BackwardMinutes int    `json:"backward_minutes"`
}

// availCacheEntry is the on-disk cache record. The full key is stored
// alongside the response for verification on read and for human inspection.
type availCacheEntry struct {
	Key           availCacheKey            `json:"key"`
	FetchedAt     time.Time                `json:"fetched_at"`
	CachedAt      time.Time                `json:"cached_at"`
	Hash          string                   `json:"hash"`
	SchemaVersion int                      `json:"schema_version"`
	Response      []RestaurantAvailability `json:"response"`
}

// dateRE and timeRE bound user-influenced fields to safe shapes BEFORE they
// reach filename construction. A malformed input is rejected outright rather
// than encoded into a hashed filename — the hash would still be safe, but
// silently accepting bad input would mask CLI-flag-validation bugs upstream.
var (
	dateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRE = regexp.MustCompile(`^\d{2}:\d{2}$`)
)

// validateAvailCacheKey rejects keys whose user-influenced fields don't match
// the expected shape. Caller should fall through to the network path on
// error rather than crash.
func validateAvailCacheKey(k availCacheKey) error {
	if k.RestID <= 0 {
		return fmt.Errorf("avail cache: rest_id must be positive, got %d", k.RestID)
	}
	if !dateRE.MatchString(k.Date) {
		return fmt.Errorf("avail cache: date %q does not match YYYY-MM-DD", k.Date)
	}
	if !timeRE.MatchString(k.Time) {
		return fmt.Errorf("avail cache: time %q does not match HH:MM", k.Time)
	}
	if k.PartySize <= 0 {
		return fmt.Errorf("avail cache: party_size must be positive, got %d", k.PartySize)
	}
	return nil
}

// availCachePath returns the on-disk path for a cache entry. The filename is
// the first 16 hex chars of SHA-256 over a canonicalized key string —
// hashing avoids path-traversal vectors from user input AND keeps filenames
// fixed-length regardless of input shape (cross-FS portable).
func availCachePath(k availCacheKey) (string, error) {
	canonical := fmt.Sprintf("%d|%s|%s|%d|%d|%d",
		k.RestID, k.Date, k.Time, k.PartySize, k.ForwardMinutes, k.BackwardMinutes)
	sum := sha256.Sum256([]byte(canonical))
	name := hex.EncodeToString(sum[:8]) + ".json" // 16 hex chars
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "table-reservation-goat-pp-cli", "ot-avail", name), nil
}

// readAvailCacheTTL reads TRG_OT_CACHE_TTL with clamp + default fallback.
// Out-of-range or unparseable values fall back to the default with a stderr
// warning so users notice the misconfiguration without the CLI crashing.
func readAvailCacheTTL() time.Duration {
	v := strings.TrimSpace(os.Getenv("TRG_OT_CACHE_TTL"))
	if v == "" {
		return availCacheTTLDefault
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TRG_OT_CACHE_TTL=%q is not a valid duration; using default %s\n", v, availCacheTTLDefault)
		return availCacheTTLDefault
	}
	if d < availCacheTTLMin || d > availCacheTTLMax {
		fmt.Fprintf(os.Stderr, "TRG_OT_CACHE_TTL=%s out of range [%s, %s]; using default %s\n",
			d, availCacheTTLMin, availCacheTTLMax, availCacheTTLDefault)
		return availCacheTTLDefault
	}
	return d
}

// availCacheLoadResult bundles the cache-read outcome so callers can
// distinguish "fresh hit" / "stale-but-readable" / "miss" without three
// return values.
type availCacheLoadResult struct {
	Entry *availCacheEntry
	// Fresh is true when the entry is within TTL — safe to return as a
	// fresh cache hit without a network call. False when the entry is past
	// TTL but within the 24h stale cap; only return on BotDetectionError
	// fallback (U5).
	Fresh bool
}

// loadAvailCache reads the cache entry for k. Returns nil (cache miss) when:
// the file is missing, corrupt, hash-drifted from currentHash, schema-drifted,
// past the 24h stale cap, or any other read error. Returns Fresh=false when
// the entry is within 24h but past TTL — caller decides whether to use it
// (only U5 should, on BotDetectionError fallback).
func loadAvailCache(k availCacheKey, currentHash string) *availCacheLoadResult {
	if err := validateAvailCacheKey(k); err != nil {
		return nil
	}
	path, err := availCachePath(k)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var e availCacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil
	}
	if e.SchemaVersion != availCacheSchemaVersion {
		return nil
	}
	if e.Hash != currentHash {
		return nil
	}
	now := time.Now()
	age := now.Sub(e.FetchedAt)
	if age > availCacheStaleHardCap {
		return nil
	}
	ttl := readAvailCacheTTL()
	return &availCacheLoadResult{
		Entry: &e,
		Fresh: age <= ttl,
	}
}

// saveAvailCache writes an entry atomically. Best-effort: errors are swallowed
// rather than surfaced — a failed write should not break the call that
// otherwise succeeded.
func saveAvailCache(k availCacheKey, currentHash string, response []RestaurantAvailability) {
	if err := validateAvailCacheKey(k); err != nil {
		return
	}
	path, err := availCachePath(k)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	now := time.Now()
	entry := availCacheEntry{
		Key:           k,
		FetchedAt:     now,
		CachedAt:      now,
		Hash:          currentHash,
		SchemaVersion: availCacheSchemaVersion,
		Response:      response,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}
