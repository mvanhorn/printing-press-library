// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-08-08: tiered cache TTLs + scoped invalidation) — coverage
// for cachepolicy.go.

package client

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/config"
)

func TestCacheClassForPath(t *testing.T) {
	cases := map[string]string{
		"/search":                   cacheClassCatalog,
		"/artists/abc":              cacheClassCatalog,
		"/browse/new-releases":      cacheClassCatalog,
		"/me":                       cacheClassLibrary,
		"/me/tracks":                cacheClassLibrary,
		"/playlists/xyz/tracks":     cacheClassLibrary,
		"/users/abc/playlists":      cacheClassLibrary,
		"/me/player":                cacheClassPlayer,
		"/me/player/devices":        cacheClassPlayer,
		"/search?q=x&type=playlist": cacheClassCatalog,
		"/me/tracks?limit=10":       cacheClassLibrary,
	}
	for path, want := range cases {
		if got := cacheClassForPath(path); got != want {
			t.Errorf("cacheClassForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestCacheTTLForPath(t *testing.T) {
	if got := cacheTTLForPath("/me/player/devices"); got > 0 {
		t.Fatalf("player TTL = %v, want non-positive (never cached)", got)
	}
	if got := cacheTTLForPath("/me/tracks"); got != libraryCacheTTL {
		t.Fatalf("library TTL = %v, want %v", got, libraryCacheTTL)
	}
	if got := cacheTTLForPath("/search"); got != catalogCacheTTL {
		t.Fatalf("catalog TTL = %v, want %v", got, catalogCacheTTL)
	}
}

func TestPlayerClassNeverCached(t *testing.T) {
	dir := t.TempDir()
	c := &Client{cacheDir: dir, Config: &config.Config{}}
	c.writeCache("/me/player/devices", nil, []byte(`{"devices":[]}`))
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("player-class write must be a no-op; found %d cache entries", len(entries))
	}
	if _, ok := c.readCache("/me/player/devices", nil); ok {
		t.Fatal("player-class read must always miss")
	}
}

func TestCacheRoundTripUsesClassPrefix(t *testing.T) {
	dir := t.TempDir()
	c := &Client{cacheDir: dir, Config: &config.Config{}}
	c.writeCache("/search", map[string]string{"q": "x"}, []byte(`{"ok":true}`))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 cache entry, got %d", len(entries))
	}
	if name := entries[0].Name(); !hasKnownCacheClassPrefix(name) {
		t.Fatalf("cache file %q lacks a class prefix", name)
	}
	if data, ok := c.readCache("/search", map[string]string{"q": "x"}); !ok || string(data) != `{"ok":true}` {
		t.Fatalf("readCache = %q, %v; want round-trip hit", data, ok)
	}
}

func TestScopedInvalidationLeavesOtherClassesWarm(t *testing.T) {
	dir := t.TempDir()
	c := &Client{cacheDir: dir, Config: &config.Config{}}
	// Warm catalog + library, plus a legacy unprefixed file from the old
	// flat format (dead weight — must be swept).
	c.writeCache("/search", map[string]string{"q": "x"}, []byte(`{"catalog":true}`))
	c.writeCache("/me/tracks", nil, []byte(`{"library":true}`))
	legacy := filepath.Join(dir, "0123456789abcdef.json")
	if err := os.WriteFile(legacy, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	// A library mutation (e.g. PUT /playlists/{id}) invalidates library
	// only; the 24h catalog warmth survives.
	c.invalidateCacheForPath("/playlists/xyz/tracks")

	if _, ok := c.readCache("/search", map[string]string{"q": "x"}); !ok {
		t.Fatal("catalog entry must survive a library-class mutation")
	}
	if _, ok := c.readCache("/me/tracks", nil); ok {
		t.Fatal("library entry must be invalidated by a library-class mutation")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("legacy unprefixed cache file must be swept during invalidation")
	}
}

func TestCacheKeyStableAcrossAccessTokenRotation(t *testing.T) {
	// Spotify rotates access tokens hourly; the cache identity must ride
	// the stable refresh token, not the rotating header.
	cfgBefore := &config.Config{RefreshToken: "stable-refresh", AccessToken: "token-hour-1"}
	cfgAfter := &config.Config{RefreshToken: "stable-refresh", AccessToken: "token-hour-2"}
	before := (&Client{Config: cfgBefore}).cacheKey("/search", map[string]string{"q": "x"})
	after := (&Client{Config: cfgAfter}).cacheKey("/search", map[string]string{"q": "x"})
	if before != after {
		t.Fatalf("cache key must survive access-token rotation: %q != %q", before, after)
	}

	// Different accounts (different refresh tokens) must not collide.
	other := (&Client{Config: &config.Config{RefreshToken: "other-account"}}).cacheKey("/search", map[string]string{"q": "x"})
	if before == other {
		t.Fatal("cache key must differ across accounts")
	}
}

// Guard: TTL constants stay ordered player < library < catalog.
func TestCacheTTLOrdering(t *testing.T) {
	if !(time.Duration(0) < libraryCacheTTL && libraryCacheTTL < catalogCacheTTL) {
		t.Fatalf("TTL ordering violated: library=%v catalog=%v", libraryCacheTTL, catalogCacheTTL)
	}
}
