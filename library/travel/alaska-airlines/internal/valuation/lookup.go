// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-05-20: value-compare) — Lookup composes the program
// registry, the TPG fetcher, and the cache into a soft-fallback chain:
//
//   override (--cpp) → fresh cache → live TPG fetch → stale cache → constant
//
// Every branch returns a Source string so the comparator's meta
// envelope can surface where the baseline came from.

package valuation

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Source values surfaced in Result.Source. These match the strings the
// CLI emits in the comparator envelope's meta.cpp_baseline_source field.
const (
	SourceOverride        = "override"
	SourceTPGLive         = "tpg-live"
	SourceTPGCached       = "tpg-cached"
	SourceFallbackStale   = "fallback-stale"
	SourceFallbackConst   = "fallback-constant"
)

// LookupOptions controls the fallback chain. Zero-value is the normal
// "cache-then-fetch" path; Override short-circuits to the user-supplied
// value; ForceRefresh skips the cache-hit branch.
type LookupOptions struct {
	// Override, when non-zero, returns immediately with the given value
	// and Source=SourceOverride. Used by the --cpp flag.
	Override float64
	// ForceRefresh skips the fresh-cache branch and always attempts the
	// live fetch. Used by --no-valuation-cache.
	ForceRefresh bool
	// Now is the time reference used for cache-freshness decisions.
	// Defaults to time.Now() when zero.
	Now time.Time
	// Fetcher is the function used to fetch from TPG. Defaults to
	// FetchTPGValuation; tests inject a stub.
	Fetcher func(ctx context.Context, def ProgramDef) (float64, error)
	// CacheDir overrides the default cache directory. Tests set this to
	// a t.TempDir().
	CacheDir string
}

// Result is the outcome of a Lookup call.
type Result struct {
	// CPPCents is the cents-per-point value to use.
	CPPCents float64
	// Source is one of the Source* constants above.
	Source string
	// FetchedAt is the timestamp the value was originally fetched from
	// TPG (or the override/fallback fired). For Source=SourceOverride
	// this is the time of the override call; for SourceFallbackConst
	// this is the time of the lookup.
	FetchedAt time.Time
	// Warning carries the wrapped error that caused a fallback to fire,
	// if any. The CLI surfaces this on stderr but does not fail the
	// command.
	Warning error
}

// Lookup returns the cents-per-point for the given program, applying the
// soft-fallback chain. It never returns a non-nil error — failures
// degrade to a Warning + a usable Source. Callers should always get a
// value back; they only need to inspect Source/Warning if they want to
// surface staleness to the user.
func Lookup(ctx context.Context, p Program, opts LookupOptions) (Result, error) {
	def, ok := BySlug(p)
	if !ok {
		// Unknown program: caller passed something not in the
		// registry. This is a programmer error, not a runtime fallback
		// case; surface it as a hard error so the CLI exits non-zero.
		return Result{}, fmt.Errorf("unknown program %q (known: %v)", p, Slugs())
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	// Branch 1: user override.
	//
	// Negative --cpp is a caller mistake worth surfacing — silently
	// falling through to the TPG chain would emit cpp_baseline_source
	// = tpg-live and hide the typo. Override == 0 still means "no
	// override; look up TPG" per the --cpp flag help string.
	if opts.Override != 0 {
		if opts.Override < 0 {
			return Result{}, fmt.Errorf("invalid override cpp %.4f: must be positive", opts.Override)
		}
		return Result{
			CPPCents:  opts.Override,
			Source:    SourceOverride,
			FetchedAt: now,
		}, nil
	}

	cacheDir := opts.CacheDir
	if cacheDir == "" {
		dir, err := DefaultCacheDir()
		if err != nil {
			cacheDir = ""
		} else {
			cacheDir = dir
		}
	}
	var cache *Cache
	if cacheDir != "" {
		cache = NewCache(cacheDir)
	}

	// Branch 2: fresh cache hit (unless ForceRefresh).
	if !opts.ForceRefresh && cache != nil {
		if rec, _, fresh, ok := cache.Get(p); ok && fresh {
			return Result{
				CPPCents:  rec.CPPCents,
				Source:    SourceTPGCached,
				FetchedAt: rec.FetchedAt,
			}, nil
		}
	}

	// Branch 3: live TPG fetch.
	fetcher := opts.Fetcher
	if fetcher == nil {
		fetcher = FetchTPGValuation
	}
	cpp, fetchErr := fetcher(ctx, def)
	if fetchErr == nil {
		fetchedAt := now
		if cache != nil {
			rec := ValuationRecord{
				Program:   p,
				CPPCents:  cpp,
				SourceURL: TPGValuationsURL,
				FetchedAt: fetchedAt,
			}
			// Best-effort persist; if writing the cache file fails,
			// still return the live value with a Warning.
			if err := cache.Set(p, rec); err != nil {
				return Result{
					CPPCents:  cpp,
					Source:    SourceTPGLive,
					FetchedAt: fetchedAt,
					Warning:   fmt.Errorf("persist cache: %w", err),
				}, nil
			}
		}
		return Result{
			CPPCents:  cpp,
			Source:    SourceTPGLive,
			FetchedAt: fetchedAt,
		}, nil
	}

	// Branch 4: stale cache fallback.
	if cache != nil {
		if rec, _, _, ok := cache.Get(p); ok {
			return Result{
				CPPCents:  rec.CPPCents,
				Source:    SourceFallbackStale,
				FetchedAt: rec.FetchedAt,
				Warning:   fmt.Errorf("live TPG fetch failed, using stale cached value: %w", fetchErr),
			}, nil
		}
	}

	// Branch 5: constant fallback. Never empty — every registered
	// program declares a FallbackCPP.
	if def.FallbackCPP <= 0 {
		// Defensive: if we ever ship a program with no fallback, that's
		// a coding error worth surfacing as a hard error.
		return Result{}, errors.New("no fallback cpp available")
	}
	return Result{
		CPPCents:  def.FallbackCPP,
		Source:    SourceFallbackConst,
		FetchedAt: now,
		Warning:   fmt.Errorf("live TPG fetch failed and no cache present, using constant fallback %.2f: %w", def.FallbackCPP, fetchErr),
	}, nil
}
