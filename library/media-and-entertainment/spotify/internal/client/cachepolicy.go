// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-08-08: tiered cache TTLs + scoped invalidation)
// Library-side addition, not generator output. The generated cache used one
// flat 5-minute TTL and wholesale-removed the cache directory on any
// mutation. Both are wrong for Spotify's data shape:
//
//   - Catalog data (artists, albums, tracks, search, browse, ...) is
//     upstream-immutable on CLI timescales — a 5-minute TTL makes a
//     `discover via-playlists` re-run cost ~12 fresh calls against a daily
//     quota when it could cost ~0.
//   - Library data (me, playlists, users) is user-mutable — it gets a
//     short TTL and is the only class a local mutation can stale.
//   - Playback state (me/player) is realtime — caching it at all returns
//     lies.
//
// Cache filenames carry the class as a prefix (`catalog-<hash>.json`) so a
// mutation invalidates only its own class instead of nuking 24h of catalog
// warmth. Legacy unprefixed files from the flat-TTL format are unreadable
// dead weight (the key format changed too) and are removed opportunistically
// during invalidation.

package client

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	cacheClassCatalog = "catalog"
	cacheClassLibrary = "library"
	cacheClassPlayer  = "player"

	catalogCacheTTL = 24 * time.Hour
	libraryCacheTTL = 20 * time.Minute
)

// cacheClassForPath maps a request path to its cache class. Paths may carry
// an inline query string (GetWithHeadersValues folds params into the path),
// so strip it before classifying.
func cacheClassForPath(path string) string {
	p := strings.TrimPrefix(path, "/")
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	switch {
	case p == "me/player" || strings.HasPrefix(p, "me/player/"):
		return cacheClassPlayer
	case p == "me" || strings.HasPrefix(p, "me/"),
		p == "playlists" || strings.HasPrefix(p, "playlists/"),
		p == "users" || strings.HasPrefix(p, "users/"):
		return cacheClassLibrary
	default:
		return cacheClassCatalog
	}
}

// cacheTTLForPath returns the freshness window for a path's class. A
// non-positive TTL means the class must never be cached.
func cacheTTLForPath(path string) time.Duration {
	switch cacheClassForPath(path) {
	case cacheClassPlayer:
		return 0
	case cacheClassLibrary:
		return libraryCacheTTL
	default:
		return catalogCacheTTL
	}
}

// cacheFileName returns the class-prefixed on-disk name for a cache entry.
func (c *Client) cacheFileName(path string, params map[string]string) string {
	return cacheClassForPath(path) + "-" + c.cacheKey(path, params) + ".json"
}

func hasKnownCacheClassPrefix(name string) bool {
	for _, class := range []string{cacheClassCatalog, cacheClassLibrary, cacheClassPlayer} {
		if strings.HasPrefix(name, class+"-") {
			return true
		}
	}
	return false
}

// invalidateCacheForPath removes cache entries in the mutated path's class,
// leaving other classes warm. A playlist edit must not evict 24h of catalog
// cache. Legacy unprefixed entries (pre-class format, unreadable under the
// current key scheme) are swept out at the same time.
func (c *Client) invalidateCacheForPath(path string) {
	if c.cacheDir == "" {
		return
	}
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return
	}
	class := cacheClassForPath(path)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, class+"-") || !hasKnownCacheClassPrefix(name) {
			_ = os.Remove(filepath.Join(c.cacheDir, name))
		}
	}
}
