// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: scaffold-endpoint-redirects — issue #406 failures 1 + 3.
//
// Tock metroArea hydration with disk cache.
//
// Tock's $REDUX_STATE hydrates `state.app.config.metroArea` with the
// full 253-metro registry on every city-search SSR. This file fetches
// that array once per CLI invocation (cheap) and caches the result for
// 24h on disk (`<UserCacheDir>/table-reservation-goat-pp-cli/
// tock-metros.json`) so subsequent invocations skip the fetch entirely.
//
// Failure semantics: every step is best-effort. Cache read failures,
// HTTP failures, parse failures all silently fall back to the static
// 20-entry registry — the CLI never regresses below pre-#406 behavior
// because of a hydration miss.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

// metroHydrationTTL bounds how long a cached Tock metro registry is
// reused. Tock's metroArea is config-tier and changes maybe once a
// quarter; 24h is well within the staleness budget.
const metroHydrationTTL = 24 * time.Hour

// metroCacheFile is the on-disk JSON shape. Pinned to a tiny stable
// surface so future schema changes can detect-and-rebuild via
// SchemaVersion.
type metroCacheFile struct {
	SchemaVersion int       `json:"schema_version"`
	FetchedAt     time.Time `json:"fetched_at"`
	Metros        []Metro   `json:"metros"`
}

// metroCacheSchemaVersion bumps every time the on-disk Place shape
// changes incompatibly. v2 = the Metro→Place rename: ProviderCoverage,
// ParentMetro, RadiusKm, Tier all became part of the cached payload.
// Loading a v1 cache as v2 would yield mixed-shape entries (no
// RadiusKm, so ReverseLookup would skip every dynamic Place), so we
// invalidate on schema mismatch and let the next CLI invocation
// rebuild from the live Tock SSR.
const metroCacheSchemaVersion = 2

// metroCachePath returns the on-disk cache location, creating the
// parent directory on demand. Returns ok=false (instead of an error)
// because every cache miss is non-fatal.
func metroCachePath() (string, bool) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(dir, "table-reservation-goat-pp-cli", "tock-metros.json"), true
}

// loadMetroCache returns the cache contents if present and within TTL.
// Returns nil on any failure — the caller proceeds to fetch.
func loadMetroCache() []Metro {
	path, ok := metroCachePath()
	if !ok {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cf metroCacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil
	}
	if cf.SchemaVersion != metroCacheSchemaVersion {
		return nil
	}
	if time.Since(cf.FetchedAt) > metroHydrationTTL {
		return nil
	}
	if len(cf.Metros) == 0 {
		return nil
	}
	return cf.Metros
}

// saveMetroCache writes the cache atomically (temp + rename) at mode
// 0600. Silent on failure — cache rebuild on next invocation.
func saveMetroCache(metros []Metro) {
	if len(metros) == 0 {
		return
	}
	path, ok := metroCachePath()
	if !ok {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	cf := metroCacheFile{
		SchemaVersion: metroCacheSchemaVersion,
		FetchedAt:     time.Now(),
		Metros:        metros,
	}
	data, err := json.Marshal(cf)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// projectTockMetros converts the source-layer Tock shape into the
// CLI's Place registry shape. Drops aliases since Tock has no alias
// concept — agents typing `--metro nyc` get matched via the curated
// fallback's alias chain, and the dynamic `new-york-city` entry (or
// whatever Tock canonicalizes to) takes over the centroid.
//
// Tock's metroArea entries are metro-tier (Tock lists Bellevue WA,
// Brooklyn, etc. as their own "metros" — its hierarchy is flat), so
// RadiusKm defaults to the metro band (75 km) and Tier to
// PlaceTierMetroCentroid. ProviderCoverage["tock"] captures the live
// BusinessCount so downstream callers can prefer Tock metros with
// active inventory. ParentMetro["tock"] points at the entry's own
// slug — Tock self-routes a query for Bellevue WA to the "bellevue"
// metro, so the provider-routing answer for the place IS the place
// itself.
func projectTockMetros(in []tock.MetroArea) []Place {
	out := make([]Place, 0, len(in))
	for _, m := range in {
		p := Place{
			Slug:        m.Slug,
			Name:        m.Name,
			Lat:         m.Lat,
			Lng:         m.Lng,
			RadiusKm:    75,
			Tier:        PlaceTierMetroCentroid,
			ParentMetro: map[string]string{"tock": m.Slug},
		}
		if m.BusinessCount > 0 {
			p.ProviderCoverage = map[string]int{"tock": m.BusinessCount}
		}
		out = append(out, p)
	}
	return out
}

// hydrateMetrosFromTock is the high-level entry point: cache first,
// then HTTP, then silent. Designed to be called once per CLI
// invocation before the first metro lookup.
func hydrateMetrosFromTock(ctx context.Context, session *auth.Session) {
	// Cache fast-path.
	if cached := loadMetroCache(); len(cached) > 0 {
		setDynamicMetros(cached, time.Now().Unix())
		return
	}
	// Cache miss — fetch from Tock SSR.
	if session == nil {
		return
	}
	c, err := tock.New(session)
	if err != nil {
		return
	}
	areas, err := c.FetchMetroAreas(ctx)
	if err != nil || len(areas) == 0 {
		return
	}
	metros := projectTockMetros(areas)
	setDynamicMetros(metros, time.Now().Unix())
	saveMetroCache(metros)
}

// invalidateMetroCache wipes the on-disk metro cache. Called by
// `auth login --chrome` (future) and exposed for tests.
func invalidateMetroCache() {
	if path, ok := metroCachePath(); ok {
		_ = os.Remove(path)
	}
}
