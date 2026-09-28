package walkerplus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/cliutil"
)

// pp:data-source live
const maxBody = 4 << 20

var ErrNotFound = errors.New("Walkerplus event or route not found")

// ErrInvalidQuery identifies an input absent from a successfully loaded catalog.
var ErrInvalidQuery = errors.New("invalid Walkerplus query")

type StatusError struct {
	URL        string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("Walkerplus HTTP %d for %s", e.StatusCode, e.URL)
}
func (e *StatusError) Unwrap() error {
	if e.StatusCode == 404 {
		return ErrNotFound
	}
	return nil
}

type Client struct {
	opts    Options
	base    *url.URL
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
	mu      sync.Mutex
	cacheMu sync.Mutex
	stats   Coverage
}

type operation struct {
	started  time.Time
	mu       sync.Mutex
	coverage Coverage
}

func newOperation() *operation { return &operation{started: time.Now(), coverage: emptyCoverage()} }
func emptyCoverage() Coverage {
	return Coverage{Reasons: []string{}, Routes: []string{}, CatalogRoutes: []string{}, NativeYearLabels: []string{}}
}
func (c *Client) finish(op *operation) Coverage {
	op.mu.Lock()
	defer op.mu.Unlock()
	op.coverage.ElapsedMS = time.Since(op.started).Milliseconds()
	c.mu.Lock()
	c.stats = op.coverage
	c.mu.Unlock()
	return op.coverage
}

// Stats returns the completed operation's bounded request/coverage accounting.
func (c *Client) Stats() Coverage { c.mu.Lock(); defer c.mu.Unlock(); return c.stats }

func NewClient(opts Options) (*Client, error) {
	if opts.BaseURL == "" {
		opts.BaseURL = "https://www.walkerplus.com"
	}
	base, err := url.Parse(opts.BaseURL)
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || (base.Path != "" && base.Path != "/") {
		return nil, fmt.Errorf("invalid source base URL")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 15 * time.Second
	}
	if opts.Timeout < 0 || opts.Timeout > 15*time.Second {
		return nil, fmt.Errorf("request timeout must be positive and at most 15s")
	}
	if opts.Concurrency == 0 {
		opts.Concurrency = 2
	}
	if opts.Concurrency < 1 || opts.Concurrency > 4 || opts.Retries < 0 || opts.Retries > 3 {
		return nil, fmt.Errorf("concurrency must be 1..4 and retries 0..3")
	}
	if opts.CacheTTL == 0 {
		opts.CacheTTL = time.Hour
	}
	if opts.CacheTTL < 0 || opts.CacheTTL > 24*time.Hour {
		return nil, fmt.Errorf("cache TTL must be 0..24h (0 uses the 1h default); use no-cache to disable")
	}
	if opts.CacheDir == "" && !opts.NoCache {
		dir, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		opts.CacheDir = filepath.Join(dir, "walkerplus-pp-cli", "html")
	}
	hc := &http.Client{}
	if opts.HTTPClient != nil {
		copy := *opts.HTTPClient
		hc = &copy
	}
	hc.Timeout = opts.Timeout
	hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || req.URL.Host != base.Host || req.URL.Scheme != base.Scheme {
			return fmt.Errorf("redirect outside Walkerplus source refused")
		}
		return nil
	}
	return &Client{opts: opts, base: base, http: hc, limiter: cliutil.NewAdaptiveLimiter(2), stats: emptyCoverage()}, nil
}

type cachedPage struct {
	URL       string    `json:"url"`
	FetchedAt time.Time `json:"fetched_at"`
	Body      []byte    `json:"body"`
}
type page struct {
	body   []byte
	source Source
}

type cacheEntry struct {
	name string
	size int64
	mod  time.Time
}

var ownedCacheFilenameRE = regexp.MustCompile("^wp-[0-9a-f]{64}\\.json$")

// A failed or concurrently missing removal must not exhaust a slice and panic.
func evictEntries(files []cacheEntry, size int64, remove func(string) error) (int, int64) {
	remaining := len(files)
	for len(files) > 0 && (remaining > 128 || size > 32<<20) {
		f := files[0]
		files = files[1:]
		if err := remove(f.name); err == nil || os.IsNotExist(err) {
			size -= f.size
			remaining--
		}
	}
	return remaining, size
}

func (c *Client) absolute(path string) string { return strings.TrimRight(c.base.String(), "/") + path }
func (c *Client) cachePath(u string) string {
	sum := sha256.Sum256([]byte(u))
	return filepath.Join(c.opts.CacheDir, "wp-"+hex.EncodeToString(sum[:])+".json")
}

func (c *Client) readCache(u string) (page, bool) {
	if c.opts.NoCache || c.opts.Refresh {
		return page{}, false
	}
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	f, err := os.Open(c.cachePath(u))
	if err != nil {
		return page{}, false
	}
	defer f.Close()
	var cp cachedPage
	if json.NewDecoder(io.LimitReader(f, 6<<20)).Decode(&cp) != nil || cp.URL != u || len(cp.Body) > maxBody || cp.FetchedAt.IsZero() {
		return page{}, false
	}
	age := time.Since(cp.FetchedAt)
	if age < 0 || age > c.opts.CacheTTL {
		return page{}, false
	}
	return page{body: cp.Body, source: Source{URL: u, FetchedAt: cp.FetchedAt.UTC().Format(time.RFC3339), CacheHit: true, CacheAgeSeconds: int64(age.Seconds())}}, true
}

func (c *Client) writeCache(u string, p page) {
	if c.opts.NoCache {
		return
	}
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if os.MkdirAll(c.opts.CacheDir, 0700) != nil {
		return
	}
	fetched, _ := time.Parse(time.RFC3339, p.source.FetchedAt)
	cp := cachedPage{URL: u, FetchedAt: fetched, Body: p.body}
	b, err := json.Marshal(cp)
	if err != nil {
		return
	}
	f, err := os.CreateTemp(c.opts.CacheDir, "wp-tmp-")
	if err != nil {
		return
	}
	name := f.Name()
	defer os.Remove(name)
	if f.Chmod(0600) != nil {
		_ = f.Close() // The permission failure already abandons this cache entry.
		return
	}
	if _, err = f.Write(b); err != nil {
		_ = f.Close() // The write failure already abandons this cache entry.
		return
	}
	if f.Close() != nil {
		return
	}
	if os.Rename(name, c.cachePath(u)) != nil {
		return
	}
	entries, err := os.ReadDir(c.opts.CacheDir)
	if err != nil {
		return
	}
	files := []cacheEntry{}
	var size int64
	for _, e := range entries {
		if !ownedCacheFilenameRE.MatchString(e.Name()) || !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err == nil && info.Mode().IsRegular() {
			files = append(files, cacheEntry{e.Name(), info.Size(), info.ModTime()})
			size += info.Size()
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	remaining, size := evictEntries(files, size, func(name string) error { return os.Remove(filepath.Join(c.opts.CacheDir, name)) })
	// Failed eviction must not retain this operation's new entry beyond the cap.
	if remaining > 128 || size > 32<<20 {
		_ = os.Remove(c.cachePath(u))
	}
}

func waitContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryableTransport(err error) bool {
	var networkErr net.Error
	return (errors.As(err, &networkErr) && (networkErr.Timeout() || networkErr.Temporary())) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

func (c *Client) fetch(ctx context.Context, path string, op *operation) (page, error) {
	u := c.absolute(path)
	if p, ok := c.readCache(u); ok {
		op.mu.Lock()
		op.coverage.CacheHits++
		op.mu.Unlock()
		return p, nil
	}
	var last error
	for attempt := 0; attempt <= c.opts.Retries; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return page{}, err
		}
		requestCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, u, nil)
		if err != nil {
			cancel()
			return page{}, err
		}
		req.Header.Set("User-Agent", "walkerplus-pp-cli/1.0 (bounded public event discovery)")
		req.Header.Set("Accept", "text/html")
		op.mu.Lock()
		op.coverage.RequestCount++
		op.mu.Unlock()
		resp, err := c.http.Do(req)
		if err != nil {
			cancel()
			last = fmt.Errorf("fetch %s: %w", u, err)
			if ctx.Err() != nil {
				return page{}, ctx.Err()
			}
			if attempt < c.opts.Retries && retryableTransport(err) {
				if err = waitContext(ctx, time.Duration(attempt+1)*100*time.Millisecond); err != nil {
					return page{}, err
				}
				continue
			}
			return page{}, last
		}
		status := resp.StatusCode
		if status == 429 {
			c.limiter.OnRateLimit()
			retry := cliutil.RetryAfter(resp)
			_ = resp.Body.Close() // Preserve the actionable rate-limit error.
			cancel()
			last = &cliutil.RateLimitError{URL: u, RetryAfter: retry}
			if attempt == c.opts.Retries || retry > 10*time.Second {
				return page{}, last
			}
			if retry < 100*time.Millisecond {
				retry = time.Duration(attempt+1) * 100 * time.Millisecond
			}
			if err = waitContext(ctx, retry); err != nil {
				return page{}, err
			}
			continue
		}
		if status >= 500 && status <= 599 {
			_ = resp.Body.Close() // Preserve the upstream HTTP status.
			cancel()
			last = &StatusError{URL: u, StatusCode: status}
			if attempt < c.opts.Retries {
				if err = waitContext(ctx, time.Duration(attempt+1)*100*time.Millisecond); err != nil {
					return page{}, err
				}
				continue
			}
			return page{}, last
		}
		if status != 200 {
			_ = resp.Body.Close() // Preserve the upstream HTTP status.
			cancel()
			return page{}, &StatusError{URL: u, StatusCode: status}
		}
		c.limiter.OnSuccess()
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
		_ = resp.Body.Close() // ReadAll determines source-body completeness.
		cancel()
		if err != nil {
			return page{}, fmt.Errorf("read %s: %w", u, err)
		}
		if len(body) > maxBody {
			return page{}, fmt.Errorf("Walkerplus response exceeds 4MiB limit")
		}
		p := page{body: body, source: Source{URL: u, FetchedAt: time.Now().UTC().Format(time.RFC3339)}}
		c.writeCache(u, p)
		return p, nil
	}
	return page{}, last
}

func (c *Client) eventPath(idOrURL string) (string, error) {
	if eventIDRE.MatchString(idOrURL) {
		return "/event/" + idOrURL + "/", nil
	}
	u, err := url.Parse(idOrURL)
	if err != nil || u.Scheme != "https" || (u.Host != "www.walkerplus.com" && u.Host != "walkerplus.com") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("event must be a Walkerplus event ID or canonical HTTPS event URL")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "event" || !eventIDRE.MatchString(parts[1]) {
		return "", fmt.Errorf("invalid Walkerplus event path")
	}
	return "/event/" + parts[1] + "/", nil
}
