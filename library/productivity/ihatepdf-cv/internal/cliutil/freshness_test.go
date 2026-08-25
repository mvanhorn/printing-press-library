package cliutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureFreshEmptyAndFreshStore(t *testing.T) {
	dir := t.TempDir()
	missing, err := EnsureFresh(context.Background(), filepath.Join(dir, "missing.db"))
	if err != nil || missing["status"] != "empty" || missing["fresh"] != true {
		t.Fatalf("empty store report = %#v, err = %v", missing, err)
	}
	path := filepath.Join(dir, "store.db")
	if err := os.WriteFile(path, []byte("sqlite"), 0600); err != nil {
		t.Fatal(err)
	}
	fresh, err := EnsureFresh(context.Background(), path)
	if err != nil || fresh["status"] != "fresh" || fresh["source"] != "local" {
		t.Fatalf("fresh store report = %#v, err = %v", fresh, err)
	}
}
