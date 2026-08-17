// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// Package recipes implements the generalization layer of the learning
// subsystem: when a user has taught the CLI to map two or more
// structurally-similar queries to two or more structurally-similar
// resources, this package extracts a template ("recipe") from the
// group and applies it to substitute new entities at recall time.
//
// # The story this package tells
//
// A user teaches:
//
//	teach --query "odds portugal wins world cup"  --resource KXMENWORLDCUP-26-PT
//	teach --query "odds usa wins world cup"       --resource KXMENWORLDCUP-26-US
//
// After Extract runs, search_recipes carries:
//
//	{ query_template: "odds {entity} wins world cup",
//	  resource_template: "KXMENWORLDCUP-26-{entity:country_iso2}",
//	  resource_type: "kalshi_markets",
//	  venue: "kalshi",
//	  strategy: "substitute",
//	  entity_kind: "country_iso2",
//	  source: "inferred",
//	  confidence: 2 }
//
// On the next recall call:
//
//	recall "odds england wins world cup"
//
// the direct-lookup path returns no hits (England was never taught).
// recipes.Apply walks search_recipes, matches the non-entity normalized
// form against the recipe's query_template, and substitutes
// lookups.Lookup("country_iso2", "England") -> "GB" into the
// resource_template, yielding the candidate ID
// KXMENWORLDCUP-26-GB. The candidate is verified against the local
// resources table; the match is returned with source="recipe" and
// entity_match="exact" because the substitution binding guarantees
// the entity is present in the resource ID.
//
// # Lifecycle
//
// 1. Teach side. CLI `teach` writes one search_learnings row per
//    (query, resource) pair via store.UpsertLearning. After a
//    successful upsert, the CLI fires Extract on the current DB. The
//    call is cheap, idempotent (the (query_template, resource_template,
//    strategy) unique index silences duplicates) and safe to run on
//    every teach.
//
// 2. Extract side. Walks the most recent N (50) search_learnings rows,
//    groups them by structural signature (query_pattern + resource_id
//    with entity-shaped substrings masked), and for each group of
//    size >= 2 tries to find an entity_kind from the lookups table
//    such that substituting the lookup value for each row's query
//    entity reproduces that row's resource_id. When found, the group
//    becomes one search_recipes row.
//
// 3. Recall side. internal/learn/recall.go calls Apply after the
//    direct-lookup path. For each recipe whose query_template matches
//    the live query (token-set Jaccard >= the same recall floor),
//    substitute the live query's entity via lookups.Lookup, verify the
//    substituted candidate exists in the resources table (or in a
//    prefix LIKE search for the substitute-then-search-prefix
//    strategy), and emit it as a recall.Hit with source="recipe".
//
// # Substitution strategies
//
// Two strategies are supported, distinguished by the trailing shape
// of the resource_template:
//
//   - "substitute" — full deterministic ID. The resource_template
//     names exactly one resource per entity lookup. Example: Kalshi
//     market tickers KXMENWORLDCUP-26-PT / KXMENWORLDCUP-26-US.
//
//   - "substitute-then-search-prefix" — the resource_template ends
//     with "*". The substituted candidate is treated as a LIKE prefix
//     search against the resources table. Example: Polymarket slugs
//     will-portugal-win-the-2026-fifa-world-cup-912 carry an
//     unpredictable trailing numeric ID; the recipe stores
//     "will-{entity:lowercase}-win-the-2026-fifa-world-cup-*" and
//     Apply does the prefix scan.
//
// # Entity kinds
//
// Substitution kinds are resolved by internal/learn/lookups.Lookup.
// Table-backed kinds (country_iso2, country_iso3, nfl_team_abbrev,
// etc.) are seeded at v4->v5 migration time. Computed kinds
// (lowercase, uppercase, kebab-case, capitalize-first, slug) are
// resolved purely by string transform — they have no rows in
// entity_lookups but are still legal in resource_templates. The
// extraction engine tries every legal kind for each group and picks
// the first one that reproduces every member's resource_id.
//
// # Future CLI plug-in
//
// A future generator template will lift internal/learn/{entities,
// match, normalize, lookups, recipes} into a reusable package set.
// Each downstream CLI plugs in:
//
//   - its domain-specific entity extractor config (kalshi/polymarket
//     ticker patterns in normalize.go; the same pattern works for
//     espn-team-codes, openart-model-slugs, etc.)
//   - its seed lookups (the seeds/ files under lookups/seeds/)
//   - its resource-type → JSON-field map (entityFieldsFor in
//     match.go)
//
// Nothing in this package itself is prediction-goat-specific: it
// operates on the abstract LearningRow shape and consumes lookups via
// the package-level Lookup function. The "templates with typed entity
// slots" abstraction is the whole interface; everything else is just
// CRUD around it.
package recipes
