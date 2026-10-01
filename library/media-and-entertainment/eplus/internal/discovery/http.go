package discovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eplus/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const MaxBody = 4 << 20
const CacheTTL = 2 * time.Minute

type Observation struct {
	URL       string `json:"url"`
	FetchedAt string `json:"fetched_at"`
	Cached    bool   `json:"cached"`
	Status    int    `json:"http_status"`
}
type Stats struct {
	Requests  int   `json:"requests"`
	CacheHits int   `json:"cache_hits"`
	Bytes     int64 `json:"response_bytes"`
	LatencyMS int64 `json:"latency_ms"`
}
type Client struct {
	HTTP     *http.Client
	CacheDir string
	Fresh    bool
	Timeout  time.Duration
	mu       sync.Mutex
	stats    Stats
	start    time.Time
	limiter  *cliutil.AdaptiveLimiter
}
type cached struct {
	URL     string    `json:"url"`
	Body    []byte    `json:"body"`
	Fetched time.Time `json:"fetched"`
}

func NewClient(cache string, fresh bool, timeout time.Duration) *Client {
	if timeout <= 0 || timeout > 20*time.Second {
		timeout = 20 * time.Second
	}
	c := &Client{CacheDir: cache, Fresh: fresh, Timeout: timeout, start: time.Now(), limiter: cliutil.NewAdaptiveLimiter(2)}
	c.HTTP = &http.Client{Timeout: timeout, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 4 {
			return fmt.Errorf("redirect limit")
		}
		if e := safeURL(r.URL.String()); e != nil {
			return e
		}
		if len(via) > 0 && r.URL.Hostname() != via[0].URL.Hostname() {
			r.Header.Del("X-APIToken")
		}
		return c.beforeRequest(r.Context())
	}}
	return c
}
func (c *Client) beforeRequest(ctx context.Context) error {
	if e := c.limiter.Wait(ctx); e != nil {
		return e
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stats.Requests >= 32 {
		return fmt.Errorf("public request budget of 32 exhausted; narrow the search")
	}
	c.stats.Requests++
	return nil
}

func safeURL(s string) error {
	u, e := url.Parse(s)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return fmt.Errorf("only canonical public eplus HTTPS URLs are supported")
	}
	switch u.Hostname() {
	case "eplus.jp":
		if u.Path == "/s/eplus/js/property.js" || strings.HasPrefix(u.Path, "/sf/") {
			return nil
		}
	case "api.eplus.jp":
		if u.Path == "/v3/koen" || u.Path == "/v3/koen/keyword" {
			return nil
		}
	case "ib.eplus.jp":
		d := u.Query().Get("dispatch")
		if d == "" || d == "index.index" || d == "tour_group.view" || d == "products.view" || d == "products.get_ticket_type" || d == "pages.view" {
			return nil
		}
	}
	return fmt.Errorf("unsupported public discovery URL")
}

var securityRE = regexp.MustCompile(`(?m)(?:_.security_hash\s*=\s*'[^']*'|var APIV3_TOKEN\s*=\s*[^;]*;)`)

func sanitize(b []byte) []byte {
	return securityRE.ReplaceAll(b, []byte("/* ephemeral website token removed */"))
}
func (c *Client) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.stats
	s.LatencyMS = time.Since(c.start).Milliseconds()
	return s
}
func (c *Client) Fetch(ctx context.Context, u string) ([]byte, Observation, error) {
	return c.fetch(ctx, u, nil, true)
}
func (c *Client) fetch(ctx context.Context, u string, headers map[string]string, cacheable bool) ([]byte, Observation, error) {
	o := Observation{URL: u}
	if e := safeURL(u); e != nil {
		return nil, o, e
	}
	hash := sha256.Sum256([]byte(u))
	path := filepath.Join(c.CacheDir, hex.EncodeToString(hash[:1])+".json")
	if cacheable && !c.Fresh && c.CacheDir != "" {
		if b, e := readCache(path); e == nil {
			var v cached
			if json.Unmarshal(b, &v) == nil && v.URL == u && time.Since(v.Fetched) >= 0 && time.Since(v.Fetched) < CacheTTL && len(v.Body) <= MaxBody {
				c.mu.Lock()
				c.stats.CacheHits++
				c.mu.Unlock()
				o.FetchedAt = v.Fetched.UTC().Format(time.RFC3339)
				o.Cached = true
				o.Status = 200
				return v.Body, o, nil
			}
		}
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if e := c.beforeRequest(ctx); e != nil {
			return nil, o, e
		}
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if e != nil {
			return nil, o, e
		}
		req.Header.Set("User-Agent", "eplus-cli/0.1 (+read-only public discovery)")
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		r, e := c.HTTP.Do(req)
		if e != nil {
			last = fmt.Errorf("public GET failed: %w", e)
			if ctx.Err() != nil {
				return nil, o, ctx.Err()
			}
		} else {
			o.Status = r.StatusCode
			if rem, reset, ok := cliutil.ParseRateLimitHeaders(r.Header); ok {
				c.limiter.ObserveHeaders(rem, reset)
			}
			if r.StatusCode == 429 {
				c.limiter.OnRateLimit()
			} else if r.StatusCode < 400 {
				c.limiter.OnSuccess()
			}
			b, readErr := io.ReadAll(io.LimitReader(r.Body, MaxBody+1))
			_ = r.Body.Close()
			c.mu.Lock()
			c.stats.Bytes += int64(len(b))
			c.mu.Unlock()
			if len(b) > MaxBody {
				return nil, o, fmt.Errorf("public response exceeds %d bytes", MaxBody)
			}
			if r.StatusCode == 200 && readErr == nil {
				now := time.Now()
				o.FetchedAt = now.UTC().Format(time.RFC3339)
				if cacheable {
					b = sanitize(b)
				}
				if cacheable && c.CacheDir != "" {
					if os.MkdirAll(c.CacheDir, 0700) == nil {
						v, _ := json.Marshal(cached{URL: u, Body: b, Fetched: now})
						f, e := os.CreateTemp(c.CacheDir, ".response-*")
						if e == nil {
							name := f.Name()
							_ = f.Chmod(0600)
							_, we := f.Write(v)
							ce := f.Close()
							if we == nil && ce == nil {
								_ = os.Rename(name, path)
							}
							_ = os.Remove(name)
						}
					}
				}
				return b, o, nil
			}
			if readErr != nil {
				last = fmt.Errorf("read public response: %w", readErr)
			} else {
				last = fmt.Errorf("public GET HTTP %d", r.StatusCode)
			}
			if r.StatusCode != 429 && r.StatusCode < 500 {
				return nil, o, last
			}
			wait := time.Duration(attempt+1) * 250 * time.Millisecond
			if r.StatusCode == 429 {
				last = &cliutil.RateLimitError{URL: u, RetryAfter: cliutil.RetryAfter(r)}
				if d := cliutil.RetryAfter(r); d > 0 {
					wait = d
					if wait > 2*time.Second {
						wait = 2 * time.Second
					}
				}
			}
			if attempt < 2 {
				if e := pause(ctx, wait); e != nil {
					return nil, o, e
				}
			}
			continue
		}
		if attempt < 2 {
			if e := pause(ctx, time.Duration(attempt+1)*250*time.Millisecond); e != nil {
				return nil, o, e
			}
		}
	}
	return nil, o, last
}
func pause(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// SetRateLimit can lower the default politeness ceiling, but never raises it.
func (c *Client) SetRateLimit(rate float64) {
	if rate > 0 && rate < 2 {
		c.limiter = cliutil.NewAdaptiveLimiter(rate)
	}
}

func readCache(path string) ([]byte, error) {
	f, e := os.Open(path) // #nosec G304 -- operator-selected cache directory; URL-hashed basename and bounded reader.
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, MaxBody*2+1))
	if len(b) > MaxBody*2 {
		return nil, fmt.Errorf("cached response too large")
	}
	return b, e
}
