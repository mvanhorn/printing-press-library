// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.EnsureHereNowTables(); err != nil {
		t.Fatalf("ensure here.now tables: %v", err)
	}
	return s
}

func TestClaimVaultRoundTrip(t *testing.T) {
	s := openTestStore(t)

	rec := ClaimRecord{
		Slug:        "my-site",
		ClaimToken:  "tok_123",
		URL:         "https://here.now/my-site",
		PublishedAt: "2026-05-30T12:00:00Z",
		ExpiresAt:   "2026-05-31T12:00:00Z",
		Claimed:     false,
	}
	if err := s.SaveClaim(rec); err != nil {
		t.Fatalf("SaveClaim: %v", err)
	}

	got, err := s.GetClaim("my-site")
	if err != nil {
		t.Fatalf("GetClaim: %v", err)
	}
	if got == nil {
		t.Fatal("GetClaim returned nil for saved slug")
	}
	if got.ClaimToken != "tok_123" || got.URL != rec.URL || got.Claimed {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	if err := s.MarkClaimed("my-site"); err != nil {
		t.Fatalf("MarkClaimed: %v", err)
	}
	got, _ = s.GetClaim("my-site")
	if !got.Claimed {
		t.Error("expected claimed=true after MarkClaimed")
	}

	list, err := s.ListClaims()
	if err != nil {
		t.Fatalf("ListClaims: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(list))
	}

	missing, err := s.GetClaim("nope")
	if err != nil {
		t.Fatalf("GetClaim(missing): %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for missing slug, got %+v", missing)
	}
}

func TestPublishStateRoundTrip(t *testing.T) {
	s := openTestStore(t)

	rec := PublishStateRecord{
		Slug:        "site-a",
		VersionID:   "ver_1",
		Dir:         "/tmp/site-a",
		UploadsJSON: `[{"path":"a.png"}]`,
		Finalized:   false,
		CreatedAt:   "2026-05-30T12:00:00Z",
	}
	if err := s.SavePublishState(rec); err != nil {
		t.Fatalf("SavePublishState: %v", err)
	}

	got, err := s.GetPublishState("site-a")
	if err != nil {
		t.Fatalf("GetPublishState: %v", err)
	}
	if got == nil || got.VersionID != "ver_1" || got.Finalized {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	if err := s.MarkFinalized("site-a"); err != nil {
		t.Fatalf("MarkFinalized: %v", err)
	}
	got, _ = s.GetPublishState("site-a")
	if !got.Finalized {
		t.Error("expected finalized=true after MarkFinalized")
	}

	n, err := s.RecentPublishCount("2026-05-30T00:00:00Z")
	if err != nil {
		t.Fatalf("RecentPublishCount: %v", err)
	}
	if n != 1 {
		t.Errorf("RecentPublishCount = %d, want 1", n)
	}
	n, _ = s.RecentPublishCount("2026-06-01T00:00:00Z")
	if n != 0 {
		t.Errorf("RecentPublishCount (future cutoff) = %d, want 0", n)
	}
}
