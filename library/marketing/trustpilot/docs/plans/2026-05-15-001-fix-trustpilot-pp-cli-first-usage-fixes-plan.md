---
title: Trustpilot CLI - first-usage fixes from ThriftBooks and June Oven testing
type: fix
created: 2026-05-15
status: active
plan_depth: standard
target_repo: ~/printing-press/library/trustpilot
---

# Trustpilot CLI - first-usage fixes from ThriftBooks and June Oven testing

## Summary

The first two real users of `trustpilot-pp-cli` ran the same agent-bundle and review-sync workflows against ThriftBooks (2.78M reviews, actively collecting) and June Oven (1,462 reviews, collection stopped Sept 2023). Together those sessions surfaced 18 distinct findings. Many are the same root cause manifesting in multiple commands. This plan consolidates them into eight implementation units, ordered so the foundational meta-envelope work lands first.

The two threads agree on three things that matter most:

1. The CLI returns null or empty fields without telling agents why, which causes downstream LLMs to confidently misreport "no recent reviews" when the truth is "the sync did not populate."
2. Trustpilot's 10-page-per-filter pagination cap is the single most surprising fact about this API, and the CLI knows about it but does not surface it.
3. Cookie auth and Next.js build-id errors are conflated: a stale token, a stale build id, and an unsupported query parameter all produce error messages that point the user to the wrong fix.

The two threads disagree on nothing of substance. Everything else is incremental UX or DX polish.

## Problem Frame

trustpilot-pp-cli is a brand-new printed CLI built for agent integration with last30days-style research workflows. First-usage testing surfaced systematic gaps between what the SKILL.md promises and what commands actually deliver in real use. The gaps fall into three categories:

- **Silent failures**: agent-bundle and top-recent return null fields when the local store is empty, info returns empty histogram and AI summary on companies that stopped collecting reviews, topics returns null - all without a meta envelope explaining whether the data is genuinely absent, the parser drifted, or the sync was never run.
- **Misleading errors**: HTTP 404 on the JSON API is reported as "Next.js build id rejected; re-run auth login" even when the actual cause is an unsupported query param (the `--date last3months` regression). The auth-login command requires an explicit `--chrome` flag even though it is the only supported mode in v1.
- **Crashed commands**: search-reviews crashes with `fts5: syntax error near ""` when called with an empty term. There is no first-class way to list synced reviews by filter without inventing a search term.

These problems are amplified by the pagination cap (200 reviews per filter, 10 pages × 20 each). When sync hits the cap, the API returns `totalPages: 0` for the next request, the CLI advances its cursor anyway, and `--resume` then refuses to make progress until the user manually passes `--no-resume`. The cap is documented nowhere in the CLI surface.

## Scope Boundaries

### In scope

- All P0/P1 findings from both threads
- The P2 findings that improve agent UX (meta envelope, freshness fields, search-reviews empty-term handling, doctor cache check)
- One P3 finding when the fix is trivial (auth login default to chrome)

### Deferred to Follow-Up Work

- Flag-shape consistency across commands (top-recent uses `--good/--bad`, search-reviews uses `--limit`, sync-trustpilot uses `--max-pages`). Each command's flags are internally coherent; cross-command symmetry is polish that risks breaking shipped agent invocations.
- `--deliver file:` writes 0600. Document in README, do not change behavior - 0600 is the right default for an agent-written file that may contain review text.
- Proxy rotation for sustained high-volume sync. Mentioned in the README as a known limitation; out of scope for first-usage fixes.
- The spec-derived `companies` shortcut. It is already `cmd.Hidden = true` in the current build, so the leaky `<search_build_id>` UX is no longer visible to users. The user-facing `search "<name>"` command is the correct entry point.

### Outside this product's identity

- Trustpilot's 10-page-per-filter pagination cap. This is an upstream constraint and cannot be removed; the plan surfaces it instead of trying to defeat it.
- Trustpilot's WAF challenge lifespan (5-15 minutes). The CLI auto-refreshes; that is the right answer.
- Generator-side fixes for the doctor command. The doctor uses generic `resources` / `sync_state` tables and our Trustpilot tables are `tp_*`. The right machine-side fix is a generator hook so doctor can be extended per-CLI; that fix should land in cli-printing-press, not here. This plan adds a printed-CLI-side override and leaves a retro candidate for the generator.

## Requirements Trace

### Thread 1 (ThriftBooks)

| Origin # | Severity | Finding | Unit |
|---|---|---|---|
| T1-1 | P1 | search-reviews crashes on empty term (fts5 syntax error) | U3 |
| T1-2 | P1 | sync_state table empty after successful sync (doctor confused) | U6 |
| T1-3 | P1 | resources table empty even though CLI can read data | U6 |
| T1-4 | P1 | agent-browser daemon "Connection refused" on cold start | U7 |
| T1-5 | P2 | No first-class "list/export local reviews" command | U3 |
| T1-6 | P2 | doctor reports "Cache: unknown" after sync | U6 |
| T1-7 | P2 | spec-derived `reviews <build_id> <domain>` leaks Next.js build id | Resolved (already Hidden) |
| T1-8 | P2 | Trustpilot 200-per-filter cap not surfaced | U4 |
| T1-9 | P3 | Flag inconsistency across commands | Deferred |
| T1-10 | P3 | top-recent --window 365d returns 2 days of data without noting it | U2 |
| T1-11 | P3 | --deliver file: writes 0600 silently | Deferred (README only) |

### Thread 2 (June Oven)

| Origin # | Severity | Finding | Unit |
|---|---|---|---|
| T2-1 | P0 | agent-bundle / top-recent return null for good/bad on empty local store, no signal | U1, U2 |
| T2-2 | P0 | info returns empty ratingHistogram and aiSummary, ambiguous between drift and genuine empty | U2, U8 |
| T2-3 | P0 | topics returns null with same ambiguity | U2 |
| T2-4 | P1 | auth login requires --chrome even though it is the only supported mode | U5 |
| T2-5 | P1 | reviews-fetch --date last3months returns "build id rejected" misleadingly | U5 |
| T2-6 | P1 | Pagination cap (10 pages × 20) not surfaced anywhere | U4 |
| T2-7 | P1 | Resume cursor poisoned when sync hits cap | U4 |
| T2-8 | P2 | companies "shortcut" is worse than search | Resolved (already Hidden) |
| T2-9 | P2 | search-reviews empty query crashes | U3 |
| T2-10 | P2 | isCollectingReviews: false not surfaced at top level of agent-bundle / top-recent | U2 |

### Aggregate

- 18 raw findings -> 8 implementation units + 3 already-resolved + 3 deferred + 1 retro candidate (doctor generalization)

---

## Key Technical Decisions

1. **Auto-fall-through to live on empty local store**, not auto-sync. The `--data-source auto` flag already documents this behavior; agent-bundle and top-recent are violating their own contract. Fall-through is one HTTP request; auto-sync is many requests plus a WAF cookie harvest, which is heavy and surprising. Decision: when a read command runs in `--data-source auto` mode and the local store has no matching rows, transparently fall through to live fetch. Emit `meta.source: "live (fallback)"` and `meta.notices: ["local_store_empty_for_domain"]` so the agent can tell.

2. **Adopt a `meta` envelope on every read command's JSON output**. The shape is `meta.source` (local | live | "live (fallback)"), `meta.notices[]` (machine-readable enum strings), and command-specific freshness fields (`meta.newestReviewAt`, `meta.localCount`, `meta.isCollectingReviews`). This is the foundation that all other "give agents a signal" fixes depend on. The notices array uses snake_case enums an agent can switch on, not prose.

3. **Distinguish HTTP 404 stale-buildid from 404 unsupported-filter**. The current transport treats every 404 as `BuildIDStaleError`. In testing, the `--date last3months` filter caused a 404 that was NOT a build id problem (a fresh re-auth did not fix it). Decision: when 404 occurs, inspect the response body for `__N_REDIRECT` signature (Next.js soft-redirect). If present, the filter is the issue; emit a distinct `FilterUnsupportedError`. Only treat opaque 404s as build id staleness.

4. **`auth login` defaults to chrome harvest, no flag required**. The `--chrome` flag is currently required even though it is the only valid mode in v1. Default to chrome. Keep `--chrome` as a no-op flag for compatibility with any agent scripts that explicitly pass it.

5. **`search-reviews <domain> ""` falls through to non-FTS QueryReviews**. Instead of failing with `fts5: syntax error near ""`, an empty term means "no full-text filter" and the command serves as a unified list-and-filter surface over the local store. Star and date filters still apply.

6. **Doctor command extension lives in a new hand-written file, not by editing the generated `doctor.go`**. The generated doctor's check looks at generic `resources` / `sync_state` tables that this CLI does not use. Since `internal/cli/doctor.go` is generated (`DO NOT EDIT`), the fix is to add `internal/cli/tp_doctor.go` that registers a Trustpilot-specific cache check via the doctor's plugin hook. If the generated doctor has no plugin hook, the printed-CLI override is a generator-side change, which goes to retro instead. This plan inspects the actual extension point before committing the shape.

7. **Pagination cap detection is data-driven, not heuristic**. When `pp.Pagination.TotalPages == 0` AND the previous page returned reviews, the CLI infers "filter cap hit." It does not advance the cursor, does not save the cap-hit state, emits `meta.notices: ["trustpilot_filter_cap_hit_at_page_N"]`, and the human output suggests `--bust-cutoff`. The bust-cutoff path is unchanged.

8. **Browser daemon cold-start retry is bounded and silent on success**. agent-browser's daemon-backed model means the first invocation on a fresh shell can fail with "Connection refused." Decision: when `runAgentBrowser` sees "Connection refused" stderr, sleep 500ms and retry once. If it still fails, emit a clear error pointing at the manual warmup command. Do not auto-start the daemon; that is agent-browser's job, not ours.

---

## Output Structure

```
internal/
├── trustpilot/
│   ├── transport.go       (modified - U5 filter-unsupported error type)
│   ├── parser.go          (modified - U8 histogram diagnostics)
│   ├── browser.go         (modified - U7 cold-start retry)
│   └── store.go           (modified - U4 cursor-poisoning guard)
├── cli/
│   ├── tp_helpers.go      (modified - U1 fallthrough helper, U2 meta envelope helpers)
│   ├── tp_doctor.go       (new - U6 Trustpilot-specific cache check; gated on hook existence)
│   ├── agent_bundle.go    (modified - U1 fallthrough, U2 meta envelope)
│   ├── top_recent.go      (modified - U1 fallthrough, U2 meta envelope)
│   ├── info.go            (modified - U2 meta envelope, U8 histogram_empty notice)
│   ├── topics.go          (modified - U2 meta envelope)
│   ├── search_reviews.go  (modified - U3 empty-term fallthrough)
│   ├── sync_tp.go         (modified - U4 cap detection + meta notice)
│   └── auth.go            (modified - U5 default-to-chrome)
```

No new top-level commands. All work is amendments to existing files plus one new `tp_doctor.go` if the doctor hook exists.

---

## Implementation Units

### U1. Auto-fall-through to live when local store is empty

**Goal**: agent-bundle and top-recent return real data (or a typed error) in `--data-source auto` mode against a domain that has never been synced, instead of returning null.

**Requirements**: T2-1 (P0)

**Dependencies**: U2 (the fallthrough emits notices via the meta envelope)

**Files**:
- internal/cli/tp_helpers.go (modify)
- internal/cli/agent_bundle.go (modify)
- internal/cli/top_recent.go (modify)

**Approach**:
- Add a helper `resolveDataSource(flags, db, domain) (source string, fallbackTriggered bool, err error)` to tp_helpers.go. It returns `"local"` when the local store has rows for the domain within the window; `"live"` when the user explicitly passed `--data-source live`; and `"live (fallback)"` when `--data-source auto` was the default and local was empty.
- top-recent and agent-bundle call this helper first, then route bucket fetches to either the local store (existing path) or `fetchPageWithRetry` (new live path inside these two commands).
- When fallback triggers, attach `local_store_empty_for_domain` to `meta.notices`.
- Do NOT auto-write the live fallback into the local store. That is what `sync-trustpilot` is for, and the user has not consented to a sync side effect.

**Patterns to follow**:
- Mirror the existing `fetchBucket` shape from `internal/cli/top_recent.go` for the live path so agent-bundle and top-recent share the same logic.
- The `--data-source` flag already exists in `rootFlags`. Treat empty string and "auto" as the same case.

**Test scenarios**:
- Happy path (local hit): sync 20 reviews, then `top-recent --window 90d --good 5 --bad 5`. Expect 5+5 reviews and `meta.source: "local"`, no notices.
- Empty local: clear `tp_reviews`, run `top-recent --window 90d --good 3 --bad 3 --data-source auto`. Expect live fetch, `meta.source: "live (fallback)"`, `meta.notices` contains `local_store_empty_for_domain`.
- Explicit local fails fast: `top-recent --data-source local` on an empty store returns an empty payload with `meta.notices: ["local_store_empty_for_domain"]`, exit 0 (not 1). Agents need to distinguish "no data here" from "command crashed."
- Explicit live: `top-recent --data-source live` always hits the API, never reads local store, `meta.source: "live"`.

**Verification**: every JSON response from agent-bundle and top-recent has a non-null `meta.source` field and, when the local store is empty, a non-null `meta.notices` array explaining why.

---

### U2. meta envelope with notices and freshness fields on every read command

**Goal**: every read-command JSON output carries a `meta` object so an agent can distinguish "no data" from "stale data" from "frozen company" from "scraper drift."

**Requirements**: T2-1, T2-2, T2-3, T2-10, T1-10 (4 × P0/P1, 1 × P3)

**Dependencies**: none (foundational unit)

**Files**:
- internal/cli/tp_helpers.go (modify - add `newMeta(...)` and `addNotice(...)` helpers)
- internal/cli/agent_bundle.go (modify)
- internal/cli/top_recent.go (modify)
- internal/cli/info.go (modify)
- internal/cli/topics.go (modify)
- internal/cli/reviews.go (modify)
- internal/cli/search_reviews.go (modify - prepare for U3)

**Approach**:
- Add a `Meta` struct in tp_helpers.go with fields: `Source string`, `Notices []string`, `NewestReviewAt *time.Time`, `LocalCount *int`, `IsCollectingReviews *bool`, `FetchedAt time.Time`. Pointer types let JSON omit fields the command did not populate.
- Add helper `attachMeta(payload map[string]any, meta Meta)` that sets `payload["meta"] = meta`.
- Each read command builds a Meta and attaches it before printJSON.
- Notice enum strings (snake_case, agent-switchable):
  - `local_store_empty_for_domain` (U1)
  - `isCollectingReviews_false` (frozen company)
  - `last_review_older_than_window` (the T1-10 case)
  - `histogram_empty_from_api` (U8 / T2-2)
  - `ai_summary_empty_from_api` (T2-2)
  - `topics_empty_from_api` (T2-3)
  - `trustpilot_filter_cap_hit` (U4 / T2-6)
  - `live_fallback_after_local_miss` (U1)
- For agent-bundle and top-recent specifically, also surface `isCollectingReviews` and the newest synced/fetched review date at the TOP LEVEL of the payload (not just under meta), since they are the most-cited fields when an LLM is summarizing.

**Patterns to follow**:
- The existing `flags.printJSON(cmd, payload)` shape is unchanged. Meta is just one more key in the payload.
- The existing `--select` mechanism still works; `--select meta` lets the agent fetch just the envelope, `--select 'meta.notices'` works for dotted-path narrowing.

**Test scenarios**:
- agent-bundle on a fresh sync: `meta.source = "local"`, `meta.notices = []`, `meta.newestReviewAt` is set, top-level `isCollectingReviews` is set.
- agent-bundle on juneoven.com (frozen): `meta.notices` contains `isCollectingReviews_false`, top-level `isCollectingReviews: false`, `meta.newestReviewAt` is 2023-09-06.
- top-recent --window 365d when synced data only spans 48h: `meta.notices` contains `last_review_older_than_window`. Verify the notice is computed from the actual newest synced review, not the window flag value.
- info on juneoven.com (empty AI summary): `meta.notices` contains `ai_summary_empty_from_api`. ratingHistogram is `{}` and `histogram_empty_from_api` is in notices. (U8 will sharpen this further.)
- topics on a company without per-topic AI: `meta.notices` contains `topics_empty_from_api`.
- All read commands have `meta.source` and `meta.fetchedAt` non-null.

**Verification**: every JSON output of agent-bundle, top-recent, info, topics, reviews-fetch, and search-reviews has a `meta` object. The notices enum is documented in the SKILL.md.

---

### U3. search-reviews falls through to QueryReviews on empty term

**Goal**: `search-reviews <domain> ""` (and equivalent empty-quoted invocations) returns a filter-list of locally synced reviews instead of crashing with an FTS5 syntax error.

**Requirements**: T1-1, T1-5, T2-9 (1 × P1, 2 × P2)

**Dependencies**: U2 (response payload uses the meta envelope)

**Files**:
- internal/cli/search_reviews.go (modify)

**Approach**:
- Add an empty-term branch in search_reviews RunE. When `term == ""` (or after trim), skip `tpkg.FullTextSearchReviews` and call `tpkg.QueryReviews` directly with the same `QueryFilters` (stars, window, language, limit).
- Update `--help` Example to show both modes: full-text and list-by-filter.
- Update payload to include `mode: "fts" | "list"` under `meta` so the agent knows which path served the request.
- Document the dual mode in the SKILL.md unique-capabilities entry for search-reviews.

**Patterns to follow**:
- `tpkg.QueryReviews` already accepts every filter `FullTextSearchReviews` does. No store changes needed.

**Test scenarios**:
- `search-reviews thriftbooks.com ""` returns all locally synced reviews (capped at --limit, default 50), `meta.mode = "list"`, no crash.
- `search-reviews thriftbooks.com "" --stars 1 --window 30d` returns 1-star reviews from the window, `meta.mode = "list"`.
- `search-reviews thriftbooks.com refund` (no quotes) still uses FTS, `meta.mode = "fts"`.
- `search-reviews thriftbooks.com '   '` (whitespace-only) routes to list mode after Trim, not FTS.

**Verification**: the failing invocation from Thread 1 now returns reviews. `--help` documents both modes.

---

### U4. sync-trustpilot detects pagination cap, does not poison the cursor

**Goal**: When sync hits Trustpilot's 10-page filter cap, the CLI stops cleanly, does NOT save a degenerate cursor state, and emits a notice plus a concrete suggestion (`--bust-cutoff`).

**Requirements**: T1-8, T2-6, T2-7 (2 × P1, 1 × P2)

**Dependencies**: U2 (notice mechanism)

**Files**:
- internal/cli/sync_tp.go (modify)
- internal/trustpilot/store.go (modify - SaveCursor input validation)

**Approach**:
- In the sync loop, when a page returns 0 reviews AND `pp.Pagination.TotalPages == 0`, treat as cap-hit. Break the loop without calling SaveCursor for that page.
- Add a guard inside `tpkg.SaveCursor` that refuses to persist `(lastPage > 0, totalPages == 0)` and returns a sentinel error. Belt-and-suspenders: even if the call site forgets, the store rejects the poisoned write.
- Emit `meta.notices: ["trustpilot_filter_cap_hit_at_page_N"]` and a human-readable hint: "Trustpilot caps each filter at 10 pages (200 reviews). Use --bust-cutoff to iterate stars=1..5 and reach more."
- Update `sync-trustpilot --help` to document the cap and the workaround.

**Patterns to follow**:
- The existing `--bust-cutoff` path is unchanged. This unit only fixes the non-bust-cutoff path's silent-poison failure mode.

**Test scenarios**:
- sync-trustpilot on a domain with > 10 pages, no `--bust-cutoff`: stops at page 10, emits `trustpilot_filter_cap_hit_at_page_10`, exits 0.
- Immediately re-run `sync-trustpilot --resume`: detects the unpoisoned cursor at page 10 with totalPages still set correctly from the last good page, and the resume behavior is sensible (either no-op with a notice, or routes the user to `--bust-cutoff`).
- sync-trustpilot with `--bust-cutoff` on the same domain: iterates stars=1..5, no cap notice on the final completion (each star bucket completed cleanly).
- SaveCursor unit test: calling SaveCursor with `lastPage=11, totalPages=0` returns the sentinel error and does not modify the row.

**Verification**: the exact regression from Thread 2 ("Sync only fetched 1 page and got 0 results - looks like the cursor or API hit a weird state") cannot reproduce.

---

### U5. auth login defaults to chrome; transport distinguishes 404 stale-buildid from 404 filter-unsupported

**Goal**: Two separate UX fixes, both about misleading errors that send the user down the wrong path.

**Requirements**: T2-4, T2-5 (2 × P1)

**Dependencies**: U2 (typed errors propagate into meta.notices on failure)

**Files**:
- internal/cli/auth.go (modify)
- internal/trustpilot/transport.go (modify)

**Approach**:

*Part A - auth login default*:
- Remove the gate that requires `--chrome` to be explicitly set. Default to chrome harvest.
- Keep `--chrome` flag as a no-op for backwards compatibility with any agent scripts that set it.
- Update Long description so the help text matches the new default.

*Part B - 404 disambiguation*:
- In `Client.FetchPage`, when HTTP 404 is returned, read the body and check if it parses as a Next.js soft-redirect envelope (`{"pageProps": {"__N_REDIRECT": "...", "__N_REDIRECT_STATUS": 308}}`). Note: 404 + redirect body would be unusual; the actual failure mode for `--date last3months` was likely a 308 redirect that the current `--page` fix handles. Investigate whether `--date last3months` returns 404 (build id) OR a redirect (filter not allowed in JSON-API) before finalizing the error type.
- Define a new typed error `FilterUnsupportedError` for the "filter param triggered a redirect" case.
- Update `fetchPageWithRetry` to NOT re-harvest the session when it sees `FilterUnsupportedError` (re-harvest will not fix it).
- Update error message: "filter parameter '<name>=<value>' is not supported by the Trustpilot JSON API; try omitting it or using --local on synced data."

**Patterns to follow**:
- The existing `CookieExpiredError` and `BuildIDStaleError` types in transport.go. The new `FilterUnsupportedError` follows the same shape.

**Test scenarios**:
- `auth login` (no flag) launches Chrome and harvests. Same effect as `auth login --chrome`.
- `auth login --chrome` still works.
- `auth login --foo` returns an "unknown flag" error.
- `reviews-fetch <domain> --date last3months` (the regression): emits `FilterUnsupportedError` with the helpful message, does NOT advise `auth login`.
- Genuinely stale build id (manually set the persisted buildId to garbage): emits `BuildIDStaleError`, advises re-auth.
- `reviews-fetch --date last30days` (a date window we know works): succeeds. We use this case to verify the disambiguation does not over-trigger.

**Verification**: agents that hit a filter-param error are told the truth about the cause and given a workable next step.

---

### U6. doctor cache check reports actual local-store state

**Goal**: `trustpilot-pp-cli doctor` reports `Cache: ok (N reviews, M companies, last synced <ts>)` after a successful sync, instead of "Cache: unknown; sync_state is empty."

**Requirements**: T1-2, T1-3, T1-6 (P1, P1, P2 - all the same root)

**Dependencies**: none

**Files**:
- internal/cli/tp_doctor.go (NEW - if the generated doctor exposes a hook)
- internal/cli/doctor.go (NO MODIFY - it is generated)

**Approach**:
- The first 5 minutes of this unit are an investigation, not a build: read `internal/cli/doctor.go` carefully to find the extension point. Three possibilities, in priority order:
  1. The doctor has a registry/hook for per-CLI custom checks (e.g., a slice of CheckFunc that user code can append to in an init()). If yes: add tp_doctor.go with an init() that registers a Trustpilot-specific cache check querying `tp_reviews`, `tp_companies`, `tp_session`, `tp_sync_cursors`.
  2. The doctor has no hook but its functions are exported and can be wrapped. If yes: register a new `doctor-tp` command, or call the wrapping function from the existing doctor command.
  3. Neither - the doctor is fully sealed. If so, this fix belongs in the generator; file as a retro candidate. Plan-side: leave a clear `## Known Gaps` README note explaining the doctor output is generic, and tell users to verify sync via `auth status` + a small SQL query.
- Whichever path applies, the cache check should report: count of companies (`SELECT COUNT(*) FROM tp_companies`), count of reviews, count of distinct synced domains (from `tp_sync_cursors`), and the most-recent `harvested_at` from `tp_session`.

**Patterns to follow**:
- Whatever pattern the existing `tp_doctor.go`-equivalent in other Printing Press CLIs uses (if any). If no precedent, this CLI sets it.

**Test scenarios**:
- After `sync-trustpilot www.thriftbooks.com --max-pages 3`: `doctor` reports `Cache: ok (60 reviews, 1 company, 1 cursor)`.
- Fresh install, before any sync: `Cache: empty (0 reviews; run 'sync-trustpilot <domain>' to hydrate)`.
- Session present but no reviews: `Cache: empty (0 reviews; session ok, harvested <ts>)`.

**Verification**: the exact regression in Thread 1 ("doctor says Cache: unknown even after a successful sync") cannot reproduce.

**Retro candidate flag**: if path (3) is the answer, file a `/printing-press-retro` to add a per-CLI doctor extension hook to the generator. The right home for this fix is the machine, not every printed CLI.

---

### U7. agent-browser cold-start retry

**Goal**: the first invocation on a fresh shell does not require a manual `agent-browser open <url>` warmup.

**Requirements**: T1-4 (P1)

**Dependencies**: none

**Files**:
- internal/trustpilot/browser.go (modify)

**Approach**:
- In `runAgentBrowser`, detect "Connection refused" in stderr or in the returned error message.
- On match, run a warmup call: `agent-browser open https://www.trustpilot.com` (a no-op page that the daemon will resolve quickly), then retry the original eval call once.
- If the retry also fails, surface a clear error: "agent-browser daemon is not responding; try running `agent-browser open https://www.trustpilot.com` manually."
- Do NOT loop indefinitely; one retry is the contract.

**Patterns to follow**:
- The existing `runAgentBrowser` already swallows non-zero exit codes from agent-browser. The retry slots in after the first failed call.

**Test scenarios**:
- agent-browser daemon is not running, fire `auth login`: succeeds after the silent warmup + retry.
- agent-browser daemon is already running, fire `auth login`: succeeds on first try, no retry overhead.
- agent-browser binary is missing entirely: the existing `exec.LookPath` check catches it first, no change to that path.
- agent-browser daemon returns a different error (e.g., Chrome not installed): no retry, surface the original error.

**Verification**: a fresh shell can run `trustpilot-pp-cli auth login` (or any command that triggers auto-harvest) without manual daemon warmup.

---

### U8. histogram parser drift detection

**Goal**: when `parseHistogram` receives keys it does not recognize, the CLI emits a `histogram_empty_from_api` (or a more specific) notice instead of silently returning an empty map.

**Requirements**: T2-2 (P0)

**Dependencies**: U2 (notice mechanism)

**Files**:
- internal/trustpilot/parser.go (modify)
- internal/cli/info.go (modify - check the returned histogram and add notice)
- internal/cli/agent_bundle.go (modify - same)

**Approach**:
- `parseHistogram` already handles `"fiveStars"`, `"5"` etc. Add a second return value: `unknownKeys []string` listing input keys that did not match any case.
- info.go and agent_bundle.go check the result. If the input map was non-empty but the output is empty, emit `histogram_keys_unrecognized` with the unknown keys in the notice payload. If both input and output are empty, emit `histogram_empty_from_api` (the API genuinely returned no histogram).
- This turns "we don't know if the parser is broken" into "we have a structured signal we can grep logs for and fix."

**Patterns to follow**:
- The `default` case in parseHistogram's switch statement already exists; replace the `_ = strings.TrimSpace` no-op with appending to the unknownKeys slice.

**Test scenarios**:
- Hypothetical API response with key `"5stars"` (not in the existing switch): parseHistogram returns empty map + unknownKeys=`["5stars"]`. info.go's notice list includes `histogram_keys_unrecognized`.
- juneoven.com response (where histogram was empty in testing): determine whether the cause is empty input or unrecognized keys, and the notice tells us which.
- thriftbooks.com response (where histogram was empty in testing despite having reviews): same - the notice clarifies whether it's parser drift or API behavior.
- Healthy response with all five keys: parseHistogram returns the full map, no notice.

**Verification**: every "empty histogram" outcome is now explicable from the meta envelope. If it is parser drift, the unknown-keys list tells us what to add.

---

## System-Wide Impact

- All read commands gain a `meta` object in JSON output. The SKILL.md must be updated to document the meta envelope, the notices enum, and the dual-mode behavior of `search-reviews`. Agents that hard-coded the existing JSON shape will keep working (additive change); agents that consume the new meta envelope get strictly better behavior.
- The `tp_doctor.go` (if path 1 applies) is a new pattern that future Trustpilot-shaped CLIs will follow. The generator-side hook (if added via retro) becomes a cross-CLI win.
- README needs three small additions: the pagination cap, the agent-browser cold-start note (only if we cannot make U7 fully silent), and the `--deliver file:` 0600 mode note.

## Risks and Mitigations

1. **Risk**: meta envelope changes break existing agent integrations.
   - **Mitigation**: meta is additive (new top-level key). Existing payload keys are unchanged. The new top-level `isCollectingReviews` field on agent-bundle and top-recent is also additive.
2. **Risk**: U5's 404 disambiguation over-triggers and treats real build-id-stale errors as filter problems.
   - **Mitigation**: the test scenario list includes both cases. The disambiguation is keyed on the actual `__N_REDIRECT` JSON shape, not a heuristic.
3. **Risk**: U6 path (3) - doctor is fully sealed, fix needs to land in the generator.
   - **Mitigation**: documented as a retro candidate up front. The printed-CLI work in this plan does not block on that. A `## Known Gaps` README note covers the temporary state.
4. **Risk**: U7's silent retry hides agent-browser problems we should be telling the user about.
   - **Mitigation**: retry is bounded to one. If retry fails, the error is surfaced clearly. The expected case is "first call after fresh shell" which is a transient daemon-startup race, not a user-actionable problem.

## Deferred Questions

- Should `meta.notices` be a flat array of strings, or a structured array of `{code, message, detail}` objects? This plan uses flat strings for v1 to keep the JSON small for agents using `--select meta.notices`. Revisit if we need machine-readable detail (e.g., the unknown-keys list from U8 needs a place to live; for now, encode in the notice string itself).
- Whether `--data-source local` returning empty should exit 0 (current direction) or exit 2. Exit 0 makes agent loops simpler; exit 2 makes shell scripts louder. Going with 0 for v1; revisit on usage feedback.

## Verification (whole-plan)

- A re-run of the Thread 1 ThriftBooks workflow does not crash, doctor reports a real cache state, and the pagination cap is surfaced when hit.
- A re-run of the Thread 2 June Oven workflow returns non-null good/bad even on a fresh shell, the frozen-collection state is visible at the top level of every relevant payload, the auth login command works without `--chrome`, and the `--date last3months` error message tells the user the actual cause.
- `printing-press shipcheck` still passes 6/6 legs. Scorecard delta target: +0 to +6 (the cache-correctness fix lifts Cache Freshness from 5/10).
