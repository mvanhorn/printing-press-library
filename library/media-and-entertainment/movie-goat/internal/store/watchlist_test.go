package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "watchlist_test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestWatchlistCRUD(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	// Initially empty
	entries, err := s.WatchlistList(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty list, got %d", len(entries))
	}

	// Add Inception (movie 27205) and Breaking Bad (tv 1396)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)
	if err := s.WatchlistAdd(ctx, WatchlistEntry{TMDBID: 27205, Kind: "movie", Title: "Inception", AddedAt: t0}); err != nil {
		t.Fatalf("Add inception: %v", err)
	}
	if err := s.WatchlistAdd(ctx, WatchlistEntry{TMDBID: 1396, Kind: "tv", Title: "Breaking Bad", AddedAt: t1}); err != nil {
		t.Fatalf("Add breaking bad: %v", err)
	}

	// Contains
	ok, err := s.WatchlistContains(ctx, "movie", 27205)
	if err != nil || !ok {
		t.Fatalf("Contains(movie,27205): ok=%v err=%v", ok, err)
	}
	ok, err = s.WatchlistContains(ctx, "movie", 9999)
	if err != nil || ok {
		t.Fatalf("Contains(movie,9999): ok=%v err=%v", ok, err)
	}
	// kind mismatch must return false
	ok, err = s.WatchlistContains(ctx, "tv", 27205)
	if err != nil || ok {
		t.Fatalf("Contains(tv,27205): expected false, got ok=%v err=%v", ok, err)
	}

	// List ordered by added_at ascending
	entries, err = s.WatchlistList(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Title != "Inception" || entries[1].Title != "Breaking Bad" {
		t.Fatalf("unexpected order: %+v", entries)
	}

	// Remove
	if err := s.WatchlistRemove(ctx, "movie", 27205); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	ok, _ = s.WatchlistContains(ctx, "movie", 27205)
	if ok {
		t.Fatalf("Inception still present after Remove")
	}
}

func TestWatchlistUniqueConstraint(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	if err := s.WatchlistAdd(ctx, WatchlistEntry{TMDBID: 27205, Kind: "movie", Title: "Inception"}); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := s.WatchlistAdd(ctx, WatchlistEntry{TMDBID: 27205, Kind: "movie", Title: "Inception 2"}); err == nil {
		t.Fatalf("expected UNIQUE constraint to reject duplicate, got nil error")
	}
	// But the same tmdb_id with a different kind should succeed.
	if err := s.WatchlistAdd(ctx, WatchlistEntry{TMDBID: 27205, Kind: "tv", Title: "tv stub"}); err != nil {
		t.Fatalf("Add same tmdb_id with kind=tv: %v", err)
	}
}

func TestWatchlistRejectsBadKind(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	if err := s.WatchlistAdd(ctx, WatchlistEntry{TMDBID: 1, Kind: "podcast", Title: "x"}); err == nil {
		t.Fatalf("expected kind validation, got nil")
	}
	if err := s.WatchlistAdd(ctx, WatchlistEntry{TMDBID: 0, Kind: "movie", Title: "x"}); err == nil {
		t.Fatalf("expected tmdb_id validation, got nil")
	}
}

func TestWatchlistRemoveMissingIsNoop(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	if err := s.WatchlistRemove(ctx, "movie", 1); err != nil {
		t.Fatalf("Remove on empty store should be no-op: %v", err)
	}
}
