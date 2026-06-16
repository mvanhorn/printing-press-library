// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func (c *Client) responseCacheEnabled(binaryResponse bool) bool {
	return !binaryResponse && !c.NoCache && !c.DryRun && c.cacheDir != ""
}

func (c *Client) cacheKey(path string, params map[string]string) string {
	key := path
	key += "|base_url=" + c.BaseURL
	if c.Config != nil {
		key += "|auth_source=" + c.Config.AuthSource
		if authHeader := c.Config.AuthHeader(); authHeader != "" {
			authHash := sha256.Sum256([]byte(authHeader))
			key += "|auth=" + hex.EncodeToString(authHash[:8])
		}
		if c.Config.Path != "" {
			key += "|config_path=" + c.Config.Path
		}
	}
	paramKeys := make([]string, 0, len(params))
	for k := range params {
		paramKeys = append(paramKeys, k)
	}
	sort.Strings(paramKeys)
	for _, k := range paramKeys {
		key += k + "=" + params[k]
	}
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:8])
}

func (c *Client) readCache(path string, params map[string]string) (json.RawMessage, bool) {
	cacheFile := filepath.Join(c.cacheDir, c.cacheKey(path, params)+".json")
	info, err := os.Stat(cacheFile)
	if err != nil || time.Since(info.ModTime()) > 5*time.Minute {
		return nil, false
	}
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, false
	}
	return json.RawMessage(data), true
}

func (c *Client) writeCache(path string, params map[string]string, data json.RawMessage) {
	os.MkdirAll(c.cacheDir, 0o700)
	cacheFile := filepath.Join(c.cacheDir, c.cacheKey(path, params)+".json")
	os.WriteFile(cacheFile, []byte(data), 0o600)
}

// invalidateCache wholesale-removes the cache directory so the next read
// after a mutation cannot return a stale snapshot. Selective per-resource
// invalidation rejected: cache keys are opaque sha256 hashes.
func (c *Client) invalidateCache() {
	if c.cacheDir == "" {
		return
	}
	_ = os.RemoveAll(c.cacheDir)
}
