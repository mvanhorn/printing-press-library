---
title: "feat(cli): contact-goat friction-point fixes from SF-friends task"
type: feat
status: active
date: 2026-05-04
deepened: 2026-05-04
---

# feat(cli): contact-goat friction-point fixes from SF-friends task

## Summary

Targeted fixes to contact-goat-pp-cli surfaced by a real "list my 1st-degree connections in SF and their company" task. Headline: `api hpn search`'s emit path passes an empty `currentUUID` into bridge normalization, which forces every bridge to `kind == "friend"` and erases the self_graph (1st-degree) signal that `coverage` uses correctly — that's the root reason an agent can't tell 1st-degree from 2nd-degree on the bearer surface. Plan plumbs the user UUID through, adds matching `--first-degree-only` and `--min-score` filters, fast-fails cookie broad-query timeouts, adds `coverage --location`, single-command pagination with hard credit safety, doctor stale-graph warning, native CSV for `hp people`, score docs. The terminology rename and the `sync linkedin-graph` subcommand are deferred — the rename amplifies confusion under "no deprecation" policy, and HP's upload-trigger endpoint isn't documented to exist.

---

## Problem Frame

The CLI fans out across LinkedIn, Happenstance cookie + bearer, and Deepline, but when a user asked "list my 1st-degree friends in SF and their company" the assistant had to fight the tool: cookie timed out silently after 4 minutes on a broad query, `friends` returned 14 top connectors with no location, `graph export json` had no location field, the bearer search mixed score-2 weak-signal entries from the public graph alongside real 1st-degree matches, pagination was a two-step `find-more` then `get --page-id` poll loop, and the global `--csv` flag failed on the nested `bridges[]` array. Net effect: a task that should have been one command became ten manual retries plus hand-filtering. The CLI is internally well-structured (cobra commands, shared poll helper, fixture-backed tests); these are surface-level UX gaps, not architectural ones.

---

## Requirements

- R1. `api hpn search` can be scoped to "only my 1st-degree connections" without weak-signal public-graph leak-through. The bearer emit path must preserve the self_graph bridge signal so downstream consumers can tell 1st-degree from 2nd-degree (today the path explicitly drops it).
- R2. `api hpn search` supports a `--min-score` filter that drops results below a documented threshold.
- R3. A single command paginates and polls `api hpn search` to completion or a `--max-results N` cap, without manual `find-more` + `get --page-id` choreography.
- R4. `coverage` answers the symmetric "who do I know in city X" question.
- R5. `doctor` warns visibly when the synced LinkedIn graph is older than a configurable threshold; a dedicated subcommand triggers a re-sync.
- R6. When cookie-surface search hits its poll timeout or a known-broad-query failure, the CLI exits fast with a clear suggestion to retry with `--source api`.
- R7. The three meanings of "friends" (top-connectors command, 2nd-degree tier flag, colloquial 1st-degree) are disambiguated in flag names, command names, and help text without breaking existing scripts.
- R8. `hp people` produces native CSV (`--csv`) that flattens the `bridges[]` array into stable columns.
- R9. The score field is documented: range, what 50 vs 9000+ mean, recommended `--min-score` thresholds.

---

## Scope Boundaries

- Not changing the underlying Happenstance API contracts. The cookie and bearer endpoints behave as they do today; this plan adds CLI-side filtering, polling, and routing.
- Not adding a local-data location index. Filtering by location continues to go through bearer search; making the synced LinkedIn graph location-aware is its own initiative.
- Not refactoring `printCSV` for all flagship commands. Only `hp people` gets first-class CSV here; `coverage`, `prospect`, `warm-intro`, `dossier` stay table+JSON only and are deferred.
- Not deprecating or renaming existing flags in this plan. Both the deprecation precedent and any rename of `--connections` / `--friends` / `friends` are deferred (see U6 and Deferred to Follow-Up Work).
- Not adding a CLI re-sync subcommand. `sync linkedin-graph` is deferred until HP's upload-trigger endpoint is documented; doctor's stale-graph warning points at the web UI in the interim.
- Not changing the bearer credit-cost model or the cookie-vs-bearer auto-routing default. Fast-fail in R6 only fires after the existing routing already chose cookie.

### Deferred to Follow-Up Work

- Terminology rename for `hp people --connections` / `--friends` / `friends` command (originally R7 / U6): separate plan that decides between (a) introducing `MarkDeprecated` as a precedent and renaming, or (b) help-text-only refresh on the existing flags. Either path is a deliberate plan-time decision and doesn't fit this plan's scope.
- `sync linkedin-graph` CLI subcommand (originally part of R5 / U4): separate plan that fires after HP ships (or we sniff) an upload-trigger endpoint distinct from `GET /api/uploads/status/user`. Until then the user re-uploads via happenstance.ai/connections.
- Native CSV for the other flagship commands (`coverage`, `prospect`, `warm-intro`, `dossier`): same `bridges[]`-style nesting issue, same fix shape, follow-up PR.
- Generic `printCSV` upgrade in `helpers.go:481` to JSON-encode nested arrays: would let every command's `--csv` work without per-command emitters, but touches multiple flagship structs and is its own refactor.
- Local-graph location index: would let `friends`, `coverage`, and `intersect` filter by city without spending bearer credits. Material refactor; separate plan.
- `coverage --location` results caching: this plan keeps the existing 5-minute GET cache; a longer-lived "people I know in city X" cache is deferred.
- Upstream the `Aliases` field generator support to `cli-printing-press` so future hand-edits to `promoted_*.go` survive regen — only relevant when the U6 follow-up actually picks a rename strategy.

---

## Context & Research

### Relevant Code and Patterns

- `internal/cli/api_hpn_search.go:72` `newAPIHpnSearchCmd`, line 168 `newAPIHpnSearchFindMoreCmd`, line 232 `newAPIHpnSearchGetCmd`, line 265 `runHpnSearch`, line 358 `buildPollSearchOptions`, line 398 `classifyHpnError`. Primary surface for R1-R3, R6.
- `internal/cli/coverage.go:22` `newCoverageCmd`. Primary surface for R4. Today scopes by company; the location variant rides the same fan-out shape.
- `internal/cli/doctor.go:24` `newDoctorCmd`, line 271 `checkHappenstanceGraphCoverage` (uses `humanAge`). Primary surface for R5 warning. Re-sync subcommand lands in `internal/cli/sync.go:29` `newSyncCmd` next to existing `defaultSyncResources` (line 449).
- `internal/cli/hp_people.go:37` `newHPPeopleCmd` with `--connections` / `--friends` BoolVar pattern (lines 147-149) and `--no-*` alias merge in `PreRunE` (lines 156-168). Pattern for R7 aliases. `printHPPeopleTable` at line 252 is the today-bypass for CSV (R8).
- `internal/cli/promoted_friends.go:14` `newFriendsPromotedCmd`. Primary surface for the `top-connectors` rename in R7. Note: this file is the generated promoted-command entrypoint; rename strategy must consider regen.
- `internal/cli/helpers.go:481` `printCSV` flattens `[]map[string]any` via `fmt.Sprintf("%v", v)` (line 510). This is the line that produces `[{... ...}]` output for `bridges[]`. R8 fix lives here or as a per-command emitter.
- `internal/cli/flagship_helpers.go` defines `bridgeRef`, `bearerRationale`, `bearerScore`. These are the structs that need flatten rules for R8.
- `internal/cli/root.go:55-71` persistent flag declarations and `--agent` cascade in `PersistentPreRunE` (lines 73-90). R6 hint lives near this output layer.
- `internal/happenstance/api/search.go:167` `Client.PollSearch`, lines 20-23 terminal status constants, line 215 `isTerminalStatus`. Shared poll helper for R3.
- `internal/client/people_search.go:62` cookie-mirror poll-timeout (180s default). `TestDefaultPollTimeoutIs180s` at `people_search_test.go:15` regression-guards this. R6 surfaces the gap between this 180s and the http-client timeout in `client.New`.
- `internal/cli/api_hpn_test.go` table-driven tests for `checkSearchBudget` (line 400), `classifyHpnError` (line 423). Pattern for R1-R3, R6 unit tests. `newFakeBearerServer` (line 305) is the standard fixture pattern.
- `internal/happenstance/api/search_test.go` for the api-package poll-helper tests; relevant for R3.
- `docs/plans/2026-04-19-001-fix-bearer-api-mutuals-plan.md` is the local plan precedent: section ordering, file-reference style with line ranges, regen-as-final-unit, decision-table style for Key Technical Decisions.
- `.claude-plugin/plugin.json` and `cli-skills/pp-contact-goat/SKILL.md` are the regen targets that must stay in sync per AGENTS.md.

### Institutional Learnings

- AGENTS.md (repo root) requires `go run ./tools/generate-skills/main.go` after any change under `library/**/internal/cli/**`. The SKILL verifier (`.github/scripts/verify-skill/verify_skill.py`) checks every documented `--flag` resolves on the right cobra command. Any new flag in this plan must round-trip through SKILL.md before merge.
- AGENTS.md also notes: prefer surgical edits over broad regen, conventional-commit scopes (`feat(cli)` / `fix(cli)` / `docs(cli)`), bump `.claude-plugin/plugin.json` only if `skills/ppl/**` or `.claude-plugin/**` changed.
- `2026-04-19-001-fix-bearer-api-mutuals-plan.md` Unit 5 establishes the regen-and-bump as a dedicated terminal unit; this plan follows that pattern.
- No `MarkDeprecated` precedent anywhere in the repo. The local idiom for renames is "add new alias, keep old one working, update help text" rather than formal deprecation.

### External References

- Cobra alias / persistent-flag docs (`spf13/cobra`): used to confirm that `cmd.Flags().BoolVar` followed by a second `BoolVar` pointing at the same destination variable is the local-idiom alias pattern; preferred over `Aliases` for flag-level renames.
- No external references needed for the substantive changes; this is a UX surface refinement on a known codebase.

---

## Key Technical Decisions

- R1 implementation: two-part fix. (a) **Plumb `currentUUID` through `api hpn search`'s emit path.** Today `internal/cli/api_hpn_search.go:300` calls `api.ToClientPersonWithBridges(r, env.Mutuals, "")` with an explicit empty `currentUUID`, which forces the retag in `internal/happenstance/api/normalize.go:99-102` to skip and every bridge to be tagged `BridgeKindFriend`. The fix passes the authenticated user's UUID (already available via the user/clerk endpoint and cached locally) so self-bridges retag to `BridgeKindSelfGraph`. This is the actual root cause the bearer surface couldn't distinguish 1st-degree from 2nd-degree. (b) **Add `--first-degree-only` filter** that drops results with no `BridgeKindSelfGraph` bridge (after the retag is in place). Mirrors `coverage.go:296`'s existing `hasSelfGraphBridge` predicate. Naming: `--first-degree-only` to match the codebase's internal vocabulary (`BridgeKindSelfGraph`) rather than introducing a third meaning of "connection". Bearer payload `include_my_connections=true` is implied by the flag (auto-set when `--first-degree-only` is passed); existing `--include-my-connections` continues to work standalone for the "include 1st-degree alongside others" case.

- R2 default and threshold: `--min-score float` defaulting to 0 (no filter). Per `internal/cli/flagship_helpers.go:138-160` the score is `top.AffinityScore` directly (typical 10-100, observed up to ~300; weak-signal public-graph hits show as 0-2). Document `>= 5` as "drops weak-signal public-graph entries" in `--help` text. Do NOT recommend a "high-confidence 1st-degree" threshold — real 1st-degree bridges sit at 49.99 (medium affinity) or lower in captured fixtures, so any blanket high threshold would cut real matches. The flag is a noise filter, not a quality filter; pair it with `--first-degree-only` for the SF-task use case. Filter is post-fetch, applied after `--first-degree-only` if both set. No extra credits.

- R3 pagination shape: extend the existing `api hpn search <text>` command with `--all` (bool) and `--max-results int` (cap). When set, the run loop calls POST search, polls, then loops `find-more` + `get --page-id` + poll until `has_more=false`, the cap is hit, or `--budget` would be exceeded. The two existing subcommands (`find-more`, `get`) stay untouched; this is purely additive. Rationale: the manual flow is still useful for scripted resumption, but the common case ("get me up to N matching results") deserves one command.

- R3 budget interaction: `--all` MUST be paired with at least one of `--max-results` or `--budget`; passing `--all` alone with both unset returns a usage error suggesting an explicit cap. Rationale: an agent calling `--all --yes` against a broad query with default `--budget 0` could otherwise burn 100+ credits silently. Each `find-more` is 2 credits; the loop tracks running total and prints a per-page cost line on stderr matching the existing "cost spent" log convention. When the next `find-more` would exceed `--budget`, emit accumulated results plus a budget-exhausted notice rather than discarding.

- R4 surface: extend `coverage` with a mutually-exclusive `--location <string>` flag rather than a new top-level command. Rationale: the user mental model is "coverage of <thing>" where <thing> is a company today and a city tomorrow. Argument-free positional with `--location` flag avoids a positional-vs-flag conflict. Help text gets a "scope: company or location" preamble.

- R5 thresholds: `doctor` emits `WARN` (yellow text, status field `"stale"` in JSON) when `linkedin_ext.last_refreshed` is older than 90 days; `ERROR` (red, status `"very_stale"`) when older than 180 days. Hardcoded — no env override. Rationale: the SF task ran fine on a 9-month-old graph, so a 30-day threshold would have cried wolf on a working setup; 90/180 keeps the warning meaningful for a tool that's clearly degraded but doesn't fire on the routine "I haven't re-uploaded my LinkedIn export this quarter" case. A single env knob can't tune two thresholds independently anyway, so dropping the override removes a half-baked design surface.

- R5 re-sync subcommand: deferred. The sniff at `.manuscripts/happenstance-sniff-2026-04-19/` only documents `GET /api/uploads/status/user`; no POST upload-trigger has been observed and the captured response's `s3_uri: "s3://connections-uploads/linkedin_ext/...csv"` suggests user-uploaded CSVs through the web UI rather than a triggerable endpoint. Adding a CLI command for an endpoint that may not exist is a fake feature. Doctor still gets the stale-graph warning (R5 partial); the suggested action it prints is "re-upload your LinkedIn export at happenstance.ai/connections" until an actual trigger endpoint is discovered.

- R6 fast-fail trigger: classify cookie poll-timeout AND http-client timeout AND HTTP 524/502/503 upstream timeouts as `ErrCookieBroadQuery`. On hit, `runHpnSearch`, `runCoverage`, and `runHPPeople` print to stderr: `cookie surface timed out (likely a broad query). Retry with --source api to use the bearer surface (2 credits/call).` Exit code is `5` (API error / upstream issue) for all three flavors — a single consistent code, distinct from `7` (rate-limit, asserted by `api_hpn_test.go:433` for `RateLimitError`). Conflating cookie timeouts with rate-limit caused programmatic-consumer confusion in the prior decision matrix; using `5` keeps the rate-limit semantic clean. Rationale: 4-minute silent retry-loops are the worst observed UX; failing fast with the actionable next step is the cheapest fix.

- R7 terminology strategy: deferred. Adding `--first-degree` / `--second-degree` aliases without deprecating `--connections` / `--friends` produces 4 flag names for 2 concepts, plus `top-connectors` alongside `friends` for one command — the cumulative confusion exceeds the readability win. Two paths forward, both deferred to a separate plan: (a) introduce `MarkDeprecated` as a precedent and rename properly, or (b) drop the rename and just expand existing `--help` text on `friends` (the `Long` already says "your network's top connectors"). The actual disambiguation work this plan needs from the SF-task — making 1st-degree distinguishable from 2nd-degree on the bearer surface — is solved by R1, not by flag renames.

- R8 CSV flatten implementation: per-command emitter in `printHPPeopleTable`'s sibling, not a generic `printCSV` upgrade. Rationale: the nested-output problem touches multiple flagship structs (`bridgeRef`, etc.) and a generic flattener that handles all of them is the deferred follow-up. For `hp people`, define stable columns: `name, current_title, current_company, linkedin_url, score, relationship_tier, bridge_count, bridge_names, bridge_kinds, top_bridge_affinity, rationale`. `relationship_tier` derives from the strongest bridge's `kind` (`self_graph` -> `1st_degree`, `friend` -> `2nd_degree`, no bridge -> `3rd_degree`). `bridge_count` preserves cardinality; `bridge_names` and `bridge_kinds` are semicolon-joined to keep multi-bridge identity without going long-form. This avoids the lossy "collapse to single strongest bridge" silent-data-drop the earlier draft proposed.

- R9 docs location: new `docs/scoring.md` (not in CLI tree, not regenerated) plus a brief expansion of `--help` text on `api hpn search`. Doc covers what the CLI does with the score (`--min-score N` drops rows with score < N), defers the score-semantics-and-range to Happenstance's own docs with a stable link, and explains that 1st-degree vs 2nd-degree should be filtered with `--first-degree-only` rather than via score thresholds. Rationale: hard-coding HP's scoring conventions in the CLI repo means stale guidance the moment HP changes its model; pointing at HP's docs keeps the CLI surface honest. Captures the observation that 0-2 range is weak-signal (public graph), 10-100 is typical (per `flagship_helpers.go:138-160`), with anything higher indicating a bridge-amplified signal.

---

## Open Questions

### Resolved During Planning

- "Why couldn't the agent tell 1st-degree from 2nd-degree on the bearer surface?" Resolved: `internal/cli/api_hpn_search.go:300` calls `api.ToClientPersonWithBridges(r, env.Mutuals, "")` with empty `currentUUID`, which forces every bridge through `internal/happenstance/api/normalize.go:99-102` to `BridgeKindFriend`. The self_graph signal exists in the API response but is erased in the emit path. Fix is plumbing the user's UUID through; predicate becomes `hasSelfGraphBridge` matching `coverage.go:296`.

- "What's the right name for the 1st-degree filter flag?" Resolved: `--first-degree-only`. The codebase's internal vocabulary is `BridgeKindSelfGraph` and `BridgeKindFriend`; "connection" doesn't exist as a kind, and `hp people --connections` already overloads the word. `--first-degree-only` is unambiguous against both.

- "Should `--all` paginate `find-more` automatically or require explicit opt-in?" Resolved: explicit, plus a hard safety guard — `--all` requires at least one of `--max-results` or `--budget` to be non-zero. Implicit pagination or default-disabled budget would let an agent burn 100+ credits on a broad query.

- "Should `coverage --location` use cookie or bearer?" Resolved: bearer-only via `--source api`. The cookie surface has no city-search semantic and the LinkedIn arm of the existing fan-out (which is company-keyword based) doesn't generalize to locations. `coverage --location` skips the LinkedIn arm and routes to bearer.

- "How should `coverage` accept zero positional args for `--location` mode?" Resolved: change `Args: cobra.ExactArgs(1)` (`coverage.go:52`) to `Args: cobra.MaximumNArgs(1)` plus a `PreRunE` validator that requires exactly one of `<positional>` or `--location`. PreRunE alone can't accept zero positionals.

- "Should the cookie fast-fail attempt automatic bearer fallback?" Resolved: no, hint only. Auto-fallback would silently spend credits the user didn't authorize. The hint is one keystroke away from a retry.

- "What exit code should cookie-broad-query timeouts use?" Resolved: 5 (API error). Reusing 7 (rate-limit) confuses programmatic consumers since `api_hpn_test.go:433` already asserts `RateLimitError -> 7`. A consistent 5 across all three timeout flavors (poll-timeout, http-deadline, 5xx) keeps the rate-limit semantic clean.

- "Should `sync linkedin-graph` ship in this plan?" Resolved: no. Sniff at `.manuscripts/happenstance-sniff-2026-04-19/` only documents `GET /api/uploads/status/user`; the captured `s3://connections-uploads/linkedin_ext/...csv` URI suggests user-uploaded CSVs through the web UI. Doctor's "suggested action" text points to happenstance.ai/connections instead of a CLI command until an upload-trigger endpoint is discovered.

- "Should the terminology rename (R7) ship?" Resolved: no, deferred to a separate plan. Adding `--first-degree`/`--second-degree` as forever-aliases produces 4 names for 2 concepts; the right path is either a deprecation precedent or a help-text-only refresh, neither of which fits this plan's scope.

### Deferred to Implementation

- Exact wording of doctor's stale-graph warning line (text only; thresholds and color are decided here).
- Whether to expose `--max-results` as `--limit` instead. `--limit` is taken in `hp people` but not in `api hpn search`; consistency check at implementation time.
- Whether the `bridge_kind` CSV column should be `"connection"` / `"friend"` literally or normalized to `"1st-degree"` / `"2nd-degree"`. Consistency with score docs decides at implementation.
- Generated-skill verifier failure modes when a flag is added but SKILL.md isn't yet regenerated. The regen unit handles the success case; the local pre-merge dance is an implementation detail.

---

## High-Level Technical Design

> This illustrates the intended approach and is directional guidance for review, not implementation specification. The implementing agent should treat it as context, not code to reproduce.

Filter pipeline for `api hpn search` after R1+R2+R3:

```
POST /v1/search                       (initial call, costs 2 credits)
  payload includes:
    include_my_connections=true       (when --first-degree-only or --include-my-connections set)
    include_friends_connections=true  (when --include-friends-connections set)
    group_id=...                      (when --group-id passed)
  ->
poll status: PENDING -> RUNNING -> COMPLETED   (existing PollSearch, 180s default)
  ->
emit envelope -> ToClientPersonWithBridges(results, env.Mutuals, currentUUID)
  // KEY FIX: currentUUID is now the authenticated user's UUID, NOT "".
  // Self-bridges retag to BridgeKindSelfGraph; everything else stays BridgeKindFriend.
  ->
results[] (now correctly tagged)
  if --first-degree-only: drop where !hasSelfGraphBridge(p.Bridges)
  if --min-score N:        drop where score < N
  ->
  if --all and (--max-results unset OR running_count < --max-results)
            and (--budget unset OR running_credits + 2 <= --budget):
    POST /v1/search/{id}/find-more   (costs 2 more credits per page)
    poll page status to terminal
    re-apply ToClientPersonWithBridges with currentUUID, then filters
    accumulate results
    loop
  ->
emit (table | json | csv)
emit per-page cost lines on stderr (when --all)
```

Decision matrix for the cookie-vs-bearer fast-fail (R6):

| Trigger                                   | Today behavior                            | New behavior                                                |
|-------------------------------------------|-------------------------------------------|-------------------------------------------------------------|
| Cookie poll exceeds 180s                  | Returns timeout error to caller, raw      | Classify as `ErrCookieBroadQuery`, exit 5, hint `--source api` |
| http-client deadline exceeded (broad q)   | Silent 4-minute hang, then generic error  | Classify as `ErrCookieBroadQuery`, exit 5, hint `--source api` |
| 524 / 502 / 503 from cookie               | Generic upstream error                    | Classify as `ErrCookieBroadQuery`, exit 5, hint `--source api` |
| Cookie status stays `"Thinking"` > 90s    | Polled until 180s, then error             | Bail early as `ErrCookieBroadQuery`, exit 5, hint `--source api` |
| Bearer 402 (out of credits)               | Existing handling                         | Unchanged                                                   |
| Bearer 429 (rate limit)                   | Existing exit 7 via `RateLimitError`      | Unchanged — exit 7 reserved for rate-limit only             |

Doctor stale-graph state machine (R5):

| Age of `linkedin_ext.last_refreshed`    | doctor status | exit-code contribution     | Suggested action                                            |
|-----------------------------------------|---------------|----------------------------|-------------------------------------------------------------|
| < 90 days                               | `ok`          | none                       | none                                                        |
| 90-179 days                             | `stale`       | none (warning, not error)  | re-upload your LinkedIn export at happenstance.ai/connections |
| >= 180 days                             | `very_stale`  | adds 1 to overall doctor exit code | re-upload your LinkedIn export at happenstance.ai/connections (urgent) |
| missing (never synced)                  | `never`       | adds 1 to overall doctor exit code | upload your LinkedIn export at happenstance.ai/connections  |

---

## Implementation Units

- U1. **Plumb currentUUID through api hpn search; add --first-degree-only and --min-score**

Goal: Restore the self_graph bridge signal in `api hpn search`'s emit path so 1st-degree is distinguishable from 2nd-degree, then add two filter flags. This is the root-cause fix the SF-friends task surfaced.

Requirements: R1, R2

Dependencies: none

Files:
- Modify: `internal/cli/api_hpn_search.go` (emit path at line 300; flag declarations near line 140-160; runHpnSearch handler)
- Modify: `internal/happenstance/api/normalize.go` (only if call signature needs to change for safety — the existing function already accepts currentUUID)
- Modify: `internal/cli/api_hpn_test.go`
- Modify: `internal/cli/flagship_helpers.go` (if a shared `hasSelfGraphBridge` helper is extracted from `coverage.go:296` to be importable here)

Approach:
- **Pass the authenticated user's UUID into `ToClientPersonWithBridges`.** Today the call at `api_hpn_search.go:300` is `api.ToClientPersonWithBridges(r, env.Mutuals, "")` with a comment apologizing for the empty `currentUUID`. Look up the current user's UUID — `clerk` already retrieves user profiles (`internal/cli/clerk.go`) and the bearer surface returns the authenticated UUID via the user endpoint surfaced by `doctor.go`'s `happenstance_api_validity` check. Cache it on the bearer client (one-time fetch, no extra credits since user-info is free) and pass it on every emit.
- **Verify the retag semantics with a unit test before adding flags.** Test that with the real UUID, a search response containing the user's own self-bridge results in `BridgeKindSelfGraph` on the emitted person; without the UUID (regression case), all bridges stay `BridgeKindFriend`.
- Add `firstDegreeOnly bool` and `minScore float64` to the per-command flag struct.
- When `firstDegreeOnly` is true: auto-set the bearer payload's `IncludeMyConnections=true` (so the API includes 1st-degree in the result set), AND post-filter using `hasSelfGraphBridge(p.Bridges)` matching `coverage.go:296`. The combined effect is the intuitive "give me only my 1st-degree connections" semantic.
- When `minScore > 0`: post-filter dropping `score < minScore`. Documented threshold in `--help` is `>= 5` to drop weak-signal public-graph noise; no high-end recommendation (real 1st-degree bridges sit at 49.99 medium affinity per fixtures).
- Apply both filters in the result-emit path so JSON / table / CSV all see the filtered set.

Execution note: Plumb the currentUUID and add a regression test for the retag BEFORE adding the user-facing flags. The flags only work correctly once the retag is restored.

Patterns to follow:
- `internal/cli/coverage.go:296` `hasSelfGraphBridge` — the working predicate this plan extends to bearer search.
- `internal/happenstance/api/normalize.go:99-102` for the retag logic the empty currentUUID currently bypasses.
- `internal/cli/api_hpn_search.go:140-160` for the BoolVar / Float64Var declaration block.

Test scenarios:
- Happy path: with `currentUUID` plumbed, a fixture response containing a self-bridge entry retags to `BridgeKindSelfGraph`. Without the plumbing (regression test), the same fixture leaves all bridges as `BridgeKindFriend`. Confirms the root cause is fixed.
- Happy path: `--first-degree-only` against a fixture with mixed self_graph / friend / no-bridge results returns only rows where at least one bridge has `kind == BridgeKindSelfGraph`.
- Happy path: `--min-score 5` against a fixture with scores [50, 49.99, 12, 2, 0] returns [50, 49.99, 12].
- Edge case: `--first-degree-only` with `--include-my-connections` already explicitly set is a no-op for the include-add (idempotent); the filter still applies.
- Edge case: `--first-degree-only` against a fixture where the API returns no 1st-degree matches returns "0 results" with a clear message, not a silent empty array.
- Edge case: `--min-score 0` (default) is a no-op.
- Edge case: bearer surface returns an authenticated-user UUID that's the empty string (mock failure mode); fall back to today's behavior with a stderr warning rather than crashing.
- Error path: `--min-score -1` is rejected at flag-parse time with a usage error.
- Error path: bearer user-info lookup fails on first call; cache the failure for the session, log on stderr, fall back to empty-currentUUID behavior so the search still returns something.
- Integration: `--first-degree-only --min-score 5` against the SF-task fixture (page 1 with 30 results, 6 of which were score-2 weak-signal) returns the 24 real 1st-degree rows the user actually wanted.

Verification:
- `go test ./internal/cli -run TestHpnSearchSelfGraphRetag` passes.
- `go test ./internal/cli -run TestHpnSearchFilter` passes.
- Running `contact-goat-pp-cli api hpn search "in San Francisco" --first-degree-only --min-score 5 --json` against the captured fixture returns only rows where the JSON output's `bridges[].kind` includes `"self_graph"`.

---

- U2. hpn search auto-paginate: --all and --max-results

Goal: One command paginates and polls `api hpn search` to completion or a result cap, replacing the manual `find-more` + `get --page-id` + poll loop.

Requirements: R3

Dependencies: U1 (filters apply per page)

Files:
- Modify: `internal/cli/api_hpn_search.go`
- Modify: `internal/cli/api_hpn_test.go`
- Modify: `internal/happenstance/api/search.go` (if a new "iterator" helper is justified; otherwise stay in CLI layer)

Approach:
- Add `--all bool` and `--max-results int` to `newAPIHpnSearchCmd`.
- New helper `runHpnSearchAll(ctx, ...)` wraps the existing single-page `runHpnSearch`. Loop: POST + poll first page; while `has_more && running < max && budget allows`, call `find-more`, then `get --page-id` with poll, accumulate, apply filters, repeat.
- Surface per-page cost on stderr in the existing "cost spent" log convention.
- Bail with a clear status when `--budget` would be exceeded by the next page; emit results so far rather than discarding.
- `--all` REQUIRES at least one of `--max-results` or `--budget` to be non-zero; without either, return a usage error suggesting `--max-results 100` as a default cap. Rationale: an agent calling `--all --yes` against a broad query would otherwise burn unbounded credits.
- `--max-results` without `--all` is an error (it only makes sense in pagination mode).

Patterns to follow:
- `api_hpn_search.go:265` `runHpnSearch` for the single-page shape.
- `api_hpn_search.go:358` `buildPollSearchOptions` for poll-options threading.
- `api/search.go:167` `Client.PollSearch` for the terminal-status check.

Test scenarios:
- Happy path: `--all` with a fixture that returns 3 pages then `has_more=false` accumulates all 90 results. Inputs: bearer fake server with seeded multi-page response; expect 90-row output.
- Happy path: `--all --max-results 50` stops at 50, prints a "stopped at cap" notice on stderr.
- Edge case: `--budget 4 --all` allows initial 2-credit call and one find-more (2 more = 4), then bails before the second find-more; emits results so far + budget warning.
- Edge case: `has_more=false` on first page returns identically to single-page mode; no extra calls made.
- Error path: `--all` alone (no `--max-results`, no `--budget`) returns a usage error suggesting `--max-results 100` as a default cap.
- Error path: `--max-results` without `--all` returns a usage error.
- Error path: `--all` with mid-pagination bearer 402 (out of credits) bails gracefully, emits accumulated results, exit code 5.
- Integration: `--all --max-results 100 --first-degree-only --min-score 5` filters every page, accumulates only 1st-degree matches above the noise threshold.

Verification:
- `go test ./internal/cli -run TestHpnSearchAll` passes.
- Running `contact-goat-pp-cli api hpn search "in San Francisco" --all --max-results 100 --first-degree-only --json` produces a single deduped, filtered, sorted result set.

---

- U3. coverage --location

Goal: `coverage` answers "who do I know in city X" alongside its existing "who do I know at company X" semantics.

Requirements: R4

Dependencies: none

Files:
- Modify: `internal/cli/coverage.go`
- Modify: `internal/cli/coverage_test.go` (create if absent; see existing nearby `flagship_helpers_test.go` for shape)

Approach:
- Add `--location string` flag.
- Change `Args: cobra.ExactArgs(1)` (today at `coverage.go:52`) to `Args: cobra.MaximumNArgs(1)`. Add a `PreRunE` validator that requires exactly one of `<positional>` or `--location`; both or neither returns a usage error.
- When `--location` is set, force `--source api` (bearer-only). Rationale: the cookie surface has no city-search semantic and the LinkedIn arm of today's fan-out is company-keyword-based and doesn't generalize to locations. The `--location` query goes only through bearer search with `--first-degree-only` auto-set so it returns my-network results, not the public graph.
- Result schema is unchanged (name, title, company, linkedin URL, relationship tier, score). Add a `location` column when `--location` was set, populated from bearer's `current_location` field where available.
- Help text gains a `Scope: <company> | --location <city>` preamble explaining the LinkedIn-arm restriction.

Patterns to follow:
- `coverage.go:22-275` for the existing fan-out and result-merge pattern.
- `printCoverageTable` at `coverage.go:402` for the table extension.

Test scenarios:
- Happy path: `coverage --location "San Francisco"` returns rows from a fixture matching the location query (bearer arm only).
- Happy path: `coverage Stripe` (existing) still works unchanged with the original ExactArgs-equivalent validation.
- Edge case: positional arg + `--location` returns a usage error ("specify company or --location, not both").
- Edge case: zero positionals AND no `--location` returns a usage error suggesting one or the other.
- Edge case: empty location string is rejected.
- Edge case: `--location "San Francisco" --source hp` is rejected with a usage error explaining cookie surface doesn't support city search.
- Error path: location with no matches returns a clear "0 results" message not an empty table.
- Integration: `coverage --location "San Francisco" --json` flows through to bearer search with `--first-degree-only` auto-set and returns my-network rows.

Verification:
- `go test ./internal/cli -run TestCoverageLocation` passes.
- Running `contact-goat-pp-cli coverage --location "San Francisco" --json | jq '.results | length'` returns a positive number.
- Running `contact-goat-pp-cli coverage` (no args, no flags) prints a usage error.

---

- U4. **doctor stale-graph warning**

Goal: Doctor visibly flags an out-of-date LinkedIn graph and points the user at the web UI to refresh it. The CLI re-sync subcommand is deferred (see Scope Boundaries) until the upload-trigger endpoint is documented.

Requirements: R5

Dependencies: none

Files:
- Modify: `internal/cli/doctor.go` (extend `checkHappenstanceGraphCoverage` at line 271)
- Modify: `internal/cli/doctor_test.go` (or add to a colocated test file)

Approach:
- In `checkHappenstanceGraphCoverage`, parse `last_refreshed` (the existing field name on the response, `doctor.go:308`); compute age in days.
- Map to one of `ok | stale | very_stale | never` per the state machine table in High-Level Technical Design.
- Emit a structured `{status, age_days, last_refreshed, suggested_action}` field on the doctor JSON. In human-friendly mode, render with color via the existing `humanAge` neighbor: `ok` is uncolored, `stale` prints with a yellow `WARN` prefix, `very_stale` and `never` print with a red `ERR` prefix and contribute 1 to the doctor exit code.
- `suggested_action` text for non-`ok` states points to `https://happenstance.ai/connections` (the web UI for re-uploading the LinkedIn export). When/if HP ships a triggerable endpoint, this text becomes a CLI command in a follow-up plan.
- Hardcoded thresholds 90/180; no env override.

Patterns to follow:
- `internal/cli/doctor.go:271` for the existing `checkHappenstanceGraphCoverage` extension shape.
- `humanAge` neighbor in the same file for date-format rendering.

Test scenarios:
- Happy path: graph age 30 days returns `status: ok`, no warning, no exit-code contribution.
- Happy path: graph age 120 days returns `status: stale`, `WARN` prefix, suggested action references happenstance.ai/connections.
- Edge case: graph age 200 days returns `status: very_stale`, `ERR` prefix, doctor exit code includes 1.
- Edge case: missing or empty `last_refreshed` (never synced) returns `status: never`, exit code includes 1.
- Edge case: malformed `last_refreshed` timestamp (parse fails) returns `status: unknown`, no exit-code contribution, structured field includes the parse error.

Verification:
- `go test ./internal/cli -run TestDoctorStaleGraph` passes.
- `contact-goat-pp-cli doctor --json | jq '.happenstance_graph_status'` returns one of `ok|stale|very_stale|never|unknown`.
- `contact-goat-pp-cli doctor` against the captured fixture (graph age ~9 months) prints `WARN` + the happenstance.ai/connections URL and contributes 1 to the exit code.

---

- U5. cookie-timeout fast-fail with bearer-fallback hint

Goal: Cookie surface fails fast with an actionable hint instead of hanging silently for 4 minutes on broad queries.

Requirements: R6

Dependencies: none (but interacts with U1-U2 since hpn search is one of the surfaces affected)

Files:
- Modify: `internal/client/people_search.go`
- Modify: `internal/cli/api_hpn_search.go` (the source-routing path)
- Modify: `internal/cli/coverage.go` (same routing)
- Modify: `internal/cli/hp_people.go` (same routing)
- Modify: `internal/client/people_search_test.go`

Approach:
- Declare `ErrCookieBroadQuery` as a sentinel error in `internal/client/people_search.go`.
- In the cookie poll/request path, classify four conditions to that sentinel: poll exceeded with status still `"Thinking"`; http-client deadline exceeded; HTTP 524/502/503 upstream; non-terminal `"Thinking"` status persisting past 90 seconds (early-bail threshold below the 180s default poll timeout).
- In each routing call site (`api_hpn_search.go`, `coverage.go`, `hp_people.go`), when `errors.Is(err, ErrCookieBroadQuery)`, print to stderr: `cookie surface timed out (likely a broad query). Retry with --source api to use the bearer surface (2 credits/call).` Return exit code 5 (API error) consistently across all four flavors. Exit 7 stays reserved for `RateLimitError`, asserted by the existing test at `api_hpn_test.go:433`.

Patterns to follow:
- `api_hpn_search.go:398` `classifyHpnError` for the error-classification idiom.
- `client/client.go:134` for the http-client timeout source.
- The existing "cost spent" stderr log convention for the hint format.

Test scenarios:
- Happy path: cookie returns COMPLETED before timeout; no hint emitted, normal output.
- Edge case: cookie poll times out after 180s; `ErrCookieBroadQuery` returned, hint printed, exit 5.
- Edge case: cookie stuck at `"Thinking"` for 90s; bail early, hint printed, exit 5.
- Edge case: 524 from cookie surface; hint printed, exit 5.
- Edge case: 502 from cookie surface; hint printed, exit 5.
- Edge case: explicit `--source api` skips cookie entirely; no hint regardless of bearer behavior.
- Edge case: bearer 429 returns exit 7 unchanged (rate-limit path); confirm not conflated with cookie timeout.
- Error path: bearer 402 after the user takes the hint; existing handling, no double-hint.
- Integration: `hp people "people in San Francisco"` (the original failing query) with cookie source bails in <=95s with the hint.

Verification:
- `go test ./internal/client -run TestErrCookieBroadQuery` passes.
- `time contact-goat-pp-cli hp people "people in San Francisco" --source hp` exits in <=95s with the suggested hint.

---

- U6. **DEFERRED — terminology rename for hp people / friends**

Deferred to a separate plan. Adding `--first-degree` / `--second-degree` as forever-aliases without deprecating `--connections` / `--friends` produces 4 flag names for 2 concepts, plus `top-connectors` alongside `friends` for one command. The cumulative confusion exceeds the readability win. Two paths for the follow-up plan: (a) introduce `MarkDeprecated` as a precedent and rename the originals, or (b) drop the rename and just expand `--help` text on the existing flags. The actual disambiguation work needed from the SF-task — making 1st-degree distinguishable from 2nd-degree on the bearer surface — is solved by U1, not by flag renames. See Scope Boundaries → Deferred to Follow-Up Work.

The U-ID stays U6 (gap rule: never renumber); a follow-up plan would assign a new U-ID in its own document.

---

- U7. hp people native CSV with bridges flatten

Goal: `hp people --csv` produces a clean, agent-pipeable CSV that flattens the nested `bridges[]` into stable columns.

Requirements: R8

Dependencies: none

Files:
- Modify: `internal/cli/hp_people.go` (add `printHPPeopleCSV` sibling to `printHPPeopleTable`)
- Modify: `internal/cli/flagship_helpers.go` (small helper to pick strongest bridge)
- Modify: `internal/cli/hp_people_test.go`

Approach:
- Hook `printHPPeopleCSV` into the output-mode branch in `runHPPeople` (parallel to existing `--json` and table branches). Today the global `--csv` flag is silently ignored by hp people; this routes it.
- Columns (stable): `name, current_title, current_company, linkedin_url, score, relationship_tier, bridge_count, bridge_names, bridge_kinds, top_bridge_affinity, rationale`. `relationship_tier` derives from the strongest bridge's `kind`: `BridgeKindSelfGraph` -> `1st_degree`, `BridgeKindFriend` -> `2nd_degree`, no bridge -> `3rd_degree`. `bridge_count` preserves cardinality. `bridge_names` and `bridge_kinds` are semicolon-joined to keep multi-bridge identity without going long-form. `top_bridge_affinity` is the highest affinity score across bridges.
- This avoids the lossy "collapse to single strongest bridge" the earlier draft proposed. Multi-bridge data is preserved in CSV; users who need the full structured form still use `--json`.
- Use stdlib `encoding/csv` rather than handrolling.
- Keep `printHPPeopleTable` and `--json` paths untouched.

Patterns to follow:
- `internal/cli/helpers.go:481` `printCSV` for stdlib `encoding/csv` usage.
- `printHPPeopleTable` at `hp_people.go:252` for the per-mode emitter shape.

Test scenarios:
- Happy path: 3 results with single-bridge each produce 3 CSV rows + header; `bridge_count` is 1, `bridge_names` and `bridge_kinds` each have one value, `top_bridge_affinity` matches the single bridge's affinity.
- Happy path: result with 3 bridges produces `bridge_count: 3`, `bridge_names: "Alice;Bob;Carol"`, `bridge_kinds: "self_graph;friend;friend"`, `top_bridge_affinity` is the max of the three.
- Happy path: result with one self_graph bridge gives `relationship_tier: 1st_degree`; one friend bridge gives `2nd_degree`; no bridges gives `3rd_degree`.
- Edge case: result with empty `bridges[]` produces empty `bridge_count: 0`, empty `bridge_names`, `bridge_kinds`, `top_bridge_affinity` cells; no panic.
- Edge case: name or bridge_name containing a comma or semicolon is correctly quoted by stdlib csv writer.
- Edge case: empty result set produces just the header row.
- Integration: `hp people "..." --csv | wc -l` matches `--json | jq '.results | length'` + 1 for header.

Verification:
- `go test ./internal/cli -run TestHPPeopleCSV` passes.
- `contact-goat-pp-cli hp people "people in SF" --csv | head -3` shows header + two data rows; no nested-array junk.

---

- U8. scoring docs + --help expansion

Goal: Document the score field's semantics so users know what `--min-score 5` vs `--min-score 40` means.

Requirements: R9

Dependencies: U1 (the doc references `--min-score`)

Files:
- Create: `docs/scoring.md`
- Modify: `internal/cli/api_hpn_search.go` (expand the `Long` help text on the search command)
- Modify: `README.md` (add a one-line link to `docs/scoring.md` near the search command discussion)

Approach:
- `docs/scoring.md` covers: what the CLI does with the score (`--min-score N` drops rows where score < N), the observed range from local code (`flagship_helpers.go:138-160`: typical 10-100 with strong-graph signals observed up to ~300; weak-signal public-graph entries show as 0-2), and a recommendation that `--first-degree-only` (not `--min-score`) is the right tool for tier filtering. Defers scoring-model semantics to Happenstance's own docs with a stable link rather than re-documenting HP product behavior in the CLI repo.
- Expand `api hpn search` `Long` help to include a brief paragraph linking to `docs/scoring.md` (one sentence, not the full doc).
- Add a one-liner to README's existing "agent mode" section pointing at `docs/scoring.md`.

Patterns to follow:
- Local doc precedent: existing `README.md` and `SKILL.md` for tone and example formatting.
- `api_hpn_search.go` Long-text format for the help expansion.

Test scenarios:
- Test expectation: none. This unit is documentation only with no behavioral change.

Verification:
- `docs/scoring.md` exists and is rendered correctly by GitHub markdown.
- `contact-goat-pp-cli api hpn search --help` shows the new pointer line.
- README diff is minimal and additive.

---

- U9. Skills regen + plugin bump

Goal: Keep the CLI SKILL surface in sync with new flags and commands; verify SKILL.md docs all flags resolve.

Requirements: all (every preceding unit changes the user-visible flag surface or command list)

Dependencies: U1-U8

Files:
- Modify: `cli-skills/pp-contact-goat/SKILL.md` (regenerated)
- Modify: `skills/ppl/references/registry.json` (regenerated)
- Modify: `.claude-plugin/plugin.json` (bump only if `skills/ppl/**` or `.claude-plugin/**` changed)

Approach:
- Run `go run ./tools/generate-skills/main.go` from repo root.
- Run `.github/scripts/verify-skill/verify_skill.py` locally; resolve any flag-mismatch failures by updating SKILL.md prose, not by reverting code.
- Inspect the regen diff: confirm new flags (`--first-degree-only`, `--min-score`, `--all`, `--max-results`, `--location`) are documented. The deferred terminology rename (U6) and deferred sync subcommand mean no `--first-degree` / `--second-degree` / `top-connectors` / `sync linkedin-graph` entries should appear.
- Commit regen as a separate commit after the substantive units land, matching the precedent set by `2026-04-19-001-fix-bearer-api-mutuals-plan.md` Unit 5.

Patterns to follow:
- `2026-04-19-001-fix-bearer-api-mutuals-plan.md` Unit 5 for the regen-as-final-unit shape.
- AGENTS.md "regen step" guidance.

Test scenarios:
- Test expectation: none. This unit is regeneration only with no source code change.

Verification:
- `python3 .github/scripts/verify-skill/verify_skill.py` exits 0.
- `go test ./...` passes (no unintended source changes).
- The regen diff touches only `cli-skills/pp-contact-goat/SKILL.md` and `skills/ppl/references/registry.json` (and `.claude-plugin/plugin.json` only if applicable).

---

## System-Wide Impact

- **Cross-cutting touch on `api_hpn_search.go`.** U1, U2, and U5 all modify `internal/cli/api_hpn_search.go` — the currentUUID plumbing, the `--all` loop, and the `ErrCookieBroadQuery` classification all live in or adjacent to `runHpnSearch`. **Recommended PR sequencing**: bundle U1 + U2 + U5 into a single PR rather than three. Three separate PRs against the same flag-declaration block and handler will produce textual conflicts in the same lines and risk one PR reverting another's filter logic. U3 (coverage), U4 (doctor), U7 (hp people CSV) are independent and can land in their own PRs.
- **Interaction graph**: `runHpnSearch` is the central seam. `runCoverage` and `runHPPeople` get a parallel U5 hook because they share the cookie-vs-bearer routing decision. Doctor's `checkHappenstanceGraphCoverage` is independent.
- **Error propagation**: U5 introduces a new sentinel `ErrCookieBroadQuery` in `internal/client/people_search.go` that propagates up through cobra `RunE` returns into stderr-printed hints. Exit codes: 5 (API error) for `ErrCookieBroadQuery`; 7 (rate-limit) reserved for `RateLimitError` only; no new codes.
- **State lifecycle risks**: U2's `--all` loop accumulates results in memory; mitigated by the hard requirement that one of `--max-results` or `--budget` be set.
- **API surface parity**: U1's currentUUID plumbing affects the bearer emit path only. Cookie surface for `hp people` is unaffected (cookie already retags correctly). `coverage` (U3) routes through the same auto-router; no payload change required.
- **Integration coverage**: U2 + U1 together change the cost profile of a single command invocation. The existing budget check (`checkSearchBudget` at api_hpn_test.go:400) needs extension to account for projected pagination cost.
- **Unchanged invariants**:
  - The cookie-vs-bearer auto-routing default. U5 only fires after routing already chose cookie; it does not override routing.
  - The 5-minute GET cache. None of the units change cache TTL or invalidation.
  - The `--agent` cascade in `PersistentPreRunE`. New flags inherit the cascade automatically.
  - The MCP server tool registry. None of these CLI changes ship as new MCP tools; the existing 16 tools' behavior is unchanged. (A follow-up plan can decide whether `--first-degree-only` becomes an MCP-exposed parameter.)
  - The shared `PollSearch` helper at `api/search.go:167`. U2's auto-pagination wraps it; the helper itself is unchanged.
  - Existing flag names (`--connections`, `--friends`, `friends` command) all keep working; the rename is deferred.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| `--all` users misjudge the credit cost and burn through their balance. | `--all` requires at least one of `--max-results` or `--budget`; calling `--all --yes` alone returns a usage error. Per-page cost line on stderr. |
| Currentuser-UUID lookup adds a network call to every `api hpn search`. | Cache the UUID on the bearer client at construction time (one-time fetch from a free user-info endpoint). Fall back to today's empty-UUID behavior with a stderr warning if the lookup fails. |
| HP changes the bridge-kind enum in the API response, breaking the `BridgeKindSelfGraph` predicate. | Plan ties the predicate to the codebase's existing constants (`internal/client/people_search.go:151-154`); a HP enum change would already break `coverage`'s `hasSelfGraphBridge`, so this plan does not introduce a new contract surface. |
| SKILL verifier fails after regen due to mismatch between new flag and SKILL.md prose. | U9's verification step explicitly runs `verify_skill.py` and resolves prose mismatches; the regen unit is the explicit place this happens. |
| `coverage --location` fails to find rows because of cookie-side issues. | U3 routes `--location` to bearer-only via `--source api`; cookie surface is excluded from city searches by design. |
| Stale-graph thresholds (90/180 days) miss someone who actively wants a tighter warning. | Threshold tuning is deferred follow-up; observed user data (the SF task ran fine on a 9-month graph) suggests 90/180 is the right starting bar, with room to add tunability later if needed. |
| `--min-score` recommendation in docs ages poorly when HP changes scoring. | `docs/scoring.md` (U8) documents only the CLI's behavior (`--min-score N` drops rows with score < N), defers HP-specific score-semantics to HP's own docs. |
| Multi-bridge data collapsed in CSV. | U7 preserves `bridge_count`, `bridge_names`, `bridge_kinds` (semicolon-joined) so cardinality and identity survive flattening. `--json` remains the structured-form fallback. |

---

## Documentation / Operational Notes

- Update `README.md` Notable Commands table for the new behaviors (`--first-degree-only`, `--min-score`, `--all` with `--max-results`, `coverage --location`, `hp people --csv`) and the doctor stale-graph status field.
- Update `SKILL.md` via the regen unit (U9). The SKILL verifier enforces flag-text consistency.
- Add `docs/scoring.md` (U8). Link it from `README.md` and from `api hpn search --help`.
- No migration steps required; all changes are additive.
- No rollout monitoring required; this is a CLI surface, not a service.
- `CHANGELOG.md` entry under `feat(cli)`: "contact-goat: bearer search now distinguishes 1st-degree from 2nd-degree (currentUUID plumbed through emit path), `--first-degree-only` and `--min-score` filters, `--all` auto-pagination with hard credit safety, `coverage --location`, doctor stale-graph warning, native CSV for `hp people`."

---

## Sources & References

- Origin task: live SF-friends task (no upstream brainstorm doc; planning bootstrap from user request directly). The bug exposed by the task is the empty `currentUUID` in `api_hpn_search.go`'s emit path; doc-review surfaced it during deepening on 2026-05-04.
- Prior plan precedent: `docs/plans/2026-04-19-001-fix-bearer-api-mutuals-plan.md` (mutuals-bearer fix; same surface area).
- Repo guidance: `AGENTS.md` (regen requirement, conventional-commit scopes, no-broad-regen norm, SKILL verifier policy).
- Bridge-kind constants and the working 1st-degree predicate: `internal/client/people_search.go:151-154` (`BridgeKindFriend`, `BridgeKindSelfGraph`); `internal/cli/coverage.go:296` (`hasSelfGraphBridge`).
- Bridge retag logic gated on currentUUID: `internal/happenstance/api/normalize.go:99-102`.
- Empty-UUID call site (the fix target): `internal/cli/api_hpn_search.go:300`.
- Poll helper and terminal-status constants: `internal/happenstance/api/search.go:20-23, 167, 215`.
- Cookie-mirror poll-timeout regression test: `internal/client/people_search_test.go:15`.
- Score range observation: `internal/cli/flagship_helpers.go:138-160` ("typical 10-100 ... observed up to ~300").
- Sniff archive (no upload-trigger endpoint observed): `.manuscripts/happenstance-sniff-2026-04-19/README.md`.
- Happenstance public API surfaces: cookie web app + bearer REST API documented in `SKILL.md`.

### Deepening notes

- 2026-05-04 doc review: surfaced the empty-`currentUUID` root cause (P0, anchor 100), corrected the bridge-kind predicate from the imagined `"connection"` to the real `"self_graph"`, deferred U6 (terminology rename) and the U4 sync-subcommand half (no upload-trigger endpoint), tightened `--all` credit safety, fixed exit-code semantics (5 not 7 for cookie timeouts), corrected score-range claims to match `flagship_helpers.go`, dropped the `CONTACT_GOAT_GRAPH_STALE_DAYS` env override, bumped stale-graph thresholds from 30/90 to 90/180, replaced lossy bridge-collapse-by-affinity in CSV with multi-bridge preserving columns, and called for U1+U2+U5 to ship as a single PR.
