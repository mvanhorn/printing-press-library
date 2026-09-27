package tenki

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/cliutil"
)

var JST = time.FixedZone(Timezone, 9*60*60)

const maxBodyBytes = 4 << 20
const forecastTTL = time.Hour
const catalogTTL = 7 * 24 * time.Hour

type RateLimitError = cliutil.RateLimitError

type Client struct {
	cfg     Config
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
	mu      sync.Mutex
	metrics Metrics
	mem     map[string]cacheEntry
}

type cacheEntry struct {
	URL       string    `json:"url"`
	FetchedAt time.Time `json:"fetched_at"`
	Body      string    `json:"body"`
}

func NewClient(cfg Config) *Client {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 25 * time.Second
	}
	rate := cfg.RateLimit
	if rate <= 0 || rate > 1 {
		rate = 1
	}
	transport := cfg.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	c := &Client{cfg: cfg, limiter: cliutil.NewAdaptiveLimiter(rate), mem: map[string]cacheEntry{}}
	c.http = &http.Client{Transport: countingTransport{client: c, base: transport}, Timeout: cfg.Timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("tenki.jp redirect limit exceeded")
		}
		_, err := validateURL(req.URL.String())
		return err
	}}
	return c
}

type countingTransport struct {
	client *Client
	base   http.RoundTripper
}

func (t countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if _, err := validateURL(req.URL.String()); err != nil {
		return nil, err
	}
	if err := t.client.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	t.client.mu.Lock()
	t.client.metrics.HTTPRequests++
	t.client.mu.Unlock()
	return t.base.RoundTrip(req)
}

func (c *Client) Metrics() Metrics { c.mu.Lock(); defer c.mu.Unlock(); return c.metrics }

var municipalityPath = regexp.MustCompile(`^/forecast/([0-9]+)/([0-9]+)/([0-9]+)/([0-9]+)/(?:1hour\.html|10days\.html)?$`)
var detailPath = regexp.MustCompile(`^(?:/leisure/(?:[0-9]+/){4}|/(?:mountain/(?:famous100|normal)|sakura|kouyou)/(?:[0-9]+/){2}[0-9]+\.html)$`)
var directoryPath = regexp.MustCompile(`^/(?:mountain/|leisure/(?:[0-9]+/){0,3}|(?:sakura|kouyou)/(?:[0-9]+/){0,2}|(?:leisure/)?search/|(?:sakura|kouyou)/search/(?:bloom/)?)$`)

func validateURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host != "tenki.jp" || u.User != nil || u.Fragment != "" || u.RawPath != "" {
		return "", fmt.Errorf("target must be a canonical https://tenki.jp product URL")
	}
	if !municipalityPath.MatchString(u.Path) && !detailPath.MatchString(u.Path) && !directoryPath.MatchString(u.Path) {
		return "", fmt.Errorf("unsupported tenki.jp product path: %s", u.Path)
	}
	if u.RawQuery != "" {
		if !strings.Contains(u.Path, "/search/") {
			return "", errors.New("query parameters are supported only on search pages")
		}
		for key := range u.Query() {
			if key != "keyword" && key != "page" && key != "p" && key != "search_type" {
				return "", fmt.Errorf("unsupported search parameter %q", key)
			}
		}
		u.RawQuery = u.Query().Encode()
	}
	return u.String(), nil
}

func canonicalPlaceURL(raw string) (string, error) {
	u, err := validateURL(raw)
	if err != nil {
		return "", err
	}
	p, _ := url.Parse(u)
	if municipalityPath.MatchString(p.Path) {
		p.Path = strings.TrimSuffix(strings.TrimSuffix(p.Path, "1hour.html"), "10days.html")
	}
	if !municipalityPath.MatchString(p.Path) && !detailPath.MatchString(p.Path) {
		return "", errors.New("URL identifies a directory, not a place")
	}
	return p.String(), nil
}

func absoluteURL(href string) string {
	base, _ := url.Parse("https://tenki.jp/")
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	u = base.ResolveReference(u)
	u.Fragment = ""
	valid, err := validateURL(u.String())
	if err != nil {
		return ""
	}
	return valid
}

func (c *Client) cachePath(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return filepath.Join(c.cfg.CacheDir, hex.EncodeToString(h[:])+".json")
}

func (c *Client) source(entry cacheEntry, ttl time.Duration, cached bool) Source {
	now := c.cfg.Now().In(JST)
	return Source{URL: entry.URL, FetchedAt: stamp(entry.FetchedAt), ExpiresAt: stamp(entry.FetchedAt.Add(ttl)), Timezone: Timezone,
		FromCache: cached, CacheStale: !now.Before(entry.FetchedAt.Add(ttl)) || entry.FetchedAt.After(now), Freshness: "unknown"}
}

func (c *Client) readCache(raw string) (cacheEntry, bool) {
	if c.cfg.CacheDir == "" {
		return cacheEntry{}, false
	}
	f, err := os.Open(c.cachePath(raw))
	if err != nil {
		return cacheEntry{}, false
	}
	defer f.Close()
	var entry cacheEntry
	if json.NewDecoder(io.LimitReader(f, 6*maxBodyBytes+4096)).Decode(&entry) != nil || entry.URL != raw || entry.FetchedAt.IsZero() || len(entry.Body) > maxBodyBytes || entry.FetchedAt.After(c.cfg.Now()) {
		return cacheEntry{}, false
	}
	return entry, true
}

func (c *Client) writeCache(entry cacheEntry) error {
	if c.cfg.CacheDir == "" {
		return nil
	}
	if err := os.MkdirAll(c.cfg.CacheDir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(c.cfg.CacheDir, ".tenki-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	if err = enc.Encode(entry); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, c.cachePath(entry.URL))
}

func (c *Client) fetch(ctx context.Context, raw string, ttl time.Duration) (string, Source, error) {
	raw, err := validateURL(raw)
	if err != nil {
		return "", Source{}, err
	}
	c.mu.Lock()
	mem, inMem := c.mem[raw]
	c.mu.Unlock()
	entry, hasCache := mem, inMem
	if hasCache && entry.FetchedAt.After(c.cfg.Now()) {
		entry, hasCache = cacheEntry{}, false
	}
	if !hasCache && !c.cfg.Refresh {
		entry, hasCache = c.readCache(raw)
	}
	now := c.cfg.Now()
	cacheFresh := hasCache && now.Before(entry.FetchedAt.Add(ttl)) && !entry.FetchedAt.After(now.Add(time.Minute))
	if hasCache && (cacheFresh || (c.cfg.Local && c.cfg.AllowStale)) {
		c.mu.Lock()
		c.metrics.CacheHits++
		c.mem[raw] = entry
		c.mu.Unlock()
		return entry.Body, c.source(entry, ttl, true), nil
	}
	if c.cfg.Local {
		if hasCache {
			return "", Source{}, fmt.Errorf("cached source expired; use --allow-stale explicitly: %s", raw)
		}
		return "", Source{}, fmt.Errorf("no cached tenki.jp source: %s", raw)
	}
	// Refresh bypasses disk cache. A stale fallback is permitted only by the
	// explicit AllowStale policy, including when the refresh request fails.
	if !hasCache && c.cfg.AllowStale {
		entry, hasCache = c.readCache(raw)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", Source{}, err
	}
	req.Header.Set("User-Agent", "tenki-pp-cli/0.0.0 (+bounded public HTML reader)")
	req.Header.Set("Accept", "text/html")
	resp, err := c.http.Do(req)
	if err != nil {
		return c.fallback(entry, hasCache, ttl, err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	c.mu.Lock()
	c.metrics.ResponseBytes += int64(len(body))
	c.mu.Unlock()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return "", Source{}, &RateLimitError{URL: raw, RetryAfter: cliutil.RetryAfter(resp), Body: boundedText(string(body), 200)}
	}
	if resp.StatusCode != http.StatusOK {
		return c.fallback(entry, hasCache, ttl, fmt.Errorf("tenki.jp HTTP %d for %s: %s", resp.StatusCode, raw, boundedText(string(body), 160)))
	}
	if readErr != nil {
		return c.fallback(entry, hasCache, ttl, readErr)
	}
	if len(body) > maxBodyBytes {
		return "", Source{}, errors.New("tenki.jp HTML exceeded 4 MiB body limit")
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "text/html") && !strings.Contains(ct, "application/xhtml") {
		return "", Source{}, fmt.Errorf("unexpected source content type: %s", ct)
	}
	c.limiter.OnSuccess()
	entry = cacheEntry{URL: raw, Body: string(body), FetchedAt: c.cfg.Now()}
	if err = c.writeCache(entry); err != nil {
		return "", Source{}, fmt.Errorf("write tenki cache: %w", err)
	}
	c.mu.Lock()
	c.mem[raw] = entry
	c.mu.Unlock()
	return entry.Body, c.source(entry, ttl, false), nil
}

func (c *Client) fallback(entry cacheEntry, exists bool, ttl time.Duration, err error) (string, Source, error) {
	if exists && c.cfg.AllowStale {
		c.mu.Lock()
		c.metrics.CacheHits++
		c.mem[entry.URL] = entry
		c.mu.Unlock()
		source := c.source(entry, ttl, true)
		source.CacheStale = true
		source.Freshness = "stale"
		return entry.Body, source, nil
	}
	return "", Source{}, err
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(JST).Format(time.RFC3339)
}

func applyIssue(source *Source, issue time.Time, raw string, age time.Duration, now time.Time) {
	source.IssueAt, source.IssueRaw = stamp(issue), raw
	source.FreshnessReference, source.FreshnessAt = "issue_at", stamp(issue)
	applyFreshness(source, issue, age, now)
}

func applyFreshness(source *Source, issue time.Time, age time.Duration, now time.Time) {
	if issue.IsZero() {
		source.Freshness = "unknown"
		return
	}
	source.SourceStale = now.Sub(issue) > age || issue.After(now.Add(time.Minute))
	if source.SourceStale || source.CacheStale {
		source.Freshness = "stale"
	} else {
		source.Freshness = "fresh"
	}
}

func sourceWarnings(source Source) []string {
	warnings := []string{}
	if source.Freshness == "unknown" {
		warnings = append(warnings, "Source issue/report/initialization time is unknown; freshness cannot be established.")
	}
	if source.SourceStale {
		warnings = append(warnings, "Source is stale under the CLI age policy; this policy is not a provider update guarantee.")
	}
	if source.CacheStale {
		warnings = append(warnings, "Explicitly selected stale cache; refresh did not establish fresh evidence.")
	}
	return warnings
}

func boundedText(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > limit {
		return string(r[:limit]) + "…"
	}
	return s
}
