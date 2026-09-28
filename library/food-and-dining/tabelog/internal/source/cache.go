package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

const cacheLimit = 32 << 20

type cacheEntry struct {
	URL       string    `json:"url"`
	FetchedAt time.Time `json:"fetched_at"`
	Body      []byte    `json:"body"`
}

var csrfMeta = regexp.MustCompile(`(?i)(<meta[^>]+name=["']csrf-token["'][^>]+content=["'])[^"']+`)
var csrfInput = regexp.MustCompile(`(?i)(<input[^>]+name=["'](?:authenticity_token|csrf_token)["'][^>]+value=["'])[^"']+`)
var publicVendorKey = regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`)
var signedAssetQuery = regexp.MustCompile(`(?i)([?&](?:amp;)?signature=)[^&"'<>\s]+`)

func (c *Client) cachePath(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return filepath.Join(c.CacheDir, hex.EncodeToString(sum[:])+".json")
}
func (c *Client) cacheRead(raw string) ([]byte, time.Time, error) {
	if c.CacheDir == "" {
		return nil, time.Time{}, fs.ErrNotExist
	}
	if stat, e := os.Stat(c.cachePath(raw)); e != nil {
		return nil, time.Time{}, e
	} else if stat.Size() > 6<<20 {
		return nil, time.Time{}, errors.New("oversized cache entry")
	}
	b, e := os.ReadFile(c.cachePath(raw))
	if e != nil {
		return nil, time.Time{}, e
	}
	if len(b) > 6<<20 {
		return nil, time.Time{}, errors.New("oversized cache entry")
	}
	var ent cacheEntry
	if e = json.Unmarshal(b, &ent); e != nil || ent.URL != raw || ent.FetchedAt.IsZero() || len(ent.Body) > BodyLimit {
		return nil, time.Time{}, errors.New("invalid cache entry")
	}
	return ent.Body, ent.FetchedAt, nil
}
func (c *Client) cacheDelete(raw string) {
	if c.CacheDir != "" {
		_ = os.Remove(c.cachePath(raw))
	}
}
func (c *Client) cacheWrite(raw string, b []byte, t time.Time) error {
	if c.CacheDir == "" {
		return nil
	}
	if e := os.MkdirAll(c.CacheDir, 0700); e != nil {
		return e
	}
	s := csrfMeta.ReplaceAllString(string(b), "${1}REDACTED")
	s = csrfInput.ReplaceAllString(s, "${1}REDACTED")
	s = publicVendorKey.ReplaceAllString(s, "REDACTED")
	s = signedAssetQuery.ReplaceAllString(s, "${1}REDACTED")
	data, e := json.Marshal(cacheEntry{raw, t, []byte(s)})
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(c.CacheDir, ".response-")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if e = tmp.Chmod(0600); e == nil {
		_, e = tmp.Write(data)
	}
	ce := tmp.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if e = os.Rename(name, c.cachePath(raw)); e != nil {
		return e
	}
	entries, e := os.ReadDir(c.CacheDir)
	if e != nil {
		return e
	}
	type file struct {
		path string
		size int64
		time time.Time
	}
	files := make([]file, 0)
	var total int64
	for _, de := range entries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".json" {
			continue
		}
		info, e := de.Info()
		if e != nil {
			continue
		}
		files = append(files, file{filepath.Join(c.CacheDir, de.Name()), info.Size(), info.ModTime()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].time.Before(files[j].time) })
	for _, f := range files {
		if total <= cacheLimit {
			break
		}
		if e = os.Remove(f.path); e == nil {
			total -= f.size
		}
	}
	return nil
}
