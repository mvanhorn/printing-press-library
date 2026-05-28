// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 cookie -> free -> paid dispatcher.

package dispatch

import (
	"context"
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/acquired"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/dwarkesh"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/founders"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/huberman"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/peterattia"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/rss"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/spoken"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/spotify"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/taddy"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/whisperapi"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/youtube"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

// Options control dispatcher behavior.
type Options struct {
	AllowPaid        bool     // false by default; user must pass --paid (or --auto-paid + explicit confirm)
	AllowedProviders []string // empty = all; non-empty = only these adapters
	DryRun           bool     // explain-only; never fetch
	Explain          bool     // emit Trace even on success
}

// TraceEntry is one adapter's verdict in the dispatch walk.
type TraceEntry struct {
	Source  string  `json:"source"`
	Tier    string  `json:"tier"`
	Verdict string  `json:"verdict"` // match | skip | error | success
	Reason  string  `json:"reason"`
	CostUSD float64 `json:"cost_usd,omitempty"`
}

// DispatchResult is the dispatcher's return shape.
type DispatchResult struct {
	Transcript *transcript.Transcript `json:"transcript,omitempty"`
	Trace      []TraceEntry           `json:"trace"`
	FiredBy    string                 `json:"fired_by,omitempty"`
}

// Process-singleton adapter chain in priority order:
// 1) cookie publishers, 2) free, 3) paid.
//
// Reusing the same Adapter values across Dispatch calls is what lets stateful
// adapters cache bootstrapped credentials in-process. Spotify in particular
// derives a TOTP-signed Bearer once per sp_dc and reuses it for the bearer
// TTL (~1h); a fresh adapter on every call would defeat the cache and force
// a full bootstrap on every `episode get`.
var registered = []source.Adapter{
	huberman.New(),
	acquired.New(),
	founders.New(),
	peterattia.New(),
	dwarkesh.New(),
	rss.New(),
	youtube.New(),
	spotify.New(),
	spoken.New(),
	taddy.New(),
	whisperapi.New(),
}

// Registered returns the shared adapter chain. Returned slice values are
// process-singleton — callers must not mutate adapter fields.
func Registered() []source.Adapter { return registered }

// providerAllowed returns true if the named adapter is in the allow-list (or
// the allow-list is empty).
func providerAllowed(name string, allow []string) bool {
	if len(allow) == 0 {
		return true
	}
	for _, p := range allow {
		if p == name {
			return true
		}
	}
	return false
}

// Dispatch walks the registered adapter chain.
//
// Order of operations:
//  1. For every adapter, evaluate Match(url). Skip non-matches (record reason).
//  2. Skip paid-tier unless opts.AllowPaid.
//  3. Skip adapters not in opts.AllowedProviders.
//  4. The first match that's not gated runs (or, in DryRun, gets reported and we stop).
//  5. On adapter error: record verdict=error, continue to the next.
//  6. On adapter success: record verdict=success and return.
func Dispatch(ctx context.Context, url string, opts Options) (*DispatchResult, error) {
	adapters := Registered()
	result := &DispatchResult{}

	for _, a := range adapters {
		entry := TraceEntry{Source: a.Name(), Tier: string(a.Tier())}
		if !a.Match(url) {
			entry.Verdict = "skip"
			entry.Reason = "URL pattern does not match"
			result.Trace = append(result.Trace, entry)
			continue
		}
		if !providerAllowed(a.Name(), opts.AllowedProviders) {
			entry.Verdict = "skip"
			entry.Reason = "not in --provider allow-list"
			result.Trace = append(result.Trace, entry)
			continue
		}
		if a.Tier() == transcript.TierPaid && !opts.AllowPaid {
			entry.Verdict = "skip"
			entry.Reason = "paid tier; pass --paid (or --auto-paid) to enable"
			result.Trace = append(result.Trace, entry)
			continue
		}
		// First eligible match.
		if opts.DryRun {
			entry.Verdict = "match"
			entry.Reason = "would fire (dry-run)"
			result.Trace = append(result.Trace, entry)
			result.FiredBy = a.Name()
			return result, nil
		}
		tr, err := a.Fetch(ctx, url)
		if err != nil {
			entry.Verdict = "error"
			entry.Reason = err.Error()
			result.Trace = append(result.Trace, entry)
			// CookieMissing / NotApplicable / NotImplemented are recoverable:
			// fall through to next adapter.
			if isRecoverable(err) {
				continue
			}
			// Hard error: stop and return it.
			return result, err
		}
		entry.Verdict = "success"
		entry.Reason = fmt.Sprintf("fetched %d segments", len(tr.Segments))
		result.Trace = append(result.Trace, entry)
		result.Transcript = tr
		result.FiredBy = a.Name()
		return result, nil
	}
	// Walked the whole chain with no successful fetch. If a recoverable error
	// was the last actionable signal — e.g., a cookie-tier adapter matched
	// but had no cookie captured — surface that to the user instead of the
	// generic "no adapter matched". The trace already shows the full walk.
	for i := len(result.Trace) - 1; i >= 0; i-- {
		if result.Trace[i].Verdict == "error" {
			return result, fmt.Errorf("%s: %s", result.Trace[i].Source, result.Trace[i].Reason)
		}
	}
	return result, fmt.Errorf("no adapter matched %s", url)
}

func isRecoverable(err error) bool {
	var cm *source.CookieMissingError
	var km *source.KeyMissingError
	var na *source.NotApplicableError
	var ni *source.NotImplementedError
	switch {
	case errors.As(err, &cm),
		errors.As(err, &km),
		errors.As(err, &na),
		errors.As(err, &ni):
		return true
	}
	return false
}
