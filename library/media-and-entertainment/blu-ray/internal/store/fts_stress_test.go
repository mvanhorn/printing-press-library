package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// TestMigrateBluRayCatalogIdempotentRemigrate exercises the ensureReleasesFTS
// `default` branch — the path that runs on EVERY CLI invocation after the first
// migrate (search/sync/export/watch all call MigrateBluRayCatalog). A second
// migrate of a healthy store must be a no-op: no rebuild, no double-posting, DDL
// unchanged.
func TestMigrateBluRayCatalogIdempotentRemigrate(t *testing.T) {
	ctx := context.Background()
	s := testBluRayStore(t)

	if err := s.UpsertCatalogRows(ctx, []CatalogRow{{ID: 52, Kind: "bluray", Slug: "x-blu-ray", TitleNormalized: "alpha unique", Country: "US", YearHint: 2000}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var ddlBefore string
	if err := s.DB().QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='releases_fts'`).Scan(&ddlBefore); err != nil {
		t.Fatalf("read fts ddl: %v", err)
	}

	// Re-run migrate on the already-migrated (healthy) store.
	if err := s.MigrateBluRayCatalog(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var ddlAfter string
	if err := s.DB().QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='releases_fts'`).Scan(&ddlAfter); err != nil {
		t.Fatalf("read fts ddl after: %v", err)
	}
	if ddlAfter != ddlBefore {
		t.Fatalf("re-migrate changed FTS DDL:\n before=%q\n after =%q", ddlBefore, ddlAfter)
	}
	if strings.Contains(strings.ToLower(ddlAfter), "distributor") {
		t.Fatalf("FTS DDL unexpectedly contains 'distributor': %q", ddlAfter)
	}

	// An UPDATE after the re-migrate must still produce exactly one posting (no
	// duplicate/stale from a spurious re-create on the default branch).
	if err := s.UpsertCatalogRows(ctx, []CatalogRow{{ID: 52, Kind: "bluray", Slug: "x-blu-ray", TitleNormalized: "beta unique", Country: "US", YearHint: 2001}}); err != nil {
		t.Fatalf("post-remigrate update: %v", err)
	}
	stale, err := s.SearchCatalog(ctx, CatalogSearchOpts{Query: "alpha", Limit: 10})
	if err != nil {
		t.Fatalf("search alpha: %v", err)
	}
	for _, r := range stale {
		if r.ID == 52 {
			t.Fatalf("stale 'alpha' posting after re-migrate+update: %#v", stale)
		}
	}
	got, err := s.SearchCatalog(ctx, CatalogSearchOpts{Query: "beta", Limit: 10})
	if err != nil {
		t.Fatalf("search beta: %v", err)
	}
	if len(got) != 1 || got[0].ID != 52 {
		t.Fatalf("post-remigrate 'beta' search = %#v, want exactly id 52", got)
	}
}

// TestReleasesFTSUpdateTitleOnly isolates the literal "update drift" case: only
// title_normalized changes (slug held constant) — the bare title-drift the
// BLOCKER is named for.
func TestReleasesFTSUpdateTitleOnly(t *testing.T) {
	ctx := context.Background()
	s := testBluRayStore(t)
	if err := s.UpsertCatalogRows(ctx, []CatalogRow{{ID: 52, Kind: "bluray", Slug: "stable-slug-blu-ray", TitleNormalized: "Alphaword", Country: "US", YearHint: 2000}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Same slug, new title only.
	if err := s.UpsertCatalogRows(ctx, []CatalogRow{{ID: 52, Kind: "bluray", Slug: "stable-slug-blu-ray", TitleNormalized: "Betaword", Country: "US", YearHint: 2000}}); err != nil {
		t.Fatalf("title-only update: %v", err)
	}
	stale, err := s.SearchCatalog(ctx, CatalogSearchOpts{Query: "Alphaword", Limit: 10})
	if err != nil {
		t.Fatalf("search old title: %v", err)
	}
	for _, r := range stale {
		if r.ID == 52 {
			t.Fatalf("stale title-only posting: 'Alphaword' still returns id 52: %#v", stale)
		}
	}
	got, err := s.SearchCatalog(ctx, CatalogSearchOpts{Query: "Betaword", Limit: 10})
	if err != nil {
		t.Fatalf("search new title: %v", err)
	}
	if len(got) != 1 || got[0].ID != 52 {
		t.Fatalf("new title search = %#v, want id 52", got)
	}
}

// TestFTSStress_NoStalePostingUnderPoolRotation hammers update+search so the
// 2-connection pool rotates between the write and the subsequent read, trying
// to surface any FTS5 external-content cross-connection visibility staleness.
func TestFTSStress_NoStalePostingUnderPoolRotation(t *testing.T) {
	ctx := context.Background()
	s := testBluRayStore(t)
	const N = 600
	prev := ""
	for i := 0; i < N; i++ {
		cur := fmt.Sprintf("uniqtoken%dz", i)
		if err := s.UpsertCatalogRows(ctx, []CatalogRow{{
			ID: 52, Kind: "bluray", Slug: "x-blu-ray", TitleNormalized: cur, Country: "US", YearHint: 2000,
		}}); err != nil {
			t.Fatalf("iter %d upsert: %v", i, err)
		}
		if prev != "" {
			stale, err := s.SearchCatalog(ctx, CatalogSearchOpts{Query: prev, Limit: 10})
			if err != nil {
				t.Fatalf("iter %d search prev: %v", i, err)
			}
			for _, r := range stale {
				if r.ID == 52 {
					t.Fatalf("iter %d STALE: search %q still returned id 52 (current title %q)", i, prev, cur)
				}
			}
		}
		got, err := s.SearchCatalog(ctx, CatalogSearchOpts{Query: cur, Limit: 10})
		if err != nil {
			t.Fatalf("iter %d search cur: %v", i, err)
		}
		if len(got) != 1 || got[0].ID != 52 {
			t.Fatalf("iter %d current %q search = %#v, want id 52", i, cur, got)
		}
		prev = cur
	}
}

// TestFTSStress_ConcurrentReaderWriter runs a writer rotating the title and a
// concurrent reader, the worst case for a pooled FTS visibility seam.
func TestFTSStress_ConcurrentReaderWriter(t *testing.T) {
	ctx := context.Background()
	s := testBluRayStore(t)
	if err := s.UpsertCatalogRows(ctx, []CatalogRow{{ID: 52, Kind: "bluray", Slug: "x-blu-ray", TitleNormalized: "sharedtok start0z", Country: "US", YearHint: 2000}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	const N = 500
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < N; i++ {
			cur := fmt.Sprintf("sharedtok ctoken%dz", i)
			if err := s.UpsertCatalogRows(ctx, []CatalogRow{{ID: 52, Kind: "bluray", Slug: "x-blu-ray", TitleNormalized: cur, Country: "US", YearHint: 2000}}); err != nil {
				t.Errorf("writer iter %d: %v", i, err)
				return
			}
		}
	}()
	// Reader: "sharedtok" matches id 52's CURRENT posting; an external-content FTS
	// with one posting per rowid must return id 52 at most once. >1 means a stale
	// or duplicate posting leaked under pool rotation.
	for i := 0; i < N*2; i++ {
		got, err := s.SearchCatalog(ctx, CatalogSearchOpts{Query: "sharedtok", Limit: 50})
		if err != nil {
			t.Fatalf("reader iter %d: %v", i, err)
		}
		count52 := 0
		for _, r := range got {
			if r.ID == 52 {
				count52++
			}
		}
		if count52 > 1 {
			t.Fatalf("reader iter %d: id 52 has %d FTS postings (duplicate/stale)", i, count52)
		}
	}
	wg.Wait()
}
