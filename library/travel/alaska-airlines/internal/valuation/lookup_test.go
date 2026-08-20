// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package valuation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLookup_OverrideShortCircuits(t *testing.T) {
	fetcherCalled := false
	res, err := Lookup(context.Background(), ProgramAtmos, LookupOptions{
		Override: 2.0,
		Fetcher: func(_ context.Context, _ ProgramDef) (float64, error) {
			fetcherCalled = true
			return 99.0, nil
		},
		CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}
	if res.CPPCents != 2.0 {
		t.Errorf("CPPCents = %v; want 2.0", res.CPPCents)
	}
	if res.Source != SourceOverride {
		t.Errorf("Source = %q; want %q", res.Source, SourceOverride)
	}
	if fetcherCalled {
		t.Errorf("fetcher was called; should not be when Override is set")
	}
}

func TestLookup_FreshCacheHit(t *testing.T) {
	dir := t.TempDir()
	rec := ValuationRecord{Program: ProgramAtmos, CPPCents: 1.4, SourceURL: TPGValuationsURL, FetchedAt: time.Now().Add(-time.Hour)}
	if err := NewCache(dir).Set(ProgramAtmos, rec); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	fetcherCalled := false
	res, err := Lookup(context.Background(), ProgramAtmos, LookupOptions{
		Fetcher: func(_ context.Context, _ ProgramDef) (float64, error) {
			fetcherCalled = true
			return 99.0, nil
		},
		CacheDir: dir,
	})
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}
	if res.CPPCents != 1.4 {
		t.Errorf("CPPCents = %v; want 1.4", res.CPPCents)
	}
	if res.Source != SourceTPGCached {
		t.Errorf("Source = %q; want %q", res.Source, SourceTPGCached)
	}
	if fetcherCalled {
		t.Errorf("fetcher was called; should not be on fresh cache hit")
	}
}

func TestLookup_CacheMissTriggersFetch(t *testing.T) {
	dir := t.TempDir()
	res, err := Lookup(context.Background(), ProgramAtmos, LookupOptions{
		Fetcher: func(_ context.Context, def ProgramDef) (float64, error) {
			if def.Slug != ProgramAtmos {
				t.Errorf("fetcher got def.Slug=%q; want %q", def.Slug, ProgramAtmos)
			}
			return 1.5, nil
		},
		CacheDir: dir,
	})
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}
	if res.CPPCents != 1.5 {
		t.Errorf("CPPCents = %v; want 1.5", res.CPPCents)
	}
	if res.Source != SourceTPGLive {
		t.Errorf("Source = %q; want %q", res.Source, SourceTPGLive)
	}
	// Confirm cache was written.
	rec, _, fresh, ok := NewCache(dir).Get(ProgramAtmos)
	if !ok || !fresh || rec.CPPCents != 1.5 {
		t.Errorf("cache not written: ok=%v fresh=%v cpp=%v", ok, fresh, rec.CPPCents)
	}
}

func TestLookup_StaleCacheFallbackOnFetchError(t *testing.T) {
	dir := t.TempDir()
	// Write a stale cache file (mtime 60 days ago).
	staleRec := ValuationRecord{Program: ProgramAtmos, CPPCents: 1.3, SourceURL: TPGValuationsURL, FetchedAt: time.Now().Add(-60 * 24 * time.Hour)}
	data, _ := json.MarshalIndent(staleRec, "", "  ")
	path := filepath.Join(dir, "atmos.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}
	staleTime := time.Now().Add(-60 * 24 * time.Hour)
	if err := os.Chtimes(path, staleTime, staleTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	res, err := Lookup(context.Background(), ProgramAtmos, LookupOptions{
		Fetcher: func(_ context.Context, _ ProgramDef) (float64, error) {
			return 0, ErrTPGBlocked
		},
		CacheDir: dir,
	})
	if err != nil {
		t.Fatalf("err = %v; want nil (soft-fallback)", err)
	}
	if res.CPPCents != 1.3 {
		t.Errorf("CPPCents = %v; want 1.3 (stale)", res.CPPCents)
	}
	if res.Source != SourceFallbackStale {
		t.Errorf("Source = %q; want %q", res.Source, SourceFallbackStale)
	}
	if res.Warning == nil {
		t.Errorf("Warning is nil; want non-nil on fallback")
	}
}

func TestLookup_NoCacheNoNetwork(t *testing.T) {
	dir := t.TempDir()
	res, err := Lookup(context.Background(), ProgramAtmos, LookupOptions{
		Fetcher: func(_ context.Context, _ ProgramDef) (float64, error) {
			return 0, ErrTPGFetch
		},
		CacheDir: dir,
	})
	if err != nil {
		t.Fatalf("err = %v; want nil (constant fallback)", err)
	}
	def, _ := BySlug(ProgramAtmos)
	if res.CPPCents != def.FallbackCPP {
		t.Errorf("CPPCents = %v; want %v (constant fallback)", res.CPPCents, def.FallbackCPP)
	}
	if res.Source != SourceFallbackConst {
		t.Errorf("Source = %q; want %q", res.Source, SourceFallbackConst)
	}
	if res.Warning == nil {
		t.Errorf("Warning is nil; want non-nil on constant fallback")
	}
}

func TestLookup_ForceRefreshSkipsCache(t *testing.T) {
	dir := t.TempDir()
	rec := ValuationRecord{Program: ProgramAtmos, CPPCents: 1.4, SourceURL: TPGValuationsURL, FetchedAt: time.Now()}
	if err := NewCache(dir).Set(ProgramAtmos, rec); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	fetcherCalled := false
	res, err := Lookup(context.Background(), ProgramAtmos, LookupOptions{
		ForceRefresh: true,
		Fetcher: func(_ context.Context, _ ProgramDef) (float64, error) {
			fetcherCalled = true
			return 1.6, nil
		},
		CacheDir: dir,
	})
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}
	if !fetcherCalled {
		t.Errorf("fetcher was NOT called; should be on ForceRefresh")
	}
	if res.CPPCents != 1.6 {
		t.Errorf("CPPCents = %v; want 1.6", res.CPPCents)
	}
	if res.Source != SourceTPGLive {
		t.Errorf("Source = %q; want %q", res.Source, SourceTPGLive)
	}
}

func TestLookup_NegativeOverrideErrors(t *testing.T) {
	// Negative --cpp is a caller mistake. Silently falling through to
	// the TPG lookup would hide the typo behind a tpg-live source
	// label. Surface as a hard error.
	_, err := Lookup(context.Background(), ProgramAtmos, LookupOptions{
		Override: -1.4,
		CacheDir: t.TempDir(),
	})
	if err == nil {
		t.Errorf("err = nil; want non-nil for negative override")
	}
}

func TestLookup_UnknownProgramErrors(t *testing.T) {
	_, err := Lookup(context.Background(), Program("not-a-program"), LookupOptions{
		CacheDir: t.TempDir(),
	})
	if err == nil {
		t.Errorf("err = nil; want non-nil for unknown program")
	}
}

// Sanity: ensure typed errors propagate via Warning so the CLI can
// stderr-log them.
func TestLookup_WarningWrapsFetchError(t *testing.T) {
	dir := t.TempDir()
	res, _ := Lookup(context.Background(), ProgramAtmos, LookupOptions{
		Fetcher: func(_ context.Context, _ ProgramDef) (float64, error) {
			return 0, ErrTPGBlocked
		},
		CacheDir: dir,
	})
	if res.Warning == nil || !errors.Is(res.Warning, ErrTPGBlocked) {
		t.Errorf("Warning = %v; want wrapping ErrTPGBlocked", res.Warning)
	}
}
