// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/json"
	"os"
	"testing"
)

func TestWriteCacheUsesOwnerOnlyPermissions(t *testing.T) {
	cacheDir := t.TempDir()
	c := &Client{cacheDir: cacheDir}
	c.writeCache("/v1/projects", map[string]string{"page": "1"}, json.RawMessage(`{"ok":true}`))

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("cache file count = %d, want 1", len(entries))
	}
	info, err := entries[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("cache permissions = %04o, want 0600", got)
	}
}
