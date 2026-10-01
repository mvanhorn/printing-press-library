// Package evidence normalizes public first-party Weathernews travel evidence.
package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/weathernews/internal/cliutil"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxBody = 4 << 20

var JST = time.FixedZone("JST", 9*3600)

func stamp(t time.Time) string { return t.In(JST).Format(time.RFC3339) }

type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string                { return e.Message }
func fail(code int, f string, a ...any) error { return &Error{code, fmt.Sprintf(f, a...)} }

type Fetch struct {
	URL             string `json:"url"`
	RetrievedAt     string `json:"retrieved_at"`
	CacheHit        bool   `json:"cache_hit"`
	CacheAgeSeconds int    `json:"cache_age_seconds"`
	Bytes           int    `json:"bytes"`
	Body            []byte `json:"-"`
}
type Metrics struct {
	Requests      int   `json:"requests"`
	CacheHits     int   `json:"cache_hits"`
	DownloadBytes int   `json:"download_bytes"`
	LatencyMS     int64 `json:"latency_ms"`
}
type Client struct {
	CacheDir                  string
	Refresh, NoCache, Offline bool
	Timeout                   time.Duration
	HTTP                      *http.Client
	Metrics                   Metrics
	Started                   time.Time
	seen                      map[string]Fetch
	Limiter                   *cliutil.AdaptiveLimiter
}
type cacheEntry struct {
	URL  string    `json:"url"`
	At   time.Time `json:"at"`
	Body []byte    `json:"body"`
}

func NewClient(dir string, timeout time.Duration) *Client {
	if timeout > 20*time.Second {
		timeout = 20 * time.Second
	}
	return &Client{CacheDir: dir, Timeout: timeout, Started: time.Now(), seen: map[string]Fetch{}, HTTP: &http.Client{Timeout: timeout, CheckRedirect: func(r *http.Request, v []*http.Request) error {
		if len(v) > 3 {
			return fmt.Errorf("too many redirects")
		}
		return allowed(r.URL)
	}}}
}
func allowed(u *url.URL) error {
	if u.Scheme != "https" || (u.Host != "weathernews.jp" && u.Host != "site.weathernews.jp") || u.User != nil {
		return fail(2, "only public first-party Weathernews HTTPS origins are supported")
	}
	return nil
}
func (c *Client) Get(ctx context.Context, raw string, ttl time.Duration) (Fetch, error) {
	u, e := url.Parse(raw)
	if e != nil {
		return Fetch{}, fail(2, "invalid source URL")
	}
	if e = allowed(u); e != nil {
		return Fetch{}, e
	}
	if c.Timeout <= 0 {
		return Fetch{}, fail(2, "--timeout must be positive")
	}
	if v, ok := c.seen[raw]; ok {
		return v, nil
	}
	p := cacheFile(c.CacheDir, raw)
	if c.CacheDir != "" && !c.Refresh && !c.NoCache {
		if b, e := readCache(p); e == nil {
			var v cacheEntry
			if json.Unmarshal(b, &v) == nil && v.URL == raw && len(v.Body) <= maxBody && time.Since(v.At) >= 0 && time.Since(v.At) < ttl {
				f := Fetch{raw, stamp(v.At), true, int(time.Since(v.At).Seconds()), len(v.Body), v.Body}
				c.Metrics.CacheHits++
				c.seen[raw] = f
				return f, nil
			}
		}
	}
	if c.Offline {
		return Fetch{}, fail(3, "no fresh cached source for %s; refresh online first", u.Path)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(350 * time.Millisecond):
			case <-ctx.Done():
				return Fetch{}, ctx.Err()
			}
		}
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if e != nil {
			return Fetch{}, e
		}
		req.Header.Set("User-Agent", "weathernews-pp-cli/0.1 (public read-only weather evidence)")
		req.Header.Set("Accept", "application/json,text/html;q=0.9")
		if e = c.Limiter.Wait(ctx); e != nil {
			return Fetch{}, e
		}
		c.Metrics.Requests++
		resp, e := c.HTTP.Do(req)
		if e != nil {
			if ctx.Err() != nil {
				return Fetch{}, ctx.Err()
			}
			if attempt == 0 {
				continue
			}
			return Fetch{}, fail(5, "Weathernews request failed: %v", e)
		}
		b, re := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
		_ = resp.Body.Close() // Response has been fully read; closing error cannot change parsed evidence.
		c.Metrics.DownloadBytes += len(b)
		if resp.StatusCode == 429 {
			c.Limiter.OnRateLimit()
			wait := time.Duration(0)
			if n, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && n > 0 {
				wait = time.Duration(n) * time.Second
			}
			if wait > 2*time.Second || attempt == 1 {
				return Fetch{}, &cliutil.RateLimitError{URL: raw, RetryAfter: wait}
			}
			if wait > 0 {
				select {
				case <-time.After(wait):
				case <-ctx.Done():
					return Fetch{}, ctx.Err()
				}
			}
			continue
		}
		if resp.StatusCode >= 500 && attempt == 0 {
			continue
		}
		if resp.StatusCode == 404 {
			return Fetch{}, fail(3, "Weathernews source not found: %s", u.Path)
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return Fetch{}, fail(4, "Weathernews access restricted (HTTP %d); this CLI uses public coverage only", resp.StatusCode)
		}
		if resp.StatusCode != 200 {
			return Fetch{}, fail(5, "Weathernews HTTP %d for %s", resp.StatusCode, u.Path)
		}
		if re != nil || len(b) > maxBody {
			return Fetch{}, fail(5, "Weathernews response exceeds bounded size or is incomplete")
		}
		c.Limiter.OnSuccess()
		at := time.Now()
		f := Fetch{raw, stamp(at), false, 0, len(b), b}
		c.seen[raw] = f
		if !c.NoCache && c.CacheDir != "" {
			c.writeCache(cacheEntry{raw, at, b})
		}
		return f, nil
	}
	return Fetch{}, fail(5, "Weathernews retry budget exhausted")
}
func (c *Client) writeCache(v cacheEntry) {
	if c.CacheDir == "" {
		return
	}
	dir := filepath.Join(c.CacheDir, cacheNamespace)
	p := cacheFile(c.CacheDir, v.URL)
	if os.MkdirAll(dir, 0700) != nil {
		return
	}
	b, e := json.Marshal(v)
	if e != nil {
		return
	}
	f, e := os.CreateTemp(dir, ".cache-*")
	if e != nil {
		return
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e != nil {
		_ = f.Close() // Preserve the primary write error.
		return
	}
	if f.Close() != nil {
		return
	}
	if os.Rename(tmp, p) != nil {
		return
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return
	}
	type entry struct {
		path string
		at   time.Time
	}
	var list []entry
	var total int64
	for _, x := range entries {
		name := x.Name()
		if !x.Type().IsRegular() || !ownedCacheName(name) {
			continue
		}
		if i, e := x.Info(); e == nil {
			list = append(list, entry{filepath.Join(dir, name), i.ModTime()})
			total += i.Size()
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].at.Before(list[j].at) })
	for i, x := range list {
		if len(list)-i <= 128 && total <= 64<<20 {
			break
		}
		if v, e := os.Stat(x.path); e == nil {
			if os.Remove(x.path) == nil {
				total -= v.Size()
			}
		}
	}
}
func (c *Client) Stats() Metrics {
	m := c.Metrics
	m.LatencyMS = time.Since(c.Started).Milliseconds()
	return m
}
func provenance(f Fetch) map[string]any {
	return map[string]any{"source_url": f.URL, "retrieved_at": f.RetrievedAt, "cache_hit": f.CacheHit, "cache_age_seconds": f.CacheAgeSeconds}
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%g", x)
	default:
		return ""
	}
}
func text(v any) any {
	s := strings.TrimSpace(str(v))
	if s == "" {
		return nil
	}
	return s
}
func num(v any) *float64 {
	var n float64
	switch x := v.(type) {
	case float64:
		n = x
	case int:
		n = float64(x)
	case int64:
		n = float64(x)
	case string:
		if x == "" {
			return nil
		}
		var e error
		n, e = strconv.ParseFloat(x, 64)
		if e != nil {
			return nil
		}
	default:
		return nil
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || n < -9000 {
		return nil
	}
	return &n
}
func number(v any) any {
	if n := num(v); n != nil {
		return *n
	}
	return nil
}
func first(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if text(m[k]) != nil {
			return m[k]
		}
	}
	return nil
}

const (
	cacheNamespace  = "weathernews"
	cacheNamePrefix = "wn-"
)

// cacheFile is the only path this package writes. Entries live under
// CacheDir/weathernews and use a wn- prefix so eviction can ignore every
// other file in a shared --cache-dir.
func cacheFile(dir, raw string) string {
	digest := sha256.Sum256([]byte(raw))
	name := cacheNamePrefix + hex.EncodeToString(digest[:]) + ".json"
	return filepath.Join(dir, cacheNamespace, name)
}

func ownedCacheName(name string) bool {
	rest, ok := strings.CutPrefix(name, cacheNamePrefix)
	if !ok || !strings.HasSuffix(rest, ".json") {
		return false
	}
	hexPart := strings.TrimSuffix(rest, ".json")
	if len(hexPart) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(hexPart)
	return err == nil
}

func readCache(p string) ([]byte, error) {
	f, e := os.Open(p) // #nosec G304 -- p is a SHA256 URL filename under the caller-selected CLI cache root.
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 6<<20+1))
	if e != nil {
		return nil, e
	}
	if len(b) > 6<<20 {
		return nil, fmt.Errorf("oversized cache entry")
	}
	return b, nil
}

func providerTime(v any) any {
	if text(v) == nil {
		return nil
	}
	t, e := time.Parse(time.RFC3339Nano, str(v))
	if e != nil {
		return nil
	}
	return stamp(t)
}
