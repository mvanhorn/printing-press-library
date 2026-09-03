// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
)

func TestNewUsesFedExDataDirForCache(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FEDEX_DATA_DIR", root)
	c := New(&config.Config{BaseURL: "https://apis-sandbox.fedex.com"}, time.Second, 0)
	want := filepath.Join(root, "cache")
	if c.cacheDir != want {
		t.Fatalf("cacheDir=%q, want %q", c.cacheDir, want)
	}
}
