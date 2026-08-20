// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

// Package cache stores per-call API responses as TTL'd JSON files in
// ~/.cache/gorgias-pp-cli/. Keys are sha256 hashes of (path|params|auth),
// so cache invalidation after a mutation is a single RemoveAll on the
// directory — selective per-resource invalidation has no faster path
// when keys are opaque hashes. The local SQLite mirror in internal/store
// is the right place for queryable cached data; this cache only handles
// fast retries of identical GETs within the 5-minute TTL.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Store is a key-value cache backed by the filesystem.
type Store struct {
	Dir string
	TTL time.Duration
}

// New creates a file-based cache store.
func New(dir string, ttl time.Duration) *Store {
	return &Store{Dir: dir, TTL: ttl}
}

// Get retrieves a cached value. Returns nil if not found or expired.
func (s *Store) Get(key string) (json.RawMessage, bool) {
	path := s.path(key)
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > s.TTL {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return json.RawMessage(data), true
}

// Set stores a value in the cache.
func (s *Store) Set(key string, value json.RawMessage) {
	_ = os.MkdirAll(s.Dir, 0o755)
	_ = os.WriteFile(s.path(key), []byte(value), 0o644)
}

// Clear removes all cached entries.
func (s *Store) Clear() error {
	return os.RemoveAll(s.Dir)
}

func (s *Store) path(key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(s.Dir, hex.EncodeToString(h[:8])+".json")
}
