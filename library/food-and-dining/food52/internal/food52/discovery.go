// HTTP requests in this file go through doWithLimiter (see ratelimit.go),
// which paces calls with cliutil.AdaptiveLimiter and surfaces a typed
// *cliutil.RateLimitError when 429 retries are exhausted. Empty-on-throttle
// is indistinguishable from "no data exists" — callers must treat the
// limiter's typed error as a hard failure rather than swallow it.
package food52

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Discovery holds the runtime-discovered values that depend on Food52's
// current deploy: the Next.js buildId and the public Typesense host +
// search-only key extracted from the active _app.js bundle.
//
// Both are public (the buildId is in every page's __NEXT_DATA__; the
// Typesense key is in the public JS bundle as searchOnlyApiKey by Typesense's
// intended design — like an Algolia public app key or a Stripe publishable
// key). We never write either value into source, the spec, the discovery
// report, or any committed artifact.
type Discovery struct {
	BuildID         string
	TypesenseHost   string
	TypesenseAPIKey string
	DiscoveredAt    time.Time
}

// CachedDiscovery is the on-disk shape under
// $XDG_CACHE_HOME/food52-pp-cli/discovery.json. The cache prevents hitting
// the homepage + bundle on every invocation; we refresh on TTL or on a 404
// from the Next.js JSON endpoint or a 401/403 from Typesense.
type CachedDiscovery struct {
	BuildID         string    `json:"build_id"`
	TypesenseHost   string    `json:"typesense_host"`
	TypesenseAPIKey string    `json:"typesense_api_key"`
	DiscoveredAt    time.Time `json:"discovered_at"`
}

const (
	discoveryTTL      = 6 * time.Hour
	discoveryCacheDir = "food52-pp-cli"
	discoveryFileName = "discovery.json"
)

// HTTPClient is the small subset of http.Client the discovery helper needs.
// The CLI passes the same Surf-built client used everywhere else so the
// bundle fetch goes through the Vercel-clearing transport.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var (
	discoveryMu       sync.Mutex
	discoveryInMemory *Discovery
)

// LoadDiscovery returns the discovery cache entry, refreshing it from the
// live site when stale or absent. The Refresh path is a single in-process
// section so concurrent goroutines don't double-fetch.
func LoadDiscovery(httpc HTTPClient) (*Discovery, error) {
	discoveryMu.Lock()
	defer discoveryMu.Unlock()

	if discoveryInMemory != nil && time.Since(discoveryInMemory.DiscoveredAt) < discoveryTTL {
		return discoveryInMemory, nil
	}
	if d, ok := readDiscoveryCache(); ok && time.Since(d.DiscoveredAt) < discoveryTTL {
		discoveryInMemory = d
		return d, nil
	}
	d, err := refreshDiscovery(httpc)
	if err != nil {
		return nil, err
	}
	discoveryInMemory = d
	writeDiscoveryCache(d)
	return d, nil
}

// InvalidateDiscovery clears the cached buildId/key. Commands call this when
// they get a 404 from /_next/data or a 401/403 from Typesense — the next
// LoadDiscovery() will refetch.
func InvalidateDiscovery() {
	discoveryMu.Lock()
	defer discoveryMu.Unlock()
	discoveryInMemory = nil
	if path, err := discoveryCachePath(); err == nil {
		_ = os.Remove(path)
	}
}

func refreshDiscovery(httpc HTTPClient) (*Discovery, error) {
	html, err := getHTML(httpc, "https://food52.com/")
	if err != nil {
		return nil, fmt.Errorf("food52 discovery: fetching homepage: %w", err)
	}
	if LooksLikeChallenge(html) {
		return nil, fmt.Errorf("food52 discovery: homepage returned a Vercel challenge page — the CLI's transport is not clearing it (rebuild the binary, or run `doctor`)")
	}
	nd, err := ExtractNextData(html)
	if err != nil {
		return nil, fmt.Errorf("food52 discovery: %w", err)
	}
	buildID := BuildID(nd)
	if buildID == "" {
		return nil, fmt.Errorf("food52 discovery: __NEXT_DATA__.buildId missing — Food52 changed the SSR shape")
	}

	bundleURL, err := findAppBundleURL(html)
	if err != nil {
		return nil, fmt.Errorf("food52 discovery: %w", err)
	}
	bundle, err := getHTML(httpc, bundleURL)
	if err != nil {
		return nil, fmt.Errorf("food52 discovery: fetching app bundle %s: %w", bundleURL, err)
	}
	host, key, err := extractTypesenseConfig(bundle)
	if err != nil {
		return nil, fmt.Errorf("food52 discovery: %w", err)
	}

	return &Discovery{
		BuildID:         buildID,
		TypesenseHost:   host,
		TypesenseAPIKey: key,
		DiscoveredAt:    time.Now().UTC(),
	}, nil
}

// findAppBundleURL locates the active _app-<hash>.js script tag on the page.
// Food52's deploy rotates the hash; we never hardcode it.
var appBundleRe = regexp.MustCompile(`(/_next/static/chunks/pages/_app-[A-Za-z0-9_-]+\.js[^"']*)`)

func findAppBundleURL(html []byte) (string, error) {
	m := appBundleRe.FindSubmatch(html)
	if len(m) < 2 {
		return "", fmt.Errorf("could not locate _app-*.js script tag (Next.js layout changed?)")
	}
	return "https://food52.com" + string(m[1]), nil
}

// typesenseRe matches the JS literal Food52 emits in the bundle:
//
//	typesense:{host:"<cluster-id>-1.a1.typesense.net",searchOnlyApiKey:"<key>"}
var typesenseRe = regexp.MustCompile(`typesense\s*:\s*\{\s*host\s*:\s*"([^"]+)"\s*,\s*searchOnlyApiKey\s*:\s*"([^"]+)"`)

func extractTypesenseConfig(bundle []byte) (host, key string, err error) {
	m := typesenseRe.FindSubmatch(bundle)
	if len(m) < 3 {
		return "", "", fmt.Errorf("typesense config not found in app bundle (Food52 changed config shape)")
	}
	return string(m[1]), string(m[2]), nil
}

func getHTML(httpc HTTPClient, url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, body, err := doWithLimiter(httpc, req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	const maxResponseBytes = 8 * 1024 * 1024 // 8 MB ceiling for any single page or bundle
	if len(body) > maxResponseBytes {
		body = body[:maxResponseBytes]
	}
	return body, nil
}

func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 32*1024)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" || err.Error() == "io: EOF" {
				return buf, nil
			}
			// http.MaxBytesReader returns *http.MaxBytesError when the cap is hit.
			if strings.Contains(err.Error(), "http: request body too large") {
				return nil, fmt.Errorf("response exceeded 8 MB cap")
			}
			// Treat any normal close as success.
			if strings.Contains(err.Error(), "EOF") {
				return buf, nil
			}
			return buf, err
		}
	}
}

func discoveryCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, discoveryCacheDir, discoveryFileName), nil
}

func readDiscoveryCache() (*Discovery, bool) {
	path, err := discoveryCachePath()
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var c CachedDiscovery
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, false
	}
	if c.BuildID == "" || c.TypesenseHost == "" || c.TypesenseAPIKey == "" {
		return nil, false
	}
	return &Discovery{
		BuildID:         c.BuildID,
		TypesenseHost:   c.TypesenseHost,
		TypesenseAPIKey: c.TypesenseAPIKey,
		DiscoveredAt:    c.DiscoveredAt,
	}, true
}

func writeDiscoveryCache(d *Discovery) {
	path, err := discoveryCachePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	c := CachedDiscovery{
		BuildID:         d.BuildID,
		TypesenseHost:   d.TypesenseHost,
		TypesenseAPIKey: d.TypesenseAPIKey,
		DiscoveredAt:    d.DiscoveredAt,
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}
