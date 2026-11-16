// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
)

func openSerpTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "serp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSerpSnapshotRoundTrip(t *testing.T) {
	s := openSerpTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	key := SerpKey{Query: "Serp API Pricing", Engine: "Google", Country: "FR"}
	id1, err := s.SaveSerpSnapshot(ctx, SerpSnapshot{Query: key.Query, Engine: key.Engine, Country: key.Country, TakenAt: t0},
		[]serp.Result{{Position: 1, URL: "https://a.com", Domain: "a.com"}, {Position: 2, URL: "https://autom.dev", Domain: "autom.dev"}})
	if err != nil {
		t.Fatal(err)
	}
	id2, err := s.SaveSerpSnapshot(ctx, SerpSnapshot{Query: key.Query, Engine: key.Engine, Country: key.Country, TakenAt: t0.Add(7 * 24 * time.Hour)},
		[]serp.Result{{Position: 1, URL: "https://autom.dev", Domain: "autom.dev"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveSerpSnapshot(ctx, SerpSnapshot{Query: "", Engine: "google"}, nil); err == nil {
		t.Error("snapshot without query accepted")
	}

	list, err := s.SerpSnapshots(ctx, SerpKey{Query: "serp api pricing"}, 0)
	if err != nil || len(list) != 2 || list[0].ID != id2 {
		t.Fatalf("SerpSnapshots newest-first = %+v, %v", list, err)
	}
	if list[0].Engine != "google" || list[0].Country != "fr" {
		t.Errorf("engine/country not normalized: %+v", list[0])
	}

	latest, ok, err := s.LatestSerpSnapshot(ctx, key, time.Time{})
	if err != nil || !ok || latest.ID != id2 {
		t.Fatalf("latest = %+v %v %v", latest, ok, err)
	}
	before, ok, err := s.LatestSerpSnapshot(ctx, key, t0.Add(24*time.Hour))
	if err != nil || !ok || before.ID != id1 {
		t.Fatalf("latest before cutoff = %+v %v %v", before, ok, err)
	}
	if _, ok, _ := s.LatestSerpSnapshot(ctx, key, t0.Add(-time.Hour)); ok {
		t.Error("no snapshot should exist before the first one")
	}

	res, err := s.SerpResults(ctx, id1)
	if err != nil || len(res) != 2 || res[1].Domain != "autom.dev" {
		t.Fatalf("results = %+v %v", res, err)
	}
	keys, err := s.SerpKeys(ctx, "")
	if err != nil || len(keys) != 1 || keys[0].Query != "Serp API Pricing" {
		t.Fatalf("keys = %+v %v", keys, err)
	}
	if none, _ := s.SerpKeys(ctx, "de"); len(none) != 0 {
		t.Errorf("country filter ignored: %+v", none)
	}
	if n, _ := s.CountSerpSnapshots(ctx); n != 2 {
		t.Errorf("count = %d", n)
	}
}

func TestNewsSeen(t *testing.T) {
	s := openSerpTestStore(t)
	ctx := context.Background()
	art := serp.Result{URL: "https://news.com/a", Source: "News", Title: "Story"}
	keys := serp.NewsKeys(art)
	seen, err := s.NewsSeen(ctx, keys)
	if err != nil || seen {
		t.Fatalf("fresh store reports seen=%v err=%v", seen, err)
	}
	if err := s.MarkNewsSeen(ctx, "autom", art, keys, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkNewsSeen(ctx, "autom", art, keys, time.Now()); err != nil {
		t.Fatalf("marking twice must be idempotent: %v", err)
	}
	repub := serp.Result{URL: "https://news.com/a-v2", Source: "News", Title: "Story"}
	if seen, _ := s.NewsSeen(ctx, serp.NewsKeys(repub)); !seen {
		t.Error("same source+title under a new URL should count as seen")
	}
	other := serp.Result{URL: "https://news.com/b", Source: "News", Title: "Another"}
	if seen, _ := s.NewsSeen(ctx, serp.NewsKeys(other)); seen {
		t.Error("a different article reported as seen")
	}
}

func TestSerpSnapshotsSubSecondOrder(t *testing.T) {
	s := openSerpTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 11, 1, 9, 0, 5, 0, time.UTC)
	k := SerpKey{Query: "k", Engine: "google"}
	idA, _ := s.SaveSerpSnapshot(ctx, SerpSnapshot{Query: "k", Engine: "google", TakenAt: t0.Add(50 * time.Millisecond)}, nil)
	idB, _ := s.SaveSerpSnapshot(ctx, SerpSnapshot{Query: "k", Engine: "google", TakenAt: t0.Add(500 * time.Millisecond)}, nil)
	idC, _ := s.SaveSerpSnapshot(ctx, SerpSnapshot{Query: "k", Engine: "google", TakenAt: t0}, nil)
	list, err := s.SerpSnapshots(ctx, k, 0)
	if err != nil || len(list) != 3 {
		t.Fatalf("list = %+v %v", list, err)
	}
	if list[0].ID != idB || list[1].ID != idA || list[2].ID != idC {
		t.Errorf("pages saved within one second must sort newest first: %d %d %d", list[0].ID, list[1].ID, list[2].ID)
	}
	before, ok, err := s.LatestSerpSnapshot(ctx, k, t0.Add(100*time.Millisecond))
	if err != nil || !ok || before.ID != idA {
		t.Errorf("cutoff inside the second picked %+v %v %v, want snapshot %d", before, ok, err, idA)
	}
}

func TestSerpSnapshotsExact(t *testing.T) {
	s := openSerpTestStore(t)
	ctx := context.Background()
	idFR, _ := s.SaveSerpSnapshot(ctx, SerpSnapshot{Query: "k", Engine: "google", Country: "fr", TakenAt: time.Now().Add(-time.Hour)}, nil)
	idAny, _ := s.SaveSerpSnapshot(ctx, SerpSnapshot{Query: "K", Engine: "google", TakenAt: time.Now()}, nil)
	exact, err := s.SerpSnapshotsExact(ctx, SerpKey{Query: "k", Engine: "google"}, 0)
	if err != nil || len(exact) != 1 || exact[0].ID != idAny {
		t.Errorf("an empty country must only match country-less pages: %+v %v", exact, err)
	}
	fr, _ := s.SerpSnapshotsExact(ctx, SerpKey{Query: "K", Engine: "google", Country: "FR"}, 0)
	if len(fr) != 1 || fr[0].ID != idFR {
		t.Errorf("query case and country case must not matter: %+v", fr)
	}
}

func TestNewsScopeSeen(t *testing.T) {
	s := openSerpTestStore(t)
	ctx := context.Background()
	if seen, err := s.NewsScopeSeen(ctx, "autom|fr|"); err != nil || seen {
		t.Fatalf("fresh scope seen=%v err=%v", seen, err)
	}
	art := serp.Result{URL: "https://news.com/a", Source: "News", Title: "Story"}
	if err := s.MarkNewsSeen(ctx, "autom|fr|", art, []string{"autom|fr|#url:news.com/a"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if seen, _ := s.NewsScopeSeen(ctx, "autom|fr|"); !seen {
		t.Error("scope should be seen after marking")
	}
	if seen, _ := s.NewsScopeSeen(ctx, "serp api|fr|"); seen {
		t.Error("another scope must stay unseen")
	}
}
