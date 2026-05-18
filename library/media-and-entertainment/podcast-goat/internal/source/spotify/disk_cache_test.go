// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package spotify

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withTempCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "bearer-cache.json")
	t.Setenv("PODCAST_GOAT_BEARER_CACHE", path)
	return path
}

func TestSaveLoadDiskCache_RoundTrip(t *testing.T) {
	withTempCache(t)
	spDC := "sp_dc_test_value"
	expMs := time.Now().Add(30 * time.Minute).UnixNano() / int64(time.Millisecond)
	if err := SaveDiskCache(spDC, "bearer_xyz", expMs); err != nil {
		t.Fatalf("save: %v", err)
	}
	tok, exp, hit := LoadDiskCache(spDC)
	if !hit {
		t.Fatal("expected cache hit")
	}
	if tok != "bearer_xyz" {
		t.Errorf("token = %q", tok)
	}
	if exp.UnixMilli() != expMs {
		t.Errorf("expiresAt mismatch: %d vs %d", exp.UnixMilli(), expMs)
	}
}

func TestLoadDiskCache_ExpiredIsMiss(t *testing.T) {
	withTempCache(t)
	spDC := "sp_dc_test"
	expMs := time.Now().Add(-5 * time.Minute).UnixNano() / int64(time.Millisecond)
	_ = SaveDiskCache(spDC, "bearer_old", expMs)
	_, _, hit := LoadDiskCache(spDC)
	if hit {
		t.Error("expected miss on expired cache")
	}
}

func TestLoadDiskCache_WrongSpDCIsMiss(t *testing.T) {
	withTempCache(t)
	expMs := time.Now().Add(30 * time.Minute).UnixNano() / int64(time.Millisecond)
	_ = SaveDiskCache("sp_dc_user1", "bearer_xyz", expMs)
	_, _, hit := LoadDiskCache("sp_dc_user2")
	if hit {
		t.Error("expected miss when sp_dc hash differs (cookie rotation simulated)")
	}
}

func TestLoadDiskCache_MissingFileIsMissNoError(t *testing.T) {
	withTempCache(t)
	_, _, hit := LoadDiskCache("anything")
	if hit {
		t.Error("expected miss when cache file does not exist")
	}
}

func TestLoadDiskCache_CorruptFileIsMiss(t *testing.T) {
	path := withTempCache(t)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte("this is not json"), 0o600)
	_, _, hit := LoadDiskCache("anything")
	if hit {
		t.Error("expected miss on corrupt cache file (no panic, no error propagation)")
	}
}

func TestSaveDiskCache_FilePermissions(t *testing.T) {
	path := withTempCache(t)
	expMs := time.Now().Add(time.Hour).UnixNano() / int64(time.Millisecond)
	if err := SaveDiskCache("sp_dc", "tok", expMs); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("perms = %o, want 0600", st.Mode().Perm())
	}
}

func TestClearDiskCache_RemovesFile(t *testing.T) {
	path := withTempCache(t)
	expMs := time.Now().Add(time.Hour).UnixNano() / int64(time.Millisecond)
	_ = SaveDiskCache("sp_dc", "tok", expMs)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("setup: file should exist: %v", err)
	}
	if err := ClearDiskCache(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be gone, stat err = %v", err)
	}
}

func TestClearDiskCache_NoErrorWhenMissing(t *testing.T) {
	withTempCache(t)
	if err := ClearDiskCache(); err != nil {
		t.Errorf("ClearDiskCache on missing file should not error, got: %v", err)
	}
}
