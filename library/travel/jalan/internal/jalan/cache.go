package jalan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const cacheVersion = 1

type cacheEntry struct {
	Version    int       `json:"version"`
	URL        string    `json:"url"`
	ObservedAt time.Time `json:"observed_at"`
	Body       string    `json:"body"`
}

func (c *Client) cachePath(sourceURL string) (string, error) {
	dir := c.options.CacheDir
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "jalan-pp-cli", "observations")
	}
	sum := sha256.Sum256([]byte(sourceURL))
	return filepath.Join(dir, "v1-"+hex.EncodeToString(sum[:])+".json"), nil
}

func (c *Client) readCache(sourceURL string) (cacheEntry, bool) {
	if c.options.DisableCache || c.options.Refresh || c.options.MaxAge <= 0 {
		return cacheEntry{}, false
	}
	path, err := c.cachePath(sourceURL)
	if err != nil {
		return cacheEntry{}, false
	}
	f, err := os.Open(path) // #nosec G304 -- cachePath uses a SHA-256 filename under the caller-selected cache directory; no source path component is used.
	if err != nil {
		return cacheEntry{}, false
	}
	defer f.Close()
	var entry cacheEntry
	decoder := json.NewDecoder(&boundedReader{reader: f, remaining: maxResponseBytes*4 + 1024})
	if decoder.Decode(&entry) != nil || entry.Version != cacheVersion || entry.URL != sourceURL || entry.ObservedAt.IsZero() {
		return cacheEntry{}, false
	}
	age := c.now().Sub(entry.ObservedAt)
	if age < 0 || age > c.options.MaxAge || age > maxCacheAge {
		return cacheEntry{}, false
	}
	return entry, true
}

func (c *Client) writeCache(entry cacheEntry) error {
	if c.options.DisableCache {
		return nil
	}
	path, err := c.cachePath(entry.URL)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".observation-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
