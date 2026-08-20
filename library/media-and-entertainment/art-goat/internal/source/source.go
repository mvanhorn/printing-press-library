// Package source defines the Source interface art-goat uses to aggregate
// content from museum and astronomy APIs into a unified local works table.
//
// Each source is a hand-authored client living under internal/source/<slug>/.
// MVP ships two: AIC (Art Institute of Chicago) and APOD (NASA's Astronomy
// Picture of the Day). The interface is intentionally narrow so additional
// sources (Met, Cleveland, Rijksmuseum, etc.) can be added in follow-up
// PRs without touching the unified schema or the contemplative-spine
// commands.
package source

import (
	"context"
	"time"
)

// Work is the unified record art-goat stores per piece. The fields are
// lossy across sources by design — the contemplative spine doesn't care
// which museum a piece came from, just what it is. Source-specific
// metadata stays in RawJSON for future extraction without re-syncing.
type Work struct {
	ID               string // composite "<source>:<source_id>"
	Source           string // "aic" | "apod" | ...
	SourceID         string // native ID inside the source
	Title            string
	Creator          string    // display form, e.g., "Katsushika Hokusai"
	CreatorCanonical string    // normalized lowercase form for cross-source match
	DateText         string    // display form, e.g., "ca. 1830-32"
	DateStart        int       // sortable year, 0 if unknown
	DateEnd          int       // sortable year, 0 if unknown
	Medium           string    // e.g., "Woodblock print"
	Classification   string    // e.g., "Prints"
	Period           string    // e.g., "Edo period"
	CultureRegion    string    // "Japan" | "Europe" | "Cosmos" | etc.
	Description      string    // curator-written, may be empty
	ImageURL         string    // full-resolution image
	ThumbnailURL     string    // smaller variant when available
	License          string    // "CC0" | "Public domain" | source-specific terms
	SourceURL        string    // museum/source page for this work
	RawJSON          string    // full original record for forward compatibility
	SyncedAt         time.Time // when art-goat fetched this record
}

// Source is the interface every art-goat source client implements.
// Sync is the bulk-populate path that reads from the upstream and emits
// normalized Work records. Bounded sources (APOD, daily) may return the
// full collection; large sources (AIC) honor opts.Limit to keep first-sync
// fast.
type Source interface {
	// Name returns the short slug, e.g., "aic" or "apod".
	Name() string

	// Description returns a one-line human-readable description for the
	// `sources` command output.
	Description() string

	// AuthRequired reports whether this source needs a user-supplied key.
	// Sources that work anonymously (AIC) or with a built-in DEMO_KEY
	// (APOD) return false. The art-goat auth wizard reads this to decide
	// which sources to prompt about.
	AuthRequired() bool

	// Sync fetches Work records from the upstream API and returns them
	// to the caller. The caller is responsible for upserting into the
	// store. Honors ctx cancellation and opts.Limit when set.
	Sync(ctx context.Context, opts SyncOpts) ([]Work, error)
}

// SyncOpts controls how much a Sync call pulls. Limit=0 means "use the
// source's natural curated default" — for AIC, that's the highlights
// subset (~10-30k works); for APOD, the full archive (~10k entries).
// Limit>0 caps regardless of source.
type SyncOpts struct {
	Limit int
	Full  bool // when true, ignore the curated-default ceiling
}
