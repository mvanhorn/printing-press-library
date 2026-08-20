// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// defaultFuzzyThreshold is the Jaro-Winkler similarity bar used by the fuzzy
// clustering pass when --fuzzy-threshold is not set. Two canonical names cluster
// together only when their similarity is >= this value.
const defaultFuzzyThreshold = 0.92

// classifyOpts controls the classify pipeline run.
type classifyOpts struct {
	// ClassifierVersion is stamped onto every row written by this run.
	ClassifierVersion int
	// Fuzzy enables a second-pass Jaro-Winkler clustering of near-duplicate
	// canonical names into a shared canonical ID. Off by default so the
	// primary path is fully deterministic.
	Fuzzy bool
	// FuzzyThreshold is the Jaro-Winkler similarity bar for the fuzzy pass. A
	// zero (unset) value resolves to defaultFuzzyThreshold via fuzzyThreshold().
	FuzzyThreshold float64
}

// fuzzyThreshold returns the effective clustering threshold: the configured
// value when set to a positive number, otherwise defaultFuzzyThreshold. A value
// outside (0,1] is treated as unset so a stray 0 or a nonsensical bound cannot
// silently collapse every name into one cluster or disable clustering.
func (o classifyOpts) fuzzyThreshold() float64 {
	if o.FuzzyThreshold > 0 && o.FuzzyThreshold <= 1 {
		return o.FuzzyThreshold
	}
	return defaultFuzzyThreshold
}

// classifyResult summarises what the classify pipeline produced.
type classifyResult struct {
	// CanonicalCount is the number of distinct canonical forms seen across all
	// source values (matched and unmatched); equals the post-fuzzy cluster count
	// when Fuzzy is enabled.
	CanonicalCount int
	// Matched is the number of distinct raw source values that were classified.
	Matched int
	// Unmatched is the number of distinct raw source values that could not be
	// classified and were stored with method="unmatched".
	Unmatched int
}

// mintCanonicalID derives a stable, deterministic canonical ID from the entity
// type and the already-canonicalized name. The ID is a SHA-1 hex digest
// prefixed by the entity type so IDs from different entity types never collide,
// even if the canonical name is identical. Truncated to prefix + 12 hex chars
// to keep IDs compact while retaining negligible collision probability for
// realistic catalog sizes.
func mintCanonicalID(entityType, canonicalName string) string {
	h := sha1.Sum([]byte(canonicalName))
	hex := fmt.Sprintf("%x", h)
	return fmt.Sprintf("%s:%s", entityType, hex[:12])
}

// hardcodedTierEntity is the defensive fallback declaration for the ticket_type
// spine, used only if the embedded starter config somehow lacks the entity.
func hardcodedTierEntity() normalizecfg.Entity {
	return normalizecfg.Entity{
		Source:     "tickets.ticketType.name",
		Shape:      normalizecfg.ShapeAttributes,
		Attributes: []string{axisAccessClass, axisSalesStage, axisEntryWindowType, axisEntryWindowTime, axisGroupSize, axisCompFlag},
		Rules:      nil,
	}
}

// hardcodedVenueEntity is the defensive fallback declaration for the venue
// spine, used only if the embedded starter config somehow lacks the entity.
func hardcodedVenueEntity() normalizecfg.Entity {
	return normalizecfg.Entity{
		Source:     "events.venues[*].name",
		Shape:      normalizecfg.ShapeAttributes,
		Attributes: []string{axisComplex, axisRoom},
		Rules:      nil,
	}
}

// entityFromConfig resolves the declaration for entityType from the loaded
// normalization config (embedded starter overlaid by any operator config). If
// the config cannot be loaded or omits the entity, it falls back to fallback so
// the classify path stays robust even with a missing or malformed config.
func entityFromConfig(entityType string, fallback normalizecfg.Entity) normalizecfg.Entity {
	cfg, err := loadNormalizeConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: normalize config load failed (%v); using built-in defaults\n", err)
		return fallback
	}
	if ent, ok := cfg.Entities[entityType]; ok {
		return ent
	}
	return fallback
}

// classifyTiers classifies distinct ticketType.name values into the
// ticket_type spine. It sources the entity declaration (source path, attributes
// shape, axis columns, and any promoted rules) from the loaded normalization
// config and delegates to the generic classifyEntity. With the embedded
// starter's empty rules every non-manual tier name resolves unmatched until
// rules are promoted or the LLM-tail fills them.
func classifyTiers(ctx context.Context, s *store.Store, opts classifyOpts) (classifyResult, error) {
	ent := entityFromConfig("ticket_type", hardcodedTierEntity())
	return classifyEntity(ctx, s, "ticket_type", ent, opts)
}

// classifyVenues classifies distinct venue names into the venue spine. It
// sources the entity declaration from the loaded normalization config and
// delegates to the generic classifyEntity. With the embedded starter's empty
// rules every non-manual venue name resolves unmatched until rules are promoted
// or the LLM-tail fills them.
func classifyVenues(ctx context.Context, s *store.Store, opts classifyOpts) (classifyResult, error) {
	ent := entityFromConfig("venue", hardcodedVenueEntity())
	return classifyEntity(ctx, s, "venue", ent, opts)
}

// manualCrosswalkSet returns the set of source_value strings that have an
// existing method="manual" crosswalk row for the given entity type and source
// system. Used to gate re-classification so operator overrides survive a run.
func manualCrosswalkSet(_ context.Context, s *store.Store, entityType, sourceSystem string) (map[string]bool, error) {
	rows, err := s.ListCrosswalk(entityType, sourceSystem)
	if err != nil {
		return nil, fmt.Errorf("loading manual crosswalk: %w", err)
	}
	m := map[string]bool{}
	for _, r := range rows {
		if r.Method == methodManual {
			m[r.SourceValue] = true
		}
	}
	return m, nil
}

// countDistinctCanonicals returns the number of distinct canonical IDs in the
// map. When fuzzy is off, this equals the number of distinct canonical forms.
func countDistinctCanonicals(canonToID map[string]string, _ bool) int {
	seen := map[string]bool{}
	for _, id := range canonToID {
		seen[id] = true
	}
	return len(seen)
}
