package hiking

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
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/yamap/internal/cliutil"
)

const BaseURL = "https://api.yamap.com/v6"
const maxBody = 4 << 20
const maxCacheFiles = 64
const maxCacheBytes = 16 << 20

type SourceError struct {
	Kind    string
	Status  int
	Message string
}

func (e *SourceError) Error() string { return fmt.Sprintf("YAMAP %s: %s", e.Kind, e.Message) }

type FetchMeta struct {
	URL       string    `json:"url"`
	FetchedAt time.Time `json:"fetched_at"`
	CacheHit  bool      `json:"cache_hit"`
	Stale     bool      `json:"stale"`
	Bytes     int       `json:"response_bytes"`
}
type cacheEntry struct {
	URL       string          `json:"url"`
	FetchedAt time.Time       `json:"fetched_at"`
	Body      json.RawMessage `json:"body"`
}

type Client struct {
	HTTP                      *http.Client
	CacheDir                  string
	Refresh, NoCache, Offline bool
	TTL                       time.Duration
	Requests                  int
	CacheHits                 int
	ResponseBytes             int
	Warnings                  []string
	Base                      string
	limiter                   *cliutil.AdaptiveLimiter
}

func NewClient(cacheDir string, rate float64) *Client {
	// 0 disables pacing. A negative value is the unset/--rate-limit auto
	// sentinel; focused hiking reads have no server budget headers, so that
	// default stays at the two-request ceiling.
	if rate < 0 || rate > 2 {
		rate = 2
	}
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}, CacheDir: cacheDir, TTL: 15 * time.Minute, Base: BaseURL, limiter: cliutil.NewAdaptiveLimiter(rate)}
}

func (c *Client) Fetch(ctx context.Context, path string, params url.Values) (map[string]any, FetchMeta, error) {
	var meta FetchMeta
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.ContainsAny(path, "?#") {
		return nil, meta, &SourceError{Kind: "usage", Message: "invalid source path"}
	}
	u := strings.TrimRight(c.Base, "/") + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	meta.URL = u
	hash := sha256.Sum256([]byte("ja\n" + u))
	file := filepath.Join(c.CacheDir, hex.EncodeToString(hash[:])+".json")
	if !c.NoCache && !c.Refresh && c.CacheDir != "" {
		if b, err := readBounded(c.CacheDir, filepath.Base(file), maxBody+4096); err == nil {
			var e cacheEntry
			if json.Unmarshal(b, &e) == nil && e.URL == u && !e.FetchedAt.IsZero() && !e.FetchedAt.After(time.Now().Add(time.Minute)) {
				stale := time.Since(e.FetchedAt) > c.TTL
				if !stale || c.Offline {
					obj, err := decodeObject(e.Body)
					if err == nil {
						meta.FetchedAt = e.FetchedAt
						meta.CacheHit = true
						meta.Stale = stale
						meta.Bytes = len(e.Body)
						c.CacheHits++
						return obj, meta, nil
					}
				}
			}
		}
	}
	if c.Offline {
		return nil, meta, &SourceError{Kind: "not_found", Message: "exact request absent from cache; run online with the same query/page/limit first"}
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, meta, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, meta, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Language", "ja")
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; YAMAP-CLI/0.1)")
		c.Requests++
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, meta, &SourceError{Kind: "network", Message: err.Error()}
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
		closeErr := resp.Body.Close()
		if readErr == nil {
			readErr = closeErr
		}
		c.ResponseBytes += len(body)
		if readErr != nil {
			return nil, meta, &SourceError{Kind: "network", Message: readErr.Error()}
		}
		if len(body) > maxBody {
			return nil, meta, &SourceError{Kind: "schema", Message: "source response exceeds 4 MiB limit"}
		}
		if resp.StatusCode == 429 {
			c.limiter.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			if attempt == 0 && wait <= 2*time.Second {
				if wait <= 0 {
					wait = time.Second
				}
				if err := pause(ctx, wait); err != nil {
					return nil, meta, err
				}
				continue
			}
			return nil, meta, &cliutil.RateLimitError{URL: u, RetryAfter: wait}
		}
		if resp.StatusCode >= 500 && attempt == 0 {
			if err := pause(ctx, time.Second); err != nil {
				return nil, meta, err
			}
			continue
		}
		if resp.StatusCode != 200 {
			kind := "upstream"
			if resp.StatusCode == 404 {
				kind = "not_found"
			}
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				kind = "restricted"
			}
			if resp.StatusCode == 202 {
				kind = "challenge"
			}
			return nil, meta, &SourceError{Kind: kind, Status: resp.StatusCode, Message: fmt.Sprintf("HTTP %d; public-only command; no authenticated fallback", resp.StatusCode)}
		}
		obj, err := decodeObject(body)
		if err != nil {
			return nil, meta, err
		}
		if _, ok := obj["error"]; ok {
			return nil, meta, &SourceError{Kind: "upstream", Message: "source returned an error envelope"}
		}
		c.limiter.OnSuccess()
		meta.FetchedAt = time.Now().UTC()
		meta.Bytes = len(body)
		if !c.NoCache && c.CacheDir != "" {
			entry, _ := json.Marshal(cacheEntry{URL: u, FetchedAt: meta.FetchedAt, Body: body})
			if err := c.save(file, entry); err != nil {
				c.Warnings = append(c.Warnings, "cache write failed: "+err.Error())
			}
		}
		return obj, meta, nil
	}
	return nil, meta, &SourceError{Kind: "upstream", Message: "retry budget exhausted"}
}

func decodeObject(b []byte) (map[string]any, error) {
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.UseNumber()
	var v map[string]any
	if err := d.Decode(&v); err != nil || v == nil {
		return nil, &SourceError{Kind: "schema", Message: "expected source JSON object"}
	}
	var tail any
	if err := d.Decode(&tail); err != io.EOF {
		return nil, &SourceError{Kind: "schema", Message: "trailing JSON data"}
	}
	return v, nil
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
func readBounded(dir, name string, limit int) ([]byte, error) {
	if !cacheName(name) {
		return nil, errors.New("invalid cache filename")
	}
	root, e := os.OpenRoot(dir)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	f, e := root.Open(name)
	if e != nil {
		return nil, e
	}
	b, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if len(b) > limit {
		return nil, errors.New("cache entry too large")
	}
	return b, e
}

func (c *Client) save(file string, b []byte) error {
	if err := os.MkdirAll(c.CacheDir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(c.CacheDir, ".entry-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		_ = f.Close() // preserve the original write failure; cleanup close cannot replace it
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, file); err != nil {
		return err
	}
	entries, err := os.ReadDir(c.CacheDir)
	if err != nil {
		return err
	}
	type cached struct {
		name string
		size int64
		at   time.Time
	}
	var files []cached
	var total int64
	for _, e := range entries {
		if e.IsDir() || !cacheName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, cached{e.Name(), info.Size(), info.ModTime()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].at.Before(files[j].at) })
	for len(files) > maxCacheFiles || total > maxCacheBytes {
		e := files[0]
		files = files[1:]
		if err := os.Remove(filepath.Join(c.CacheDir, e.name)); err == nil {
			total -= e.size
		}
	}
	return nil
}
func cacheName(s string) bool {
	if len(s) != 69 || !strings.HasSuffix(s, ".json") {
		return false
	}
	_, e := hex.DecodeString(strings.TrimSuffix(s, ".json"))
	return e == nil
}

func (c *Client) Inventory() (map[string]any, error) {
	result := map[string]any{"kind": "exact_request_cache", "complete_inventory": false, "entry_count": 0, "stale_entries": 0, "invalid_entries": 0, "bytes": 0, "ttl_seconds": c.TTL.Seconds(), "max_entries": maxCacheFiles, "max_bytes": maxCacheBytes, "newest_fetched_at": nil, "oldest_fetched_at": nil, "refresh": "repeat the chosen source command with --refresh; no bulk inventory crawl"}
	entries, err := os.ReadDir(c.CacheDir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	count, stale, invalid, bytes := 0, 0, 0, 0
	var oldest, newest time.Time
	for _, e := range entries {
		if e.IsDir() || !cacheName(e.Name()) {
			continue
		}
		b, err := readBounded(c.CacheDir, e.Name(), maxBody+4096)
		if err != nil {
			invalid++
			continue
		}
		var entry cacheEntry
		if json.Unmarshal(b, &entry) != nil || entry.FetchedAt.IsZero() {
			invalid++
			continue
		}
		count++
		bytes += len(b)
		if time.Since(entry.FetchedAt) > c.TTL {
			stale++
		}
		if oldest.IsZero() || entry.FetchedAt.Before(oldest) {
			oldest = entry.FetchedAt
		}
		if newest.IsZero() || entry.FetchedAt.After(newest) {
			newest = entry.FetchedAt
		}
	}
	result["entry_count"] = count
	result["stale_entries"] = stale
	result["invalid_entries"] = invalid
	result["bytes"] = bytes
	if count > 0 {
		result["oldest_fetched_at"] = oldest.UTC()
		result["newest_fetched_at"] = newest.UTC()
	}
	return result, nil
}
