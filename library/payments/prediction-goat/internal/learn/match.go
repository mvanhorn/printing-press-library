// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// match.go implements the entity-aware match validator that turns
// raw Jaccard hits into structured EntityMatch verdicts. This is the
// guardrail that stops "odds England wins world cup" from returning a
// Portugal-tagged learning just because the non-entity tokens overlap.
//
// Design note: the validator is split from recall.go so the same
// classification function can be unit-tested in isolation, and so a
// future template-extraction plan can lift this file plus normalize.go
// plus entities/ into a generator package without dragging in the
// recall query path. Per the U3 section of
// docs/plans/2026-05-23-002-feat-prediction-goat-smart-learning-plan.md.

package learn

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/entities"
)

// EntityMatch classifies how well a stored learning's resource-side
// entities overlap with the query's entities. The recall layer uses
// this to decide whether a Jaccard-positive row belongs in `results`
// (exact / partial), in `mismatches` (debug-only surface for the
// LLM to see why a high-Jaccard candidate was dropped), or stays in
// `results` with an `unknown` warning so the LLM can decide whether
// to direct-fetch the ticker anyway.
//
// Values match the strings in the JSON envelope read by the LLM; do
// not rename without updating SKILL.md.
const (
	EntityMatchExact    = "exact"    // every query entity has a match on the resource side
	EntityMatchPartial  = "partial"  // either side has no entities; categorical match only
	EntityMatchMismatch = "mismatch" // both sides have entities AND no overlap
	EntityMatchUnknown  = "unknown"  // resource not in local store; entities can't be derived
)

// Per-hit warning constants. Kept as string constants so the recall
// envelope's `warnings` array stays a stable schema for the LLM. Add
// new warning kinds here rather than constructing free-form strings
// at call sites — that pattern caused the v3 surface to drift between
// SKILL.md and the actual warnings the CLI emitted.
const (
	WarningParentEventWhenChildExists = "parent_event_when_child_exists"
	WarningLowConfidence              = "low_confidence"
	WarningResourceNotInStore         = "resource_not_in_store"
	// WarningCrossAliasMatch is attached on a per-result Warnings slice
	// when EntityMatch was promoted from Mismatch to Exact via the
	// cross-alias canonical-overlap path. PATCH(learn-loop-backport U3):
	// ported from ESPN PR #851.
	WarningCrossAliasMatch = "cross_alias_match"
)

// Top-level recall envelope warnings.
const (
	TopWarningNoLearningsForQueryFamily = "no_learnings_for_query_family"
	// WarningSimilarShapeDifferentEntity is the envelope warning prefix
	// surfaced when stored rows share the query's structural shape but
	// resolve to a different canonical. Suffix is the alternative
	// canonical name. PATCH(learn-loop-backport U3): ported from ESPN
	// PR #851.
	WarningSimilarShapeDifferentEntity = "similar_shape_different_entity"
	// WarningAmbiguousAlias fires once on the envelope when ANY single
	// query entity resolves to more than one canonical. Multi-entity
	// queries where each entity resolves to a distinct canonical do
	// NOT trip this warning (per Greptile PR #851 round 2).
	WarningAmbiguousAlias = "ambiguous_alias"
)

// Jaccard returns the token-set Jaccard coefficient of two string
// slices. Tokens are compared case-insensitively after trimming
// whitespace. An empty slice on either side yields 0.0; identical
// non-empty sets yield 1.0.
//
// Why exported: U10 (recipe extraction) needs the same coefficient
// over the non-entity normalized form. Keeping one definition avoids
// two implementations drifting on threshold semantics.
func Jaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := tokenSet(a)
	setB := tokenSet(b)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	inter := 0
	for tok := range setA {
		if _, ok := setB[tok]; ok {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// JaccardTokens returns the same coefficient over space-separated
// token strings, the form NonEntityNormalized lands in on the
// search_learnings row. Splitting here keeps the recall SQL path
// from rebuilding token sets it already has as strings.
func JaccardTokens(a, b string) float64 {
	return Jaccard(strings.Fields(a), strings.Fields(b))
}

// ClassifyEntityMatch returns the EntityMatch verdict for a query's
// entities against a resource's entities. The classification is
// symmetric in semantics but not in API: queryEntities is the live
// input, resourceEntities is what the validator extracted from the
// stored resource's title/ticker/subtitle (or empty when the resource
// isn't in the local store).
//
// Rules (per the U3 decision matrix in
// docs/plans/2026-05-23-002-feat-prediction-goat-smart-learning-plan.md):
//
//	- both empty -> partial (no entity signal either way; categorical)
//	- query empty, resource has entities -> partial (query is a category
//	  question against an entity-tagged resource; the LLM may want it)
//	- query has entities, resource empty -> partial (resource is a hub
//	  or event-level page that doesn't carry a specific entity)
//	- both non-empty AND any overlap -> exact
//	- both non-empty AND zero overlap -> mismatch
//
// Comparison is case-insensitive. A query entity matches a resource
// entity when either string contains the other after lowercasing.
// Substring matching is deliberately permissive on the resource side:
// resource entities come from titles like "FIFA World Cup - USA" where
// the entity token "USA" may be embedded in a longer compound entity
// like "USA-Mexico-Canada" that the extractor produced as a single
// token. The query entity is the source of truth; the resource side
// is best-effort.
func ClassifyEntityMatch(queryEntities, resourceEntities []string) string {
	qEmpty := len(queryEntities) == 0
	rEmpty := len(resourceEntities) == 0
	if qEmpty && rEmpty {
		// Both lack entity signal. The match (if any) is purely on
		// non-entity Jaccard. Treat as partial so the LLM sees the
		// result but knows to verify before acting.
		return EntityMatchPartial
	}
	if qEmpty || rEmpty {
		// One side has entities, the other doesn't. Categorical match;
		// keep but flag partial.
		return EntityMatchPartial
	}
	// Both sides have entities. Look for any overlap.
	for _, q := range queryEntities {
		ql := strings.ToLower(strings.TrimSpace(q))
		if ql == "" {
			continue
		}
		for _, r := range resourceEntities {
			rl := strings.ToLower(strings.TrimSpace(r))
			if rl == "" {
				continue
			}
			if ql == rl || strings.Contains(rl, ql) || strings.Contains(ql, rl) {
				return EntityMatchExact
			}
		}
	}
	return EntityMatchMismatch
}

// ResourceEntities pulls entity tokens from a stored resource's
// content fields. The dispatch is per resource_type because each
// table stores its identity-bearing fields under different keys:
// Polymarket markets carry `question` + `title` + `slug`, Kalshi
// markets carry `title` + `yes_sub_title` + `ticker`, etc.
//
// Returns nil when:
//
//	- data is empty (resource not found in store; caller should mark
//	  the hit EntityMatchUnknown and attach WarningResourceNotInStore)
//	- the JSON shape doesn't carry any of the entity-bearing fields
//	  for the supplied resource_type
//
// The extractor used here is the prediction-goat config from
// normalize.go (DefaultPredictionGoatConfig), so Kalshi tickers like
// KXMENWORLDCUP-26-US correctly classify as Tickers and not as
// ALL-CAPS entities. Including Tickers in the returned slice is
// load-bearing: a query for "KXMENWORLDCUP-26-US" must match the
// resource whose ticker IS that string, even though it's a ticker
// rather than an entity on the query side.
func ResourceEntities(resourceType string, data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	fields := entityFieldsFor(resourceType)
	if len(fields) == 0 {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		raw, ok := obj[field]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		s = strings.TrimSpace(s)
		if s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	// Run the same prediction-goat extractor used by the live query
	// path. Joining with spaces is safe because trimPunct in the
	// extractor strips sentence-boundary punctuation; joined whitespace
	// becomes a single token-break.
	parsed := entities.Extract(strings.Join(parts, " "), DefaultPredictionGoatConfig())
	out := make([]string, 0, len(parsed.Entities)+len(parsed.Tickers))
	out = append(out, parsed.Entities...)
	out = append(out, parsed.Tickers...)
	return out
}

// entityFieldsFor returns the JSON field keys whose values carry
// identity-bearing tokens for a given resource_type. Field order
// matters: the first non-empty field is the most title-like and goes
// first in the joined extractor input.
//
// Why a switch instead of a registry: the resource_type set is small
// and stable (six types as of v4); a function-table dispatch would
// add an indirection without enabling extension we'd actually use.
// When U10's recipe layer adds new resource types this switch grows
// by one case per type — that's the right level of friction.
func entityFieldsFor(resourceType string) []string {
	switch resourceType {
	case "markets":
		// Polymarket market. question is the human-readable title.
		return []string{"question", "title", "slug"}
	case "kalshi_markets":
		return []string{"title", "yes_sub_title", "ticker"}
	case "kalshi_events":
		return []string{"title", "event_ticker", "series_ticker"}
	case "kalshi_series":
		return []string{"title", "ticker"}
	case "events":
		// Polymarket event hub.
		return []string{"title", "slug"}
	case "tags":
		return []string{"label", "slug"}
	default:
		return nil
	}
}

// IsKalshiParentTicker reports whether a Kalshi resource_id looks
// like an event-level ticker (e.g., KXMENWORLDCUP-26) rather than a
// market-level ticker (e.g., KXMENWORLDCUP-26-US). Parent tickers
// have at most one hyphen separator; market tickers have at least
// two segments because the per-outcome suffix follows the event-date
// suffix.
//
// Why this matters: the U3 warning WarningParentEventWhenChildExists
// fires when a recall result points at a parent event whose specific
// child market would be a better target for the query's entity. This
// helper is the first-pass classifier; the warning logic in
// recall.go does the secondary "does a matching child exist" check.
func IsKalshiParentTicker(ticker string) bool {
	if !strings.HasPrefix(ticker, "KX") {
		return false
	}
	hyphens := strings.Count(ticker, "-")
	return hyphens <= 1
}

// ParseStoredEntities decodes the JSON array stored in
// search_learnings.query_entities into a Go slice. Empty / "null" /
// "[]" all return nil. Used by recall.go when reading stored
// learnings; surfaced for tests so they can round-trip the same
// shape the v3->v4 migration writes.
func ParseStoredEntities(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse stored entities: %w", err)
	}
	return out, nil
}

// tokenSet builds a lowercased deduped set from a slice. Helper for
// Jaccard; exposed package-locally so tests can verify set semantics
// without re-deriving them.
func tokenSet(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		out[t] = struct{}{}
	}
	return out
}
