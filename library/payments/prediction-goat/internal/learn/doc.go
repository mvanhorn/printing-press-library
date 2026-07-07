// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// Package learn is the entity-aware teach/recall/recipe subsystem that
// turns one-shot LLM discoveries into reusable shortcuts. The goal: a
// new user question that has been answered before should cost two
// calls (recall + live price fetch) instead of seven (a full discovery
// walk across topic/compare/series/event/markets).
//
// # Subsystem purpose
//
// The CLI ships with a SQLite store. After each successful agent
// response, the agent calls `teach --query "<question>" --resource <id>`
// in the background; the next time the same (or structurally similar)
// question arrives, `recall` returns the cached resource IDs and the
// agent skips discovery. The whole subsystem is *additive* — every
// learning path degrades to "no hit, run discovery normally" on the
// slow path. Nothing here is allowed to make a cold query slower than
// the unenhanced CLI did before; the recall floor is conservative
// (Jaccard >= 0.6 by default), the entity validator filters false
// positives back into a debug-only mismatches array, and `--no-learn`
// short-circuits the whole pipeline for deterministic flows.
//
// # Lifecycle: teach -> recall -> preseed -> recipes
//
// Four pipelines feed the same search_learnings + search_recipes
// tables:
//
//  1. Teach. internal/cli/teach.go writes one search_learnings row per
//     (query_pattern, resource_id, action) tuple. The first teach for
//     a tuple lands at confidence=2 (the U4 floor) so the next recall
//     trips the SKILL.md "skip discovery" branch. Re-confirmations
//     bump confidence. Teach validates resource shape (teach.go,
//     teach_log.go) and emits warnings to ~/.local/share/.../teach.log
//     when the agent taught the wrong shape (parent event when a
//     child exists; resource with no entity overlap).
//
//  2. Recall. recall.go is the read path. Normalize -> Jaccard ->
//     entity-aware classify -> sort -> return. The envelope shape is
//     {found, results, mismatches, query_entities, normalized,
//     warnings}; see SKILL.md "Automatic learning" for the four-branch
//     decision tree the LLM follows.
//
//  3. Preseed. preseed.go runs at sync time and writes boost rows for
//     multi-outcome event families (Kalshi mutually_exclusive=true,
//     Polymarket negRisk=true). Each child market becomes a learning
//     for "<question template> {entity}" -> child_resource_id with
//     source=preseed and confidence>=2 so a cold first-ever query for
//     a known entity in a known family resolves on the first call.
//
//  4. Recipes. internal/learn/recipes generalizes two or more
//     structurally-similar teaches into a template, e.g.
//     "odds {country} wins world cup" ->
//     "KXMENWORLDCUP-26-{country:country_iso2}". On a future query
//     whose entity isn't taught, recall falls back to recipe Apply,
//     which substitutes via internal/learn/lookups and verifies the
//     candidate against the local resources table. The result is
//     surfaced as a recall.Hit with source="recipe".
//
// # Package layout
//
//	internal/learn/
//	  doc.go            — this file
//	  normalize.go      — NormalizedQuery + prediction-goat config
//	  match.go          — Jaccard + ClassifyEntityMatch + warning constants
//	  recall.go         — Recall(ctx, db, query, opts) read path
//	  teach.go          — teach-time resource-shape validation
//	  teach_log.go      — JSONL writer/reader for teach.log warnings
//	  preseed.go        — sync-time multi-outcome family preseed driver
//	  entities/         — domain-agnostic entity extractor (U1)
//	  lookups/          — canonical-to-value lookups + seeded data (U9)
//	  lookups/seeds/    — ISO countries, sports rosters, generic kinds
//	  recipes/          — query/resource template engine (U10)
//
// Each subpackage carries its own doc.go with subpackage-level design
// notes. Read those for the why on individual files.
//
// # Extension surface for a future CLI
//
// The whole point of factoring this code into a package is to lift it
// into the cli-printing-press generator template later. The extension
// surface is:
//
//   - **Per-CLI ticker patterns**. normalize.go's
//     DefaultPredictionGoatConfig() registers Kalshi (KX[A-Z0-9-]+)
//     and Polymarket (will-[a-z0-9-]+...) ticker regexes. A new CLI
//     ships its own DefaultFooConfig() with whatever shape its
//     resource IDs take.
//
//   - **Per-CLI stopwords**. Same Config object accepts a stopword
//     slice. Prediction-goat adds {"odds","wins","win","lose","beat"}
//     — the question-shape vocabulary that wraps every prediction-
//     market query. A weather CLI would add {"forecast","weather",
//     "predict"}; a podcast CLI {"episode","listen","summarize"}.
//     Keep stopwords domain-shape, not entity-shape.
//
//   - **Per-CLI preseed scanners**. preseed.RegisterScanner("foo_*",
//     fooScanner) lets a new CLI plug in its own multi-outcome
//     enumerator. The core driver handles upsert + dedup; the
//     scanner only has to walk the corpus and emit PreseedRow values.
//
//   - **Per-CLI lookup kinds**. internal/learn/lookups/seeds is the
//     reference data. A new CLI either reuses the seeded country and
//     sports rows verbatim, or adds its own seed files (one
//     []LookupRow per file) and concatenates them in init.go::Seeds().
//
//   - **Per-CLI resource->entity field maps**. internal/learn/match.go
//     has an entityFieldsFor(resource_type) hook that names the JSON
//     fields to extract entities from for each resource type. A new
//     CLI registers its types ("foo_markets" -> ["title", "subtitle",
//     "ticker"]) and inherits the rest of the pipeline.
//
// What does *not* extend per-CLI: the schema, the recall sort order,
// the entity-match decision matrix, the Jaccard threshold semantics,
// and the teach.log JSONL format. Those are the cross-CLI contract.
//
// # Schema migration path
//
// The store side (internal/store/store.go) tracks user_version. The
// learning subsystem requires:
//
//   - v3 added search_learnings with the legacy {query_pattern,
//     resource_id, action, confidence} shape. Pre-this-plan.
//   - v4 added query_entities (JSON array of capitalized/ticker
//     tokens) so recall can validate entity overlap. Schema-driven
//     by U2/U3.
//   - v5 added entity_lookups (canonical -> value mapping) seeded
//     with ISO 3166 and sports rosters. U9.
//   - v6 added search_recipes (query_template -> resource_template
//     with typed entity slots) for generalization. U10.
//
// Each migration is forward-only. A re-run on a v3 DB walks every
// step in order. The migration code is the canonical reference for
// what columns each table carries; this doc only names the levels.
//
// # Lifting into a generator template
//
// The eventual cli-printing-press template ships this directory
// (minus normalize.go's prediction-goat-specific Config defaults and
// minus the seeded rows in lookups/seeds/{countries,sports}.go) as
// `internal/learn/`. The generator then writes a per-CLI
// `internal/learn/config.go` shaped like normalize.go's current
// DefaultPredictionGoatConfig — except with the new CLI's ticker
// patterns and stopwords — and per-CLI lookup seed files where
// useful. Everything else (the recall envelope shape, the entity-
// match validator, the teach-time warning logic, the recipe engine,
// the lookups CRUD, the preseed driver) is reusable verbatim.
//
// The package boundary has been kept clean for that lift: no
// internal/cli imports, no internal/source imports, no internal/store
// imports. Storage access uses *sql.DB directly with raw SQL against
// table names that are stable across CLIs. The one direction the
// boundary leaks is internal/store importing internal/learn for
// Normalize-during-write, which keeps the search_learnings.query_pattern
// column populated consistently across migration and live writes.
//
// # See also
//
// docs/plans/2026-05-23-002-feat-prediction-goat-smart-learning-plan.md
// for the full design rationale, the U1-U10 implementation breakdown,
// the failure traces this subsystem was built to address, and the
// extension protocol for the next CLI absorbing this code.
package learn
