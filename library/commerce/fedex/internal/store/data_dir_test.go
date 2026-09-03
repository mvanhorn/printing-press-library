// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"path/filepath"
	"testing"
)

func TestDefaultPathUsesFedExDataDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FEDEX_DATA_DIR", root)
	want := filepath.Join(root, "fedex.db")
	if got := DefaultPath(); got != want {
		t.Fatalf("DefaultPath()=%q, want %q", got, want)
	}
}
