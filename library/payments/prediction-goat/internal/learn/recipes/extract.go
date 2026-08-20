// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package recipes

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/lookups"
)

// extractWindow caps how many of the most-recently-observed
// search_learnings rows the extractor considers per call. Keeping
// the window bounded keeps Extract O(1) in steady state: a long-
// lived DB with thousands of teaches doesn't pay an ever-growing
// recipe scan on every teach. Recent teaches are also the most
// likely to belong to the same active "story" the user is working
// on, which is exactly the signal Extract wants to amplify.
const extractWindow = 50

// candidateKinds is the ordered set of lookup kinds Extract tries
// when searching for an entity_kind binding. Order is significant:
// table-backed kinds (country_iso2, country_iso3, sports
// abbreviations) are tried before computed kinds (lowercase,
// kebab-case, slug) because table-backed kinds carry the strongest
// "the user meant this specific alias" signal. A query about
// "Portugal" that maps to "PT" via country_iso2 is stronger
// evidence of a templatable pattern than a coincidental
// lowercase("Portugal") == "portugal" substring hit.
//
// New domains can extend this slice via Config (not yet implemented;
// the current set is hard-coded to the seeded kinds plus the
// computedKinds from internal/learn/lookups/store.go's
// computedLookup transforms). When the next CLI plugs in, this
// becomes a registry per-CLI.
var candidateKinds = []string{
	// Table-backed country kinds (seeded).
	"country_iso2",
	"country_iso3",
	"country_name",
	"country_lowercase",
	// Sports team abbreviations (seeded).
	"nfl_team_abbrev",
	"nba_team_abbrev",
	"mlb_team_abbrev",
	"nhl_team_abbrev",
	"mls_team_abbrev",
	// Computed kinds (no DB rows; pure transforms).
	"lowercase",
	"uppercase",
	"capitalize-first",
	"kebab-case",
	"slug",
}

// teachRow is the subset of search_learnings columns Extract reads.
// Kept here (not imported from internal/store) so this package can
// stand alone in the future generator template without depending on
// the store package's column-set evolution.
type teachRow struct {
	id            int64
	queryPattern  string
	queryEntities []string
	resourceID    string
	resourceType  string
	venue         string
}

// Extract walks the most-recent search_learnings rows, groups them
// by structural signature, infers an entity-kind binding for each
// group, and writes one search_recipes row per successful inference.
// Returns the count of NEW rows created (i.e., rows where Upsert's
// `inserted` flag was true; re-asserted rows that just bumped
// confidence are not counted as created but are also not skipped).
//
// Idempotency: re-running Extract on the same DB state produces zero
// new rows. The unique index on (query_template, resource_template,
// strategy) catches duplicates; Upsert flips them into confidence
// bumps. Combined with the bounded extractWindow this means Extract
// is safe and cheap to fire on every successful teach, even if the
// teach didn't change anything Extract would consider.
//
// A nil db returns an error; an empty learnings table returns
// (0, nil).
func Extract(db *sql.DB) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("recipes.Extract: db is nil")
	}

	rows, err := db.Query(`SELECT id, query_pattern, COALESCE(query_entities, ''),
			resource_id, COALESCE(resource_type, ''), COALESCE(venue, '')
		FROM search_learnings
		WHERE source IN ('taught', 'inferred-followup', 'inferred-reach', 'inferred-pair')
		ORDER BY last_observed_at DESC, id DESC
		LIMIT ?`, extractWindow)
	if err != nil {
		return 0, fmt.Errorf("recipes.Extract query: %w", err)
	}
	defer rows.Close()

	var teaches []teachRow
	for rows.Next() {
		var r teachRow
		var entJSON string
		if err := rows.Scan(&r.id, &r.queryPattern, &entJSON, &r.resourceID, &r.resourceType, &r.venue); err != nil {
			return 0, fmt.Errorf("recipes.Extract scan: %w", err)
		}
		// Empty entities -> nothing to template against. Skip.
		ents, err := parseEntityJSON(entJSON)
		if err != nil {
			// Malformed JSON in this row is non-fatal; skip and continue.
			continue
		}
		if len(ents) == 0 {
			continue
		}
		// Multi-entity rows flow through to grouping so the structural
		// form generalizes correctly (a multi-entity row about "$HOME
		// vs $AWAY tonight" shares the same stripped stem as the same-
		// shape single-entity rows in nearby teaches and shouldn't
		// split that stem just because it has two entities). Inference
		// itself is single-slot today and skips multi-entity groups via
		// the anyMulti guard below; this preserves the existing
		// binding-search behavior while letting the grouping key
		// generalize. Multi-entity binding is future work tracked in
		// the U4 backport plan.
		r.queryEntities = ents
		teaches = append(teaches, r)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("recipes.Extract rows: %w", err)
	}

	// Group rows by (queryStructural, resource_type, venue). The
	// queryStructural is the row's query_pattern minus the
	// lowercased entity tokens (the store-side NormalizeQuery keeps
	// entity tokens lowercased in query_pattern; we strip them here
	// so two rows for "Portugal wins" and "USA wins" group on the
	// shared "wins world cup" stem). We deliberately do NOT key on
	// the resource ID's structural signature: that would split
	// Polymarket slugs (whose trailing numeric IDs differ per
	// outcome) into singleton groups even though they share the
	// same templatable pre-prefix shape. The binding search inside
	// inferRecipe is responsible for confirming the resource IDs
	// actually have a coherent (prefix, entity-slot, [trailing])
	// shape; the grouping key is the looser join.
	groups := map[groupKey][]teachRow{}
	for _, t := range teaches {
		// Strip every entity token (not just queryEntities[0]) so the
		// structural form is invariant to entity count. A multi-entity
		// row about "$HOME vs $AWAY tonight" must collapse to the same
		// stem as the single-entity rows in its bucket so grouping
		// can find them together; the anyMulti guard below ensures we
		// still don't synthesize a single-slot template from a multi-
		// entity row.
		structural := queryStructural(t.queryPattern, t.queryEntities)
		// A row whose query_pattern collapses to nothing once entities
		// are stripped has no meaningful content to template against;
		// the resulting recipe would over-match every future cold
		// query. Skip.
		if structural == "" {
			continue
		}
		key := groupKey{
			queryPattern: structural,
			resourceType: t.resourceType,
			venue:        t.venue,
		}
		groups[key] = append(groups[key], t)
	}

	created := 0
	// Iterate groups in a stable order so test assertions on
	// inserted-row order are deterministic. The map iteration is
	// randomized; sorting by key normalizes that.
	keys := make([]groupKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].queryPattern != keys[j].queryPattern {
			return keys[i].queryPattern < keys[j].queryPattern
		}
		if keys[i].resourceType != keys[j].resourceType {
			return keys[i].resourceType < keys[j].resourceType
		}
		return keys[i].venue < keys[j].venue
	})

	for _, key := range keys {
		members := groups[key]
		// Need 2+ teaches to call something a pattern. One teach is
		// just a single learning; templating it would create wild
		// inferences from any user's first-ever taught row.
		if len(members) < 2 {
			continue
		}

		// Multi-entity members fall through to grouping (so the
		// structural form generalizes correctly) but skip inference
		// here: tryExactBinding / tryPrefixBinding both index
		// queryEntities[0] only. Emitting a single-slot template
		// against a multi-entity row would synthesize a wrong pattern
		// (the second entity would be baked into prefix/suffix as a
		// literal). Multi-entity binding is future work; for now we
		// only infer when every group member is single-entity.
		anyMulti := false
		for _, m := range members {
			if len(m.queryEntities) > 1 {
				anyMulti = true
				break
			}
		}
		if anyMulti {
			continue
		}

		recipe, ok := inferRecipe(db, key, members)
		if !ok {
			continue
		}
		_, inserted, err := Upsert(db, recipe)
		if err != nil {
			// One failed Upsert shouldn't abort the whole pass; the
			// remaining groups are independent. Aggregate the count
			// and return the first error we see at the end if any
			// future caller wants stricter handling. For now, swallow
			// to keep the post-teach hook safe.
			continue
		}
		if inserted {
			created++
		}
	}
	return created, nil
}

// inferRecipe tries to derive a Recipe from a group of teaches that
// share the same query_pattern + masked resource signature. Returns
// (recipe, true) when a kind binding is found across every member of
// the group; (zero, false) otherwise. The caller is responsible for
// the >=2-member precondition.
//
// The algorithm:
//
//  1. For each candidate kind K, check that lookups.Lookup(K,
//     member.queryEntity) returns a value V_member such that the
//     masked region of member.resourceID equals V_member (for the
//     substitute strategy) OR the masked-region-prefix matches
//     V_member followed by an unpredictable trailing segment (for
//     the substitute-then-search-prefix strategy).
//
//  2. The first kind whose binding holds for every member wins. Ties
//     are broken by the order of candidateKinds so the strongest
//     "this is the alias the user meant" signal sorts first.
//
// Why first-match wins instead of best-match: a stricter scoring
// (e.g. prefer the kind whose value reproduces the resource ID with
// the smallest prefix-search residue) is tempting but adds
// complexity and risk of mis-binding when a user's pattern legitimately
// uses a less-canonical kind. The single-domain examples in scope
// (Kalshi country tickers, Polymarket country slugs) all bind on the
// first kind tried (country_iso2 for Kalshi, lowercase for
// Polymarket).
func inferRecipe(db *sql.DB, key groupKey, members []teachRow) (Recipe, bool) {
	if len(members) == 0 {
		return Recipe{}, false
	}

	for _, kind := range candidateKinds {
		// Try the substitute strategy first. Every member's resource
		// ID must exactly equal prefix + lookups.Lookup(kind, entity)
		// + suffix where prefix/suffix come from the shared template.
		if tmpl, ok := tryExactBinding(db, kind, members); ok {
			return Recipe{
				QueryTemplate:    buildQueryTemplate(key.queryPattern, members[0].queryEntities),
				ResourceTemplate: tmpl,
				ResourceType:     key.resourceType,
				Venue:            key.venue,
				Strategy:         StrategySubstitute,
				EntityKind:       kind,
				Confidence:       DefaultConfidence,
				Source:           SourceInferred,
				ExampleQuery:     buildExampleQuery(key.queryPattern, members[0].queryEntities[0]),
				ExampleResource:  members[0].resourceID,
			}, true
		}
		// Try the prefix strategy. Each member's resource ID must
		// equal prefix + lookups.Lookup(kind, entity) + literal-or-
		// variable suffix; we treat any per-member difference after
		// the entity as the unpredictable trailing segment.
		if tmpl, ok := tryPrefixBinding(db, kind, members); ok {
			return Recipe{
				QueryTemplate:    buildQueryTemplate(key.queryPattern, members[0].queryEntities),
				ResourceTemplate: tmpl,
				ResourceType:     key.resourceType,
				Venue:            key.venue,
				Strategy:         StrategySubstituteThenSearchPrefix,
				EntityKind:       kind,
				Confidence:       DefaultConfidence,
				Source:           SourceInferred,
				ExampleQuery:     buildExampleQuery(key.queryPattern, members[0].queryEntities[0]),
				ExampleResource:  members[0].resourceID,
			}, true
		}
	}
	return Recipe{}, false
}

// tryExactBinding checks whether `kind` reproduces every member's
// resource_id by substituting lookups.Lookup(kind, entity) at the
// SAME position in a shared prefix/suffix template. Returns the
// constructed resource_template on success.
//
// The position is fixed by the first member: we find the substring
// of member[0].resourceID equal to lookups.Lookup(kind,
// member[0].entity); the remaining members must produce a value at
// the same position relative to their own resource IDs that splits
// them into the same prefix+suffix.
func tryExactBinding(db *sql.DB, kind string, members []teachRow) (string, bool) {
	// Resolve the lookup for every member up front; if any miss,
	// this kind doesn't bind.
	values := make([]string, len(members))
	for i, m := range members {
		v, ok, err := lookups.Lookup(db, kind, m.queryEntities[0])
		if err != nil || !ok || v == "" {
			return "", false
		}
		values[i] = v
	}

	// Find the first member's substring position. There may be more
	// than one — e.g., the resource ID coincidentally contains the
	// lookup value somewhere unrelated. Try every occurrence; the
	// binding is "good" if at SOME occurrence index, every other
	// member's resource_id has its own value at a position that
	// produces the same prefix+suffix.
	first := members[0]
	idx := 0
	for {
		pos := strings.Index(first.resourceID[idx:], values[0])
		if pos < 0 {
			return "", false
		}
		pos += idx
		prefix := first.resourceID[:pos]
		suffix := first.resourceID[pos+len(values[0]):]

		allMatch := true
		for i, m := range members {
			if m.resourceID != prefix+values[i]+suffix {
				allMatch = false
				break
			}
		}
		if allMatch {
			return prefix + entitySlot(kind) + suffix, true
		}
		// Advance past this occurrence and try the next one.
		idx = pos + 1
		if idx >= len(first.resourceID) {
			return "", false
		}
	}
}

// tryPrefixBinding handles the case where the resource_id has a
// shared prefix, the entity value, and then a per-member trailing
// segment that is NOT shared. Polymarket slugs like
// "will-portugal-win-the-2026-fifa-world-cup-912" and
// "will-usa-win-the-2026-fifa-world-cup-467" share the prefix
// "will-" and the literal suffix "-win-the-2026-fifa-world-cup" but
// have different trailing numeric IDs.
//
// Algorithm: split first member's resource_id on values[0]; require
// the suffix portion to start with a literal segment shared by every
// other member, then everything after that literal segment is the
// unpredictable trailing portion (collapsed to "*"). The shared
// literal segment is found by longest-common-prefix between
// suffix(first) and suffix(other) for every other member after each
// has its own entity value sliced out.
//
// Returns the template "prefix + {entity:kind} + sharedSuffix + *"
// on success.
func tryPrefixBinding(db *sql.DB, kind string, members []teachRow) (string, bool) {
	values := make([]string, len(members))
	for i, m := range members {
		v, ok, err := lookups.Lookup(db, kind, m.queryEntities[0])
		if err != nil || !ok || v == "" {
			return "", false
		}
		values[i] = v
	}

	first := members[0]
	// Iterate occurrences of values[0] in first.resourceID; for each
	// occurrence, build a candidate (prefix, suffixFirst) and check
	// the other members.
	idx := 0
	for {
		pos := strings.Index(first.resourceID[idx:], values[0])
		if pos < 0 {
			return "", false
		}
		pos += idx
		prefix := first.resourceID[:pos]
		suffixFirst := first.resourceID[pos+len(values[0]):]

		// Every other member must start with the same prefix and,
		// after slicing out their entity value, share a non-empty
		// longest-common-prefix with suffixFirst.
		sharedSuffix := suffixFirst
		ok := true
		for i := 1; i < len(members); i++ {
			m := members[i]
			if !strings.HasPrefix(m.resourceID, prefix) {
				ok = false
				break
			}
			// Find values[i] in m.resourceID at the same offset
			// (right after the prefix).
			if !strings.HasPrefix(m.resourceID[len(prefix):], values[i]) {
				ok = false
				break
			}
			suffixI := m.resourceID[len(prefix)+len(values[i]):]
			sharedSuffix = longestCommonPrefix(sharedSuffix, suffixI)
		}
		if !ok {
			idx = pos + 1
			if idx >= len(first.resourceID) {
				return "", false
			}
			continue
		}

		// Require a non-empty shared suffix to anchor the prefix
		// search; otherwise the substitution alone would be the
		// template and there'd be no point in the prefix variant.
		// Also: if sharedSuffix == suffixFirst for every member, this
		// is actually an exact match (no per-member variation), and
		// tryExactBinding would have caught it — return false here so
		// the caller tries the next kind only if the exact path
		// didn't already win.
		if sharedSuffix == "" {
			return "", false
		}
		if isFullyShared(sharedSuffix, suffixFirst, members, values, prefix) {
			// Exact match in disguise; let tryExactBinding handle it.
			return "", false
		}
		return prefix + entitySlot(kind) + sharedSuffix + "*", true
	}
}

// isFullyShared reports whether every member's suffix equals
// sharedSuffix exactly (no per-member trailing variation). When this
// holds, the data fits the exact-substitute shape and should not be
// emitted as a prefix recipe.
func isFullyShared(sharedSuffix, suffixFirst string, members []teachRow, values []string, prefix string) bool {
	if sharedSuffix != suffixFirst {
		return false
	}
	for i := 1; i < len(members); i++ {
		suffixI := members[i].resourceID[len(prefix)+len(values[i]):]
		if suffixI != sharedSuffix {
			return false
		}
	}
	return true
}

// queryStructural strips every stored entity from queryPattern and
// returns the alphabetized non-entity token form so two rows that
// differ only in their entity (e.g., "portugal wins world cup" and
// "usa wins world cup") group together under the shared stem "wins
// world cup". Multi-entity queries have all entities stripped:
// single-entity-only stripping previously blocked pattern emergence
// whenever a row's queryEntities held more than one element (the
// second entity stayed in the structural form as a literal and
// split otherwise-identical-shape rows into singleton groups).
// Tokens are re-sorted to keep the structural key stable across
// arbitrary entity insertion positions.
//
// Returns the empty string when stripping the entities would leave
// no content tokens behind (a degenerate row whose query was just
// the entity(s) themselves; nothing to template against).
func queryStructural(queryPattern string, entitiesToStrip []string) string {
	tokens := strings.Fields(queryPattern)
	skip := make(map[string]struct{}, len(entitiesToStrip))
	for _, e := range entitiesToStrip {
		if v := strings.ToLower(strings.TrimSpace(e)); v != "" {
			skip[v] = struct{}{}
		}
	}
	kept := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, drop := skip[strings.ToLower(t)]; drop {
			continue
		}
		kept = append(kept, t)
	}
	sort.Strings(kept)
	return strings.Join(kept, " ")
}

// buildQueryTemplate builds the canonical query_template that goes
// into search_recipes.query_template. The template carries the
// shared non-entity tokens (from the row's structural form) plus
// a single "{entity}" placeholder, all re-sorted for byte stability.
// Multi-entity rows are excluded upstream via the anyMulti guard in
// Extract so this stays a valid single-slot template even though it
// accepts a full entity slice (every entity gets stripped from the
// stem, then exactly one {entity} placeholder appended).
//
// Why re-sort: the Apply path token-set Jaccards the live query's
// non-entity tokens against this template's non-entity tokens, and
// the recall path already sorts its non-entity tokens lexically
// (see normalize.NonEntityNormalized). Matching sort orders keeps
// the Jaccard symmetric.
func buildQueryTemplate(queryPattern string, entitiesToStrip []string) string {
	structural := queryStructural(queryPattern, entitiesToStrip)
	tokens := strings.Fields(structural)
	tokens = append(tokens, "{entity}")
	sort.Strings(tokens)
	return strings.Join(tokens, " ")
}

// buildExampleQuery reconstructs an approximation of the user-
// facing query the recipe was inferred from. Returns
// "query_pattern with entity inserted" — not the exact raw query
// (which the migration discarded down to query_pattern at teach
// time) but close enough for `recipes list` to be useful.
//
// The example is not used by the Apply path; it's purely a
// diagnostic / human-readable hint.
func buildExampleQuery(queryPattern, entity string) string {
	if queryPattern == "" {
		return entity
	}
	return queryPattern + " " + entity
}

// entitySlot formats a typed entity placeholder for a
// resource_template. Computed kinds and table-backed kinds use the
// same shape; the Apply path keys off the kind name to decide which
// lookup path to use.
func entitySlot(kind string) string {
	return "{entity:" + kind + "}"
}

// longestCommonPrefix returns the longest string that is a prefix
// of both a and b. Used by tryPrefixBinding to detect the literal
// suffix shared across members before the per-member trailing
// variability.
func longestCommonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:n]
}

// parseEntityJSON unmarshals the JSON array stored in
// search_learnings.query_entities into a Go slice. Empty / "null" /
// "[]" all return nil. Identical to learn.ParseStoredEntities but
// duplicated here to avoid an import cycle between learn and
// recipes (learn imports recipes via recall.go integration).
func parseEntityJSON(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse entities: %w", err)
	}
	return out, nil
}

// groupKey is the join key Extract uses to bucket search_learnings
// rows that potentially template into the same recipe. Two rows
// land in the same bucket when they share the same non-entity query
// stem, the same resource_type, and the same venue. The actual
// resource-ID-shape binding check happens later in inferRecipe; the
// groupKey is intentionally loose so the binding search has enough
// candidates to find a real pattern.
//
// Declared at the package level so inferRecipe (defined below
// Extract for readability) can reference it as a parameter.
type groupKey struct {
	queryPattern string
	resourceType string
	venue        string
}
