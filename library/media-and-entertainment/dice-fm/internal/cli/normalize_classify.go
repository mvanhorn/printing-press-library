// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// parseGroupSizeAxis parses an axis integer token, returning 0 when the token
// is empty or not a valid integer. Like its parseBoolAxis sibling it swallows
// the parse error so a missing or malformed group_size axis stores 0 rather
// than failing the run.
func parseGroupSizeAxis(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// classifyEntity is the single config-driven classifier that drives ANY entity
// from its normalizecfg.Entity declaration. It generalizes the former
// hand-written classifyTiers/classifyVenues bodies, parameterized by entityType
// and the entity's source path, shape, vocabulary, and promoted rules.
//
// It is also the production realization of the alias escalation ladder:
//   - rung 1 (manual): source values with an existing method=manual crosswalk
//     are skipped (manualCrosswalkSet) and preserved, so operator overrides are
//     never overwritten by a derived classification.
//   - rung 2 (canonical): every remaining value is canonicalized and minted to a
//     deterministic canonical ID via mintCanonicalID; two raw values that
//     canonicalize identically share an ID without any store lookup.
//   - rung 3 (fuzzy): the optional opts.Fuzzy batch pass (fuzzyRemap) clusters
//     near-duplicate canonical names and remaps their crosswalk rows to a shared
//     lexicographically-smallest representative.
//
// The per-shape overlays (attributes/vocab/alias) are then applied on top of the
// ladder per the entity's declared shape; the resolution runs as a single batch
// over all distinct source values rather than per-value.
//
// Behavior by shape:
//   - attributes (or empty shape, treated as alias-core): apply the entity's
//     attribute rules; on a match write the typed attribute row routed by
//     entityType plus a "regex" crosswalk; on no match write an "unmatched"
//     crosswalk for the LLM-tail.
//   - vocab: map the canonicalized value against the controlled set; known →
//     "regex" crosswalk, unknown → "unmatched".
//   - alias: every value resolves to a canonical entity ("canonical" crosswalk);
//     there is no unmatched case for a pure alias spine.
//
// Derived rows (method != "manual") are cleared at the start of each run so
// stale entries do not linger; method="manual" rows are preserved so operator
// overrides survive re-runs.
func classifyEntity(ctx context.Context, s *store.Store, entityType string, ent normalizecfg.Entity, opts classifyOpts) (classifyResult, error) {
	if err := s.ClearNormalization(entityType); err != nil {
		return classifyResult{}, fmt.Errorf("clearing stale %s normalization: %w", entityType, err)
	}

	raws, err := distinctSourceValues(ctx, s.DB(), ent.Source)
	if err != nil {
		return classifyResult{}, fmt.Errorf("reading %s source values: %w", entityType, err)
	}

	// Build a map from canonical form to canonical ID so two raw values that
	// canonicalize identically share the same ID.
	canonToID := map[string]string{}
	var res classifyResult

	// Collect existing manual crosswalk entries so we can skip them.
	manual, err := manualCrosswalkSet(ctx, s, entityType, "dice")
	if err != nil {
		return classifyResult{}, err
	}

	// Optional value transform: compile the entity's strip_pattern once and
	// reuse it across every value. Matches are removed from the raw value
	// before canonicalization (e.g. a namespace prefix), so two values that
	// differ only by the stripped span fold to one canonical id. The crosswalk
	// SourceValue stays the raw value (see writeCrosswalk callers below).
	var stripRE *regexp.Regexp
	if ent.StripPattern != "" {
		stripRE, err = regexp.Compile(ent.StripPattern)
		if err != nil {
			return classifyResult{}, fmt.Errorf("compiling %s strip_pattern %q: %w", entityType, ent.StripPattern, err)
		}
	}

	// Compile the entity's attribute rules ONCE for reuse across every source
	// value, rather than recompiling each rule's regexp per value in the inner
	// loop. Uncompilable rules are warned-and-skipped (config-load validation
	// already rejects them; this is the defense-in-depth diagnostic).
	crules := compileRules(entityType, ent.Rules, os.Stderr)

	for _, raw := range raws {
		if manual[raw] {
			continue
		}
		canon := effectiveCanon(raw, stripRE)
		cid, ok := canonToID[canon]
		if !ok {
			cid = mintCanonicalID(entityType, canon)
			canonToID[canon] = cid
		}

		switch ent.Shape {
		case normalizecfg.ShapeVocab:
			if err := classifyVocabValue(s, entityType, ent, raw, cid, canon, opts, &res); err != nil {
				return classifyResult{}, err
			}
		case normalizecfg.ShapeAlias:
			if err := classifyAliasValue(s, entityType, raw, cid, canon, opts, &res); err != nil {
				return classifyResult{}, err
			}
		default:
			// attributes and the empty shape ("", alias-core) both run the
			// attribute overlay; with empty rules every value resolves
			// unmatched until rules are promoted or the LLM-tail fills them.
			if err := classifyAttributesValue(s, entityType, crules, raw, cid, canon, opts, &res); err != nil {
				return classifyResult{}, err
			}
		}
	}

	// Optional fuzzy pass: cluster near-duplicate canonical names and remap
	// their crosswalk entries to a shared representative canonical ID.
	if opts.Fuzzy && len(canonToID) > 1 {
		if err := fuzzyRemap(s, entityType, canonToID, opts); err != nil {
			return classifyResult{}, err
		}
	}

	res.CanonicalCount = countDistinctCanonicals(canonToID, opts.Fuzzy)
	return res, nil
}

// effectiveCanon returns the canonicalization input for a raw value: when re is
// non-nil, regexp matches are stripped from raw first (e.g. a namespace prefix)
// so values differing only by the stripped span fold together; otherwise the
// raw value is canonicalized directly. The raw value itself is untouched and
// remains the crosswalk SourceValue.
func effectiveCanon(raw string, re *regexp.Regexp) string {
	if re != nil {
		raw = re.ReplaceAllString(raw, "")
	}
	return canonicalizeName(raw)
}

// writeCrosswalk upserts a single source-system="dice" crosswalk row, sharing
// the row construction across the matched/unmatched/alias classify branches.
// Callers wrap the returned error with branch-specific context.
func writeCrosswalk(s *store.Store, entityType, raw, cid, method string, opts classifyOpts) error {
	return s.UpsertCrosswalk(store.CrosswalkRow{
		EntityType:        entityType,
		SourceSystem:      "dice",
		SourceValue:       raw,
		CanonicalID:       cid,
		Method:            method,
		ClassifierVersion: opts.ClassifierVersion,
	})
}

// classifyAttributesValue applies the entity's precompiled attribute rules to
// one canonicalized value and writes the matched/unmatched outcome. On a match
// it writes the typed attribute row routed by entityType.
func classifyAttributesValue(s *store.Store, entityType string, rules []compiledRule, raw, cid, canon string, opts classifyOpts, res *classifyResult) error {
	axisMap := applyAttributesOverlay(canon, rules)
	if len(axisMap) == 0 {
		if err := writeCrosswalk(s, entityType, raw, cid, methodUnmatched, opts); err != nil {
			return fmt.Errorf("upsert crosswalk (unmatched): %w", err)
		}
		res.Unmatched++
		return nil
	}

	if err := s.UpsertCanonicalEntity(entityType, cid, canon); err != nil {
		return fmt.Errorf("upsert canonical entity: %w", err)
	}
	if err := writeAttributeRow(s, entityType, cid, axisMap, opts); err != nil {
		return err
	}
	if err := writeCrosswalk(s, entityType, raw, cid, methodRule, opts); err != nil {
		return fmt.Errorf("upsert crosswalk (matched): %w", err)
	}
	res.Matched++
	return nil
}

// writeAttributeRow routes a matched axis map to the typed attribute table for
// the entity type. ticket_type and venue keep their dedicated typed tables
// (authoritative for the existing query paths). Any other entity type persists
// each axis key/value generically into entity_attributes so attributes-shaped
// entities beyond tier/venue retain their classified attributes.
func writeAttributeRow(s *store.Store, entityType, cid string, axisMap map[string]string, opts classifyOpts) error {
	switch entityType {
	case "ticket_type":
		if err := s.UpsertTierAttributes(cid, store.TierAttributesRow{
			CanonicalID:       cid,
			AccessClass:       axisMap[axisAccessClass],
			SalesStage:        axisMap[axisSalesStage],
			EntryWindowType:   axisMap[axisEntryWindowType],
			EntryWindowTime:   axisMap[axisEntryWindowTime],
			GroupSize:         parseGroupSizeAxis(axisMap[axisGroupSize]),
			CompFlag:          parseBoolAxis(axisMap[axisCompFlag]),
			ClassifierVersion: opts.ClassifierVersion,
			Method:            methodRule,
		}); err != nil {
			return fmt.Errorf("upsert tier attributes: %w", err)
		}
	case "venue":
		if err := s.UpsertVenueAttributes(cid, store.VenueAttributesRow{
			CanonicalID:       cid,
			Complex:           axisMap[axisComplex],
			Room:              axisMap[axisRoom],
			ClassifierVersion: opts.ClassifierVersion,
			Method:            methodRule,
		}); err != nil {
			return fmt.Errorf("upsert venue attributes: %w", err)
		}
	default:
		// Generic attribute storage for any other entity type. Iterate keys in
		// sorted order so writes (and any error attribution) are deterministic.
		keys := make([]string, 0, len(axisMap))
		for k := range axisMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if err := s.UpsertEntityAttribute(cid, entityType, k, axisMap[k], methodRule, opts.ClassifierVersion); err != nil {
				return fmt.Errorf("upsert entity attribute %q for %s: %w", k, entityType, err)
			}
		}
	}
	return nil
}

// classifyVocabValue maps one canonicalized value against the entity's
// controlled vocabulary. A known member writes the canonical entity (named by
// the mapped vocab value) plus a "regex" crosswalk; an unknown value writes an
// "unmatched" crosswalk for the LLM-tail.
func classifyVocabValue(s *store.Store, entityType string, ent normalizecfg.Entity, raw, cid, canon string, opts classifyOpts, res *classifyResult) error {
	v, known := mapVocab(canon, ent.Vocab)
	if !known {
		if err := writeCrosswalk(s, entityType, raw, cid, methodUnmatched, opts); err != nil {
			return fmt.Errorf("upsert vocab crosswalk (unmatched): %w", err)
		}
		res.Unmatched++
		return nil
	}
	if err := s.UpsertCanonicalEntity(entityType, cid, v); err != nil {
		return fmt.Errorf("upsert canonical vocab entity: %w", err)
	}
	if err := writeCrosswalk(s, entityType, raw, cid, methodRule, opts); err != nil {
		return fmt.Errorf("upsert vocab crosswalk (matched): %w", err)
	}
	res.Matched++
	return nil
}

// classifyAliasValue resolves one value to a canonical entity. A pure alias
// spine resolves an entity for every value, so the crosswalk method is
// "canonical" and there is no unmatched case.
func classifyAliasValue(s *store.Store, entityType, raw, cid, canon string, opts classifyOpts, res *classifyResult) error {
	if err := s.UpsertCanonicalEntity(entityType, cid, canon); err != nil {
		return fmt.Errorf("upsert canonical alias entity: %w", err)
	}
	if err := writeCrosswalk(s, entityType, raw, cid, methodCanonical, opts); err != nil {
		return fmt.Errorf("upsert alias crosswalk: %w", err)
	}
	res.Matched++
	return nil
}

// fuzzyRemap clusters near-duplicate canonical names and remaps their crosswalk
// entries to a shared representative canonical ID. The representative is the
// lexicographically smallest cluster member so the mapping is deterministic
// regardless of map iteration order. canonToID is mutated in place to reflect
// the merges so countDistinctCanonicals reports the post-fuzzy count.
func fuzzyRemap(s *store.Store, entityType string, canonToID map[string]string, opts classifyOpts) error {
	canonNames := make([]string, 0, len(canonToID))
	for cn := range canonToID {
		canonNames = append(canonNames, cn)
	}
	// Sort before clustering so the representative (cluster[0]) is chosen
	// deterministically regardless of map iteration order.
	sort.Strings(canonNames)
	clusters := clusterNames(canonNames, opts.fuzzyThreshold())

	// Hoist a single ListCrosswalk call and build an index keyed by
	// canonical_id so we remap in O(n) instead of O(k×n).
	allRows, err := s.ListCrosswalk(entityType, "dice")
	if err != nil {
		return fmt.Errorf("fuzzy pass list crosswalk: %w", err)
	}
	byCanonID := map[string][]store.CrosswalkRow{}
	for _, r := range allRows {
		byCanonID[r.CanonicalID] = append(byCanonID[r.CanonicalID], r)
	}

	for _, cluster := range clusters {
		if len(cluster) < 2 {
			continue
		}
		// Use the first (lexicographically smallest) cluster member as the representative.
		repID := canonToID[cluster[0]]
		for _, cn := range cluster[1:] {
			old := canonToID[cn]
			if old == repID {
				continue
			}
			// Remap crosswalk rows pointing to old ID using the hoisted index.
			for _, r := range byCanonID[old] {
				if err := s.UpsertCrosswalk(store.CrosswalkRow{
					EntityType:        r.EntityType,
					SourceSystem:      r.SourceSystem,
					SourceValue:       r.SourceValue,
					CanonicalID:       repID,
					Method:            r.Method,
					ClassifierVersion: opts.ClassifierVersion,
				}); err != nil {
					return fmt.Errorf("fuzzy pass remap crosswalk %q: %w", r.SourceValue, err)
				}
			}
			canonToID[cn] = repID
		}
	}
	return nil
}
