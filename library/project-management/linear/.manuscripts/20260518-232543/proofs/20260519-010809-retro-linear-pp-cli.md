# Printing Press Retro: Linear v4.9.0 Reprint

## Session Stats
- API: Linear (GraphQL)
- Spec source: GraphQL SDL from `linear/linear/packages/sdk/src/schema.graphql` (reused from prior run)
- Scorecard: 81/100 → 88/100 after polish (Grade A)
- Verify pass rate: 98% → 100%
- Fix loops: 1 (shipcheck) + 1 (post-publish freshness fixes)
- Manual code edits: ~25 (mostly v3-port adaptations + 4 post-publish freshness fixes)
- Features built from scratch: 1 new (`milestones at-risk`); 12 ported from v3

## Findings

### F1: Mutation commands silently fail to write the response back to the local store (Missing scaffolding)

- **What happened:** When a mutation command (e.g., `issues create`) succeeds against the API, the framework invalidates the HTTP response cache via `client.do`'s post-success `invalidateCache()`. But the local SQLite store — which is what hand-written or framework-emitted "list" commands actually read from in offline-first mode — is NOT updated. An agent's next `issues list` call doesn't see what was just created. The result: agents who write code like "create issue, then verify by listing" hit a confusing empty-result loop until they realize they need to re-sync. Manual write-back required ~50 LoC per mutation (Upsert call with response → typed payload mapping for the relevant resource table columns).
- **Scorer correct?** N/A — the scorer doesn't penalize this.
- **Root cause:** `internal/generator/templates/` has no template that wraps mutation success with a write-through-to-store step. Mutations are emitted as pure API call + response parse + render. The pattern "POST/PATCH/DELETE succeeded → upsert the returned object into the local store table" is not in any generator template.
- **Cross-API check:** Affects every CLI that emits both a sync-into-local-store path AND mutation commands for the same entity. From the catalog with strong evidence: **Linear** (issues create / update / archive); **Cal.com** (bookings create / cancel / reschedule against the same `bookings` table); **HubSpot** (contacts create / update / delete against the same `contacts` synced table). All three have user-visible "create-then-list returns nothing" footguns today.
- **Frequency:** every API with sync + mutation. ~half the catalog.
- **Fallback if the Printing Press doesn't fix it:** Each future CLI needs ~50 LoC of write-back per mutation in hand-written code. Forgettable; agents will skip it on novel features and ship the footgun. Fallback reliability: **low**.
- **Worth a Printing Press fix?** Yes. The template can be generic: after a successful mutation, if the response includes a single entity and the spec declares a sync table for that resource type, upsert. Cost is one call to `db.UpsertX(id, json)` post-mutation.
- **Inherent or fixable:** fixable.
- **Durable fix:** Generator templates for mutation commands gain a post-success block: `if db != nil && responseShape == "single-entity" { db.UpsertX(id, response) }`. The spec's typed-upsert pattern already exists (`UpsertAttachments`, `UpsertAuditEntryTypes`, etc.); the generator just needs to wire it into the mutation handlers in addition to sync.
- **Test:** Positive: regenerate Cal.com, run `bookings create … --dry-run=false`, then `bookings list --data-source local` — the new booking is in the list without re-sync. Negative: an API where the mutation response doesn't include the full entity (e.g., a `204 No Content` delete) doesn't try to upsert and doesn't fail.
- **Evidence:** This run, `issues create ESP-1823` succeeded but `issues list --team ESP` returned 0 hits for the new ticket until a `db.UpsertIssue` was added. Took 50 LoC + a GraphQL mutation-query rewrite to fetch enough fields for proper indexing.
- **Related prior retros:** None found in dedup scan.

### F2: Local-store-backed read commands return null/empty silently on cold-start AND stale data (Missing scaffolding)

- **What happened:** Commands like `today`, `bottleneck`, `similar`, `blocking`, `issues list` query the local SQLite store. When the store hasn't been synced (cold start), they return `null` or `[]` with no signal that the user should run `sync` first. When the store has data but the last sync is hours old, they still return whatever happens to be in the store with no signal that it might be stale. Agents can't tell "no data because nothing exists" from "no data because the store is empty"; humans can't tell "current" from "5h old." Manual implementation: a `sync_hint.go` helper (~60 LoC) plus injection sites in 5 commands (~30 LoC) plus a `--max-age` persistent flag.
- **Scorer correct?** N/A.
- **Root cause:** Generator templates for transcendence/novel commands (and even the framework's auto-emitted local-store readers like `pm_stale.go`, `pm_orphans.go`, `pm_load.go`) don't include a "did we hit empty? hint" check or a "is this data older than threshold? hint" check. The information is already in the store — `GetSyncState(resourceType)` returns `last_synced_at` — but nothing reads it for user-facing nudges.
- **Cross-API check:** Affects every CLI with sync + local-store-backed reads. From the catalog with strong evidence: **GitHub** (repos sync, then `gh-pp-cli stale` reads stored issues), **Notion** (pages sync, then queries), **Cal.com** (bookings/events sync, then `bookings list` from store), **Linear** (every transcendence command). The pattern is universal for transcendence rows that exist *because* the local store enables joins.
- **Frequency:** every API with the transcendence pattern. ~70% of catalog entries.
- **Fallback if the Printing Press doesn't fix it:** Each CLI's Phase 3 needs to add a `hintIfUnsynced` + `hintIfStale` helper and remember to wire it into each transcendence command. Easy to forget; current CLIs in the library mostly don't have it. Fallback reliability: **low to moderate** depending on whether the agent thinks to add it.
- **Worth a Printing Press fix?** Yes. Emit a `internal/cli/sync_hint.go` containing `hintIfUnsynced(cmd, db, resourceType)` and `hintIfStale(cmd, db, resourceType, maxAge)` helpers. Add a persistent `--max-age` flag (default 30m, configurable). Auto-inject the calls into `pm_stale.go.tmpl`, `pm_orphans.go.tmpl`, `pm_load.go.tmpl` at empty/non-empty branch points. Document the pattern in `references/novel-features-subagent.md` so hand-written commands include it.
- **Inherent or fixable:** fixable.
- **Durable fix:** New file `internal/generator/templates/sync_hint.go.tmpl` (~60 LoC), `--max-age` persistent flag in `root.go.tmpl`, and template wiring in `pm_*.go.tmpl`. Skill instruction update so hand-written novel commands call the helpers.
- **Test:** Positive: regenerate any catalog CLI, run a transcendence command before sync — stderr emits the cold-start hint. Run again with backdated `last_synced_at` — stderr emits the stale-read hint. Negative: pass `--max-age 0` — no hint. Pass a fresh store — no hint.
- **Evidence:** In this run, before adding the helper, `today`, `issues list`, `bottleneck`, `similar`, `milestones at-risk` all returned `null` or `[]` against an empty store. The user explicitly raised this as a UX gap ("how does an agent know when to sync?"). The fix took ~90 LoC across 6 files.
- **Related prior retros:** None found in dedup scan.

### F3: Hand-written commands silently bypass `--data-source`; spec should declare per-command strategy (Skill instruction gap + template gap)

- **What happened:** The framework's `resolveRead` in `data_source.go` correctly implements live-first with local fallback for spec-emitted commands (all the `promoted_*.go` files). But hand-written transcendence and novel commands (`today`, `issues list`, `bottleneck`, `similar`, etc.) bypass `resolveRead` entirely and read directly from the local store. The persistent `--data-source` flag is therefore a partial lie: it works on some commands and is silently ignored on others. A user typing `linear-pp-cli issues list --data-source live` expects current data; today the flag is just dropped. Manual fix took ~80 LoC to refactor `issues list` to honor the flag and write through.
- **Scorer correct?** N/A.
- **Root cause:** The Printing Press skill (specifically `novel-features-subagent.md`) doesn't make the data-source decision explicit per-command. Hand-written commands inherit the agent's default of "query the store directly" without anyone asking "would this command have a sensible live equivalent?" There's no spec annotation that says "this resource has a live-equivalent filtered query" vs "this resource is snapshot-computational only" — the only signal is whether the command happens to use `resolveRead`.
- **Cross-API check:** Affects every CLI with hand-written novel/transcendence commands AND a live-equivalent API surface for some of those reads. From the catalog with strong evidence: **Linear** (`issues list` could go live via `issues(filter:...)` GraphQL; `today`/`bottleneck` cannot); **GitHub** (`issues list` could go live; `stale-issues` aggregation cannot); **Cal.com** (`bookings list` could go live; `availability summary` cannot). Every CLI today has the asymmetry, but no spec annotation captures it.
- **Frequency:** every CLI with the transcendence pattern + filtered-list commands. ~60% of catalog.
- **Fallback if the Printing Press doesn't fix it:** Each CLI's agent has to reason about each command individually and remember to honor `--data-source`. Today every shipped CLI in the library silently ignores the flag on hand-written commands. Fallback reliability: **low** (agents consistently forget; the data-source skip is invisible at code-review time).
- **Worth a Printing Press fix?** Yes. Concrete fix: add a `data_source_strategy` annotation per resource/endpoint in the spec, with values `auto` (live-first via resolveRead, default), `local` (snapshot-computational, no live equivalent), `live` (mutations or commands that MUST hit API). The generator emits `resolveRead` for `auto`, store-only code for `local`, direct-API code for `live`. Hand-written novel commands carry the same in a source comment `// pp:data-source local`. Dogfood enforces presence of the annotation for hand-authored novel commands. SKILL.md surfaces the choice in the recipes section.
- **Inherent or fixable:** fixable, but requires coordinated changes across spec, generator, dogfood, and skill.
- **Durable fix:** Three parts:
  1. Spec extension: `data_source_strategy: auto|local|live` per resource (or per endpoint).
  2. Generator: read the annotation, emit the appropriate code shape via `resolveRead`, store-only, or live-only paths.
  3. Skill: novel-features-subagent prompt asks the agent to declare `// pp:data-source <strategy>` per hand-written command; dogfood checks for the annotation.
- **Test:** Positive: regenerate Linear, run `issues list --data-source live` — actually hits the API. Run with `--data-source local` — hits store only. Negative: a no-live-equivalent command like `today` errors with "this command is local-only; --data-source has no effect here" rather than silently ignoring.
- **Evidence:** This run's refactor of `issues list` to honor `--data-source` (showing live vs local return DIFFERENT data — ESP-1817/1818/1820 live vs ESP-1793/1820/1823 local) demonstrates the bypass; the retro doc at `2026-05-19-retro-data-source-reasoning.md` captures the longer design discussion. Five prior retros (cal-com 2026-04-27, allrecipes 2026-04-27, company-goat 2026-04-27, open-meteo 2026-05-02, postman-explore 2026-04-30) flagged related concerns about data-source layering, but none addressed the specific gap of hand-written commands bypassing the flag.
- **Related prior retros:**
  - `cal-com` (2026-04-27) — `extends`. Prior finding was about per-endpoint header injection on `resolveRead`; this finding extends to "and hand-written commands that bypass `resolveRead` lose the data-source flag entirely."
  - `postman-explore`, `allrecipes`, `company-goat`, `open-meteo` — `extends`. Adjacent data-source layering concerns from other generations; this finding is the first to name the bypass-via-hand-written-commands root cause.

### F4: GraphQL parser dropped 17 promoted resources between v3.10.0 and v4.9.0 (Bug / regression)

- **What happened:** Linear's v3.10.0 reprint emitted 41 `promoted_*.go` files from the same GraphQL SDL. The v4.9.0 reprint of the same spec emits only 26. Missing: `cycles`, `documents`, `custom-views`, `workflow-states`, `issue-labels`, `issue-relations`, `team-memberships`, `organization-invites`, `organization-metas`, `integration-templates`, `integrations-settingses`, `integrations`, `issue-to-releases`, `entity-external-links`, `release-pipelines`, `customers`, `customer-statuses`, `customer-tiers`. These are real Linear entities with `<name>(id)` + `<names>(filter)` Connection pairs in the spec. Their absence forced Phase 3 to either hand-write replacements or do without. This run's `cycles compare` would have benefited from `promoted_cycles get <id>` as a base; instead it has no such command.
- **Scorer correct?** N/A — the scorer doesn't flag the regression; it just counts what's emitted.
- **Root cause:** Somewhere between v3.10.0 and v4.9.0, the GraphQL parser (`internal/graphql/parser.go` or `internal/openapi/parser.go` GraphQL path) tightened a filter that now rejects entities the prior parser accepted. Likely candidates: stricter Connection-pattern recognition, a naming heuristic that drops compound-name resources (e.g., `customer-statuses` plural-of-already-plural), or a pluralization rule that now skips when the singular form doesn't match the input identifier exactly.
- **Cross-API check:** Affects every GraphQL spec. **Linear** (this run, 17 dropped); **Shopify Admin GraphQL** (catalog entry exists; would emit fewer promoted resources than the v3-era port did); any future Hasura/PostgREST-style CLI where every Postgres table generates a Connection pair. Evidence for Linear is direct (compared v3 vs v4 file lists); evidence for Shopify and Hasura is by structural similarity.
- **Frequency:** every GraphQL CLI generated since the regression landed.
- **Fallback if the Printing Press doesn't fix it:** Each future GraphQL generation drops resources silently and the agent has to hand-write replacements. Fallback reliability: **low to moderate** — agents may not notice the missing emissions if the absorb manifest doesn't enumerate them, and the dropped resources are still queryable via `api <interface>` so they look like "advanced surface" rather than missing emits.
- **Worth a Printing Press fix?** Yes — this is a regression vs known-working v3 behavior. The fix is "find what the parser rejects that v3 accepted." Diff the parser output for the same Linear SDL across the v3 and v4 binaries.
- **Inherent or fixable:** fixable.
- **Durable fix:** Add a parser-output regression test in `internal/graphql/parser_test.go` (or wherever the GraphQL connection detection lives) that asserts on a frozen Linear SDL fixture: "this set of resource names must be detected." Pin the expected list to the v3 emit count (41 for Linear's current SDL). Run as part of `go test ./...`. Investigation work is short — diff parser output between v3.10.0 and v4.9.0 against the same SDL.
- **Test:** Positive: regenerate Linear, confirm 41 promoted_* files emitted. Negative: a GraphQL spec WITHOUT Connection-pattern Get+List pairs (e.g., a pure query API) doesn't get spurious promoted commands.
- **Evidence:** v3 published linear has 41 promoted_* files; v4 generated has 26. Listed by name in the shipcheck retro candidate at `2026-05-18-232543-fix-linear-pp-cli-shipcheck.md`.
- **Related prior retros:**
  - `linear` 2026-04-08 — `aligned`. Prior finding #1 noted GraphQL parser issues (types.go duplicate fields). This is a different parser bug (resource detection vs type emit) but related — both point at the GraphQL parser as an under-tested surface.
  - `linear` 2026-05-07 — `aligned`. Prior re-raise mentioned the GraphQL sync template gap but did not catch the resource-emit regression because v3 was the baseline. This finding is the first time anyone has compared v3 vs v4 outputs and counted.

## Prioritized Improvements

### P1 — High priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Mutation commands auto write-back response to local store | generator | every API with sync + mutations (~50% of catalog) | low (agents skip on novel features) | medium (template per resource type) | Skip when mutation response shape is `204 No Content` or omits the full entity |
| F2 | Cold-start + stale-read hints for local-store-backed reads | generator | every API with the transcendence pattern (~70% of catalog) | low to moderate | small (one new helper template + injection in pm_*.go.tmpl) | None — pure additive, --max-age 0 disables |

### P2 — Medium priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F3 | `data_source_strategy` annotation + skill instruction for hand-written commands | spec-parser + generator + skill | every API with transcendence + filtered-list commands (~60% of catalog) | low (agents consistently forget) | medium (coordinated change across three surfaces) | None — annotation defaults to current behavior when absent |
| F4 | GraphQL parser regression: 17 promoted resources dropped vs v3 | openapi-parser | every GraphQL spec | low to moderate | small (diff parser output between v3/v4 against frozen Linear SDL fixture, fix the tightened filter) | None — regression test pins expected count |

### Skip

| Finding | Title | Why it didn't make it |
|---------|-------|-----------------------|
| Skip-1 | v4 emits REST-only `client.go` for GraphQL specs (no Query/Mutate helpers) | Step D: raised 3+ times (Linear 2026-04-08 #3, Linear 2026-05-07 #5, this run). Already filed; not implemented across two prior retros. Recurrence-cost says don't re-raise at same priority. The 2026-05-07 retro already reframed it to an incremental "emit TODO scaffold" fix; that's the open work item and doesn't need a third filing. |
| Skip-2 | `--data-source` flag bypass on `pp_load`, `pp_orphans`, `pm_stale` (framework-emitted local-only commands) | Step G: case-against is strong. These commands genuinely have no live equivalent — the whole point is local SQL aggregation. The flag *should* be a no-op here. The actionable variant of this is F3 (hand-written novel commands), which IS new and IS filed. |
| Skip-3 | `helpers.go` API divergence between v3 and v4 (`classifyAPIError(err)` → `classifyAPIError(err, flags)`) | Step B: this only matters when an agent is porting v3 code to v4. Fresh v4 prints don't hit it. Can't name 3 future APIs that would be affected — porting is by definition a one-time concern per CLI. |
| Skip-4 | `verify-skill` positional-args parser misclassifies subcommand examples | Step G: known issue with low impact. Findings show "likely false positive" and don't fail the verdict. Fixing the parser to recognize subcommand paths inside backticked examples is correct but the impact ceiling is "stop emitting false-positive findings"; doesn't unblock anything. |

### Dropped at triage

| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| C1 | v3 `graphql.go` types `userPresentableMessage` as bool but Linear returns string | printed-CLI: bug is in v3's hand-written `internal/client/graphql.go` ported into v4; future v4 fresh prints wouldn't inherit it once v4 has its own GraphQL client (Skip-1's domain) |
| C2 | v3 `store.go` `SetMaxOpenConns(1)` deadlocks v4 `pm_stale.go` concurrent queries | printed-CLI: v3 store template carried over; fresh v4 prints don't have this. Same domain as C1 (port artifact). |
| C3 | 6h staleness default in `doctor.go` too long for active workspaces | covered-by F2 (the `--max-age` flag added by F2's helper makes this user-configurable; default change to 30m is part of F2's scope) |
| C4 | GraphQL SDL emitted as `spec.yaml` (filename misnomer) breaks shipcheck legs that auto-parse as OpenAPI YAML | iteration-noise: renaming to `.graphql` is a 1-line generator change; not generalizable beyond GraphQL specs which are already covered by Skip-1's domain |
| C5 | v3 `auth.go` lacks `set-api-key` for non-Bearer personal keys | printed-CLI: v3 patch applied; v4 generator already emits correct `auth_type: api_key` without Bearer prefix. No machine work needed. |
| C6 | MCP enrichment path for GraphQL specs (no `x-mcp:` equivalent) | Step B: only 1 prior retro hit (postman-explore 2026-04-30); cannot name 3 future GraphQL CLIs with concrete spec evidence. Would benefit from another GraphQL generation surfacing the same gap before re-raising. |

## Work Units

### WU-1: Mutation write-back template (from F1)

- **Priority:** P1
- **Component:** generator
- **Goal:** Generated mutation commands automatically write the response entity into the local SQLite store so subsequent local-only reads see the change without re-syncing.
- **Target:** `internal/generator/templates/` — mutation command templates (likely `command_endpoint.go.tmpl` or a new `command_mutation.go.tmpl`) plus the typed `db.UpsertX` calls already emitted from the spec's resource definitions.
- **Acceptance criteria:**
  - positive: regenerate a CLI with both `sync` and a mutation command (e.g., Cal.com `bookings create`). After `<cli> bookings create`, an immediate `<cli> bookings list --data-source local` includes the new booking.
  - positive: the mutation command's response JSON is upserted to the matching `<resource>` table; the row's `synced_at` timestamp updates.
  - negative: a mutation with no response body (HTTP 204, or `success: true` envelope without the full entity) doesn't try to upsert and doesn't fail.
- **Scope boundary:** Does NOT include cache invalidation (already handled by `client.do`). Does NOT include schema migration for fields the mutation response includes that the sync table doesn't.
- **Dependencies:** None.
- **Complexity:** medium.

### WU-2: Sync-hint helpers + `--max-age` flag (from F2)

- **Priority:** P1
- **Component:** generator
- **Goal:** Every generated CLI ships a `sync_hint.go` with `hintIfUnsynced` and `hintIfStale` helpers, a persistent `--max-age` flag (default 30m), and template wiring that calls the helpers from local-store-backed reads.
- **Target:** New file `internal/generator/templates/sync_hint.go.tmpl`. Modifications to `root.go.tmpl` (flag registration), `pm_stale.go.tmpl`, `pm_orphans.go.tmpl`, `pm_load.go.tmpl` (call the helpers at empty/non-empty branch points). Update `references/novel-features-subagent.md` so hand-written transcendence commands include the helper calls.
- **Acceptance criteria:**
  - positive: regenerate any catalog CLI, delete its local store, run any local-backed command — stderr emits the cold-start hint; `--json` output is unaffected.
  - positive: backdate `sync_state.last_synced_at` for a resource by more than 30m, run a local-backed command — stderr emits the stale-read hint with the actual age.
  - positive: pass `--max-age 0` — no hint regardless of age.
  - positive: pass `--max-age 6h` and have data 4h old — no hint.
  - negative: pass `--max-age 30m` with fresh data (synced 5 min ago) — no hint.
- **Scope boundary:** Does NOT add auto-refresh behavior (the hint stays advisory). Does NOT change the default staleness threshold in `doctor.go` (that's a `--stale-after` knob, separate from `--max-age` for read-time hints).
- **Dependencies:** None.
- **Complexity:** small.

### WU-3: Spec `data_source_strategy` annotation + skill enforcement (from F3)

- **Priority:** P2
- **Component:** spec-parser
- **Goal:** The Printing Press emits read commands whose data-source behavior matches a declared annotation, and the skill makes the agent reason about which strategy each hand-written command uses.
- **Target:**
  - `internal/spec/spec.go` — add `DataSourceStrategy` field on resource/endpoint (`auto|local|live`, default `auto`).
  - `internal/openapi/parser.go` — support reading `x-data-source-strategy` from OpenAPI specs.
  - `internal/generator/templates/command_endpoint.go.tmpl` — branch on the annotation, emit `resolveRead` for `auto`, store-only code for `local`, direct-API code for `live`.
  - `skills/printing-press/references/novel-features-subagent.md` — require hand-written novel commands to declare `// pp:data-source <strategy>` in source.
  - `printing-press dogfood` — flag hand-written novel commands missing the annotation.
- **Acceptance criteria:**
  - positive: a spec annotates `issues.list` with `data_source_strategy: auto` — the generated `issues list` honors `--data-source live|local|auto` correctly.
  - positive: a spec annotates a snapshot-computational resource with `local` — the command rejects `--data-source live` with a clear "no live equivalent for this command" error.
  - positive: dogfood reports a finding when a hand-written novel command lacks the annotation.
  - negative: specs without the annotation default to current behavior (no regression for existing CLIs).
- **Scope boundary:** Does NOT change which specific commands are spec-emitted vs hand-written. Does NOT remove the existing `--data-source` flag.
- **Dependencies:** None (additive).
- **Complexity:** medium.

### WU-4: GraphQL parser regression — restore the 17 dropped resources (from F4)

- **Priority:** P2
- **Component:** openapi-parser
- **Goal:** Regenerate Linear from the same GraphQL SDL and emit the same 41 `promoted_*` files that v3.10.0 emitted (or document and accept the new shape if the regression was intentional).
- **Target:** `internal/openapi/parser.go` GraphQL path (or `internal/graphql/parser.go` if separated) — identify the filter that now rejects entities v3 accepted. Likely candidates: stricter Connection-pattern recognition, pluralization heuristic on compound-name resources, naming filter.
- **Acceptance criteria:**
  - positive: regenerate Linear from the same SDL fixture as v3.10.0 (or any frozen test SDL with known emit count) and confirm 41 promoted_* files emitted.
  - positive: regression test in `internal/openapi/parser_test.go` asserts the expected set of resource names for the Linear SDL fixture.
  - negative: a non-Connection-pattern spec (pure-query GraphQL) doesn't acquire spurious promoted commands.
  - negative: a spec where v3 emitted a duplicate or near-duplicate resource doesn't re-emit those duplicates (i.e., only restore legitimately-dropped emits).
- **Scope boundary:** Does NOT change the spec format. Does NOT add new emit capabilities beyond what v3 supported.
- **Dependencies:** None.
- **Complexity:** small (investigation), unknown until the diff is run (fix sizing).

## Anti-patterns
- **Live-first vs local-first decision left to per-command author judgment.** Each hand-written novel command in this run had to be reasoned about individually; the resulting CLI has 4 different data-source patterns across its read commands (spec-emitted-auto, store-only-via-Upsert, store-only-direct-query, live-via-helper). Inconsistent agent-visible behavior with no spec-level coordination.
- **Silent regressions between Printing Press versions.** v3 → v4 dropped 17 GraphQL resource emits without any test or release-note signal. The user wouldn't have noticed except for this reprint comparing v3 (still in public library) against v4 (new output). Need parser-output regression tests against frozen spec fixtures so future regressions are caught at PR time.
- **"Fix it in Phase 3" treated as a substitute for machine-level scaffolding.** Multiple findings here describe patterns where "the agent can hand-write it" is the current fallback. That's true for one CLI; it doesn't compound. The retro's job is to surface where hand-written patterns have stabilized enough that the machine should absorb them.

## What the Printing Press Got Right
- **`resolveRead` framework pattern** — for spec-emitted commands, `--data-source auto|live|local` with network-error fallback and write-through-to-cache is exactly right. The pattern survives unchanged across this run and previous CLIs. The gap is only that hand-written commands don't use it.
- **Cobratree runtime MCP walker** — registering every Cobra command as an MCP tool with the correct `mcp:read-only` / `mcp:hidden` annotations made the MCP surface honest without per-command spec changes. The Linear MCP came up with 28 typed tools + ~26 walker tools without manual MCP-side wiring.
- **The publish-skill safety floor** — refused to publish without `printer_name` populated, without `phase5-acceptance.json`, and with PII in proofs. These would have shipped uncaught in the v3 reprint workflow.
- **`pp_created` ledger + `--trust-mode strict`** — agent-native plumbing that survived from v3, mutated a real workspace safely (ESP-1822/1823 created and cleaned up via `pp-cleanup`), and never touched a single pre-existing ticket. The contract is robust.
- **`hintIfUnsynced` + `--max-age` + `hintIfStale` interlock** — when implemented per-CLI in Phase 5, the three pieces compose cleanly: cold-start is one hint, stale is another, the threshold is one persistent flag, JSON pipes stay clean. The pattern is small enough to be obvious in retrospect; F2 just asks the machine to do it once.
