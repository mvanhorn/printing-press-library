package travel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

type cacheEntry struct {
	Version   int
	URL       string
	FetchedAt time.Time
	Body      []byte
}

func cacheName(raw string) string {
	x := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(x[:]) + ".json"
}
func (c *Client) load(raw string, ttl time.Duration) (document, bool) {
	p := filepath.Join(c.config.CacheDir, cacheName(raw))
	info, e := os.Lstat(p)
	if e != nil || !info.Mode().IsRegular() || info.Size() > c.config.CacheMaxBytes {
		return document{}, false
	}
	f, e := os.Open(p) // #nosec G304 -- trusted local cache root plus SHA-256 basename; Lstat requires a regular file and reads are bounded.
	if e != nil {
		return document{}, false
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, c.config.CacheMaxBytes+1))
	if e != nil || int64(len(data)) > c.config.CacheMaxBytes {
		return document{}, false
	}
	var x cacheEntry
	if json.Unmarshal(data, &x) != nil || x.Version != 1 || x.URL != raw || len(x.Body) == 0 || len(x.Body) > int(maxBodyBytes) {
		return document{}, false
	}
	age := c.config.Now().Sub(x.FetchedAt)
	if age < 0 || age >= ttl {
		return document{}, false
	}
	if checkDocument(x.Body, raw) != nil {
		return document{}, false
	}
	return document{Body: x.Body, Source: SourceInfo{Name: "Rakuten Travel", URL: raw, FetchedAt: x.FetchedAt, ObservedAt: x.FetchedAt, CacheState: "hit", CacheAgeSeconds: age.Seconds(), CacheTTLSeconds: int(ttl / time.Second)}}, true
}
func (c *Client) save(d document, inventory bool) error {
	if c.config.NoCache || c.config.CacheDir == "" || (inventory && c.config.InventoryTTL == 0) || d.Source.CacheState == "hit" || d.Source.CacheState == "invocation_snapshot" {
		return nil
	}
	data, e := json.Marshal(cacheEntry{Version: 1, URL: d.Source.URL, FetchedAt: d.Source.FetchedAt, Body: d.Body})
	if e != nil {
		return e
	}
	if int64(len(data)) > c.config.CacheMaxBytes {
		return nil
	}
	if e = os.MkdirAll(c.config.CacheDir, 0700); e != nil {
		return sourceError("cache_error", d.Source.URL, 0, "cannot create private cache directory", e)
	}
	if info, e := os.Lstat(c.config.CacheDir); e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return sourceError("cache_error", d.Source.URL, 0, "cache directory must not be a symlink", e)
	}
	// #nosec G302 -- owner traversal is required for this private cache directory; 0700 exposes nothing to other users.
	if e = os.Chmod(c.config.CacheDir, 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(c.config.CacheDir, ".travel-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	if e = os.Rename(name, filepath.Join(c.config.CacheDir, cacheName(d.Source.URL))); e != nil {
		return e
	}
	return c.prune()
}

// saveBestEffort keeps optional persistence separate from source correctness.
// Each failed save produces at most one diagnostic through the caller's hook.
func (c *Client) saveBestEffort(d document, inventory bool) {
	if c.config.NoCache || inventory && c.config.InventoryTTL == 0 {
		return
	}
	if err := c.save(d, inventory); err != nil && c.config.OnCacheWriteError != nil {
		c.config.OnCacheWriteError(err)
	}
}

var cacheFilePattern = regexp.MustCompile(`^[a-f0-9]{64}\.json$`)

func (c *Client) prune() error {
	entries, e := os.ReadDir(c.config.CacheDir)
	if e != nil {
		return e
	}
	type item struct {
		p    string
		size int64
		when time.Time
	}
	items := []item{}
	var total int64
	for _, entry := range entries {
		if !cacheFilePattern.MatchString(entry.Name()) {
			continue
		}
		info, e := entry.Info()
		if e != nil || !info.Mode().IsRegular() {
			continue
		}
		items = append(items, item{filepath.Join(c.config.CacheDir, entry.Name()), info.Size(), info.ModTime()})
		total += info.Size()
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].when.Equal(items[j].when) {
			return items[i].p < items[j].p
		}
		return items[i].when.Before(items[j].when)
	})
	for len(items) > c.config.CacheMaxEntries || total > c.config.CacheMaxBytes {
		x := items[0]
		if e := os.Remove(x.p); e != nil {
			return e
		}
		total -= x.size
		items = items[1:]
	}
	return nil
}
