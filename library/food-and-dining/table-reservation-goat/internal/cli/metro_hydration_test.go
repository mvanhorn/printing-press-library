// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

// withTempCacheDir redirects os.UserCacheDir() to a per-test temp dir
// for the duration of t. Avoids interleaving with real user cache and
// makes cleanup automatic.
func withTempCacheDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	// On macOS, os.UserCacheDir uses $HOME/Library/Caches, not XDG.
	// HOME override gets it.
	t.Setenv("HOME", tmp)
	return tmp
}

// TestProjectTockMetros verifies the source→registry projection
// preserves the four fields the geo filter actually uses (slug, name,
// lat, lng) and drops the businessCount we don't need.
func TestProjectTockMetros(t *testing.T) {
	in := []tock.MetroArea{
		{Slug: "seattle", Name: "Seattle", Lat: 47.6, Lng: -122.3, BusinessCount: 120},
		{Slug: "bellevue", Name: "Bellevue", Lat: 47.6, Lng: -122.2, BusinessCount: 38},
	}
	got := projectTockMetros(in)
	if len(got) != 2 {
		t.Fatalf("got %d; want 2", len(got))
	}
	if got[0].Slug != "seattle" || got[0].Name != "Seattle" {
		t.Errorf("slug/name not preserved: %+v", got[0])
	}
	if got[0].Lat != 47.6 || got[0].Lng != -122.3 {
		t.Errorf("centroid not preserved: %+v", got[0])
	}
}

// TestSaveLoadMetroCache_RoundTrip writes and reads a cache file via
// the public helpers. Verifies the file format is stable across calls.
func TestSaveLoadMetroCache_RoundTrip(t *testing.T) {
	withTempCacheDir(t)

	metros := []Metro{
		{Slug: "seattle", Name: "Seattle", Lat: 47.6, Lng: -122.3},
		{Slug: "bellevue", Name: "Bellevue", Lat: 47.6, Lng: -122.2},
	}
	saveMetroCache(metros)

	got := loadMetroCache()
	if len(got) != 2 {
		t.Fatalf("got %d; want 2 from round-trip", len(got))
	}
	if got[1].Slug != "bellevue" {
		t.Errorf("entry order not preserved: %+v", got)
	}
}

// TestLoadMetroCache_RejectsStale verifies the TTL guard kicks in on
// stale cache entries. Past-TTL files are silently dropped — caller
// falls back to fetch.
func TestLoadMetroCache_RejectsStale(t *testing.T) {
	withTempCacheDir(t)

	cf := metroCacheFile{
		SchemaVersion: metroCacheSchemaVersion,
		FetchedAt:     time.Now().Add(-2 * metroHydrationTTL), // way past TTL
		Metros:        []Metro{{Slug: "seattle", Name: "Seattle", Lat: 47.6, Lng: -122.3}},
	}
	data, _ := json.Marshal(cf)
	// PR #425 round-2 Greptile finding: don't hardcode `Library/Caches`
	// (macOS-only). Ask metroCachePath() for the canonical location so
	// the test passes on Linux too (where os.UserCacheDir returns
	// $XDG_CACHE_HOME directly, no Library/Caches prefix).
	path, ok := metroCachePath()
	if !ok {
		t.Fatal("metroCachePath() returned !ok in test fixture")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	if got := loadMetroCache(); got != nil {
		t.Errorf("stale cache should return nil; got %v", got)
	}
}

// TestLoadMetroCache_RejectsSchemaMismatch verifies schema-versioned
// invalidation — bumping metroCacheSchemaVersion silently invalidates
// pre-existing caches without users having to wipe them manually.
func TestLoadMetroCache_RejectsSchemaMismatch(t *testing.T) {
	withTempCacheDir(t)

	cf := metroCacheFile{
		SchemaVersion: 9999, // future schema
		FetchedAt:     time.Now(),
		Metros:        []Metro{{Slug: "seattle", Name: "Seattle", Lat: 47.6, Lng: -122.3}},
	}
	data, _ := json.Marshal(cf)
	// Same as above: ask metroCachePath() for the cross-platform path.
	path, ok := metroCachePath()
	if !ok {
		t.Fatal("metroCachePath() returned !ok in test fixture")
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, data, 0o600)

	if got := loadMetroCache(); got != nil {
		t.Errorf("schema-mismatched cache should return nil; got %v", got)
	}
}

// TestInvalidateMetroCache verifies the deletion path works even when
// the cache doesn't exist (no error to surface; silent best-effort).
func TestInvalidateMetroCache(t *testing.T) {
	withTempCacheDir(t)

	saveMetroCache([]Metro{{Slug: "seattle", Name: "Seattle", Lat: 47.6, Lng: -122.3}})
	if loadMetroCache() == nil {
		t.Fatal("setup: cache should exist after save")
	}
	invalidateMetroCache()
	if loadMetroCache() != nil {
		t.Error("post-invalidate cache should be empty")
	}
	// Second invalidate on already-empty cache must not panic.
	invalidateMetroCache()
}

// TestHydrateMetrosFromTock_UsesCache verifies the cache fast-path:
// when a fresh cache exists, the function loads it without touching
// the network at all. Done by pre-seeding the cache and passing nil
// session — if the function ignored the cache and fell through to the
// fetch, it would no-op (session is nil). The cached slug `providence`
// has no curated equivalent so the merge keeps it as a dynamic-only
// row (rule 3) — a clean signal that the cache fast-path fired.
func TestHydrateMetrosFromTock_UsesCache(t *testing.T) {
	withTempCacheDir(t)
	defer setDynamicMetros(nil, 0)

	preseeded := []Metro{
		{Slug: "providence", Name: "Providence", Lat: 41.824, Lng: -71.4128},
		{Slug: "seattle", Name: "Seattle", Lat: 47.6, Lng: -122.3},
	}
	saveMetroCache(preseeded)

	hydrateMetrosFromTock(context.Background(), nil) // nil session must not matter when cache is fresh

	if _, ok := getRegistry().Lookup("providence"); !ok {
		t.Fatal("providence should be available from cache-hydrated registry")
	}
}

// TestHydrateMetrosFromTock_NilSessionOnCacheMiss verifies that when
// the cache is absent AND session is nil, we silently no-op rather
// than panicking. This is the path agents hit when `auth login` hasn't
// been run yet.
func TestHydrateMetrosFromTock_NilSessionOnCacheMiss(t *testing.T) {
	withTempCacheDir(t)
	defer setDynamicMetros(nil, 0)

	hydrateMetrosFromTock(context.Background(), nil)

	// Registry should still respond from the static fallback.
	if _, ok := getRegistry().Lookup("seattle"); !ok {
		t.Error("static fallback should still cover seattle even when hydration no-ops")
	}
}
