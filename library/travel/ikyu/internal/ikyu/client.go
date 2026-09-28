package ikyu

import (
	"bytes"
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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/cliutil"
)

const staticTTL = 24 * time.Hour
const availabilityTTL = 5 * time.Minute

type Client struct {
	opts    Options
	base    *url.URL
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
	sem     chan struct{}
	mu      sync.Mutex
	stats   Stats
	cacheMu sync.Mutex
}
type cacheRecord struct {
	FetchedAt time.Time `json:"fetched_at"`
	Body      []byte    `json:"body"`
	Version   string    `json:"version"`
}
type HTTPError struct {
	Status  int
	Path    string
	Message string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Ikyu HTTP %d for %s: %s", e.Status, e.Path, e.Message)
}

type SchemaError struct{ Message string }

func (e *SchemaError) Error() string { return "Ikyu source schema: " + e.Message }

func NewClient(opts Options) (*Client, error) {
	if opts.BaseURL == "" {
		opts.BaseURL = "https://www.ikyu.com"
	}
	base, err := url.Parse(opts.BaseURL)
	if err != nil || base.Host == "" {
		return nil, fmt.Errorf("invalid Ikyu base URL")
	}
	if opts.HTTPClient == nil && (base.Scheme != "https" || base.Host != "www.ikyu.com") {
		return nil, fmt.Errorf("production source must be https://www.ikyu.com")
	}
	if opts.MaxRequests == 0 {
		opts.MaxRequests = 20
	}
	if opts.MaxRequests < 1 || opts.MaxRequests > 20 {
		return nil, fmt.Errorf("max requests must be 1–20")
	}
	if opts.Concurrency == 0 {
		opts.Concurrency = 2
	}
	if opts.Concurrency < 1 || opts.Concurrency > 2 {
		return nil, fmt.Errorf("concurrency must be 1–2")
	}
	if opts.Retries == 0 {
		opts.Retries = 2
	}
	if opts.Retries < 0 || opts.Retries > 2 {
		return nil, fmt.Errorf("retries must be 0–2")
	}
	if opts.RequestTimeout == 0 {
		opts.RequestTimeout = 15 * time.Second
	}
	if opts.RequestTimeout < 0 || opts.RequestTimeout > 15*time.Second {
		return nil, fmt.Errorf("request timeout must be positive and at most 15s")
	}
	if opts.MaxResponseBytes == 0 {
		opts.MaxResponseBytes = 8 << 20
	}
	if opts.MaxResponseBytes < 1 || opts.MaxResponseBytes > 8<<20 {
		return nil, fmt.Errorf("response body bound must be 1–8MiB")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	h := opts.HTTPClient
	if h == nil {
		h = &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}
	}
	copyHTTP := *h
	// Do not carry user cookies or follow authentication/booking/search redirects.
	copyHTTP.Jar = nil
	copyHTTP.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many source redirects")
		}
		if r.URL.Host != base.Host || r.URL.Scheme != base.Scheme || strings.HasPrefix(r.URL.Path, "/search") || strings.HasPrefix(r.URL.Path, "/booking") || strings.HasPrefix(r.URL.Path, "/ap/rsrv") {
			return fmt.Errorf("source redirected outside public accommodation routes")
		}
		return nil
	}
	c := &Client{opts: opts, base: base, http: &copyHTTP, limiter: cliutil.NewAdaptiveLimiter(2), sem: make(chan struct{}, opts.Concurrency)}
	next := copyHTTP.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	copyHTTP.Transport = &trackedTransport{client: c, next: next}
	return c, nil
}
func (c *Client) Stats() Stats { c.mu.Lock(); defer c.mu.Unlock(); return c.stats }
func (c *Client) fresh(t time.Time, source string, stale bool) Freshness {
	age := int64(c.opts.Now().Sub(t).Seconds())
	if age < 0 {
		age = 0
	}
	return Freshness{FetchedAt: t, AgeSeconds: age, Source: source, Stale: stale, SchemaVersion: SchemaVersion}
}
func (c *Client) fetch(ctx context.Context, method, path string, body []byte, ttl time.Duration) ([]byte, Freshness, error) {
	sum := sha256.Sum256(append([]byte(SchemaVersion+"\n"+c.base.String()+"\n"+method+"\n"+path+"\n"), body...))
	key := hex.EncodeToString(sum[:])
	cached, hit := c.readCache(key)
	if hit && !c.opts.Refresh && c.opts.Now().Sub(cached.FetchedAt) <= ttl {
		c.mu.Lock()
		c.stats.CacheHits++
		c.mu.Unlock()
		return cached.Body, c.fresh(cached.FetchedAt, "cache", false), nil
	}
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, Freshness{}, ctx.Err()
	}
	var last error
	for attempt := 0; attempt <= c.opts.Retries; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.opts.RequestTimeout)
		target := *c.base
		relative, err := url.Parse(path)
		if err != nil {
			cancel()
			return nil, Freshness{}, err
		}
		target.Path = relative.Path
		target.RawQuery = relative.RawQuery
		req, err := http.NewRequestWithContext(callCtx, method, target.String(), bytes.NewReader(body))
		if err != nil {
			cancel()
			return nil, Freshness{}, err
		}
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := c.http.Do(req)
		retry := false
		wait := time.Duration(attempt+1) * 200 * time.Millisecond
		if err != nil {
			last = fmt.Errorf("read Ikyu: %w", err)
			retry = true
			cancel()
		} else {
			raw, readErr := io.ReadAll(io.LimitReader(response.Body, c.opts.MaxResponseBytes+1))
			_ = response.Body.Close()
			cancel()
			if int64(len(raw)) > c.opts.MaxResponseBytes {
				last = fmt.Errorf("Ikyu response exceeds %d-byte bound", c.opts.MaxResponseBytes)
			} else if readErr != nil {
				last = fmt.Errorf("read Ikyu response: %w", readErr)
				retry = true
			} else if response.StatusCode == 429 {
				c.limiter.OnRateLimit()
				wait = cliutil.RetryAfter(response)
				if wait <= 0 {
					wait = time.Duration(attempt+1) * 200 * time.Millisecond
				}
				last = &cliutil.RateLimitError{URL: target.String(), RetryAfter: wait}
				retry = true
			} else if response.StatusCode < 200 || response.StatusCode >= 300 {
				message := ""
				var diagnostic struct {
					Errors []struct {
						Message string `json:"message"`
					} `json:"errors"`
				}
				if json.Unmarshal(raw, &diagnostic) == nil && len(diagnostic.Errors) > 0 {
					message = cliutil.ScrubTerminal(cliutil.CleanText(diagnostic.Errors[0].Message))
					if len(message) > 500 {
						message = message[:500]
					}
				}
				last = &HTTPError{Status: response.StatusCode, Path: path, Message: message}
				retry = response.StatusCode >= 500
			} else {
				c.limiter.OnSuccess()
				if method == http.MethodPost {
					var envelope struct {
						Data   json.RawMessage `json:"data"`
						Errors []struct {
							Message string `json:"message"`
						} `json:"errors"`
					}
					if err := json.Unmarshal(raw, &envelope); err != nil {
						return nil, Freshness{}, &SchemaError{Message: "invalid JSON response"}
					}
					if len(envelope.Errors) > 0 {
						return nil, Freshness{}, &SchemaError{Message: boundedDiagnostic(envelope.Errors[0].Message)}
					}
					if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
						return nil, Freshness{}, &SchemaError{Message: "missing data"}
					}
				}
				stamp := c.opts.Now().UTC()
				c.writeCache(key, cacheRecord{FetchedAt: stamp, Body: raw, Version: SchemaVersion})
				return raw, c.fresh(stamp, "live", false), nil
			}
		}
		if !retry || attempt == c.opts.Retries {
			break
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, Freshness{}, ctx.Err()
		}
	}
	if hit && !c.opts.Refresh && c.opts.AllowStale {
		c.mu.Lock()
		c.stats.CacheHits++
		c.stats.StaleHits++
		c.mu.Unlock()
		f := c.fresh(cached.FetchedAt, "cache", true)
		if last != nil {
			message := last.Error()
			f.SourceError = &message
		}
		return cached.Body, f, nil
	}
	return nil, Freshness{}, last
}
func (c *Client) query(ctx context.Context, name, query string, variables any, out any, ttl time.Duration) (Freshness, error) {
	body, err := json.Marshal(map[string]any{"operationName": name, "query": query, "variables": variables})
	if err != nil {
		return Freshness{}, err
	}
	raw, f, err := c.fetch(ctx, http.MethodPost, "/graphql?lang=ja-JP", body, ttl)
	if err != nil {
		return f, err
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return f, &SchemaError{Message: err.Error()}
	}
	if err = json.Unmarshal(envelope.Data, out); err != nil {
		return f, &SchemaError{Message: err.Error()}
	}
	return f, nil
}
func (c *Client) readCache(key string) (cacheRecord, bool) {
	if c.opts.CacheDir == "" {
		return cacheRecord{}, false
	}
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	path := filepath.Join(c.opts.CacheDir, key+".json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 12<<20 {
		return cacheRecord{}, false
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- filename is a 64hex request hash; Lstat rejects nonregular/oversize files.
	if err != nil {
		return cacheRecord{}, false
	}
	var entry cacheRecord
	if json.Unmarshal(raw, &entry) != nil || entry.Version != SchemaVersion || entry.FetchedAt.IsZero() || entry.FetchedAt.After(c.opts.Now()) || len(entry.Body) == 0 || int64(len(entry.Body)) > c.opts.MaxResponseBytes {
		return cacheRecord{}, false
	}
	return entry, true
}
func (c *Client) writeCache(key string, entry cacheRecord) {
	if c.opts.CacheDir == "" {
		return
	}
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if os.MkdirAll(c.opts.CacheDir, 0700) != nil {
		return
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(c.opts.CacheDir, ".entry-*")
	if err != nil {
		return
	}
	name := tmp.Name()
	defer os.Remove(name)
	if tmp.Chmod(0600) != nil {
		_ = tmp.Close()
		return
	}
	if _, err = tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	if os.Rename(name, filepath.Join(c.opts.CacheDir, key+".json")) != nil {
		return
	}
	files, _ := os.ReadDir(c.opts.CacheDir)
	type file struct {
		name string
		time time.Time
		size int64
	}
	var items []file
	var total int64
	for _, e := range files {
		if e.IsDir() || !cacheKeyFile(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		items = append(items, file{e.Name(), info.ModTime(), info.Size()})
		total += info.Size()
	}
	sort.Slice(items, func(i, j int) bool { return items[i].time.Before(items[j].time) })
	for len(items) > 256 || total > 128<<20 {
		f := items[0]
		items = items[1:]
		total -= f.size
		_ = os.Remove(filepath.Join(c.opts.CacheDir, f.name))
	}
}
func mergeFreshness(a, b Freshness) Freshness {
	if a.FetchedAt.IsZero() {
		return b
	}
	if b.FetchedAt.IsZero() {
		return a
	}
	if b.FetchedAt.Before(a.FetchedAt) {
		a.FetchedAt = b.FetchedAt
	}
	a.Stale = a.Stale || b.Stale
	if a.Source != b.Source {
		a.Source = "mixed"
	}
	if b.AgeSeconds > a.AgeSeconds {
		a.AgeSeconds = b.AgeSeconds
	}
	return a
}
func IsNotFound(err error) bool { var e *HTTPError; return errors.As(err, &e) && e.Status == 404 }

// trackedTransport counts and limits each actual request, including redirects.
type trackedTransport struct {
	client *Client
	next   http.RoundTripper
}

func (t *trackedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c := t.client
	if err := c.limiter.Wait(r.Context()); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.stats.Requests >= c.opts.MaxRequests {
		c.mu.Unlock()
		return nil, fmt.Errorf("command request budget exhausted (%d attempts)", c.opts.MaxRequests)
	}
	c.stats.Requests++
	c.mu.Unlock()
	response, err := t.next.RoundTrip(r)
	if response != nil && response.Body != nil {
		response.Body = &countedBody{ReadCloser: response.Body, client: c}
	}
	return response, err
}

type countedBody struct {
	io.ReadCloser
	client *Client
}

func (b *countedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.client.mu.Lock()
	b.client.stats.ResponseBytes += int64(n)
	b.client.mu.Unlock()
	return n, err
}
func cacheKeyFile(name string) bool {
	if len(name) != 69 || !strings.HasSuffix(name, ".json") {
		return false
	}
	_, err := hex.DecodeString(name[:64])
	return err == nil
}

func boundedDiagnostic(s string) string {
	s = cliutil.ScrubTerminal(cliutil.CleanText(s))
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}
