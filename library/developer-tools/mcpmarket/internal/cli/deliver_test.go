// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeliverFileDoesNotFollowPredictableTempSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "unrelated.txt")
	output := filepath.Join(dir, "result.json")
	if err := os.WriteFile(target, []byte("preserve me"), 0o600); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := os.Symlink(target, output+".tmp"); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	if err := deliverFile(output, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("deliverFile: %v", err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "preserve me" {
		t.Fatalf("pre-planted temp symlink target changed: contents=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(output); err != nil || string(got) != `{"ok":true}` {
		t.Fatalf("delivered output = %q, %v", got, err)
	}
}
