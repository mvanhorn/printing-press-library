// Package jma interprets first-party JMA website data without a resident browser.
package jma

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/jma/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const Origin = "https://www.jma.go.jp/bosai"
const MaxBody = 2 << 20

var JST = time.FixedZone("JST", 9*3600)

type Error struct {
	Code          int
	Kind, Message string
}

func (e *Error) Error() string { return e.Kind + ": " + e.Message }
func failure(code int, kind, format string, args ...any) error {
	return &Error{code, kind, fmt.Sprintf(format, args...)}
}

type Source struct {
	URL        string `json:"url"`
	FetchedAt  string `json:"fetched_at"`
	Cache      bool   `json:"cache_hit"`
	AgeSeconds int64  `json:"retrieval_age_seconds"`
	Bytes      int    `json:"bytes"`
}
type Meta struct {
	Provider    string   `json:"provider"`
	Source      string   `json:"source"`
	Timezone    string   `json:"timezone"`
	RetrievedAt string   `json:"retrieved_at"`
	Requests    int      `json:"requests"`
	ElapsedMS   int64    `json:"elapsed_ms"`
	Coverage    string   `json:"coverage"`
	Sources     []Source `json:"sources"`
	Notes       []string `json:"notes,omitempty"`
}
type Envelope struct {
	Meta    Meta `json:"meta"`
	Results any  `json:"results"`
	Page    any  `json:"page,omitempty"`
}

type Client struct {
	limiter                   *cliutil.AdaptiveLimiter
	CacheDir                  string
	NoCache, Refresh, Offline bool
	HTTP                      *http.Client
	base                      string
	now                       func() time.Time
	started                   time.Time
	requests                  int
	sources                   []Source
	cacheNotes                []string
	skipCacheWrites           bool
}

func New(cache string, noCache, refresh, offline bool, timeout time.Duration) *Client {
	c := &Client{CacheDir: cache, NoCache: noCache, Refresh: refresh, Offline: offline, base: Origin, now: time.Now, started: time.Now(), sources: []Source{}}
	c.limiter = cliutil.NewAdaptiveLimiter(4)
	c.HTTP = &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("redirect limit exceeded")
		}
		if req.URL.Scheme != "https" || req.URL.Host != "www.jma.go.jp" {
			return fmt.Errorf("redirect outside JMA origin")
		}
		return nil
	}}
	return c
}
func (c *Client) Envelope(results any, coverage string, notes ...string) Envelope {
	return Envelope{Meta: Meta{Provider: "Japan Meteorological Agency", Source: "first-party website JSON (normalized by jma-cli)", Timezone: "JST (UTC+09:00)", RetrievedAt: c.now().In(JST).Format(time.RFC3339), Requests: c.requests, ElapsedMS: time.Since(c.started).Milliseconds(), Coverage: coverage, Sources: c.sources, Notes: append(append([]string(nil), notes...), c.cacheNotes...)}, Results: results}
}

type cacheEntry struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Body      json.RawMessage `json:"body"`
}

func atomicWrite(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".jma-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		_ = f.Close() // Preserve the original write failure.
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (c *Client) Get(ctx context.Context, path string, ttl time.Duration, out any) error {
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		return failure(2, "invalid_path", "invalid JMA path")
	}
	full := c.base + path
	sum := sha256.Sum256([]byte(full))
	key := filepath.Join(c.CacheDir, "http", hex.EncodeToString(sum[:])+".json")
	if !c.NoCache && !c.Refresh {
		if b, err := readBounded(key, MaxBody+1024); err == nil {
			var e cacheEntry
			age := c.now().Sub(e.FetchedAt)
			if json.Unmarshal(b, &e) == nil {
				age = c.now().Sub(e.FetchedAt)
				if age >= 0 && age < ttl && json.Valid(e.Body) && json.Unmarshal(e.Body, out) == nil {
					c.sources = append(c.sources, Source{full, e.FetchedAt.In(JST).Format(time.RFC3339), true, int64(age.Seconds()), len(e.Body)})
					return nil
				}
			}
		}
	}
	if c.Offline {
		return failure(4, "offline_miss", "fresh cache unavailable for %s; use inventory refresh or a live command", path)
	}
	var body []byte
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return failure(4, "timeout", "JMA request budget expired")
			case <-time.After(250 * time.Millisecond):
			}
		}
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
		if e != nil {
			return e
		}
		req.Header.Set("User-Agent", "jma-cli/0.1 (read-only public weather)")
		req.Header.Set("Accept", "application/json")
		if e = c.limiter.Wait(ctx); e != nil {
			return failure(4, "timeout", "request pacing exceeded command budget")
		}
		c.requests++
		resp, e := c.HTTP.Do(req)
		if e != nil {
			err = failure(4, "transport", "GET %s failed: %v", path, e)
			if ctx.Err() != nil {
				break
			}
			continue
		}
		body, e = io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
		closeErr := resp.Body.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			err = failure(4, "transport", "GET %s: %v", path, e)
			continue
		}
		if len(body) > MaxBody {
			return failure(5, "incomplete", "GET %s exceeds %d byte limit", path, MaxBody)
		}
		if resp.StatusCode != 200 {
			if resp.StatusCode == 429 {
				c.limiter.OnRateLimit()
			}
			err = failure(4, "http", "GET %s returned HTTP %d", path, resp.StatusCode)
			if resp.StatusCode == 429 || resp.StatusCode >= 500 {
				continue
			}
			return err
		}
		if !json.Valid(body) {
			return failure(5, "format", "GET %s returned invalid JSON", path)
		}
		if e = json.Unmarshal(body, out); e != nil {
			return failure(5, "format", "GET %s incompatible JSON: %v", path, e)
		}
		c.limiter.OnSuccess()
		now := c.now()
		c.sources = append(c.sources, Source{full, now.In(JST).Format(time.RFC3339), false, 0, len(body)})
		if !c.NoCache && !c.skipCacheWrites {
			b, _ := json.Marshal(cacheEntry{now, body})
			if e = atomicWrite(key, b); e == nil {
				e = trimHTTPCache(filepath.Dir(key), key)
				if e != nil {
					// Do not grow an unusable cache after a failed bounded cleanup.
					if removeErr := os.Remove(key); removeErr != nil && !os.IsNotExist(removeErr) {
						c.cacheNotes = append(c.cacheNotes, "HTTP cache cleanup could not remove the newest entry; choose a fresh --cache-dir")
					}
				}
			}
			if e != nil {
				c.skipCacheWrites = true
				c.cacheNotes = append(c.cacheNotes, "HTTP cache persistence unavailable; returning live source data without caching; use --no-cache or a writable --cache-dir")
			}
		}
		return nil
	}
	return err
}
func readBounded(path string, max int64) ([]byte, error) {
	f, e := os.Open(path) // #nosec G304 -- Caller supplies a trusted cache path; network-derived keys are SHA-256 filenames.
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, max+1))
	if len(b) > int(max) {
		return nil, fmt.Errorf("file exceeds limit")
	}
	return b, e
}
func parseTime(s string) (time.Time, error) {
	t, e := time.Parse(time.RFC3339, s)
	if e != nil {
		return t, failure(5, "format", "invalid source timestamp %q", s)
	}
	return t.In(JST), nil
}
func timeJST(s string) (string, error) {
	t, e := parseTime(s)
	if e != nil {
		return "", e
	}
	return t.Format(time.RFC3339), nil
}
func ageHours(now time.Time, s string) *float64 {
	t, e := parseTime(s)
	if e != nil {
		return nil
	}
	v := float64(int64(now.Sub(t).Hours()*10)) / 10
	return &v
}
func canonical(page, kind, code string) string {
	return Origin + "/" + page + "/#area_type=" + url.QueryEscape(kind) + "&area_code=" + url.QueryEscape(code) + "&lang=ja"
}

// Keep HTTP payloads bounded independently of the durable inventory snapshot.
func trimHTTPCache(dir, keep string) error {
	f, e := os.Open(dir) // #nosec G304 -- This is the explicit local HTTP cache directory, not a provider-derived path.
	if e != nil {
		return e
	}
	defer f.Close()
	entries, e := f.ReadDir(513)
	if e != nil && e != io.EOF {
		return e
	}
	if len(entries) > 512 {
		return fmt.Errorf("HTTP cache directory exceeds scan limit; choose a fresh --cache-dir")
	}
	type item struct {
		path     string
		size     int64
		modified time.Time
	}
	xs := []item{}
	var total int64
	for _, d := range entries {
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".json") {
			info, e := d.Info()
			if e != nil {
				return e
			}
			xs = append(xs, item{filepath.Join(dir, d.Name()), info.Size(), info.ModTime()})
			total += info.Size()
		}
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i].modified.Before(xs[j].modified) })
	count := len(xs)
	for _, x := range xs {
		if count <= 128 && total <= 32<<20 {
			break
		}
		if x.path == keep {
			continue
		}
		if e = os.Remove(x.path); e != nil && !os.IsNotExist(e) {
			return e
		}
		count--
		total -= x.size
	}
	return nil
}
