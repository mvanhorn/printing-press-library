package planner

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
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/cliutil"
)

const maxResponseBytes int64 = 4 << 20
const baseURL = "https://production.tablecheck.com"

type Client struct {
	opts          Options
	http          *http.Client
	limiter       *cliutil.AdaptiveLimiter
	slots         chan struct{}
	mu            sync.Mutex
	requests      int
	responseBytes int64
	started       time.Time
	memory        map[string]cacheEntry
}
type cacheEntry struct {
	Version   int             `json:"version"`
	FetchedAt time.Time       `json:"fetched_at"`
	ExpiresAt time.Time       `json:"expires_at"`
	Body      json.RawMessage `json:"body"`
}
type observation struct {
	FetchedAt, ServedAt, ExpiresAt time.Time
	CacheHit                       bool
}

func (o observation) result() map[string]any {
	age := o.ServedAt.Sub(o.FetchedAt).Seconds()
	if age < 0 {
		age = 0
	}
	return map[string]any{"fetched_at": o.FetchedAt.UTC().Format(time.RFC3339Nano), "observed_at": o.FetchedAt.UTC().Format(time.RFC3339Nano), "served_at": o.ServedAt.UTC().Format(time.RFC3339Nano), "expires_at": o.ExpiresAt.UTC().Format(time.RFC3339Nano), "cache_hit": o.CacheHit, "age_seconds": age, "upstream_freshness": "unknown"}
}

func New(o Options) (*Client, error) {
	if o.BaseURL == "" {
		o.BaseURL = baseURL
	}
	o.BaseURL = strings.TrimRight(o.BaseURL, "/")
	u, e := url.Parse(o.BaseURL)
	if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, invalid("base URL must be an HTTP(S) origin without credentials, query or fragment")
	}
	if o.Timeout == 0 {
		o.Timeout = 10 * time.Second
	}
	if o.Timeout <= 0 || o.Timeout > 30*time.Second {
		return nil, invalid("timeout must be positive and at most 30s")
	}
	if o.MaxRequests == 0 {
		o.MaxRequests = 20
	}
	if o.MaxRequests < 1 || o.MaxRequests > 20 {
		return nil, invalid("max-requests must be between 1 and 20")
	}
	if o.Concurrency == 0 {
		o.Concurrency = 2
	}
	if o.Concurrency < 1 || o.Concurrency > 2 {
		return nil, invalid("concurrency must be 1 or 2")
	}
	if o.Retries < 0 || o.Retries > 1 {
		return nil, invalid("retries must be 0 or 1")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.CacheDir == "" {
		d, e := os.UserCacheDir()
		if e != nil {
			return nil, e
		}
		o.CacheDir = filepath.Join(d, "tablecheck-pp-cli", "planning")
	}
	hc := &http.Client{}
	if o.HTTPClient != nil {
		copy := *o.HTTPClient
		hc = &copy
	}
	hc.Timeout = o.Timeout
	// Redirects are not part of the verified contract. Refuse rather than permit
	// uncounted requests, cross-origin reads or method changes on POST reads.
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{opts: o, http: hc, limiter: cliutil.NewAdaptiveLimiter(2), slots: make(chan struct{}, o.Concurrency), started: time.Now(), memory: map[string]cacheEntry{}}, nil
}
func (c *Client) meta() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return map[string]any{"source": "live", "requests": c.requests, "response_bytes": c.responseBytes, "latency_ms": time.Since(c.started).Milliseconds(), "request_budget": c.opts.MaxRequests}
}
func (c *Client) result(r Result, obs *observation) Result {
	m := c.meta()
	if obs != nil {
		m["freshness"] = obs.result()
		if obs.CacheHit {
			m["transport"] = "local-cache"
		} else {
			m["transport"] = "live"
		}
	}
	r["meta"] = m
	return r
}
func (c *Client) waitLimiter(ctx context.Context) error {
	return c.limiter.Wait(ctx)
}
func (c *Client) reserve() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.requests >= c.opts.MaxRequests {
		return fmt.Errorf("request budget exhausted (%d HTTP attempts)", c.opts.MaxRequests)
	}
	c.requests++
	return nil
}
func (c *Client) addBytes(n int) { c.mu.Lock(); c.responseBytes += int64(n); c.mu.Unlock() }
func (c *Client) fetch(ctx context.Context, method, path string, q url.Values, body any, ttl time.Duration, validate func([]byte) error) ([]byte, observation, error) {
	endpoint := c.opts.BaseURL + path
	if len(q) > 0 {
		endpoint += "?" + q.Encode()
	}
	var payload []byte
	var e error
	if body != nil {
		payload, e = json.Marshal(body)
		if e != nil {
			return nil, observation{}, e
		}
	}
	digest := sha256.Sum256(append([]byte(method+"\n"+endpoint+"\n"), payload...))
	key := hex.EncodeToString(digest[:])
	now := c.opts.Now().UTC()
	if !c.opts.Refresh {
		if entry, ok := c.readCache(key, now); ok && now.Sub(entry.FetchedAt) < ttl && validate(entry.Body) == nil {
			expiry := entry.ExpiresAt
			if policyExpiry := entry.FetchedAt.Add(ttl); policyExpiry.Before(expiry) {
				expiry = policyExpiry
			}
			return append([]byte(nil), entry.Body...), observation{entry.FetchedAt, now, expiry, true}, nil
		}
	}
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return nil, observation{}, ctx.Err()
	}
	for attempt := 0; attempt <= c.opts.Retries; attempt++ {
		if e = ctx.Err(); e != nil {
			return nil, observation{}, e
		}
		if e = c.waitLimiter(ctx); e != nil {
			return nil, observation{}, e
		}
		if e = c.reserve(); e != nil {
			return nil, observation{}, e
		}
		reqCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
		req, e := http.NewRequestWithContext(reqCtx, method, endpoint, bytes.NewReader(payload))
		if e != nil {
			cancel()
			return nil, observation{}, e
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "tablecheck-pp-cli/0.1.0")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, requestErr := c.http.Do(req)
		if requestErr != nil {
			cancel()
			if ctx.Err() != nil {
				return nil, observation{}, ctx.Err()
			}
			if attempt < c.opts.Retries {
				if e = pause(ctx, 100*time.Millisecond); e != nil {
					return nil, observation{}, e
				}
				continue
			}
			return nil, observation{}, fmt.Errorf("TableCheck read failed: %w", requestErr)
		}
		if remaining, resetAt, ok := cliutil.ParseRateLimitHeaders(resp.Header); ok {
			c.limiter.ObserveHeaders(remaining, resetAt)
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		_ = resp.Body.Close() // The bounded read error is handled below; close-only errors do not change read results.
		cancel()
		c.addBytes(len(raw))
		if readErr != nil {
			return nil, observation{}, fmt.Errorf("read TableCheck response: %w", readErr)
		}
		if int64(len(raw)) > maxResponseBytes {
			return nil, observation{}, fmt.Errorf("TableCheck response exceeds 4 MiB bound")
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			rateErr := &cliutil.RateLimitError{URL: endpoint, RetryAfter: cliutil.RetryAfter(resp), Body: shortBody(raw)}
			if attempt < c.opts.Retries {
				wait := rateErr.RetryAfter
				// A retry never runs earlier than the server's instruction.
				// Long throttles exceed this command's small retry-wait budget.
				if wait > time.Second {
					return nil, observation{}, rateErr
				}
				if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= wait {
					return nil, observation{}, rateErr
				}
				if e = pause(ctx, wait); e != nil {
					return nil, observation{}, e
				}
				continue
			}
			return nil, observation{}, rateErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			he := &HTTPError{resp.StatusCode, endpoint, shortBody(raw)}
			if attempt < c.opts.Retries && transient(resp.StatusCode) {
				if e = pause(ctx, 100*time.Millisecond); e != nil {
					return nil, observation{}, e
				}
				continue
			}
			return nil, observation{}, he
		}
		c.limiter.OnSuccess()
		if e = validate(raw); e != nil {
			return nil, observation{}, fmt.Errorf("TableCheck schema: %w", e)
		}
		fetched := c.opts.Now().UTC()
		entry := cacheEntry{Version: 1, FetchedAt: fetched, ExpiresAt: fetched.Add(ttl), Body: json.RawMessage(raw)}
		c.writeCache(key, entry)
		return raw, observation{fetched, c.opts.Now().UTC(), entry.ExpiresAt, false}, nil
	}
	return nil, observation{}, errors.New("TableCheck read did not complete")
}
func transient(status int) bool {
	return status == 408 || status == 500 || status == 502 || status == 503 || status == 504
}
func pause(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func shortBody(raw []byte) string {
	if len(raw) > 512 {
		raw = raw[:512]
	}
	return strings.TrimSpace(string(raw))
}
func (c *Client) readCache(key string, now time.Time) (cacheEntry, bool) {
	c.mu.Lock()
	entry, ok := c.memory[key]
	c.mu.Unlock()
	if ok && validEntry(entry, now) {
		return entry, true
	}
	f, e := os.Open(filepath.Join(c.opts.CacheDir, key+".json")) // #nosec G304 -- Cache directory is explicitly user-selected; key is a computed SHA-256 fixed filename, not a source path.
	if e != nil {
		return cacheEntry{}, false
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, maxResponseBytes+1024))
	if e != nil || json.Unmarshal(raw, &entry) != nil || !validEntry(entry, now) {
		return cacheEntry{}, false
	}
	c.mu.Lock()
	c.memory[key] = entry
	c.mu.Unlock()
	return entry, true
}
func validEntry(e cacheEntry, now time.Time) bool {
	return e.Version == 1 && !e.FetchedAt.IsZero() && !now.Before(e.FetchedAt) && now.Before(e.ExpiresAt) && len(e.Body) > 0 && int64(len(e.Body)) <= maxResponseBytes
}
func (c *Client) writeCache(key string, entry cacheEntry) {
	c.mu.Lock()
	c.memory[key] = entry
	c.mu.Unlock()
	if e := os.MkdirAll(c.opts.CacheDir, 0700); e != nil {
		return
	}
	raw, e := json.Marshal(entry)
	if e != nil {
		return
	}
	f, e := os.CreateTemp(c.opts.CacheDir, ".response-*")
	if e != nil {
		return
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(raw)
	}
	if closeErr := f.Close(); e == nil {
		e = closeErr
	}
	if e == nil {
		_ = os.Rename(name, filepath.Join(c.opts.CacheDir, key+".json"))
	}
}
