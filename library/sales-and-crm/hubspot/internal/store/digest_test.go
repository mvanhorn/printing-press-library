// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestPendingDigest_RoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "digest.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	plan := []byte(`{"inputs":[{"id":"1"}]}`)
	if err := s.PutPendingDigest(ctx, "blast-aaaa", "contacts bulk-update", plan, 150, 5*time.Minute); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := s.GetPendingDigest(ctx, "blast-aaaa")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected row, got nil")
	}
	if got.RowCount != 150 || got.Command != "contacts bulk-update" {
		t.Fatalf("unexpected row: %+v", got)
	}
	if string(got.PlanJSON) != string(plan) {
		t.Fatalf("plan mismatch: %q vs %q", got.PlanJSON, plan)
	}
}

func TestPendingDigest_MissingReturnsNil(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "digest.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	got, err := s.GetPendingDigest(context.Background(), "blast-nope")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestPendingDigest_ExpiredFilteredOnRead(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "digest.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	// TTL of -1ns guarantees the row is born expired.
	if err := s.PutPendingDigest(ctx, "blast-bbbb", "cmd", []byte(`{}`), 200, -time.Nanosecond); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := s.GetPendingDigest(ctx, "blast-bbbb")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != nil {
		t.Fatalf("expected expired row filtered, got %+v", got)
	}
}

func TestPendingDigest_PurgeRemovesExpired(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "digest.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	if err := s.PutPendingDigest(ctx, "blast-fresh", "cmd", []byte(`{}`), 150, time.Hour); err != nil {
		t.Fatalf("put fresh: %v", err)
	}
	if err := s.PutPendingDigest(ctx, "blast-stale", "cmd", []byte(`{}`), 200, -time.Nanosecond); err != nil {
		t.Fatalf("put stale: %v", err)
	}

	if err := s.PurgeExpiredDigests(ctx); err != nil {
		t.Fatalf("purge: %v", err)
	}

	// Fresh row should still be there.
	got, err := s.GetPendingDigest(ctx, "blast-fresh")
	if err != nil || got == nil {
		t.Fatalf("fresh row gone after purge: %v / %+v", err, got)
	}

	// Stale row should be deleted from disk (also filtered, but we want
	// to verify the purge actually removed it).
	var count int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM hubspot_pending_digests WHERE digest = ?`, "blast-stale").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected stale row purged, got %d", count)
	}
}

func TestPendingDigest_PutReplacesExistingDigest(t *testing.T) {
	// A re-run of the same dry-run produces the same digest. The second
	// put should refresh the TTL, not error.
	s, err := Open(filepath.Join(t.TempDir(), "digest.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	if err := s.PutPendingDigest(ctx, "blast-dup", "cmd", []byte(`{}`), 200, 1*time.Second); err != nil {
		t.Fatalf("put #1: %v", err)
	}
	if err := s.PutPendingDigest(ctx, "blast-dup", "cmd", []byte(`{}`), 200, time.Hour); err != nil {
		t.Fatalf("put #2: %v", err)
	}
	got, err := s.GetPendingDigest(ctx, "blast-dup")
	if err != nil || got == nil {
		t.Fatalf("post-replace get: %v / %+v", err, got)
	}
	if time.Until(got.ExpiresAt) < 30*time.Minute {
		t.Fatalf("expected refreshed TTL, got expires-at %s (in %s)", got.ExpiresAt, time.Until(got.ExpiresAt))
	}
}
