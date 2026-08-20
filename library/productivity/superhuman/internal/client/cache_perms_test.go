// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestWriteCacheDirAndFilePerms pins the greptile-p2-cache-perms patch:
// the cache dir is 0o700 (owner-only) and cache files are 0o600 so email
// thread bodies are not world-readable on shared hosts. The token store
// already enforces 0o600; this brings the cache into parity.
func TestWriteCacheDirAndFilePerms(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "http-cache")

	c := &Client{cacheDir: cacheDir}
	c.writeCache("/v3/userdata.read", map[string]string{"k": "v"}, json.RawMessage(`{"ok":true}`))

	dirInfo, err := os.Stat(cacheDir)
	if err != nil {
		t.Fatalf("stat cache dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("cache dir perms: want 0o700, got %#o", got)
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want exactly 1 cache file, got %d", len(entries))
	}

	fileInfo, err := entries[0].Info()
	if err != nil {
		t.Fatalf("stat cache file: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("cache file perms: want 0o600, got %#o", got)
	}
}
