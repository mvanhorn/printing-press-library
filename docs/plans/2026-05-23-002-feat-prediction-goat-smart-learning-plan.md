---
slug: prediction-goat-smart-learning
type: feat
status: active
created: 2026-05-23
depth: deep
target_repo: mvanhorn/printing-press-library
target_cli: library/payments/prediction-goat
portability_intent: |
  The learning subsystem ships in prediction-goat first. Design every change so
  a future plan can lift `internal/learn/` into a generator template at
  cli-printing-press with zero refactoring: pluggable hooks, data-driven
  patterns, no hard-coded domain lists, and inline source comments that double
  as the spec for the next CLI that absorbs this layer.
---

# feat(prediction-goat): make the learning loop actually short-circuit

## Summary

The teach/recall pipeline that landed yesterday is structurally there but does not actually save sessions yet. Three real query traces collected this morning prove it:

- **"odds USA wins world cup" (cold, 12m 39s)** — `recall` returned `found=false`, agent fell into a 5-hop discovery walk (topic → siblings → events list → events get --with-markets → grep) and eventually taught the *parent* Kalshi event ticker `KXMENWORLDCUP-26` against the USA query instead of the *child* `KXMENWORLDCUP-26-US`.
- **"odds england wins the world cup" (warm but wrong, 3m 4s)** — `recall` returned `found=true, match_score=0.6, confidence=1` with **Portugal** tickers, because the normalizer stripped "england" as a low-signal token and matched a prior Portugal teach. The agent didn't notice the entity mismatch and bailed to a fresh discovery walk, wasting three minutes.
- **"odds USA wins world cup" again (warm but mixed, 39s)** — `recall` returned the correct Polymarket slug plus the wrong Kalshi resource (parent event again), forcing a 48-team `kalshi events get --with-markets` walk to read one row. Plus a wasted `--select` re-fetch using fields the response shape doesn't carry.

The deeper throughline: **the current model only learns facts, not recipes.** After teaching "portugal world cup odds → KXMENWORLDCUP-26-PT", the CLI knows how to find Portugal's odds and nothing else. The next query about *England* should be trivial — the structural pattern is the same, only the entity differs — but the system has no way to generalize from one example to the next. The user is right to push back: a learning surface that only caches the exact queries it has seen is barely better than a bookmark file.

This plan adds two layers on top of the existing teach/recall:

1. **A correctness layer** so today's literal teaches stop returning wrong answers — data-driven entity extraction, entity-preserving normalization, entity-aware match validation with structured warnings, confidence-floor adjustment, teach-time resource-shape validation, multi-outcome family pre-seeding at sync time.

2. **A generalization layer** so the system can learn *recipes*, not just facts. After one Portugal teach, a recipe like `"odds {country} wins world cup" -> "KXMENWORLDCUP-26-{country:iso2}" + "will-{country:lower}-win-the-2026-fifa-world-cup-*"` gets inferred. Future queries about England, France, Brazil — any country — resolve via substitution against a local SQLite-backed entity lookup table (countries, sports teams, etc. shipped with the binary as data), with one verification API call to confirm the substituted ticker exists.

Hard-coding **canonical entity data** in local SQLite is fine (countries, ISO codes, league rosters — slow-changing data that's easy to ship and easy to extend via `teach-lookup` per user). What stays domain-agnostic is the **protocol**: extract entities from the query, find recipes whose template matches, substitute via lookup, verify, return. A future CLI in any domain (podcast guests, stocks by ticker, recipe-by-ingredient) plugs in its own entity kinds and entity-lookup seed without touching the inference engine.

The SKILL.md "Automatic learning" section is rewritten as a real protocol document for the LLM. Every new source file lands with substantive inline comments explaining the *why* — because the next agent through this code should pick up the design instantly, and because the whole subsystem is meant to lift into a generator template at cli-printing-press later.

---

## Problem Frame

The teach/recall pipeline as shipped in PR #780 has six structural problems, surfaced by the three failure traces:

1. **Normalizer is destructively aggressive.** It lowercases + collapses whitespace + strips a fixed stopword list (`a, the, what, are, is, was`, plus tokens that get treated like noise). The result: "odds england wins the world cup" normalizes to "england world cup", and "portugal wins world cup" normalizes to "portugal world cup". Token-set Jaccard between those is 0.5 — under threshold. But "england wins world cup" → "england world cup" and "portugal wins world cup" → "portugal world cup" overlap on {world, cup} which against a 3-token set gives Jaccard 0.5 — just at the edge. With **fewer noise tokens**, the overlap becomes 0.67 and crosses the threshold. The normalizer is throwing away the most semantically important tokens (entity names) and keeping the categorical tokens (world, cup), so match scores cluster around the threshold and false positives leak through.

2. **No entity validation post-match.** Once Jaccard clears the threshold, the result is returned verbatim. There is no check that the *resource* the learning points at carries the same entity the query carries. So a Portugal-tagged resource gets returned for an England query as long as the rest of the query overlaps.

3. **Confidence floor is below the skip threshold.** The SKILL.md contract says "if `found=true` with `confidence>=2`, skip discovery." But every fresh `teach` writes `confidence=1`. So the very first re-use of a taught mapping never trips the skip path — defeating the whole point of teaching. Confidence only reaches 2 on the *third* identical query, which by then the LLM probably found a different way.

4. **No cold-start coverage for multi-outcome families.** Kalshi exposes a `mutually_exclusive` flag on events whose children are a fixed roster (e.g., all 48 World Cup teams). Polymarket exposes `negRisk` for the same shape. Today the CLI knows these flags exist but doesn't use them — so the agent has to do a full discovery walk on every cold query like "odds X wins event Y" even though the mapping is mechanically derivable from the corpus.

5. **`teach` accepts the wrong resource shape silently.** When the LLM is mid-discovery and ends up with the parent event ticker (`KXMENWORLDCUP-26`), nothing stops it from teaching that against an entity-specific query. The next recall returns the parent, the next session walks the 48-team event to extract one row.

6. **`--select` is a footgun.** Agents guess dotted paths against response shapes they haven't introspected. Wrong path returns empty objects (not an error), forcing a re-fetch. Two of the three failure traces burned ~25% of their wall-clock on `--select` errors. The CLI knows its own response field set; it should expose that to agents.

The cumulative effect: **the learning loop's failure modes are silent**. Every failure here is the CLI returning *something* that looks plausible but is wrong, and the LLM having no signal to distinguish "good answer" from "noise that resembles a good answer." The fixes below shift each failure mode from silent-wrong to either right-answer or explicit-warning.

## Requirements

Sourced from user request and the three transcripts in the original feedback message.

- **R1.** Recall must return entity-aware results. For a query carrying entity `X`, results whose underlying resource title/ticker/subtitle does not contain `X` (or an entity-extractor-recognized variant) must be **filtered out** by default, and surfaced separately under a `mismatches` field for debugging.
- **R2.** Recall results must carry an `entity_match` field per row (`exact` / `partial` / `mismatch`) and an optional `warnings` array (e.g., "resource is a parent event; consider drilling to child"). The SKILL.md protocol must instruct the LLM to read these before acting.
- **R3.** Query normalization must preserve entity tokens (capitalized words, ALL-CAPS, ticker-shape strings). The stopword list and entity recognizer must be **data-driven and pluggable** — no hard-coded country lists, sport rosters, or prediction-market-specific tokens in core code. Per-CLI customization is via config/hooks.
- **R4.** First `teach` for a (query_pattern, resource_id, action) tuple must land at `confidence=2`, clearing the skill's documented "skip discovery" threshold on the very first re-use. Subsequent re-confirmations still bump confidence by 1.
- **R5.** At sync time, the CLI must pre-seed `search_learnings` from multi-outcome event families that are mechanically derivable from the corpus (Kalshi `mutually_exclusive=true` events with child markets carrying `yes_sub_title`; Polymarket `negRisk` events with sibling slugs). Pre-seeded rows carry `source='preseed'` and confidence sufficient to clear the skip threshold. The pre-seed scanner must be **pluggable per resource type** — not hard-coded to World Cup or sports.
- **R6.** `teach` must inspect the resource being taught and warn (via `teach.log`, not stderr) when:
  - the resource is a parent event/series ticker AND a child resource exists whose entity-shape matches the query better
  - the resource has no overlap with any query entity at all
  The warning does not block the write — the LLM may have a reason. But the log captures the friction for later review.
- **R7.** Every command must expose its response field set via `agent-context` under a `commands.<name>.select_paths` section. The list is data-driven (introspected from the Go response struct tags via codegen or build-time tool, never a hand-maintained drift candidate). Agents reading `agent-context` see exactly which dotted paths are valid for `--select` per command.
- **R8.** SKILL.md "Automatic learning" section must be rewritten to a substantive protocol document covering: how to read recall results, when to trust them, when to treat as cold start despite a hit, how to handle `entity_match=mismatch`, how to handle `warnings`, and concrete worked examples of GOOD vs BAD agent behavior on the three failure traces in this plan.
- **R9.** Inline source-code comments on every new file and every modified function must explain the *why* (not the what), with explicit cross-references back to this plan's section anchors. The next agent to read `internal/learn/entities/extract.go` should be able to extend it without re-deriving the design from scratch.
- **R10.** Every new code surface must be structured so a future template-extraction plan can lift `internal/learn/` into the cli-printing-press generator without rewrites. Concretely: generic table schema, pluggable per-CLI entity-extractor and pre-seed-scanner hooks, no `kalshi_` / `polymarket_` / prediction-market-specific symbols leaking into `internal/learn/`.
- **R11.** Recipe inference: after the agent successfully resolves a query and teaches the result, the CLI must be able to extract a *template* from the (query, resource) pair. The template separates entity slots from constant structure (e.g., `"odds {country} wins world cup" -> "KXMENWORLDCUP-26-{country:iso2}"`). On a future query whose structure matches a known template, the CLI substitutes the new entity, verifies the substituted resource exists via one API call, and returns it — without the agent having to repeat discovery. Concrete data may be hard-coded in local SQLite (country ISO codes, sports team abbreviations, etc.); the inference protocol itself must be domain-agnostic.
- **R12.** Entity lookup tables in SQLite: ship a seeded `entity_lookups` table covering common kinds (`country_iso2`, `country_iso3`, `country_lowercase`, common sports team abbreviations). Provide a `teach-lookup` command so users can add per-domain entries. Auto-infer new entries from confirmed teaches when the structural pattern is unambiguous (e.g., after seeing two teaches that differ only in `(Portugal, PT)` vs `(USA, US)` at the same ticker suffix position, infer the kind is `country_iso2` and bank the mappings).

## Scope Boundaries

In scope:
- Entity extraction, entity-preserving normalization, entity-aware recall, recall response enrichment
- Confidence-floor adjustment
- Multi-outcome family pre-seeding hooked into existing Kalshi + Polymarket sync paths
- `teach`-time resource-shape validation
- Per-command `--select` cheatsheet in `agent-context`
- SKILL.md protocol rewrite
- Inline source-code comments as the spec for the next CLI to absorb the subsystem
- New package boundary `internal/learn/` separating reusable learning from prediction-goat-specific glue

Out of scope:
- Actually publishing the template into `cli-printing-press` — that's its own plan once this shape settles in production
- MCP exposure of `teach` / `recall` / `learnings list` / `forget`
- Live price caching (short-TTL price cache for repeat queries) — sometimes faster but adds a freshness foot-gun; defer
- Auto-forget of bad teaches based on inferred misuse — `forget` stays manual for v1
- Multi-user / shared registry of learnings — local-per-user only
- Trading or any write surface against the venue APIs — read-only structurally enforced by CI lint

### Deferred to Follow-Up Work

- Template extraction into `cli-printing-press` (separate plan after this shape proves out)
- Auto-pruning of low-confidence stale learnings (e.g., entries with `last_observed_at` older than 90 days and confidence 1)
- A `learn-meta` introspection command that prints the current entity-extractor config, stopword set, and pre-seed coverage — useful for debugging but not blocking v1
- MCP exposure of the learning surface
- A short-TTL live-price cache to make repeat warm queries feel snappier
- Pre-seed scanner hooks for additional resource types beyond Kalshi `mutually_exclusive` and Polymarket `negRisk` (Polymarket `seriesType=single`, Kalshi categorical series, etc.)

---

## Key Technical Decisions

**Entity extraction is data-driven, not list-driven.** No hard-coded country names, no sport rosters, no per-domain lookup tables. Entities are extracted from the query string by pattern: (a) capitalized non-sentence-start tokens, (b) ALL-CAPS tokens of length >= 2, (c) ticker-shape tokens (matching configurable per-CLI regex like `^KX[A-Z0-9-]+$` for Kalshi or `^will-[a-z0-9-]+$` for Polymarket). The same extractor runs against the resource side — pulling entity tokens from the title, yes_sub_title, ticker, slug — so match validation is symmetric. A future CLI in a different domain (e.g., podcast titles, recipe names) plugs in its own ticker regex and inherits the rest.

**Normalizer preserves entities, discards only stopwords.** The current normalizer lowercases everything indiscriminately. The replacement preserves entity tokens (as detected by the entity extractor) in a separate field on the normalized representation, and only lowercases + stopword-strips the **non-entity** tokens for the token-set Jaccard math. The matching function takes both the entity set and the normalized non-entity tokens. A query with `entities={USA}` cannot match a learning with `entities={Portugal}` regardless of how their non-entity tokens overlap.

**Stopword list is per-CLI config, not in core code.** `internal/learn/` ships a default English stopword list. Each consumer CLI declares additional domain stopwords (for prediction-goat: `odds, wins, winning, lose, win` — the question shape, not the entities) in a small registration call. No domain words in the core package.

**Recall returns structured warnings, not just data.** Today recall returns `{found, results}`. The replacement returns `{found, results: [...{resource_id, ..., entity_match, warnings}], mismatches: [...], normalized, query_entities}`. The LLM sees exactly what the CLI thinks about each result. SKILL.md teaches the LLM to read `entity_match` before trusting any hit.

**Confidence starts at 2, not 1.** A new teach writes `confidence=2`. The skill's `skip if confidence>=2` threshold then fires on the first re-use. Re-confirmations still bump (confidence=3, 4, ...) and are surfaced as "high confidence" results for diagnostic purposes, but the *fast path* doesn't require them.

**Pre-seeding is a sync-time scanner with pluggable resource-type hooks.** The pre-seed scanner runs after each sync command completes. It walks resources that match registered "multi-outcome family" patterns (Kalshi `mutually_exclusive=true` + child markets, Polymarket `negRisk` + sibling slugs). For each child, it derives a query pattern from the parent's title + the child's subtitle/entity (`"odds {entity} wins {event_title}"`, normalized) and upserts a boost learning with `source='preseed', confidence=2`. The scanner is **registered per resource type** via a function table, not hard-coded to specific tickers or sports. A future CLI registers a different scanner for its domain (e.g., podcast episodes that map to show + guest entities) and pre-seeding inherits.

**`teach` warns on resource-shape mismatches.** When `teach` is called, it loads the resource(s) being taught from the `resources` table. If the resource is a parent event/series and (a) the query carries an entity AND (b) the parent has child resources whose subtitles/yes_sub_title match the entity, `teach.log` gets a structured warning naming the better child. The teach still succeeds — the LLM may legitimately want the parent. The warning is for offline review, surfaced in `learnings list --warnings`.

**`agent-context` exposes per-command `select_paths`.** A new section under `commands.<name>.select_paths` enumerates valid dotted paths the agent can pass to `--select`. The list is generated from response struct tags at build time (via `go:generate`), so it cannot drift from the actual response shape. Agents reading `agent-context` see a definitive answer to "what fields can I select on `markets get-by-slug`?"

**Inline comments are first-class deliverables, not afterthoughts.** Every new file in `internal/learn/` ships with a top-of-file doc comment explaining *why this file exists, what design problem it solves, and how a future CLI should think about extending it*. Function-level comments document non-obvious choices (e.g., "we preserve ALL-CAPS tokens because they're either acronyms (NFL, USA) or tickers (KXMENWORLDCUP) — both load-bearing for match accuracy"). The next agent should be able to read the package directory and grasp the design without reading this plan.

**Package boundary today, template tomorrow.** `internal/learn/` is structured so a future plan can `cp -r internal/learn/ <generator-templates>/learn/` and have a working subsystem for a new CLI. The package depends only on `database/sql`, `encoding/json`, `regexp`, `strings`, `time`, and a small generic store interface. No imports from `internal/cli/`, `internal/source/`, `internal/store/` outside the interface boundary. Prediction-goat-specific glue (rerank integration in topic/compare, the pre-seed scanner implementations for Kalshi and Polymarket, the seeded `entity_lookups` content) lives in the consumer code, not in the shared package.

**Recipes are templates with typed entity slots.** A recipe is a triple `(query_template, resource_template, entity_kind)`. Both templates contain placeholder tokens of the form `{entity}` or `{entity:lookup_kind}`. The query template matches a new query by entity-aware Jaccard against the non-slot constants; the entity slot binds to the new query's entity. The resource template substitutes the entity via the named lookup table (e.g., `{country:iso2}` looks up the entity's value in `entity_lookups WHERE kind='country_iso2'`). The result is a concrete candidate resource ID. The candidate is verified by one API call (or one SQL lookup if the resource is already in `resources`) before being returned to the agent. Recipes are stored in their own table, sorted by `confidence`, and applied as a fallback in `recall` when no direct learning hits.

**Recipe extraction is data-driven, not hand-engineered.** When two or more confirmed teaches share structure — same non-entity tokens in the query, same non-entity structure in the resource ID, differing only in entity tokens — the CLI auto-extracts a recipe candidate. The extraction algorithm finds the longest common substring/template across the resource IDs, identifies the variable positions, and matches them against known entity-lookup kinds (does the variable look like a 2-letter ISO code? a lowercase country name? a 3-letter sports team code?). Successful matches write a `search_recipes` row and bank any new entity-kind mappings into `entity_lookups`. Extraction runs after every teach (cheap; just walks recent teaches) and is also exposed as `teach-recipe` for explicit user authorship.

**Entity lookups can be hard-coded in SQLite.** Ship a seeded `entity_lookups` table with the binary covering: ISO 3166-1 alpha-2 country codes (Portugal→PT, England→GB, etc.), ISO 3166-1 alpha-3, common NFL/NBA/MLB/NHL/MLS team abbreviations, plus a few generic kinds (capitalize-first, lowercase, kebab-case). This is *data*, not *domain logic* — the recipe inference engine treats `entity_lookups` as a pluggable key/value store. A future CLI adds its own kinds (e.g., podcast show abbreviations, stock tickers) by inserting into the same table. Per-user additions via `teach-lookup`.

---

## High-Level Technical Design

### Recall flow, before vs. after

```
BEFORE (current):
recall("odds england wins the world cup")
  -> normalize -> "england world cup"
  -> token-set Jaccard against search_learnings.query_pattern
  -> return all rows >= 0.6 (Portugal-tagged learning matches at 0.6 -- the false positive)
  -> {found: true, match_score: 0.6, results: [{resource_id: "...portugal...-912", ...}]}

AFTER (this plan):
recall("odds england wins the world cup")
  -> extract entities -> {entities: ["England"], tickers: [], capitalized: ["England"]}
  -> normalize non-entity tokens -> "world cup" (stopwords stripped)
  -> token-set Jaccard against search_learnings.normalized_non_entity (Portugal learning normalizes the same way -> overlap is 1.0)
  -> entity overlap check -> Portugal-tagged learning carries entities={Portugal} -- ZERO overlap with {England} -- FILTERED
  -> {found: false, results: [], mismatches: [{resource_id: "...portugal...-912", entity_match: "mismatch", reason: "query has England, resource has Portugal"}], normalized: "world cup", query_entities: ["England"]}
  -> if pre-seed scanner has run: {found: true, results: [{resource_id: "KXMENWORLDCUP-26-GB", ..., entity_match: "exact", source: "preseed"}, ...]}
```

*Directional guidance for review, not implementation specification.*

### Match-validation decision matrix

```
+-----------------------+----------------------+-----------------+
| query entities        | resource entities    | entity_match    |
+-----------------------+----------------------+-----------------+
| {USA}                 | {USA}                | exact           |
| {USA}                 | {USA, hosting}       | exact           |
| {}                    | {Portugal}           | partial         |
| {USA}                 | {} (e.g. event hub)  | partial         |
| {USA}                 | {Portugal}           | mismatch        |
| {England}             | {Portugal}           | mismatch        |
| {USA, World, Cup}     | {USA, FIFA}          | exact (USA is shared) |
+-----------------------+----------------------+-----------------+

Returned by default in `results[]`: exact + partial
Returned in `mismatches[]` (debug only): mismatch
```

*Directional guidance for review, not implementation specification.*

### Recipe inference and substitution

```
After confirmed teaches:
  teach 1: "odds portugal wins world cup" -> KXMENWORLDCUP-26-PT
                                          -> will-portugal-win-the-2026-fifa-world-cup-912
  teach 2: "odds usa wins world cup"      -> KXMENWORLDCUP-26-US
                                          -> will-usa-win-the-2026-fifa-world-cup-467

Recipe extractor diff:
  query templates align except entities {Portugal} vs {USA}
  Kalshi ticker templates align except suffix -PT vs -US
    -> matches entity_lookups kind="country_iso2" (Portugal->PT, USA->US confirmed)
    -> recipe: {query: "odds {country} wins world cup", resource: "KXMENWORLDCUP-26-{country:iso2}"}
  PM slug templates align except token "portugal" vs "usa" + trailing -912 vs -467
    -> the country-lowercase token is predictable; the trailing number is not
    -> recipe: {query: ..., resource: "will-{country:lowercase}-win-the-2026-fifa-world-cup-*", strategy: "search-by-prefix"}

Future query: "odds england wins world cup"
  recall direct hit: NO
  recipe match: query template matches with {country}=England
  substitute:
    Kalshi:  entity_lookups[country_iso2][England] = GB
             -> candidate KXMENWORLDCUP-26-GB
             -> verify via local resources lookup OR one /markets/{ticker} call -> EXISTS
             -> return as result, entity_match=exact, source=recipe
    PM:      entity_lookups[country_lowercase][England] = england
             -> candidate prefix "will-england-win-the-2026-fifa-world-cup-"
             -> one /markets?slug_prefix= search to find the trailing number
             -> return concrete slug, source=recipe-search
```

*Directional guidance for review, not implementation specification.*

### Pre-seed scanner registration

```go
// Sketch, not the final API. Each consumer registers per resource type:
learn.RegisterPreseedScanner("kalshi_events", kalshiMutuallyExclusiveScanner)
learn.RegisterPreseedScanner("events",        polymarketNegRiskScanner)

// A scanner returns the (query_pattern, resource_id, entities) tuples to upsert.
// The core library handles the actual upsert + dedup; the scanner only knows
// how to extract patterns from its resource type.
type ScannerFn func(ctx context.Context, db *sql.DB) ([]PreseedRow, error)

type PreseedRow struct {
    QueryPattern string   // already normalized
    ResourceID   string
    ResourceType string
    Entities     []string // the entities this row represents
    Source       string   // "preseed" or a more specific tag
}
```

*Directional guidance for review, not implementation specification.*

---

## Output Structure

New package layout:

```
library/payments/prediction-goat/internal/learn/
├── doc.go                    # package overview, design notes, template-extraction guidance
├── entities/
│   ├── doc.go                # entity-extraction design + why patterns over lookup tables
│   ├── extract.go            # the generic extractor: capitalized, ALL-CAPS, ticker-shape
│   ├── extract_test.go
│   └── config.go             # per-CLI ticker patterns + stopword registration
├── normalize.go              # entity-preserving normalizer
├── normalize_test.go
├── match.go                  # entity-aware Jaccard + match-validation logic
├── match_test.go
├── recall.go                 # the recall query path -- result enrichment, mismatches surface
├── recall_test.go
├── teach.go                  # the write path: confidence floor, resource-shape validation
├── teach_test.go
├── preseed.go                # the scanner registry + driver loop
└── preseed_test.go
```

Prediction-goat-specific glue stays in `internal/cli/` and `internal/source/`:

```
library/payments/prediction-goat/internal/cli/
├── teach.go                  # MODIFIED: thin shim, delegates to internal/learn
├── learn_apply.go            # NEW: applyLearningsForTopic / applyLearningsForCompare
└── select_paths.go           # NEW: per-command --select cheatsheet (generated)

library/payments/prediction-goat/internal/source/
├── kalshi/preseed.go         # NEW: Kalshi mutually_exclusive scanner
└── polymarket/preseed.go     # NEW: Polymarket negRisk scanner
```

---

## Implementation Units

### U1. Pluggable entity extractor

**Goal:** Stop relying on hard-coded stopword lists for entity recognition. Build a data-driven extractor that pulls entity tokens from a query string by pattern (capitalized, ALL-CAPS, ticker-shape) with per-CLI configuration for ticker regex and domain stopwords.

**Requirements:** R3, R9, R10.

**Dependencies:** none — foundation.

**Files:**
- `library/payments/prediction-goat/internal/learn/entities/doc.go` (new — substantive design doc)
- `library/payments/prediction-goat/internal/learn/entities/extract.go` (new)
- `library/payments/prediction-goat/internal/learn/entities/config.go` (new — per-CLI registration)
- `library/payments/prediction-goat/internal/learn/entities/extract_test.go` (new)

**Approach:**
- Extractor takes a raw query string and returns `{Entities []string, Tickers []string, NonEntityTokens []string}`.
- Default extraction rules (in core, no domain knowledge): capitalized tokens that aren't sentence-initial OR ALL-CAPS tokens of length >= 2 OR tokens matching any registered ticker regex.
- Per-CLI hooks: ticker-shape regexes (registered as a slice of compiled `*regexp.Regexp`), domain stopwords (registered as a `map[string]struct{}`).
- Default English stopword list ships in the package: `a, an, the, is, are, was, were, be, of, to, in, on, for, with, what, which, who, whose, how, when, why, where, will`. No domain words.
- Prediction-goat registers via `config.go` at package init: ticker regexes for Kalshi (`^KX[A-Z0-9]+(-[A-Z0-9]+)*$`) and Polymarket (`^will-[a-z0-9-]+$`); domain stopwords (`odds, win, wins, winning, lose, loses, losing, beat, beats, beating`).

**Patterns to follow:**
- The existing `topicFTSQuery` in `internal/cli/liquid.go` for token-splitting shape.
- Cobra's `cmd.Flags()` pattern for per-command extension — apply the same registration-at-init philosophy here.

**Test scenarios:**
- "odds USA wins world cup" extracts `Entities=["USA"]` (ALL-CAPS), `NonEntityTokens=["world", "cup"]`.
- "what are Portugal's odds at the world cup" extracts `Entities=["Portugal"]` (capitalized), `NonEntityTokens=["world", "cup"]`.
- "england wins the world cup" extracts `Entities=["England"]`, `NonEntityTokens=["world", "cup"]`. (Note: `wins` and `the` go into stopwords, not non-entity.)
- "KXMENWORLDCUP-26 odds" extracts `Tickers=["KXMENWORLDCUP-26"]`, `NonEntityTokens=[]`.
- "will-usa-win-the-2026-fifa-world-cup-467" (passed as a raw query) extracts `Tickers=["will-usa-win-the-2026-fifa-world-cup-467"]`.
- Sentence-initial capitalized words don't get captured as entities ("Portugal odds" → "Portugal" extracted; "The odds of Portugal" → "Portugal" extracted; "Portugal" must be the result either way, not "The"). Implemented by skipping the first token if it's capitalized AND would otherwise be a stopword.
- A query with no entities or tickers (e.g., "rate cut") returns empty Entities and empty Tickers, all tokens go to NonEntityTokens.
- Per-CLI stopword registration adds to the default set; calling `RegisterStopwords` twice merges; no global mutation across test runs (use per-instance config).
- A new CLI registering a different ticker regex (e.g., `^[A-Z]{3}$` for stock symbols) extracts those without touching the prediction-goat regexes.

**Verification:** `go test ./internal/learn/entities/...` passes. Manual smoke: `prediction-goat-pp-cli recall "odds USA wins world cup" --agent --debug-entities` (debug flag added in U3) returns entities=["USA"].

---

### U2. Entity-preserving normalizer

**Goal:** Replace the current lowercase-everything normalizer with one that preserves entity tokens separately from non-entity tokens. Match logic in U3 uses both fields.

**Requirements:** R1, R3.

**Dependencies:** U1.

**Files:**
- `library/payments/prediction-goat/internal/learn/normalize.go` (new)
- `library/payments/prediction-goat/internal/learn/normalize_test.go` (new)

**Approach:**
- `Normalize(query) NormalizedQuery` where `NormalizedQuery = {Original string, Entities []string, NonEntityNormalized string, Tickers []string}`.
- Non-entity tokens get lowercased, collapsed-whitespace, stopword-stripped, sorted (for stable comparison).
- Entities preserved as-is from the extractor (case-preserving for display, but compared case-insensitive in match logic).
- The on-disk schema (`search_learnings.query_pattern`) stores the **non-entity normalized form**. Entities are stored in a new column `query_entities` as a JSON array. This requires a v4→v5 schema migration.

**Patterns to follow:**
- Existing migration pattern in `internal/store/store.go` — `current < N` guarded migration block, table rebuild + reindex pattern from `migrateResourcesFTSCuratedContent`.

**Test scenarios:**
- "odds England wins the world cup" normalizes to `{Entities: ["England"], NonEntityNormalized: "cup world", Tickers: []}` (alphabetical sort makes comparison stable).
- "odds Portugal wins the world cup" normalizes to `{Entities: ["Portugal"], NonEntityNormalized: "cup world"}`. Non-entity overlap is 1.0; entity overlap is 0. (U3 will reject the match — this test only covers the normalize output.)
- Empty query returns `{Entities: [], NonEntityNormalized: "", Tickers: []}`.
- Query with only stopwords ("the of for") returns empty NonEntityNormalized.
- Schema migration from v4 to v5: existing learnings get backfilled — for each existing row, run entity extraction against the stored `query_pattern` and populate `query_entities`. Rows that fail extraction (e.g., empty queries) get `[]`.

**Verification:** `go test ./internal/learn/... -run Normalize` passes. Schema v5 stamps on first Open at new version; existing rows have `query_entities` populated.

---

### U3. Entity-aware recall with structured warnings

**Goal:** Recall returns results filtered by entity match, with per-row `entity_match` and `warnings` fields. Mismatches surface in a debug-only `mismatches` array. SKILL.md teaches the LLM to read these.

**Requirements:** R1, R2, R8.

**Dependencies:** U1, U2.

**Files:**
- `library/payments/prediction-goat/internal/learn/match.go` (new)
- `library/payments/prediction-goat/internal/learn/recall.go` (new)
- `library/payments/prediction-goat/internal/learn/recall_test.go` (new)
- `library/payments/prediction-goat/internal/cli/teach.go` (modify: update `recall` subcommand to use the new package)

**Approach:**
- `Recall(db, query, opts)` returns:
  ```
  {
    Found bool,
    MatchScore float64,
    Results []Hit,         // entity_match in {exact, partial}, sorted by entity_match DESC then confidence DESC
    Mismatches []Hit,      // entity_match=mismatch, returned only when --debug-mismatches is passed
    Normalized string,
    QueryEntities []string,
    Warnings []string,     // top-level warnings (e.g., "no learnings found for this query family")
  }
  ```
- Each `Hit` carries `{ResourceID, ResourceType, Venue, Action, Confidence, MatchScore, EntityMatch, ResourceEntities []string, Warnings []string, Source, LastObservedAt}`.
- Per-hit warnings populated by:
  - `parent_event_when_child_exists`: resource is a parent ticker (e.g., matches `^KX\w+-\d+$` with no further suffix) AND a child resource with a matching entity exists in `resources`.
  - `low_confidence`: confidence is 1 (below the skip threshold) — note that with U6 this becomes rare.
- New flag `--debug-mismatches` includes the `mismatches` array in JSON output (default: omitted).
- The `recall` CLI command in `internal/cli/teach.go` delegates to `learn.Recall`.

**Patterns to follow:**
- Existing recall logic in `internal/store/learnings.go` for the SQL shape.
- `meta.teach_hint` in `internal/cli/agent_context.go` for the warnings serialization shape.

**Test scenarios:**
- Cold query with no matching learnings: `{found: false, results: [], mismatches: [], query_entities: ["X"]}`.
- Query "odds England wins world cup" against a learning taught for "odds Portugal wins world cup":
  - Non-entity Jaccard ≈ 1.0
  - Entity overlap = 0
  - Returned in `mismatches` (only when `--debug-mismatches`), NOT in `results`
- Query "odds USA wins world cup" against a learning for `KXMENWORLDCUP-26` (parent event) when child `KXMENWORLDCUP-26-US` exists:
  - Entity match = exact (USA is in `yes_sub_title` of the parent's child markets, and entity extraction on the parent's title pulls "USA" via the child resource).
  - But warning `parent_event_when_child_exists` populated, naming the child.
- Query "rate cut" with no entities matches a learning with no entities purely on non-entity Jaccard (categorical match, low risk).
- Multiple boost rules ordered: `entity_match=exact, confidence=3` ranks above `entity_match=exact, confidence=1` ranks above `entity_match=partial, confidence=3`.
- `recall --no-learn` returns `{found: false}` regardless of stored learnings.
- A learning where the resource itself is missing from `resources` returns `{entity_match: "unknown"}` and a warning `resource_not_in_store` — the LLM can decide to use it (the ticker is still valid for direct API fetch) or treat as cold start.

**Verification:** `go test ./internal/learn/... -run Recall` passes. Manual: simulating the England→Portugal trace, `recall "odds england wins the world cup" --agent` returns `found: false` despite the Portugal learning existing.

---

### U4. Confidence floor + skill threshold sync

**Goal:** First teach lands at `confidence=2`. The SKILL.md threshold ("skip if confidence>=2") fires on first re-use.

**Requirements:** R4, R8.

**Dependencies:** U3 (so recall returns the right shape to teach against).

**Files:**
- `library/payments/prediction-goat/internal/learn/teach.go` (new)
- `library/payments/prediction-goat/internal/learn/teach_test.go` (new)
- `library/payments/prediction-goat/internal/cli/teach.go` (modify: update `teach` to call `learn.Teach`)
- `library/payments/prediction-goat/library/payments/prediction-goat/SKILL.md` (modify: update threshold language for clarity, even though the threshold itself doesn't move)

**Approach:**
- `Teach(db, opts)` writes with `confidence=2` on first insertion. Re-confirmation (same `(query_pattern, resource_id, action)` tuple) increments by 1 (so 2 → 3 → 4 ...). The unique constraint already in place handles the dedup.
- SKILL.md is updated to be unambiguous: "if `found=true` AND `results[0].entity_match` is `exact` AND `results[0].confidence >= 2`, skip discovery."
- Note: no change to the `confidence_initial` for the `inferred-followup` source — pre-seed (U5) lands at confidence=2 too.

**Patterns to follow:**
- Existing `UpsertLearning` in `internal/store/learnings.go`. The migration to the new path is straight delegation; behavior outside the initial-value bump stays identical.

**Test scenarios:**
- First teach of a (query, resource, action) tuple writes `confidence=2`.
- Re-teach same tuple bumps to `confidence=3`.
- After teach, `recall` with the exact same query returns `entity_match=exact, confidence=2` — agent's skill threshold (>=2) clears on first re-use.
- The `inferred-followup` source pathway (if it exists in the code) also starts at 2 — verify with a regression test against the existing teach behavior.

**Verification:** `go test ./internal/learn/... -run Teach` passes. Manual: after one teach of "portugal world cup odds" → `KXMENWORLDCUP-26-PT`, recall returns `confidence: 2`.

---

### U5. Multi-outcome family pre-seed at sync time

**Goal:** After each sync, scan for multi-outcome event families and pre-populate `search_learnings` with `(query_pattern, resource_id)` mappings for every child entity. Cold start vanishes for queries like "odds {team} wins {tournament}" when the tournament has been synced.

**Requirements:** R5, R10.

**Dependencies:** U1 (entity extractor), U2 (normalizer), U4 (confidence floor).

**Files:**
- `library/payments/prediction-goat/internal/learn/preseed.go` (new — registry + driver)
- `library/payments/prediction-goat/internal/learn/preseed_test.go` (new)
- `library/payments/prediction-goat/internal/source/kalshi/preseed.go` (new — Kalshi scanner)
- `library/payments/prediction-goat/internal/source/kalshi/preseed_test.go` (new)
- `library/payments/prediction-goat/internal/source/polymarket/preseed.go` (new — Polymarket scanner)
- `library/payments/prediction-goat/internal/source/polymarket/preseed_test.go` (new)
- `library/payments/prediction-goat/internal/cli/kalshi.go` (modify: invoke preseed.Run after SyncMarkets completes)
- `library/payments/prediction-goat/internal/cli/sync.go` (modify: invoke preseed.Run after Polymarket sync completes)

**Approach:**
- `preseed.RegisterScanner(resourceType string, fn ScannerFn)` registers a scanner per resource type.
- `preseed.Run(ctx, db)` iterates all registered scanners, collects `PreseedRow`s, upserts via `learn.Teach` with `source='preseed'`, confidence=2.
- Kalshi scanner: query `kalshi_events WHERE mutually_exclusive=true`; for each event, query child `kalshi_markets WHERE event_ticker=?`; for each child with non-empty `yes_sub_title`, derive query patterns:
  - `"odds {yes_sub_title} wins {event.title}"`
  - `"{yes_sub_title} wins {event.title}"`
  - `"{yes_sub_title} {extracted_topic_words_from_event_title}"` (e.g., "Portugal World Cup")
  - Upsert each as a boost learning targeting the child ticker.
- Polymarket scanner: query `events WHERE negRisk=true`; for each event, query child markets; for each child whose question starts with `Will the {entity}`, extract `{entity}` and build the same shape of query patterns.
- All upserts use the unique constraint to dedup with existing user-taught rows (same `(query_pattern, resource_id, action)` doesn't create a duplicate — it bumps confidence). User teaches are always preserved; pre-seed never overwrites a higher-confidence row.
- A new `--no-preseed` flag on the sync commands disables preseed for the run (debug/testing).
- Volume guard: preseed only writes for event families whose total child volume > 0 OR which are tagged Sports/Politics (the highest-value categories — but this filter is per-CLI config, not hard-coded; default = no filter).

**Patterns to follow:**
- Existing scanner pattern in `internal/source/kalshi/series_walk.go` (the U6 series walk from the prior plan) — uses the same kind of bounded fan-out.
- The unique-constraint dedup pattern in `internal/store/learnings.go`.

**Test scenarios:**
- Kalshi scanner against a fixture with 1 event `mutually_exclusive=true` and 3 child markets → produces 3 PreseedRow entries each with valid `(query_pattern, resource_id, entities)`.
- Polymarket scanner against a fixture with 1 `negRisk` event and 5 child markets → produces 5 PreseedRow entries.
- `preseed.Run` upserts all rows; second invocation is idempotent (no duplicate rows, no confidence inflation on pre-seeded rows because the unique constraint catches it; we should NOT bump confidence on a pre-seed/pre-seed re-observation — that's not real evidence).
- User-taught row exists for `(query_pattern="odds usa wins world cup", resource_id="KXMENWORLDCUP-26-US", confidence=3)`. Pre-seed produces the same `(query_pattern, resource_id, action)` with confidence=2. After upsert: confidence stays at 3 (max-of, not overwrite). Source stays `taught` (user signal wins over pre-seed).
- A child market with empty `yes_sub_title` is skipped (no entity to key on).
- `--no-preseed` skips the call entirely; existing learnings unchanged.
- After running preseed on a seeded fixture, `recall "odds USA wins world cup" --agent` returns `found: true, source: preseed, confidence: 2, entity_match: exact` with `KXMENWORLDCUP-26-US` at position 0.

**Verification:** `go test ./internal/learn/... -run Preseed` and `go test ./internal/source/kalshi/... -run Preseed` and `go test ./internal/source/polymarket/... -run Preseed` all pass. Manual end-to-end: drop the DB, run `kalshi sync && sync` (Polymarket), then `recall "odds USA wins world cup" --agent` returns the right tickers without any explicit `teach` call.

---

### U6. Resource-shape validation at teach time

**Goal:** `teach` warns (to `teach.log`, surfaced in `learnings list --warnings`) when the resource being taught is a parent ticker but a more specific child exists matching the query entity.

**Requirements:** R6, R8.

**Dependencies:** U1 (entity extractor), U4 (teach package).

**Files:**
- `library/payments/prediction-goat/internal/learn/teach.go` (modify: add resource-shape validator)
- `library/payments/prediction-goat/internal/learn/teach_test.go` (extend)
- `library/payments/prediction-goat/internal/cli/teach.go` (modify: pass `validate` flag — default true)

**Approach:**
- At teach time, for each `--resource` passed, look up the resource in `resources`. If `resource_type` matches the registered "parent-shape" patterns (configurable: `kalshi_events` for Kalshi, events with `negRisk=true` for Polymarket), AND the query carries entities, AND a child resource exists whose `yes_sub_title` or `question` contains the query entity, emit a warning to `teach.log`:
  ```
  ts=... action=teach query="odds USA wins world cup" resource=KXMENWORLDCUP-26 warning=parent_event_when_child_exists suggested_child=KXMENWORLDCUP-26-US
  ```
- The teach still succeeds. The LLM may legitimately want the parent (e.g., for a query like "what's the overall World Cup market"). Warning is for offline diagnostic only.
- `--no-validate` flag suppresses the check (for scripted/batch teaches).
- `learnings list --warnings` surfaces all teach.log warnings — joins against the warnings file by ts/resource_id.

**Patterns to follow:**
- Existing logging-to-file pattern for `feedback.jsonl` and `teach.log` in the existing CLI surface.
- Resource lookup pattern in `internal/cli/teach.go`'s `applyLearningsForTopic` (the existing "fetch the resource by id" call).

**Test scenarios:**
- `teach --query "odds USA wins world cup" --resource KXMENWORLDCUP-26` when `KXMENWORLDCUP-26-US` exists in `resources` → `teach.log` gets a `parent_event_when_child_exists` warning naming the child.
- `teach --query "world cup overview" --resource KXMENWORLDCUP-26` (no entity in query) → no warning emitted; the parent is the right target.
- `teach --query "..." --resource <slug>` for a Polymarket slug → resource-shape validator runs but Polymarket-side parent-vs-child check uses `events` table, not Kalshi tables.
- `--no-validate` suppresses the warning.
- A teach with multiple `--resource` args, some valid + some warning-generating → all writes succeed; warnings emitted per-resource.

**Verification:** `go test ./internal/learn/... -run Validate` passes. Manual: replay the USA failure trace — teach `KXMENWORLDCUP-26` against "odds USA wins world cup", grep `teach.log` for `parent_event_when_child_exists`.

---

### U7. Per-command `--select` cheatsheet in agent-context

**Goal:** Stop agents guessing dotted paths. Every command exposes its valid `--select` field set via `agent-context.commands.<name>.select_paths`. Data is generated at build time from response struct tags.

**Requirements:** R7.

**Dependencies:** none — orthogonal to learning changes.

**Files:**
- `library/payments/prediction-goat/internal/cli/select_paths.go` (new — generated file; commit the generated output)
- `library/payments/prediction-goat/internal/cli/select_paths_gen.go` (new — `go:generate` directive + small AST walker)
- `library/payments/prediction-goat/internal/cli/agent_context.go` (modify: include `commands.<name>.select_paths` in the response)

**Approach:**
- A small build-time tool (Go program under `tools/select-paths-gen/`) walks the AST of every command response struct (e.g., `topicResult`, `compareResult`, `mispricedResult`, `kalshiMarketsGetResult`), extracts JSON-tagged field paths recursively (descending into nested structs and slices with the `paths.subpath` convention), and emits `internal/cli/select_paths.go` as a `map[string][]string{commandName: ["field1", "nested.field2", ...]}`.
- `go:generate go run tools/select-paths-gen` runs in CI; the committed file must match the regenerated output (CI lint enforces).
- `agent-context` reads this map and exposes it under `commands.<name>.select_paths`.
- Bonus: a `which <command> --select-paths` flag dumps the paths for a single command for quick lookup.

**Patterns to follow:**
- The existing `tools/sweep-canonical/` Go tool in the repo root — same shape of small AST-walking helper that emits a generated file, run via `go:generate`.
- Existing `agent-context` envelope shape in `internal/cli/agent_context.go`.

**Test scenarios:**
- `agent-context` output contains `commands.markets get-by-slug.select_paths` with entries like `["question", "slug", "outcomes", "outcomePrices", "lastTradePrice", "bestBid", "bestAsk", "volume", "liquidity", "endDate"]`.
- Adding a new field to `markets get-by-slug`'s response struct without regenerating fails the CI lint check.
- Removing a field surfaces in the regenerated map (and removes it from agent-context).
- `which markets get-by-slug --select-paths` prints the same set as agent-context exposes.
- For a command whose response is a polymorphic envelope (`{meta: ..., results: ...}`), nested paths include `meta.price_source`, `meta.synced_at`, `results.<inner>`.

**Verification:** `go generate ./...` produces no diff in CI. `prediction-goat-pp-cli agent-context --agent | jq '.commands["markets get-by-slug"].select_paths | length'` returns the expected count.

---

### U8. SKILL.md protocol upgrade + inline source-code comments

**Goal:** SKILL.md "Automatic learning" section rewritten as a real protocol document. Every new source file in `internal/learn/` ships with substantive top-of-file design comments. Every modified function carries a *why* comment.

**Requirements:** R8, R9, R10.

**Dependencies:** U1–U7 (must describe accurate APIs).

**Files:**
- `library/payments/prediction-goat/SKILL.md` (modify: rewrite "Automatic learning" section)
- Every new file from U1-U7 (modify: ensure substantive doc comments per below)
- `library/payments/prediction-goat/internal/learn/doc.go` (new — package-level design notes + template-extraction guidance)

**Approach:**

**SKILL.md "Automatic learning" section** rewrites to roughly 30-40 lines covering:

1. The protocol in one sentence: *before any discovery command on a new user question, run `recall`; before emitting the user-facing response, fire `teach &` in the background.*

2. **How to read `recall` results.** Pseudo-code agent protocol:
   ```
   recall "<query>" --agent
   if found && results[0].entity_match == "exact" && results[0].confidence >= 2:
     -> skip discovery, fetch live prices for results[*].resource_id in parallel
   elif found && results[0].entity_match == "partial":
     -> treat as "candidate hint, not a hit": validate by reading the resource title before using
   elif found && (any result has entity_match == "mismatch" in mismatches[] when --debug-mismatches passed):
     -> treat as cold start; the stored learning is for a different entity
   else (found = false):
     -> cold start; run discovery normally; teach the answer afterward
   ```

3. **Concrete worked examples from the failure traces.** Include 3 short examples mirroring the USA/Portugal/England sessions, showing what the agent SHOULD have done.

4. **The `warnings` field.** Always read it. The most important warning today is `parent_event_when_child_exists` — when a recall hit carries it, fetch the suggested child instead of the parent, even if the parent is technically a valid resource.

5. **The `teach` protocol.** When backgrounding teach, pass resources at the most specific level. If you fetched a parent event during discovery but the answer lives in a specific child, teach the child, not the parent. The CLI will warn (to teach.log) when you don't, but the human user won't see the warning.

6. **`--no-learn` and `PREDICTION_GOAT_NO_LEARN=true`** disable the entire pipeline for deterministic agent flows. Use when running batch operations or tests.

**Inline source-code comments:**

- Every new file in `internal/learn/` starts with a top-of-file doc comment of 5-15 lines explaining: *what design problem this file solves, what alternative shapes were considered, what extension points exist for the next CLI*. Example for `entities/extract.go`:
  ```
  // Package entities extracts entity tokens from CLI queries to enable
  // entity-aware match validation in the learning subsystem.
  //
  // Why patterns over lookup tables: a hard-coded country list would tie
  // the package to the prediction-market domain, defeating the goal of
  // sharing this subsystem across all PP CLIs. Patterns (capitalized,
  // ALL-CAPS, ticker-shape) generalize to any domain whose entities have
  // distinguishing shape -- which is most of them.
  //
  // Extension points for a new CLI:
  //   - Register ticker regexes via Config.RegisterTickerPattern
  //   - Add domain stopwords via Config.RegisterStopwords
  //   - Override the default capitalization heuristic via Config.EntityHook
  //     (rare; most CLIs use the default)
  //
  // See docs/plans/2026-05-23-002-feat-prediction-goat-smart-learning-plan.md
  // section "Key Technical Decisions" for the full rationale.
  ```

- Every public function carries a `// Why:` line in its doc comment for any non-obvious choice. Plain mechanical functions don't need it.

- `internal/learn/doc.go` is a 30-50 line package-level design doc covering: subsystem purpose, the teach/recall/preseed lifecycle, the extension surface for new CLIs, the schema migration path, and how to extract this package into a generator template later.

**Patterns to follow:**
- Existing inline comments in `internal/store/store.go` around the migration logic — that pattern of "Why: ... See: ..." is the model.
- AGENTS.md at the repo root — the protocol document shape is similar.

**Test scenarios:**

This unit is mostly documentation, but the documentation has structural requirements:

- SKILL.md contains a `## Automatic learning` H2 section.
- The section describes the recall protocol with the four branches (exact / partial / mismatch / cold).
- The section names at least one specific worked example from the failure traces.
- Every new `.go` file in `internal/learn/` has a top-of-file doc comment of at least 5 non-blank lines.
- `internal/learn/doc.go` exists and contains the strings `Extension points` and `template`.
- `verify_skill.py` still passes against the modified SKILL.md.

**Verification:** `python3 .github/scripts/verify-skill/verify_skill.py --dir library/payments/prediction-goat/` passes. Manual: a fresh agent invoked on "odds X wins Y" can follow the SKILL.md protocol from the markdown alone, without reading this plan.

---

### U9. Entity-lookup table + seeded data + `teach-lookup` command

**Goal:** Ship a SQLite `entity_lookups` table seeded with canonical reference data (ISO country codes, sports team abbreviations, etc.) that recipes use for substitution. Provide a `teach-lookup` command for users to add per-domain entries. Auto-populate new entries from confirmed teaches when the structural inference is unambiguous.

**Requirements:** R10, R11, R12.

**Dependencies:** U1 (entity extractor), U2 (normalizer).

**Files:**
- `library/payments/prediction-goat/internal/learn/lookups/doc.go` (new — design notes + extension guidance)
- `library/payments/prediction-goat/internal/learn/lookups/store.go` (new — generic key/value lookups CRUD)
- `library/payments/prediction-goat/internal/learn/lookups/store_test.go` (new)
- `library/payments/prediction-goat/internal/learn/lookups/seeds/countries.go` (new — ISO 3166 data as Go constants)
- `library/payments/prediction-goat/internal/learn/lookups/seeds/sports.go` (new — common league abbreviations)
- `library/payments/prediction-goat/internal/learn/lookups/seeds/generic.go` (new — lowercase, kebab-case, capitalize-first as kinds)
- `library/payments/prediction-goat/internal/store/store.go` (modify: v5 → v6 migration adds `entity_lookups` table + seeds)
- `library/payments/prediction-goat/internal/cli/teach_lookup.go` (new — `teach-lookup` CLI command)
- `library/payments/prediction-goat/internal/cli/teach_lookup_test.go` (new)

**Approach:**

- `entity_lookups` schema:
  ```sql
  CREATE TABLE entity_lookups (
      kind TEXT NOT NULL,           -- 'country_iso2', 'nfl_team_3letter', etc.
      canonical TEXT NOT NULL,      -- 'Portugal' (case-preserving for the source name)
      value TEXT NOT NULL,          -- 'PT'
      source TEXT NOT NULL,         -- 'seeded' | 'taught' | 'inferred'
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      PRIMARY KEY (kind, canonical)
  );
  CREATE INDEX idx_entity_lookup_canonical ON entity_lookups(canonical);
  ```
- The schema migration (v5 → v6) creates the table and runs the seed inserts in one transaction. Seeds use `INSERT OR IGNORE` so re-running is a no-op.
- Generic kinds (`lowercase`, `kebab-case`, `capitalize-first`, `uppercase`) are computed kinds — no rows in the table; the lookup helper short-circuits to a string transform. Same surface, no maintenance.
- `lookups.Lookup(kind, canonical)` returns `(value, found)`. Lookup is case-insensitive on the canonical side. Computed kinds bypass the DB.
- `teach-lookup --kind country_iso2 --canonical Curaçao --value CW` writes a row with `source='taught'`. Idempotent.
- The seeded country data covers all 200+ ISO 3166-1 entries (alpha-2 + alpha-3 + lowercase variants). NFL team abbreviations cover all 32 teams. NBA covers all 30. MLB all 30. MLS all 30 (current as of 2026 season).

**Patterns to follow:**
- The existing `sync_state` table schema in `internal/store/store.go` for the per-row metadata shape.
- Generic kind ("uppercase", "lowercase", etc.) implementation pattern from `strings.ToLower` / `strings.ToUpper` — wrap in a small `ComputedKind(kind) bool` predicate.

**Test scenarios:**
- After v6 migration, `lookups.Lookup("country_iso2", "Portugal")` returns `("PT", true)`.
- Case-insensitive canonical: `lookups.Lookup("country_iso2", "portugal")` returns `("PT", true)`.
- Computed kind: `lookups.Lookup("lowercase", "Portugal")` returns `("portugal", true)` without touching the DB.
- `lookups.Lookup("country_iso2", "Atlantis")` returns `("", false)`.
- `teach-lookup --kind country_iso2 --canonical Curaçao --value CW` writes a row; subsequent lookup succeeds.
- Re-running the v6 migration on an already-migrated DB is a no-op (idempotent).
- A new CLI adding its own kind (`stock_ticker`) via direct INSERT works without code changes to the `lookups` package.
- The seeded NFL roster covers all 32 teams with both 3-letter (`NE`, `DAL`) and lowercase-city variants (`patriots`, `cowboys`).

**Verification:** `go test ./internal/learn/lookups/...` passes. Manual: `prediction-goat-pp-cli teach-lookup --kind country_iso2 --canonical "Bosnia and Herzegovina" --value BA` succeeds; subsequent recipe substitution can find it.

---

### U10. Recipe table + inference engine + recall fallback integration

**Goal:** When direct learning lookup misses, the recall path falls back to recipe substitution. Recipes are extracted automatically after confirmed teaches when structure repeats across teaches. `teach-recipe` exposes explicit authorship for power users.

**Requirements:** R10, R11.

**Dependencies:** U1, U2, U3, U4, U9.

**Files:**
- `library/payments/prediction-goat/internal/learn/recipes/doc.go` (new — design notes + extension guidance, especially "what makes a recipe inferrable")
- `library/payments/prediction-goat/internal/learn/recipes/store.go` (new — `search_recipes` table CRUD)
- `library/payments/prediction-goat/internal/learn/recipes/extract.go` (new — auto-extraction from teach pairs)
- `library/payments/prediction-goat/internal/learn/recipes/apply.go` (new — substitution + verification)
- `library/payments/prediction-goat/internal/learn/recipes/store_test.go` (new)
- `library/payments/prediction-goat/internal/learn/recipes/extract_test.go` (new)
- `library/payments/prediction-goat/internal/learn/recipes/apply_test.go` (new)
- `library/payments/prediction-goat/internal/learn/recall.go` (modify — fall back to recipe apply when direct hit misses)
- `library/payments/prediction-goat/internal/learn/teach.go` (modify — trigger extract after every teach)
- `library/payments/prediction-goat/internal/store/store.go` (modify — v6 → v7 migration for `search_recipes` table)
- `library/payments/prediction-goat/internal/cli/teach_recipe.go` (new — `teach-recipe` CLI command)
- `library/payments/prediction-goat/internal/cli/teach_recipe_test.go` (new)

**Approach:**

- `search_recipes` schema:
  ```sql
  CREATE TABLE search_recipes (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      query_template TEXT NOT NULL,        -- 'odds {country} wins world cup'
      resource_template TEXT NOT NULL,     -- 'KXMENWORLDCUP-26-{country:iso2}'
      resource_type TEXT NOT NULL,
      venue TEXT,
      strategy TEXT NOT NULL,              -- 'substitute' | 'substitute-then-search-prefix'
      confidence INTEGER NOT NULL DEFAULT 2,
      source TEXT NOT NULL,                -- 'inferred' | 'taught'
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      last_observed_at DATETIME,
      example_query TEXT,                  -- the original query that seeded the recipe
      example_resource TEXT                -- the original resource
  );
  CREATE INDEX idx_recipes_query_template ON search_recipes(query_template);
  ```

- **Extraction algorithm** (`recipes.Extract(teaches []Teach) []Recipe`):
  1. Group recent teaches (last N=20) by structural signature: normalized non-entity tokens of the query AND token-shape signature of the resource (alphanumeric prefix length, separator positions, suffix shape).
  2. For each group of 2+ teaches, find the variable positions in both query and resource by diffing.
  3. For each variable position, attempt to match against known entity-lookup kinds:
     - Walk every kind in `entity_lookups` (cheap; in-memory cache of the table at startup).
     - For each kind, check if `lookups.Lookup(kind, query_entity)` returns the resource's variable value.
     - If a kind matches for ALL teaches in the group, that's the recipe's binding.
  4. If a binding is found, write a `search_recipes` row with `source='inferred', confidence=2`.
  5. If no kind matches but the resource variable is a simple string transform of the query entity (lowercase, kebab-case, etc.), use the computed kind.
  6. If the resource variable has an unpredictable trailing segment (e.g., the `-912` on Polymarket slugs), set `strategy='substitute-then-search-prefix'` and emit a recipe whose resource template ends with `*`. Apply path searches by prefix.

- **Application** (`recipes.Apply(ctx, db, query) []Hit`):
  1. Extract entities from the query.
  2. Load all recipes; for each, attempt to match the query against the query_template (entity-aware Jaccard on the non-slot tokens).
  3. For matches, substitute the new entity via `lookups.Lookup(recipe.entity_kind, query_entity)`.
  4. For `strategy='substitute'`: verify the substituted resource exists in `resources` (free) or via one targeted API fetch.
  5. For `strategy='substitute-then-search-prefix'`: build the prefix, search via the venue's prefix-search endpoint (or local store), return the matching resource.
  6. Return hits with `source='recipe', entity_match='exact'`, ranked by recipe confidence and verification success.

- **Recall integration**: in `Recall`, after the existing direct-lookup path returns its results, ALSO run `recipes.Apply` and merge the recipe results into the response. Recipe-sourced hits are clearly tagged so the agent (and humans) can tell them apart from direct teaches.

- **Teach integration**: after a successful `Teach`, fire `recipes.Extract` in-process. This walks the recent-teaches window and produces new or updated recipes. Idempotent; safe to run on every teach.

- **`teach-recipe` command** (for explicit authorship):
  ```
  teach-recipe \
    --query-template "odds {country} wins world cup" \
    --resource-template "KXMENWORLDCUP-26-{country:country_iso2}" \
    --resource-type kalshi_markets
  ```
  Writes a recipe directly. Useful when the inference engine doesn't see enough teaches yet.

**Patterns to follow:**
- `internal/learn/match.go` (from U3) for entity-aware token comparison.
- `internal/learn/lookups/` (from U9) for the lookup interface.
- The `Upsert` idempotency pattern from `internal/store/learnings.go`.

**Test scenarios:**

*Extraction:*
- Two teaches: `("odds portugal wins world cup", "KXMENWORLDCUP-26-PT")` and `("odds usa wins world cup", "KXMENWORLDCUP-26-US")`. Extract produces a recipe `{query: "odds {country} wins world cup", resource: "KXMENWORLDCUP-26-{country:country_iso2}"}` with confidence=2.
- Two teaches with totally different shapes don't produce a recipe.
- Two Polymarket slug teaches differing only in the country word and a trailing number: extract produces a `strategy='substitute-then-search-prefix'` recipe with the country slot bound to `country:lowercase`.
- Single teach: no extraction (need 2+ for inference).
- Extraction is idempotent: running on a corpus that already produced recipe X does not duplicate.
- When teaches use ALL CAPS entity tokens that match the `country_iso2` directly (no transformation needed), the inferred kind is `country_iso2` (not `uppercase`).

*Apply:*
- After extraction of the Portugal+USA recipe above, calling `Apply` with query "odds england wins world cup" returns a hit with `resource_id="KXMENWORLDCUP-26-GB"` (verified via `resources` lookup), `source="recipe"`, `entity_match="exact"`.
- If `entity_lookups` lacks an entry for "Atlantis", `Apply` returns no hit for "odds atlantis wins world cup" — but emits a structured `meta.recipe_miss` reason so the agent knows it tried.
- `Apply` with `--no-verify` returns substituted candidates without the verification call (faster, but the agent has to verify itself).
- `strategy='substitute-then-search-prefix'` recipe for Polymarket: query "odds england wins world cup" → substitutes prefix `will-england-win-the-2026-fifa-world-cup-` → searches by prefix → returns the concrete slug.
- Recipe confidence ties broken by `last_observed_at` (newer wins).

*Recall integration:*
- Cold query against a corpus with a relevant recipe but no direct learning: `recall` returns `found=true, source="recipe"`.
- Query with both a direct learning AND a recipe match: direct learning ranks first; recipe surfaces as secondary candidate with `source="recipe"`.
- Recipe miss + no direct hit: `recall` returns `found=false, mismatches:[]` (the existing cold-start path).

*`teach-recipe` command:*
- Explicit `teach-recipe` write succeeds and is immediately applicable.
- Malformed template (e.g., unmatched braces, unknown lookup kind) returns a usage error.

**Verification:** `go test ./internal/learn/recipes/...` passes. End-to-end manual:
1. Drop DB and run sync.
2. Teach Portugal: `teach --query "odds portugal wins world cup" --resource KXMENWORLDCUP-26-PT --resource will-portugal-win-the-2026-fifa-world-cup-912`.
3. Inspect: `learnings list --recipes --agent` shows the inferred recipe.
4. Cold query for England: `recall "odds england wins world cup" --agent` returns the GB ticker and the England slug via recipe substitution, in two API calls or fewer.

---

## System-Wide Impact

- **Schema migrations:** v4 → v5 adds `query_entities` to `search_learnings`. v5 → v6 adds `entity_lookups` + seed data. v6 → v7 adds `search_recipes`. All migrations are lazy on first Open at the new version. Estimated total wall time for an existing user's DB: 2-5 seconds (the seeded country/sports data is ~500 rows of INSERT).
- **Sync wall time:** pre-seed scanner adds 1-2 seconds to `kalshi sync` and `sync` (Polymarket). Recipe extraction runs after every `teach` but only on the recent-teaches window (typically 5-20 rows in memory); adds <50ms per teach. No new API calls.
- **DB size:** entity_lookups seeded data is ~500 rows. Recipes for a heavy user typically < 50 rows in the steady state (recipes are generalizations, so the table stays small). Pre-seeded learnings still bounded as before.
- **Binary size:** seeded lookup data ships embedded in the binary as Go constants — adds ~30KB to the binary. Negligible.
- **Existing teaches:** preserved. Unique-constraint dedup ensures preseed and recipe extraction never overwrite a higher-confidence user-taught row. Recipe inference reads from existing teaches but does not modify them.
- **MCP server:** no changes. The new commands (`teach-lookup`, `teach-recipe`, `recall --debug-mismatches`, `learnings list --warnings`, `learnings list --recipes`) are CLI-only for now.
- **CI:** new `go generate` step for U7's select-paths file. New import-boundary lint for `internal/learn/`. Documentation lint for inline comments.
- **Greptile:** the SKILL.md rewrite + the new `internal/learn/lookups/seeds/` data files are substantial additive changes; expect stylistic feedback but no logic findings.

---

## Risks and Mitigations

- **Entity extractor too greedy.** A capitalized word in the middle of a query that's actually a common noun ("New" in "New York Times") could be misclassified as an entity. Mitigation: the default capitalized-word rule only fires for tokens that aren't in the per-CLI stopword list; "New York Times" lands as `Entities=["New", "York", "Times"]` and the match logic uses set overlap — a learning tagged with "New York Times" matches a query tagged with "New York Times" fine, and a learning tagged with "Times" alone partial-matches. Acceptable. The proper noun phrase extractor would solve this fully but is a v2 nice-to-have.
- **Pre-seed scanner produces stale entries.** If a multi-outcome family resolves (event ends, child markets get archived), the pre-seeded learnings for those children still exist. Mitigation: pre-seed writes `source='preseed'` and `last_observed_at` at write time. A periodic prune (deferred to follow-up) can drop pre-seeded rows whose target resource is no longer active.
- **Schema migration on a large existing DB.** The v4→v5 backfill walks every row in `search_learnings`. For a heavy user with thousands of taught rows the migration could take a few seconds. Mitigation: WAL mode (already on from prior work) means the migration doesn't block reads. Document the one-time hit in the release note.
- **Confidence floor change interacts with existing data.** Existing rows at confidence=1 stay at 1. Their next observation bumps to 2, but the *first* re-use after deploy still fails the skip threshold. Mitigation: a one-shot data migration bumps all existing rows from confidence=1 to confidence=2 at v5 migration time. (Or: skip the migration and just let existing rows decay naturally. Either is acceptable — the failure mode is "old learnings need one more re-confirmation than necessary," not "broken.")
- **`--select` cheatsheet drifts from runtime reality.** If a command's response struct is modified without regenerating, agents read stale select_paths and burn roundtrips. Mitigation: CI lint enforces commit matches regenerate. Optionally, at runtime the command can detect a `--select` path that doesn't exist in its response and return an explicit error instead of an empty result.
- **The pre-seed scanner runs slowly on huge corpuses.** If Kalshi grows to 100k events with deep multi-outcome trees, the scanner could become slow. Mitigation: the scanner is bounded by the existing per-resource type filter (`mutually_exclusive=true` is a small subset). Cap pre-seed writes per run at a configurable maximum (default 10k rows; if exceeded, log a warning and skip the tail).
- **Recipe inference produces wrong templates from noisy teaches.** Two unrelated teaches that happen to have structurally similar shape could trigger a bad recipe (e.g., a query about country X and a query about NFL team Y might share enough structure to confuse the extractor). Mitigation: extraction requires at least 2 teaches to agree AND requires the matched entity-lookup kind to be the same for all of them. A recipe with low agreement gets `source='inferred'` and `confidence=2` — the same as a single direct teach — so it's not given outsized weight. A user can `forget-recipe` to drop bad ones.
- **Recipe substitution returns wrong resources.** Substituting `england → GB → KXMENWORLDCUP-26-GB` might hit a ticker that exists but is for a different event. Mitigation: substitution always runs verification (one API call OR one `resources` lookup) before returning. Verification failures return as `meta.recipe_unverified` so the agent knows to discover normally.
- **Entity-lookup tables become stale.** ISO codes are stable; sports rosters change (teams move, expand, contract). Mitigation: the seeded data carries a `source='seeded'` tag and a creation date. Users can override via `teach-lookup`. A future plan can add a `lookups refresh` command that re-pulls canonical data from a known source.
- **Recipe extraction performance regresses with corpus growth.** Walking the recent-teach window after every teach is cheap (O(window^2)), but a power user with thousands of teaches could see degradation. Mitigation: the window is fixed at the most recent 20 teaches; older teaches participate only when explicitly bulk-re-extracted via `recipes rebuild` (deferred CLI command).

---

## Test & Verification Strategy

Each implementation unit ships with its own test file (paths listed per unit). Three cross-cutting verifications gate PR readiness:

- **Three-trace regression.** Add `internal/learn/replay_test.go` that replays the three failure traces from the Problem Frame against a seeded fixture:
  1. "odds USA wins world cup" after preseed runs → returns USA tickers at confidence=2.
  2. "odds england wins the world cup" against a Portugal-tagged learning → returns `found=false` (mismatch filtered out).
  3. "odds USA wins world cup" with a parent-event learning AND a child resource available → recall warns about parent-vs-child preference.
- **Recipe-generalization regression.** Add `internal/learn/recipes/generalization_test.go` that exercises the core "learn Portugal, get England free" story end-to-end:
  1. Drop the DB to v7 with seed data only (no learnings, no recipes).
  2. Teach Portugal's two tickers explicitly.
  3. Teach USA's two tickers explicitly.
  4. Assert that `search_recipes` now has at least one Kalshi recipe (substitute) and one Polymarket recipe (substitute-then-search-prefix).
  5. Call `recall "odds england wins world cup" --agent` (England never directly taught).
  6. Assert `found=true, source=recipe, entity_match=exact`, returns `KXMENWORLDCUP-26-GB` and the England Polymarket slug.
- **Template-portability lint.** `internal/learn/` imports only `database/sql`, `encoding/json`, `regexp`, `strings`, `time`, and the small generic `Store` interface. Add `tools/check-learn-imports/` that runs as part of CI and fails the build if `internal/learn/**/*.go` imports any prediction-goat-specific package.
- **Documentation lint.** Add `tools/check-learn-docs/` that scans `internal/learn/**/*.go` and fails the build if any file lacks a top-of-file doc comment of at least 5 non-blank lines, or if any exported function lacks a doc comment.

## Open Questions / Deferred

- **Pre-seed scanner extensibility:** should the registry support priority ordering (e.g., user-taught > preseed-Kalshi > preseed-Polymarket)? Current design uses confidence as the tiebreaker, which seems sufficient. Deferred unless a real conflict surfaces.
- **Cross-entity query handling:** queries like "odds USA OR Mexico wins" carry multiple entities — current design returns the union of results, which may surface confusing partial matches. Deferred — the LLM can split into two queries.
- **Query-time learning weight by recency:** should a learning from 6 months ago rank below a learning from this week, all else equal? Deferred — `last_observed_at` is stored but not yet used in ranking.
- **Inline price cache:** a 60-second TTL on `kalshi markets get` / `markets get-by-slug` responses would make warm queries feel instant. Worth exploring in a follow-up; out of scope here because it interacts with the freshness contract from U5 of the prior plan.
- **Auto-prune of stale pre-seed entries when target resources are archived.** Deferred.

---

## PR Strategy

Branch: `feat/prediction-goat-smart-learning` off `feat/prediction-goat` (PR #780).

Single stacked PR covering all 10 units. Total surface ~20 new files in `internal/learn/` (across entities, lookups, recipes), ~5 modified files in `internal/cli/`, ~2 new files in `internal/source/{kalshi,polymarket}/`, plus the SKILL.md rewrite. Each unit is its own commit for review-ability.

The PR description will lead with the three failure traces AND the "learn Portugal → free England" generalization demo — Greptile and human reviewers should be able to read those, scan the test cases, and convince themselves the fix lands without reading every file.
