// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// PublicAPIKey is Airbnb's public web GraphQL API key. It is a long-lived
// constant baked into airbnb.com's own frontend and is required on every API
// request. It is not a user secret.
const PublicAPIKey = "d306zoyjsyarp7ifhu67rjxn52tv0t20"

// defaultHashes is the snapshot of persisted-query operationName -> sha256Hash
// pairs captured from Airbnb's web app. Airbnb rotates these on deploys, so the
// CLI can re-harvest current values with `airbnb-outreach-pp-cli ops refresh`, which
// writes an override file consulted ahead of this map. Keep entries here that a
// command depends on so the CLI works out-of-the-box before any refresh.
var defaultHashes = map[string]string{
	// Search + discovery
	"StaysSearch":           "d39f949ec846c484f09df7d2ba282874c27aa4b4adc9a71399a8fe0ae3a9cf67",
	"StaysPdpSections":      "bcfedb08b9c4945e11a7d6de01ff6c09f526f5655363cac704d1cd1c30ab2581",
	"AutoSuggestionsQuery":  "e3beff7ce0ba2f9f41fe486d6679ac186f62f7a141123dde3d5a565eb4e39f12",

	// Messaging (reads)
	"ViaductInboxData":             "ac355053f175930eff99286bdfb7b2bf7c40db930b9757a09a4f1f74972b5836",
	"ViaductGetThreadAndDataQuery": "40c8a7d9af0f10098129495d71e2c1eb6d7fc0b40cbd20e0ff17d24bf404a609",
	"FetchInboxFiltersConfig":      "076e2b08aa8463475790dad6f5c6d6676cc2d0a46477df9f08a7faa6d5c6f56a",

	// Messaging (writes)
	"CreateBulkMessagesMutation":          "94ac2c4bd07edace539dbf2b9665d9030b6dee479db345ba8a8bbc234b3bbfa3",
	"CreateLastMessageReadViaductMutation": "74dc0e6805b0fd0041a688b9d9ea731f38cf656115438a8e94af9c138cf3ae95",
	"GetSignedUrlsMutation":                "c70cd37b719e3db2fde53e9d339d74a1b2d955acf3f49e9a78ab208045507297",
	"CreateMediaItemsMutation":             "a801b7c68088d2b55cd6dd6925694f8da6e26081a804bbdc2b7179f303def330",
	"SendContactHostMessageMutation":       "8d117119317854fbf1fc2dbb5cc8d3aade5875eaa57af0c97dca2f8791632202",

	// Wishlists
	"WishlistIndexPageQuery":  "b8b421d802c399b55fb6ac1111014807a454184ad38f198365beb7836c018c18",
	"WishlistItemsAsyncQuery": "c0f9d9474bb20eb7af2f94f8e022750a5ed9b7437613e1d9aa91aadea87e4467",

	// Account
	"Header":              "bb590cf8c21b62e4b5122e1cd19969f1f1df72832040a335fd45af52597440e4",
	"IsHostQuery":         "ff889330f06ea6bb31cf107f0c0c50910d64669ab58a1671396857a2562af3c5",
	"GetThumbnailPicQuery": "ab55c22df96bd74dfabf0f78b14f8172bf2cf52b7e2c29abc75ae65a59610d4b",

	// Booking (read-only price quote — draft checkout, no payment)
	"stayCheckout": "56542f362fecc914498dfda4738ceaed5d9d6d26d20a3ddfd92c036bece53f1f",
}

// Registry resolves operationName -> current sha256 hash, preferring a
// user-refreshed override file over the bundled snapshot.
type Registry struct {
	overrides map[string]string
	path      string
}

// LoadRegistry reads the on-disk override file (written by `ops refresh`) if it
// exists. A missing file is not an error — the bundled snapshot is used.
func LoadRegistry() *Registry {
	r := &Registry{overrides: map[string]string{}, path: registryPath()}
	if data, err := os.ReadFile(r.path); err == nil {
		_ = json.Unmarshal(data, &r.overrides)
	}
	return r
}

// Hash returns the current hash for an operation, or "" if unknown.
func (r *Registry) Hash(op string) string {
	if r != nil {
		if h, ok := r.overrides[op]; ok && h != "" {
			return h
		}
	}
	if h, ok := defaultHashes[op]; ok {
		return h
	}
	return ""
}

// Source reports whether a hash came from the refreshed override file or the
// bundled snapshot, for `ops list` diagnostics.
func (r *Registry) Source(op string) string {
	if r != nil {
		if _, ok := r.overrides[op]; ok {
			return "refreshed"
		}
	}
	if _, ok := defaultHashes[op]; ok {
		return "bundled"
	}
	return "unknown"
}

// Operations lists every known operation name (union of overrides and bundled),
// sorted, for `ops list`.
func (r *Registry) Operations() []string {
	set := map[string]struct{}{}
	for k := range defaultHashes {
		set[k] = struct{}{}
	}
	if r != nil {
		for k := range r.overrides {
			set[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func registryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "airbnb-outreach-pp-cli", "operations.json")
}

var (
	scriptSrcRe = regexp.MustCompile(`<script[^>]+src="(https://[a-z0-9.]*muscache\.com/airbnb/static/packages/web/[^"]+\.js)"`)
	hash64Re    = regexp.MustCompile(`[a-f0-9]{64}`)
	opNameRe    = regexp.MustCompile(`["']([A-Z][A-Za-z0-9_]{4,55}(?:Query|Mutation|Subscription))["']`)
)

// Harvest re-discovers current operation hashes using the authenticated client
// (cookies are needed to load login-gated route pages like /guest/messages).
// For each route it loads the page HTML, extracts the JS bundle URLs, then over
// each bundle runs a name-first pass (refreshing every operation the CLI knows,
// including non-suffixed names like StaysSearch) and a discovery pass (new
// *Query/*Mutation/*Subscription operations). Route bundles are CDN-hosted, so
// they are fetched without cookies.
// Harvest returns the discovered operation->hash map and the list of routes
// that failed to load. A non-empty failedRoutes with a non-empty map means a
// PARTIAL refresh: some route bundles (e.g. /guest/messages when the session
// cookie has expired) could not be read, so their operations may still be
// stale. Callers must surface this — reporting success on a partial harvest is
// how messaging hashes silently rot. A hard error is returned only when every
// route failed.
func Harvest(c *Client, routes []string) (found map[string]string, failedRoutes []string, err error) {
	known := knownOperationNames()
	found = map[string]string{}
	var lastErr error
	for _, route := range routes {
		html, e := c.GetHTML(route)
		if e != nil {
			lastErr = e
			failedRoutes = append(failedRoutes, route)
			continue
		}
		bundles := uniqueStrings(matchGroup(scriptSrcRe, html, 1))
		for _, b := range bundles {
			js, e := c.fetchText(b)
			if e != nil {
				continue
			}
			mergeHashes(found, js, known)
		}
	}
	if len(found) == 0 {
		if lastErr != nil {
			return nil, failedRoutes, fmt.Errorf("harvest found no operations: %w", lastErr)
		}
		return nil, failedRoutes, fmt.Errorf("harvest found no operations")
	}
	return found, failedRoutes, nil
}

// knownOperationNames is the union of bundled operations and any refreshed
// overrides — the set whose hashes a refresh should update by exact name.
func knownOperationNames() map[string]struct{} {
	names := make(map[string]struct{}, len(defaultHashes))
	for k := range defaultHashes {
		names[k] = struct{}{}
	}
	r := LoadRegistry()
	for k := range r.overrides {
		names[k] = struct{}{}
	}
	return names
}

// mergeHashes scans one JS bundle body. The name-first pass locates each known
// operation name and pairs it with the nearest 64-hex hash within 400 chars
// (Airbnb's minified modules place the operationId near the operationName). The
// discovery pass pairs any *Query/*Mutation/*Subscription name with a hash in a
// 260-char window.
func mergeHashes(dst map[string]string, js string, known map[string]struct{}) {
	// Name-first pass for known operations (handles non-suffixed names).
	for name := range known {
		if _, exists := dst[name]; exists {
			continue
		}
		if h := nearestHash(js, `"`+name+`"`, 400); h != "" {
			dst[name] = h
		}
	}
	// Discovery pass for suffixed operations near each hash.
	for _, loc := range hash64Re.FindAllStringIndex(js, -1) {
		hash := js[loc[0]:loc[1]]
		start := loc[0] - 260
		if start < 0 {
			start = 0
		}
		end := loc[1] + 260
		if end > len(js) {
			end = len(js)
		}
		for _, m := range opNameRe.FindAllStringSubmatch(js[start:end], -1) {
			if _, exists := dst[m[1]]; !exists {
				dst[m[1]] = hash
			}
		}
	}
}

// nearestHash finds needle in js and returns the closest 64-hex hash within
// radius chars on either side, or "" if none.
func nearestHash(js, needle string, radius int) string {
	from := 0
	for {
		i := strings.Index(js[from:], needle)
		if i < 0 {
			return ""
		}
		i += from
		start := i - radius
		if start < 0 {
			start = 0
		}
		end := i + len(needle) + radius
		if end > len(js) {
			end = len(js)
		}
		if m := hash64Re.FindString(js[start:end]); m != "" {
			return m
		}
		from = i + len(needle)
	}
}

// SaveOverrides merges harvested pairs into the override file, keeping existing
// entries not present in the new set. Returns the number of operations written.
func SaveOverrides(harvested map[string]string) (int, error) {
	r := LoadRegistry()
	merged := map[string]string{}
	for k, v := range r.overrides {
		merged[k] = v
	}
	for k, v := range harvested {
		merged[k] = v
	}
	path := registryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return 0, err
	}
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return 0, err
	}
	return len(merged), nil
}

// DefaultHarvestRoutes are the routes whose bundles carry the operations this
// CLI uses. Search + PDP hashes live on /s/ pages; messaging on /guest/messages;
// wishlists on /wishlists; account/header on any logged-in page.
var DefaultHarvestRoutes = []string{
	"/s/Berlin--Germany/homes",
	"/guest/messages",
	"/wishlists",
	"/trips/v1",
}

func matchGroup(re *regexp.Regexp, s string, group int) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		if len(m) > group {
			out = append(out, m[group])
		}
	}
	return out
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
