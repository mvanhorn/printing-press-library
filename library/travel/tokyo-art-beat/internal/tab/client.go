// Package tab reads the public published Tokyo Art Beat feed without credentials.
package tab

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/tokyo-art-beat/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const BaseURL = "https://cdn.prod.tabdev.net/api/cda/published/spaces/j05yk38inose/environments/master"
const maxBody = 4 << 20
const maxRequests = 20

type Error struct {
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	Status     int      `json:"status,omitempty"`
	Exit       int      `json:"-"`
	Input      string   `json:"input,omitempty"`
	Failures   []*Error `json:"failures,omitempty"`
	Candidates []Ref    `json:"candidates,omitempty"`
}

func (e *Error) Error() string               { return e.Message }
func Fail(code, msg string, exit int) *Error { return &Error{Code: code, Message: msg, Exit: exit} }

type Entry struct {
	Sys struct {
		ID          string `json:"id"`
		UpdatedAt   string `json:"updatedAt"`
		ContentType struct {
			Sys struct {
				ID string `json:"id"`
			} `json:"sys"`
		} `json:"contentType"`
	} `json:"sys"`
	Fields map[string]map[string]any `json:"fields"`
}
type Feed struct {
	Total    int     `json:"total"`
	Skip     int     `json:"skip"`
	Limit    int     `json:"limit"`
	Items    []Entry `json:"items"`
	Includes struct {
		Entry []Entry `json:"Entry"`
	} `json:"includes"`
}
type Fetch struct {
	FetchedAt time.Time `json:"fetched_at"`
	Cache     bool      `json:"cache"`
	Stale     bool      `json:"stale"`
	URL       string    `json:"url"`
}
type Stats struct {
	Requests     int   `json:"requests"`
	WireRequests int   `json:"wire_requests"`
	CacheHits    int   `json:"cache_hits"`
	Bytes        int   `json:"response_bytes"`
	ElapsedMS    int64 `json:"elapsed_ms"`
}

// ErrDryRun stops validation-only commands before any cache or source access.
var ErrDryRun = &Error{Code: "dry_run", Message: "Source lookups are omitted during dry-run"}

type Options struct {
	ValidateOnly bool
	CacheDir     string
	Fresh        bool
	Offline      bool
	NoCache      bool
	TTL          time.Duration
	Timeout      time.Duration
}
type Client struct {
	base    string
	http    *http.Client
	opt     Options
	Stats   Stats
	Fetches []Fetch
	start   time.Time
	limiter *cliutil.AdaptiveLimiter
}
type cacheRecord struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Data      json.RawMessage `json:"data"`
}
type countingTransport struct {
	inner http.RoundTripper
	c     *Client
}

func (t countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := t.c.limiter.Wait(r.Context()); err != nil {
		return nil, err
	}
	t.c.Stats.WireRequests++
	if t.c.Stats.WireRequests > maxRequests*3 {
		return nil, Fail("request_budget", "Wire request budget exhausted", 5)
	}
	return t.inner.RoundTrip(r)
}

func NewClient(opt Options) *Client {
	if opt.TTL == 0 {
		opt.TTL = time.Hour
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 15 * time.Second
	}
	if opt.CacheDir == "" {
		d, _ := os.UserCacheDir()
		opt.CacheDir = filepath.Join(d, "tokyo-art-beat-cli")
	}
	c := &Client{base: BaseURL, opt: opt, start: time.Now(), Fetches: []Fetch{}, limiter: cliutil.NewAdaptiveLimiter(2)}
	c.http = &http.Client{Timeout: opt.Timeout, Transport: countingTransport{inner: http.DefaultTransport, c: c}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return Fail("redirect_limit", "Too many upstream redirects", 5)
		}
		if req.URL.Scheme != "https" || req.URL.Host != "cdn.prod.tabdev.net" {
			return Fail("unsafe_redirect", "Upstream redirect left the published CDN", 5)
		}
		return nil
	}}
	return c
}
func (c *Client) Summary() Stats {
	s := c.Stats
	s.ElapsedMS = time.Since(c.start).Milliseconds()
	return s
}
func (c *Client) Query(ctx context.Context, q url.Values) (Feed, error) {
	var out Feed
	if c.opt.ValidateOnly {
		return out, ErrDryRun
	}
	u := c.base + "/entries?" + q.Encode()
	sum := sha256.Sum256([]byte(u))
	path := filepath.Join(c.opt.CacheDir, cachePrefix+hex.EncodeToString(sum[:])+".json")
	if !c.opt.Fresh && !c.opt.NoCache {
		if b, err := readCache(path); err == nil {
			var rec cacheRecord
			if json.Unmarshal(b, &rec) == nil && !rec.FetchedAt.IsZero() && len(rec.Data) <= maxBody {
				age := time.Since(rec.FetchedAt)
				stale := age > c.opt.TTL || age < 0
				if (c.opt.Offline || !stale) && decodeFeed(rec.Data, &out) == nil {
					c.Stats.CacheHits++
					c.Fetches = append(c.Fetches, Fetch{rec.FetchedAt, true, stale, u})
					return out, nil
				}
			}
		}
	}
	if c.opt.Offline {
		return out, Fail("cache_miss", "No cached response for this exact query; run it online first without --offline", 5)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if c.Stats.Requests >= maxRequests {
			return out, Fail("request_budget", "Bounded request budget exhausted; narrow the query", 5)
		}
		c.Stats.Requests++
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return out, Fail("request", "Invalid source request", 5)
		}
		req.Header.Set("User-Agent", "tokyo-art-beat-cli/0.1 (+read-only-public-feed)")
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return out, Fail("timeout", "Command deadline exceeded", 5)
			}
			if attempt == 0 {
				if err = wait(ctx, 250*time.Millisecond); err != nil {
					return out, Fail("timeout", "Command deadline exceeded", 5)
				}
				continue
			}
			return out, Fail("network", "Published feed unreachable: check connectivity and retry with --fresh", 5)
		}
		b, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
		closeErr := resp.Body.Close()
		if readErr == nil {
			readErr = closeErr
		}
		c.Stats.Bytes += len(b)
		if resp.StatusCode == 429 {
			c.limiter.OnRateLimit()
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			if attempt == 0 {
				delay := 500 * time.Millisecond
				if v, er := time.ParseDuration(resp.Header.Get("Retry-After") + "s"); er == nil && v > delay {
					delay = v
				}
				if delay > 2*time.Second {
					delay = 2 * time.Second
				}
				if wait(ctx, delay) != nil {
					return out, Fail("timeout", "Command deadline exceeded", 5)
				}
				continue
			}
		}
		if resp.StatusCode != 200 {
			e := Fail("http_error", fmt.Sprintf("Published feed returned HTTP %d", resp.StatusCode), 5)
			e.Status = resp.StatusCode
			if resp.StatusCode == 429 {
				return out, &cliutil.RateLimitError{URL: u, RetryAfter: cliutil.RetryAfter(resp)}
			}
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				e.Code = "access_denied"
				e.Exit = 3
			}
			return out, e
		}
		if readErr != nil {
			return out, Fail("read_error", "Could not read upstream response", 5)
		}
		if len(b) > maxBody {
			return out, Fail("response_too_large", "Upstream response exceeded 4 MiB; narrow the query", 5)
		}
		if err = decodeFeed(b, &out); err != nil {
			return out, err
		}
		c.limiter.OnSuccess()
		fetched := time.Now().UTC()
		c.Fetches = append(c.Fetches, Fetch{fetched, false, false, u})
		if !c.opt.NoCache {
			c.saveCache(path, cacheRecord{fetched, b})
		}
		return out, nil
	}
	return out, Fail("network", "Request failed", 5)
}
func Classify(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		return Fail("rate_limited", "Tokyo Art Beat public feed returned HTTP 429; wait and retry", 7)
	}
	return Fail("network", err.Error(), 5)
}
func decodeFeed(b []byte, out *Feed) error {
	var shape map[string]json.RawMessage
	if json.Unmarshal(b, &shape) != nil {
		return Fail("invalid_response", "Upstream did not return JSON; the public feed may have changed", 5)
	}
	for _, key := range []string{"total", "skip", "limit", "items"} {
		if _, ok := shape[key]; !ok {
			return Fail("invalid_response", "Upstream feed lacks "+key+"; no results were inferred", 5)
		}
	}
	if json.Unmarshal(b, out) != nil || out.Items == nil || out.Total < 0 || out.Skip < 0 || out.Limit < 0 {
		return Fail("invalid_response", "Invalid upstream feed shape", 5)
	}
	for _, e := range out.Items {
		if e.Sys.ID == "" || e.Fields == nil {
			return Fail("invalid_response", "Upstream entry lacks identity or fields", 5)
		}
	}
	return nil
}
func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func (c *Client) saveCache(path string, r cacheRecord) {
	// Cache failures do not turn a successful read into a failed command.
	if os.MkdirAll(c.opt.CacheDir, 0700) != nil {
		return
	}
	b, err := json.Marshal(r)
	if err != nil {
		return
	}
	f, err := os.CreateTemp(c.opt.CacheDir, ".tmp-")
	if err != nil {
		return
	}
	name := f.Name()
	defer os.Remove(name)
	if f.Chmod(0600) != nil {
		_ = f.Close()
		return
	}
	if _, err = f.Write(b); err != nil {
		_ = f.Close()
		return
	}
	if f.Close() != nil {
		return
	}
	if os.Rename(name, path) != nil {
		return
	}
	files, _ := os.ReadDir(c.opt.CacheDir)
	type item struct {
		path  string
		mtime time.Time
		size  int64
	}
	list := []item{}
	var total int64
	for _, x := range files {
		if !x.IsDir() && ownedCacheFile(x.Name()) {
			info, er := x.Info()
			if er == nil {
				list = append(list, item{filepath.Join(c.opt.CacheDir, x.Name()), info.ModTime(), info.Size()})
				total += info.Size()
			}
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].mtime.Before(list[j].mtime) })
	count := len(list)
	for _, x := range list {
		if count <= 128 && total <= 25<<20 && time.Since(x.mtime) < 7*24*time.Hour {
			break
		}
		if os.Remove(x.path) == nil {
			count--
			total -= x.size
		}
	}
}

func readCache(path string) ([]byte, error) {
	// #nosec G304 -- path is the SHA-256 query key beneath the caller-selected private response cache.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxBody+1025))
	if len(b) > maxBody+1024 {
		return nil, fmt.Errorf("oversized cache")
	}
	return b, err
}

func ownedCacheFile(name string) bool {
	if !strings.HasPrefix(name, cachePrefix) {
		return false
	}
	name = strings.TrimPrefix(name, cachePrefix)
	if len(name) != 69 || !strings.HasSuffix(name, ".json") {
		return false
	}
	for _, c := range name[:64] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

const cachePrefix = "tab-v1-"
