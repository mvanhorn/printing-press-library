// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// fileCache stores successful (200, parsed, non-empty) responses as one file
// per cache key: a JSON metadata line followed by the raw body. Entries
// expire at the response's max-age (capped at MaxCacheTTL, default 15 min)
// and belong to the parser generation that accepted them: an entry stored
// under another ParserVersion is a miss, so a body an older, more lenient
// parser accepted is fetched again rather than re-served.
type fileCache struct {
	dir       string
	now       func() time.Time
	pruneOnce sync.Once
}

type cacheMeta struct {
	Key          string    `json:"key"`
	URL          string    `json:"url"`
	FinalURL     string    `json:"final_url"`
	Host         string    `json:"host"`
	Status       int       `json:"status"`
	StoredAt     time.Time `json:"stored_at"`
	FetchedAt    time.Time `json:"fetched_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	CacheControl string    `json:"cache_control,omitempty"`
	XCache       string    `json:"x_cache,omitempty"`
	// AgeSeconds is the response's Age header (with AgeKnown), kept so a
	// cached result keeps its data time (fetchResult.dataAsOf).
	AgeSeconds    int64  `json:"age_seconds,omitempty"`
	AgeKnown      bool   `json:"age_known,omitempty"`
	ContentType   string `json:"content_type,omitempty"`
	ParserVersion string `json:"parser_version,omitempty"`
}

func newFileCache(dir string, now func() time.Time) *fileCache {
	if now == nil {
		now = time.Now
	}
	return &fileCache{dir: dir, now: now}
}

func (fc *fileCache) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(fc.dir, hex.EncodeToString(sum[:16])+".cache")
}

func (fc *fileCache) get(key string) (*fetchResult, bool) {
	data, err := os.ReadFile(fc.path(key)) // #nosec G304 -- path derived from a sha256 of the cache key.
	if err != nil {
		return nil, false
	}
	idx := bytes.IndexByte(data, '\n')
	if idx < 0 {
		return nil, false
	}
	var meta cacheMeta
	if err := json.Unmarshal(data[:idx], &meta); err != nil || meta.Key != key || meta.ParserVersion != ParserVersion {
		return nil, false
	}
	if !fc.now().Before(meta.ExpiresAt) {
		return nil, false
	}
	header := make(map[string][]string)
	if meta.ContentType != "" {
		header["Content-Type"] = []string{meta.ContentType}
	}
	if meta.CacheControl != "" {
		header["Cache-Control"] = []string{meta.CacheControl}
	}
	return &fetchResult{
		RequestedURL: meta.URL,
		FinalURL:     meta.FinalURL,
		Host:         meta.Host,
		Status:       meta.Status,
		Header:       header,
		Body:         data[idx+1:],
		FromCache:    true,
		FetchedAt:    meta.FetchedAt,
		CacheControl: meta.CacheControl,
		XCache:       "local-cache-hit",
		Age:          time.Duration(meta.AgeSeconds) * time.Second,
		AgeKnown:     meta.AgeKnown,
	}, true
}

func (fc *fileCache) put(key string, res *fetchResult, ttl time.Duration) {
	if res == nil || ttl <= 0 {
		return
	}
	fc.pruneOnce.Do(fc.pruneExpired)
	if err := os.MkdirAll(fc.dir, 0o700); err != nil {
		return
	}
	now := fc.now()
	meta := cacheMeta{
		Key:           key,
		URL:           res.RequestedURL,
		FinalURL:      res.FinalURL,
		Host:          res.Host,
		Status:        res.Status,
		StoredAt:      now,
		FetchedAt:     res.FetchedAt,
		ExpiresAt:     now.Add(ttl),
		CacheControl:  res.CacheControl,
		XCache:        res.XCache,
		AgeSeconds:    int64(res.Age / time.Second),
		AgeKnown:      res.AgeKnown,
		ParserVersion: ParserVersion,
	}
	if res.Header != nil {
		meta.ContentType = res.Header.Get("Content-Type")
	}
	line, err := json.Marshal(meta)
	if err != nil {
		return
	}
	var buf bytes.Buffer
	buf.Grow(len(line) + 1 + len(res.Body))
	buf.Write(line)
	buf.WriteByte('\n')
	buf.Write(res.Body)
	tmp, err := os.CreateTemp(fc.dir, ".tmp-*")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(buf.Bytes())
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmpName)
		return
	}
	_ = os.Chmod(tmpName, 0o600)
	if err := os.Rename(tmpName, fc.path(key)); err != nil {
		_ = os.Remove(tmpName)
	}
}

// pruneExpired removes expired entries and stray temp files once per process
// so the cache directory does not grow without bound.
func (fc *fileCache) pruneExpired() {
	entries, err := os.ReadDir(fc.dir)
	if err != nil {
		return
	}
	now := fc.now()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		full := filepath.Join(fc.dir, name)
		if strings.HasPrefix(name, ".tmp-") {
			if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) > time.Hour {
				_ = os.Remove(full)
			}
			continue
		}
		if !strings.HasSuffix(name, ".cache") {
			continue
		}
		f, err := os.Open(full) // #nosec G304 -- entry inside the private cache dir.
		if err != nil {
			continue
		}
		line, _ := bufio.NewReaderSize(f, 4096).ReadBytes('\n')
		_ = f.Close()
		var meta cacheMeta
		if json.Unmarshal(bytes.TrimSpace(line), &meta) != nil || !now.Before(meta.ExpiresAt) || meta.ParserVersion != ParserVersion {
			_ = os.Remove(full)
		}
	}
}
