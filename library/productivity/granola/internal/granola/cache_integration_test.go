// Copyright 2026 Granola Printing Press contributors. Licensed under Apache-2.0.

//go:build integration

package granola

import (
	"os"
	"testing"
)

// TestLoadCache_RealGranolaCache is an integration test that decrypts the
// running user's actual Granola cache. Skipped unless `go test -tags=integration`
// is passed; requires a signed-in Granola desktop and Keychain access on this
// machine. The test prints stats rather than asserting structure because the
// real cache content varies per user.
//
// Run: GRANOLA_INTEGRATION=1 go test -tags=integration -v -run TestLoadCache_RealGranolaCache ./internal/granola/
func TestLoadCache_RealGranolaCache(t *testing.T) {
	if os.Getenv("GRANOLA_INTEGRATION") == "" {
		t.Skip("set GRANOLA_INTEGRATION=1 to run; requires real Granola signin")
	}

	cache, err := LoadCache("")
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	t.Logf("cache.version=%d", cache.Version)
	t.Logf("documents=%d transcripts=%d document_lists=%d document_lists_metadata=%d",
		len(cache.Documents), len(cache.Transcripts),
		len(cache.DocumentLists), len(cache.DocumentListsMetadata))

	if len(cache.Transcripts) == 0 && len(cache.DocumentLists) == 0 {
		t.Log("WARN: cache appears empty — Granola may have purged on signout or this is a fresh install")
	}
}
