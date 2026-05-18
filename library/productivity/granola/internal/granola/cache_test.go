// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package granola

import (
	"os"
	"strings"
	"testing"
)

// TestLoadCache_RealFile loads the actual Granola cache when present.
// PATCH(encrypted-cache): on post-May-2026 Granola, the plaintext
// cache-v6.json is a stale stub; the live data is in cache-v6.json.enc.
// This test now uses the same resolver as production (LoadCache with
// empty path) so it transparently picks up the encrypted file. The
// documents>0 assertion was dropped because Granola moved documents to
// the API path - the cache now holds transcripts/folders/recipes/panels
// only. See safestorage/testdata/scheme.md.
func TestLoadCache_RealFile(t *testing.T) {
	path, _ := ResolveCachePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("no live cache at " + path)
	}
	c, err := LoadCache("")
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	if c.Version <= 0 {
		t.Errorf("expected version > 0, got %d", c.Version)
	}
	// Modern Granola: documents=0 (API-fetched), but transcripts/folders
	// should be present on any non-fresh install. If both are empty the
	// load probably succeeded against an empty plaintext stub.
	if len(c.Documents) == 0 && len(c.Transcripts) == 0 && len(c.DocumentListsMetadata) == 0 {
		t.Errorf("cache loaded but is empty (documents=0, transcripts=0, folders=0) - did decryption fall back to a stub?")
	}
	t.Logf("loaded cache v%d: %d documents, %d transcripts, %d folders, %d panels, %d recipes",
		c.Version, len(c.Documents), len(c.Transcripts), len(c.DocumentListsMetadata), len(c.PanelTemplates), len(c.RecipesAll()))
}

// PATCH(encrypted-cache): resolver tests for the .enc vs plaintext branching.
// Mock support dir via GRANOLA_SUPPORT_DIR so we control which sibling files exist.

func TestResolveCachePath_ExplicitOverrideWins(t *testing.T) {
	tmp := t.TempDir()
	overridePath := tmp + "/explicit.json"
	t.Setenv("GRANOLA_CACHE_PATH", overridePath)
	t.Setenv("GRANOLA_SUPPORT_DIR", tmp)
	// Even if both encrypted and plaintext exist in support dir, the
	// explicit GRANOLA_CACHE_PATH override wins and is treated as plaintext.
	if err := os.WriteFile(tmp+"/cache-v6.json.enc", []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmp+"/cache-v6.json", []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, enc := ResolveCachePath()
	if got != overridePath {
		t.Errorf("override path: got %s, want %s", got, overridePath)
	}
	if enc {
		t.Errorf("explicit override should never be flagged encrypted")
	}
}

func TestResolveCachePath_PrefersEncrypted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("GRANOLA_CACHE_PATH", "")
	t.Setenv("GRANOLA_SUPPORT_DIR", tmp)
	if err := os.WriteFile(tmp+"/cache-v6.json.enc", []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmp+"/cache-v6.json", []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, enc := ResolveCachePath()
	want := tmp + "/cache-v6.json.enc"
	if got != want {
		t.Errorf("prefer-encrypted: got %s, want %s", got, want)
	}
	if !enc {
		t.Errorf("expected encrypted=true when only .enc resolved path applies")
	}
}

func TestResolveCachePath_FallsBackToPlaintext(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("GRANOLA_CACHE_PATH", "")
	t.Setenv("GRANOLA_SUPPORT_DIR", tmp)
	if err := os.WriteFile(tmp+"/cache-v6.json", []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, enc := ResolveCachePath()
	want := tmp + "/cache-v6.json"
	if got != want {
		t.Errorf("fallback path: got %s, want %s", got, want)
	}
	if enc {
		t.Errorf("plaintext should not be flagged encrypted")
	}
}

func TestLoadCache_EncryptedWithoutKey(t *testing.T) {
	// Place a .enc-shaped file (garbage bytes are fine) without setting
	// GRANOLA_SAFESTORAGE_KEY_OVERRIDE. LoadCache should surface a
	// wrapped safestorage error rather than silently falling back to
	// a stale plaintext file.
	tmp := t.TempDir()
	t.Setenv("GRANOLA_CACHE_PATH", "")
	t.Setenv("GRANOLA_SUPPORT_DIR", tmp)
	t.Setenv("GRANOLA_SAFESTORAGE_KEY_OVERRIDE", "")
	garbage := make([]byte, 200)
	for i := range garbage {
		garbage[i] = byte(i)
	}
	if err := os.WriteFile(tmp+"/cache-v6.json.enc", garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmp+"/cache-v6.json", []byte(`{"cache":{"version":6,"state":{"documents":{}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCache("")
	if err == nil {
		t.Fatal("expected error reading encrypted cache without key, got nil")
	}
	// Must NOT silently fall back: if it had, we'd see a valid Cache back.
	if !strings.Contains(err.Error(), "encrypted") && !strings.Contains(err.Error(), "Keychain") && !strings.Contains(err.Error(), "DEK") {
		t.Errorf("expected error mentioning encryption/Keychain/DEK, got: %v", err)
	}
}

// TestLoadCache_Synthetic tests the v3 unwrap path with a hand-built blob.
func TestLoadCache_Synthetic(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/cache.json"
	// v6-shaped (dict).
	if err := os.WriteFile(path, []byte(`{"cache":{"version":6,"state":{"documents":{"a":{"id":"a","title":"T"}}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache v6: %v", err)
	}
	if c.DocumentByID("a") == nil || c.DocumentByID("a").Title != "T" {
		t.Errorf("missing doc 'a'")
	}

	// v3-shaped (stringified).
	if err := os.WriteFile(path, []byte(`{"cache":"{\"version\":3,\"state\":{\"documents\":{\"b\":{\"id\":\"b\",\"title\":\"U\"}}}}"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err = LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache v3: %v", err)
	}
	if c.DocumentByID("b") == nil || c.DocumentByID("b").Title != "U" {
		t.Errorf("missing doc 'b'")
	}
}
