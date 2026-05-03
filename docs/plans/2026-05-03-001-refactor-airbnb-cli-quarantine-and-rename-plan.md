---
title: "refactor: airbnb CLI bug fixes, VRBO quarantine, and rename"
type: refactor
status: active
date: 2026-05-03
---

# refactor: airbnb CLI bug fixes, VRBO quarantine, and rename

**Target repo:** mvanhorn/printing-press-library
**Working directory:** `library/travel/airbnb-vrbo/` (will become `library/travel/airbnb-pp/` in this PR)

## Summary

One PR that quarantines VRBO at every CLI entry point (keeping the source code in tree for future re-enablement when Akamai is solved), fixes the ~19 bugs surfaced in the launch dogfood session, and renames the CLI from `airbnb-vrbo-pp-cli` to `airbnb-pp-cli` with config/cache/env-var migration so existing users aren't bricked.

---

## Problem Frame

The CLI shipped in PR #217 (`feat(airbnb-vrbo): add airbnb-vrbo`). The first dogfood session exposed two structural issues and many surface bugs:

- VRBO is non-functional and lies about it. `vrbo-listing search "<any city>"` returns the same six hardcoded Tahoe AvantStay listings with the queried city stamped onto each result — a fallback path for an Akamai bot challenge that produces fake data labeled as if it were real. `cheapest` and `match` against VRBO URLs surface raw 429 HTML through the error path.
- `cheapest` silently no-ops when `--search-backend` is omitted. The default backend selection does not fire, so the host-direct arbitrage feature returns `null` candidates even when a confidence-1.0 host match exists. Passing `--search-backend ddg` explicitly produces correct results.
- A long tail of secondary bugs makes the agent-mode contract unreliable: `--select` silently drops bogus paths, `--csv` emits JSON, `--dry-run` shows almost nothing, date validation is missing, `--property-type entire_home` returns zero results, `feedback list` returns mock data, `agent-context` doesn't expose the search backend, `host portfolio` doesn't populate from the same host extraction that `cheapest` just performed, the `plan` command times out instead of returning partial results, and `cheapest` is non-deterministic across runs.

The "vrbo" in the binary name advertises a capability the CLI cannot deliver. Renaming now (while VRBO is quarantined) lets the surface match reality without locking the door against future re-enablement — the VRBO source layer stays in `internal/source/vrbo/` so a future PR can flip it back on once an Akamai workaround exists.

---

## Requirements

- R1. VRBO is unreachable from every user-facing CLI entry point and returns a clear "temporarily disabled" message — never a hardcoded fallback, never fake data, never a raw 429 HTML dump.
- R2. The VRBO source code (`internal/source/vrbo/` and any VRBO branches in cross-platform commands) remains in the tree so future re-enablement is a flag flip plus the Akamai workaround.
- R3. `cheapest` defaults to the DuckDuckGo backend when no `--search-backend` is set and no paid-backend key is configured, and falls back to the next available backend when the selected one errors.
- R4. Date validation rejects checkout-on-or-before-checkin and check-in dates in the past with a clear error and non-zero exit.
- R5. `--property-type` accepts only the four documented values (`entire_home`, `private_room`, `shared_room`, `hotel_room`); unknown values exit non-zero. The `entire_home` value actually returns entire-home listings.
- R6. Invalid Airbnb listing IDs return exit 3 with a "listing not found" message instead of exit 0 with `"Airbnb deferred state script not found"`.
- R7. `--select` warns or errors on field paths that match nothing in the response, instead of returning empty objects.
- R8. `--csv` emits real CSV for table-shaped commands, or the flag is removed from help where it cannot be honored.
- R9. `--dry-run` shows URL, method, query parameters, and headers being sent — enough to debug a request without executing it.
- R10. `feedback list` reads the actual local feedback file and never returns mock data. `agent-context` surfaces the active search backend and the current set of saved profiles.
- R11. `host portfolio` returns listings for hosts the CLI has previously extracted via `cheapest` or other commands (the local store is fed by host extraction).
- R12. The `plan` command isolates per-listing timeouts and returns successful listings with a tagged failure list, instead of failing the whole call when one listing times out.
- R13. `cheapest` returns identical totals across runs for the same listing and dates within a cache window.
- R14. The CLI is renamed end-to-end: `airbnb-vrbo-pp-cli` → `airbnb-pp-cli`, `airbnb-vrbo-pp-mcp` → `airbnb-pp-mcp`, source dir `library/travel/airbnb-vrbo` → `library/travel/airbnb-pp`, env-var prefix `AIRBNB_VRBO_*` → `AIRBNB_PP_*`, skill `pp-airbnb-vrbo` → `pp-airbnb`. Existing users running the old binary against an existing config do not lose state — config and cache directories migrate on first run.
- R15. The README, SKILL.md, manifest.json, .printing-press.json, and .goreleaser.yaml all reflect the new name and the VRBO-disabled posture.

---

## Scope Boundaries

- This PR does not solve the VRBO Akamai bot challenge. VRBO stays quarantined; the workaround is a separate future PR.
- This PR does not switch the default search backend to a paid provider (Parallel, Brave, Tavily). DDG remains the default.
- This PR does not add new commands or new platforms. It is correctness, hygiene, and rename only.
- This PR does not refactor the Airbnb scrape strategy. Airbnb works; leave it.
- This PR does not backfill tests for surfaces it does not change. New coverage targets only the changed code paths.

### Deferred to Follow-Up Work

- VRBO Akamai workaround (cookie warmup, headed browser proxy, or paid scraper) — separate future PR that flips the quarantine flag.
- A full agent-mode integration test suite that exercises every flag combination — out of scope for this PR; existing test surface stays as-is.
- Migration of `pp-airbnb` skill to a discoverable place outside `library/travel/` if directory taxonomy changes (cosmetic; do separately).

---

## Context & Research

### Relevant Code and Patterns

- `library/travel/airbnb-vrbo/cmd/airbnb-vrbo-pp-cli/main.go` — binary entry point.
- `library/travel/airbnb-vrbo/cmd/airbnb-vrbo-pp-mcp/` — MCP server entry point.
- `library/travel/airbnb-vrbo/internal/cli/` — Cobra command definitions. Files of interest:
  - `cheapest.go`, `compare.go`, `match.go`, `plan.go`, `find_twin.go` — host-direct arbitrage commands.
  - `vrbo_listing.go`, `vrbo_listing_search.go`, `vrbo_listing_get.go` — VRBO entry points to disable.
  - `airbnb_listing_search.go`, `airbnb_listing_get.go` — Airbnb commands; date validation and property-type enum live here.
  - `feedback.go` — feedback writer + lister; mock data reproduction lives here.
  - `agent_context.go` — discovery surface; needs `search_backend` and live `available_profiles`.
  - `host_extract.go`, `host_portfolio.go` — host extraction and portfolio query.
  - `helpers.go` — `--select`, `--csv`, `--dry-run`, `--deliver` plumbing.
  - `root.go` — global flag registration and default values.
- `library/travel/airbnb-vrbo/internal/searchbackend/` — backend implementations: `brave/`, `duckduckgo/`, `parallel/`, `tavily/`. The `searchbackend.go` aggregator selects which backend to use; the bug is in this selection layer, not in DDG itself.
- `library/travel/airbnb-vrbo/internal/source/vrbo/` — VRBO source layer. `client.go` contains the Akamai 429 path and the hardcoded fallback. Keep this file present but make it unreachable from CLI commands.
- `library/travel/airbnb-vrbo/internal/store/` — local SQLite. Host portfolio reads from here; host extraction needs to write here.
- `library/travel/airbnb-vrbo/internal/config/` — config loader; gets new env-var prefix and config-dir migration.
- `library/travel/airbnb-vrbo/internal/cache/` — cache layer; `cheapest` non-determinism likely traces here.
- `library/travel/airbnb-vrbo/.printing-press.json` — printing-press tool's regeneration manifest. The rename has to be coordinated with this file (it carries `cli_name`, `mcp_binary`, etc.).
- `library/travel/airbnb-vrbo/SKILL.md` and the `pp-airbnb-vrbo` skill at `cli-skills/pp-airbnb-vrbo/` — both rename together.

### Institutional Learnings

- `feedback recorded locally (483 chars)` followed by `feedback list` returning `text: "mock-value"` rows means the writer and reader paths are in different stores. Find both, point them at the same path.
- The CLI is a printed Printing Press CLI — the spec, manifest, and skill files are partially generated. Renaming requires either regenerating with the PP tool or hand-editing every generated file consistently. Confirm during implementation which approach the maintainer wants.

### External References

- Not used for this plan. The bugs are diagnosed; the rename is a known refactor pattern; no external docs needed.

---

## Key Technical Decisions

- **Quarantine, don't delete.** Every VRBO entry point in the CLI returns a "VRBO temporarily disabled" structured error. The `internal/source/vrbo/` package stays compiled in (so it doesn't bit-rot) but is unreachable from any user-facing command. This preserves the future-re-enable path without shipping known-broken behavior.
- **Remove the fake fallback.** The `fallback_after_vrbo_challenge` path that returns hardcoded Tahoe data with the queried city stamped on top is deleted. Fake results that lie about location are worse than failing. Akamai's 429 response is caught and surfaced as a structured error referring to the quarantine status.
- **DDG is the default backend, full stop.** When `--search-backend` is unset and no Parallel/Brave/Tavily key is configured, DDG runs. When a key is configured, the configured backend runs and DDG is the fallback if it errors. The selection lives in `internal/searchbackend/searchbackend.go`; the default logic is moved out of per-command code.
- **Config and cache dirs migrate, env vars dual-read.** First run of the renamed binary detects `~/.config/airbnb-vrbo-pp-cli/` and `~/.local/share/airbnb-vrbo-pp-cli/` and renames them. Env vars read both `AIRBNB_PP_*` (preferred) and `AIRBNB_VRBO_*` (legacy) for one release. A deprecation warning fires when only the legacy form is present.
- **The rename is one PR, not two.** Splitting rename from bug fixes would force two rounds of generated-file regeneration and dual messaging in the changelog. Bundle it.
- **`--csv` either works or comes out of help.** Honor the flag for table-shaped commands or remove it. Decide at implementation time which commands have a meaningful CSV shape; document the decision in the unit's PR commit.
- **`host extract` writes through to the local store.** When `cheapest` extracts a host, the local store records the host → listing relationship. `host portfolio` reads from this store. This is the source of truth that the CLI's "compounds over time" pitch already promises.

---

## Open Questions

### Resolved During Planning

- **Should the rename ship in this PR or a follow-up?** Yes, in this PR. The synthesis confirmed it; splitting introduces churn.
- **Should the fake VRBO fallback be disabled-but-kept or deleted?** Deleted. Fake data is a worse failure mode than honest absence.
- **Should `--search-backend` selection logic live in each command or in the backend package?** Backend package — current per-command duplication is the bug.

### Deferred to Implementation

- **Whether `--csv` is feasible for `airbnb-listing search` given its nested response shape.** Implementer decides during U4 whether to honor the flag or pull it from help.
- **Exact env-var migration semantics — error, warn, or silent dual-read?** Implementer decides during U7 based on how noisy the legacy users are likely to be.
- **Whether the printing-press tool can regenerate the renamed CLI from spec or whether the rename is hand-applied.** Implementer probes the PP tool during U7; if regeneration works, prefer it.
- **Cache key shape for deterministic `cheapest`.** Implementer designs the key during U2; needs to include backend identity, listing ID, dates, and the cache TTL.

---

## Implementation Units

- U1. **Quarantine VRBO at every entry point**

**Goal:** No CLI command silently calls VRBO. Every VRBO-touching code path returns a structured "VRBO temporarily disabled" error. Source code remains intact.

**Requirements:** R1, R2

**Dependencies:** None

**Files:**
- Modify: `library/travel/airbnb-vrbo/internal/cli/vrbo_listing.go`
- Modify: `library/travel/airbnb-vrbo/internal/cli/vrbo_listing_search.go`
- Modify: `library/travel/airbnb-vrbo/internal/cli/vrbo_listing_get.go`
- Modify: `library/travel/airbnb-vrbo/internal/cli/cheapest.go` (VRBO branch)
- Modify: `library/travel/airbnb-vrbo/internal/cli/match.go` (VRBO branch)
- Modify: `library/travel/airbnb-vrbo/internal/cli/plan.go` (VRBO leg)
- Modify: `library/travel/airbnb-vrbo/internal/cli/find_twin.go` (if it touches VRBO)
- Modify: `library/travel/airbnb-vrbo/internal/source/vrbo/client.go` (delete `fallback_after_vrbo_challenge` path)
- Test: `library/travel/airbnb-vrbo/internal/cli/vrbo_quarantine_test.go` (new)

**Approach:**
- Introduce a single `vrbo.Disabled()` error sentinel returned by a thin façade, so the user-facing message is consistent across commands.
- Each VRBO-fronted Cobra command calls the façade before any HTTP work and exits with the structured error. Exit code follows the existing convention used for "auth not configured" (exit 4 or a new dedicated code — implementer picks).
- `cheapest`'s output for the VRBO option changes from `{source: "vrbo", note: "not searched (single-platform mode)"}` (misleading) to `{source: "vrbo", note: "vrbo temporarily disabled — pending Akamai workaround"}`.
- Delete the `fallback_after_vrbo_challenge` branch in the VRBO source client. When the Akamai 429 fires, return a typed error wrapped through the same disabled-sentinel; do not synthesize fake results.
- `match` returns no VRBO twins when called from an Airbnb URL; the result envelope clearly says VRBO matching is disabled rather than silently returning zero matches.

**Patterns to follow:**
- Existing error-envelope pattern from `internal/cli/auth.go` for "not configured" errors.

**Test scenarios:**
- Happy path: `airbnb-pp-cli vrbo-listing search "anywhere" --agent` returns exit 4 (or chosen code) with structured error containing "vrbo temporarily disabled".
- Happy path: `airbnb-pp-cli vrbo-listing get 9076001848 --agent` returns the same disabled error (also fixes the `[id]` positional bug by accepting it before the disabled error fires; even disabled, the arg parser shouldn't reject the positional).
- Happy path: `airbnb-pp-cli cheapest <airbnb-url> --agent` succeeds with `options[].source=="vrbo"` carrying the disabled note instead of "not searched (single-platform mode)".
- Edge case: `airbnb-pp-cli cheapest <vrbo-url> --agent` exits non-zero with the disabled error, never with raw 429 HTML in the error body.
- Edge case: `airbnb-pp-cli match <airbnb-url> --agent` returns zero VRBO matches with explicit disabled annotation in the result envelope.
- Error path: `airbnb-pp-cli plan "Lake Tahoe" --agent` includes Airbnb results and a VRBO-disabled note in the response, never falls back to fake data.
- Integration: ensure `internal/source/vrbo/` package still compiles (the source code is preserved, only entry points are quarantined).

**Verification:**
- Grep for `fallback_after_vrbo_challenge` returns zero results in source.
- Every `internal/cli/vrbo_*.go` and the VRBO branch of every cross-platform command goes through the disabled sentinel before any HTTP call.
- `vrbo-listing search` for any city no longer returns the six canned Tahoe results.

---

- U2. **Fix `cheapest` search-backend default and determinism**

**Goal:** `cheapest` returns correct host-direct candidates without `--search-backend` being passed, and returns the same totals across repeated runs within a cache window.

**Requirements:** R3, R13

**Dependencies:** None

**Files:**
- Modify: `library/travel/airbnb-vrbo/internal/searchbackend/searchbackend.go`
- Modify: `library/travel/airbnb-vrbo/internal/cli/cheapest.go`
- Modify: `library/travel/airbnb-vrbo/internal/cache/` (cache key construction for cheapest results)
- Test: `library/travel/airbnb-vrbo/internal/searchbackend/searchbackend_test.go` (extend existing)
- Test: `library/travel/airbnb-vrbo/internal/cli/cheapest_test.go` (new)

**Approach:**
- Move the "which backend?" decision into `searchbackend.Select(opts)` — single source of truth. Order of preference: explicit `--search-backend` flag > env-var-configured backend > DDG default.
- When the selected backend errors (rate-limited, network, parse failure), the selector falls back to the next available backend in the same call, surfacing the failure in a `meta.fallback` field rather than swallowing it.
- For determinism: cache `cheapest` results keyed by (listing-id, checkin, checkout, backend-identity) for the existing cache TTL window. The non-determinism observed (`416` vs `297` for the same listing) traces to mixing cached and live components in a single response — make the response atomic per cache key.
- `cheapest`'s help and `agent-context` reflect the resolved active backend.

**Patterns to follow:**
- Existing cache pattern in `internal/cache/` for Airbnb listing detail.
- Existing flag-vs-env precedence pattern from `internal/config/`.

**Test scenarios:**
- Happy path: `cheapest <airbnb-url>` without `--search-backend` selects DDG and returns non-empty candidates for a known live listing (the Lakeland Village #310 case from this session is a good fixture).
- Happy path: `cheapest <airbnb-url> --search-backend tavily` with no Tavily key falls back to DDG and reports the fallback in `meta.fallback`.
- Happy path: same `cheapest` call run twice within the cache window returns identical `cheapest.total`.
- Edge case: `cheapest <airbnb-url>` when DDG returns zero candidates but the host has a real direct site (rare) — should report `cheapest: null` with `meta.reason: "no_results"`, not fall back silently.
- Error path: when all backends fail, return a structured error naming each backend's failure mode rather than empty `cheapest: null`.
- Integration: `cheapest` followed immediately by `host portfolio <extracted-host>` returns the listing the cheapest call extracted (cross-unit integration covered with U5).

**Verification:**
- Removing the `--search-backend ddg` workaround reproduces the same successful output observed in the dogfood session.
- Two consecutive `cheapest` calls produce byte-identical `cheapest.total` and `cheapest.url` fields.

---

- U3. **Input validation: dates, property-type, invalid-id exit code**

**Goal:** Bad inputs error early with a clear message and a non-zero exit code instead of silently returning empty or misleading results.

**Requirements:** R4, R5, R6

**Dependencies:** None

**Files:**
- Modify: `library/travel/airbnb-vrbo/internal/cli/airbnb_listing_search.go`
- Modify: `library/travel/airbnb-vrbo/internal/cli/airbnb_listing_get.go`
- Modify: `library/travel/airbnb-vrbo/internal/cli/cheapest.go`, `compare.go`, `plan.go` (any command that takes checkin/checkout)
- Modify: `library/travel/airbnb-vrbo/internal/source/airbnb/extract.go` (the "Airbnb deferred state script not found" error string)
- Modify: `library/travel/airbnb-vrbo/internal/cli/helpers.go` (shared date-validation helper)
- Test: `library/travel/airbnb-vrbo/internal/cli/validation_test.go` (new)

**Approach:**
- Add a `validateDates(checkin, checkout)` helper that errors when checkout ≤ checkin or checkin is in the past (relative to local date). Wire it into every command that takes the pair.
- Replace the silent enum acceptance for `--property-type` with explicit validation against the documented set: `entire_home`, `private_room`, `shared_room`, `hotel_room`. Unknown values exit with usage error (exit 2). Investigate why `entire_home` returns zero results — likely the wrong filter param name is being sent to Airbnb's SSR endpoint; fix the mapping during this unit.
- For invalid Airbnb IDs: catch the "Airbnb deferred state script not found" condition in `internal/source/airbnb/extract.go`, translate to a structured `ErrListingNotFound`, return exit 3 from the CLI layer with a "listing not found" human message.

**Patterns to follow:**
- Existing usage-error pattern (exit 2) used by Cobra's flag parsing.
- Existing exit-3-for-not-found used elsewhere in the CLI per the documented exit-code table.

**Test scenarios:**
- Happy path: `airbnb-listing search SF --checkin 2026-05-06 --checkout 2026-05-09` succeeds.
- Edge case: `--checkin 2026-05-09 --checkout 2026-05-06` exits non-zero with "checkout must be after checkin".
- Edge case: `--checkin 2025-01-01 --checkout 2025-01-04` exits non-zero with "checkin date is in the past".
- Edge case: `--checkin 2026-05-06 --checkout 2026-05-06` exits non-zero with "checkout must be after checkin".
- Happy path: `--property-type entire_home` returns at least one entire-home listing for SF (the bug fix proof).
- Error path: `--property-type bogus` exits 2 with "unknown property-type 'bogus'; expected one of entire_home, private_room, shared_room, hotel_room".
- Error path: `airbnb-listing get 99999999999999999` exits 3 with "listing not found", not exit 0 with a deferred-state error.
- Integration: a cheapest call with invalid dates errors at the validation layer before any HTTP call (verified by `--dry-run` not firing).

**Verification:**
- Every documented exit code in the README's exit-code table now matches actual behavior for the invalid cases.
- The `entire_home` filter returns non-empty for a city with known entire-home availability.

---

- U4. **Output flag fidelity: `--select`, `--csv`, `--dry-run`, `--deliver`**

**Goal:** Documented output flags do what their help says or are removed from help.

**Requirements:** R7, R8, R9

**Dependencies:** None

**Files:**
- Modify: `library/travel/airbnb-vrbo/internal/cli/helpers.go` (`--select` and `--dry-run` plumbing)
- Modify: `library/travel/airbnb-vrbo/internal/cli/deliver.go` (`--deliver` semantics)
- Modify: `library/travel/airbnb-vrbo/internal/cli/root.go` (global flag registration)
- Test: `library/travel/airbnb-vrbo/internal/cli/output_flags_test.go` (new)

**Approach:**
- `--select`: when a path matches nothing in the response, log a warning to stderr listing the unmatched paths. Optionally exit non-zero if `--strict` is added (defer the strict variant to a follow-up). Today's behavior of returning `[{}, {}, {}]` is the bug.
- `--csv`: implementer decides per command. For commands with table-shaped responses (e.g., `airbnb-listing search`'s flat result list), emit real CSV with headers from `--select` paths. For commands whose responses are nested in shapes that don't flatten cleanly, remove `--csv` from the per-command help and emit a "csv not supported for this command" error if explicitly passed.
- `--dry-run`: print the URL, HTTP method, query params, and headers (with redacted secrets) that would be sent. Currently returns `{"id": "20591785"}` only; useless.
- `--deliver`: today writes to both stdout and the sink. Add a `:silent` suffix variant (e.g., `--deliver file:/tmp/out.json:silent`) that suppresses stdout. Default behavior of duplicating to both stays for backward compat; document the new variant.

**Patterns to follow:**
- Use the existing `cliutil` field-path resolver for `--select`.
- The `deliver.go` sink registry is the right home for the new silent variant.

**Test scenarios:**
- Happy path: `--select id,title` on `airbnb-listing search` returns objects with only id and title fields.
- Edge case: `--select doesnotexist,results.fakekey` warns on stderr listing both paths and returns the response with no fields removed (or empty objects, depending on implementer choice — but explicit warning, not silent).
- Happy path: `--csv --select id,title` on `airbnb-listing search` emits a CSV with two columns and a header row.
- Error path: `--csv` on `airbnb-listing get` (which returns a deeply nested object) errors with "csv not supported for this command" or omits `--csv` from the command's help entirely.
- Happy path: `--dry-run airbnb-listing get 20591785` shows the URL, method, params, and headers.
- Happy path: `--deliver file:/tmp/x.json:silent airbnb-listing search SF` writes to the file and produces no stdout output.

**Verification:**
- Help text and runtime behavior for each output flag match.

---

- U5. **Telemetry plumbing: `feedback`, `agent-context`, `host portfolio`**

**Goal:** Three discoverability surfaces that lie today now tell the truth.

**Requirements:** R10, R11

**Dependencies:** None (but interacts with U2's host-extraction write path)

**Files:**
- Modify: `library/travel/airbnb-vrbo/internal/cli/feedback.go`
- Modify: `library/travel/airbnb-vrbo/internal/cli/agent_context.go`
- Modify: `library/travel/airbnb-vrbo/internal/cli/host_portfolio.go`
- Modify: `library/travel/airbnb-vrbo/internal/cli/host_extract.go` (write-through)
- Modify: `library/travel/airbnb-vrbo/internal/store/` (host-listing schema if missing)
- Test: `library/travel/airbnb-vrbo/internal/cli/feedback_test.go` (new), `agent_context_test.go` (extend existing), `host_portfolio_test.go` (new)

**Approach:**
- `feedback list` reads from the same path the writer uses (`~/.airbnb-pp-cli/feedback.jsonl` after rename). Remove the mock-data return path entirely. The fact that `text: "mock-value"` shipped means a test fixture leaked into the production read path; find and excise.
- `agent-context` adds `search_backend` (active backend resolved from flag/env/default), `available_profiles` populated dynamically from the profile store at call time (currently stale).
- `host_extract` writes through to the local store with the host → listing-id relationship every time it runs (i.e., as part of `cheapest`, `match`, etc.). `host portfolio` reads from this store. The store schema may need a `host_listings` table — implementer adds if missing.

**Patterns to follow:**
- The store's existing read-write pattern (already used for the `airbnb_wishlist` resource per the doctor output's resource list).

**Test scenarios:**
- Happy path: `airbnb-pp-cli feedback "test"` followed by `feedback list --agent` returns the entry just written.
- Happy path: `feedback list` after a fresh install returns an empty array, never `text: "mock-value"`.
- Happy path: `cheapest <airbnb-url>` extracts host "rnr vacation rentals" → `host portfolio "rnr vacation rentals"` returns at least one listing.
- Happy path: `agent-context` includes `search_backend` in keys with a valid value (`ddg`, `parallel`, `brave`, `tavily`, or `auto`).
- Edge case: `profile save x; profile save y; agent-context` returns both `x` and `y` in `available_profiles`.
- Integration: feedback writer and reader use the same path even when `XDG_CONFIG_HOME` is set non-default.

**Verification:**
- `feedback list` never returns mock data on any platform.
- `host portfolio` returns non-empty for any host the CLI has previously extracted in the same session.
- `agent-context` exposes both `search_backend` and live `available_profiles`.

---

- U6. **`plan` command robustness: per-listing timeout isolation and partial results**

**Goal:** `plan` returns the listings that succeeded plus a tagged failure list, instead of failing the entire call when one listing times out.

**Requirements:** R12

**Dependencies:** U1 (VRBO branch returns disabled, not timeout)

**Files:**
- Modify: `library/travel/airbnb-vrbo/internal/cli/plan.go`
- Test: `library/travel/airbnb-vrbo/internal/cli/plan_test.go` (new)

**Approach:**
- Wrap each per-listing fetch in its own `context.WithTimeout` that does not propagate cancellation to siblings. When one fan-out leg hits the deadline, capture it as a structured failure entry and continue.
- The `plan` response gains a `failures: [{listing_id, source, reason}]` field alongside the existing ranked `results` array. Today it returns `results: []` when any leg fails, hiding partial success.
- The Airbnb-side fan-out and the (now-disabled) VRBO leg both feed into this aggregator. The disabled VRBO leg appears in `failures` with `reason: "vrbo_disabled"`.

**Patterns to follow:**
- Existing concurrent fan-out elsewhere in the CLI for the cheapest-on-multiple-listings scenario.

**Test scenarios:**
- Happy path: `plan "San Francisco" --checkin <future> --checkout <future+3>` returns ranked results from Airbnb plus a `failures` entry for VRBO disabled.
- Edge case: when all Airbnb fetches succeed, `failures` contains only the VRBO-disabled entry.
- Edge case: when one of the per-listing `cheapest` legs times out, that listing appears in `failures` with reason "timeout"; other listings still appear in `results`.
- Error path: when every leg fails, `results` is empty and `failures` lists every failure. Exit non-zero only when zero listings succeeded.
- Integration: a slow upstream that exceeds the per-leg timeout doesn't cause the whole call to fail.

**Verification:**
- The dogfood session repro (`plan "Lake Tahoe"` returning empty with a context-deadline-exceeded warning) now returns at least the listings that did fetch.

---

- U7. **Rename: binary, MCP, dir, env vars, skill — with config/cache migration**

**Goal:** The CLI ships under the new name end-to-end. Existing users migrate transparently on first run.

**Requirements:** R14

**Dependencies:** U1–U6 (rename last so a scoped revert of just U7 is possible if review wants the rename split)

**Files:**
- Rename directories and files:
  - `library/travel/airbnb-vrbo/` → `library/travel/airbnb-pp/`
  - `library/travel/airbnb-pp/cmd/airbnb-vrbo-pp-cli/` → `library/travel/airbnb-pp/cmd/airbnb-pp-cli/`
  - `library/travel/airbnb-pp/cmd/airbnb-vrbo-pp-mcp/` → `library/travel/airbnb-pp/cmd/airbnb-pp-mcp/`
- Modify: `library/travel/airbnb-pp/go.mod` (module path)
- Modify: `library/travel/airbnb-pp/manifest.json` (`name`, `display_name`, `server.entry_point`, `server.mcp_config.command`)
- Modify: `library/travel/airbnb-pp/.printing-press.json` (`api_name`, `cli_name`, `mcp_binary`, `display_name`)
- Modify: `library/travel/airbnb-pp/.goreleaser.yaml`
- Modify: `library/travel/airbnb-pp/Makefile`
- Modify: `library/travel/airbnb-pp/spec.yaml`
- Modify: `library/travel/airbnb-pp/internal/config/` (env-var prefix; dual-read for legacy)
- Modify: `library/travel/airbnb-pp/internal/cliutil/` or wherever config-dir paths are resolved (config-dir migration on first run)
- Rename: `cli-skills/pp-airbnb-vrbo/` → `cli-skills/pp-airbnb/` (and update SKILL.md frontmatter `name:`)
- Modify: every internal reference to the old name in source comments, log lines, error strings, help text
- Test: `library/travel/airbnb-pp/internal/config/migration_test.go` (new)

**Approach:**
- Use grep across the directory to enumerate every occurrence of `airbnb-vrbo`, `airbnb_vrbo`, `AIRBNB_VRBO`, and `pp-airbnb-vrbo`. Rename consistently. The PP tool's regeneration is the cleaner path if it works for already-published CLIs — implementer probes during this unit; if it can't handle an in-place rename, hand-edit and check the result against `bun run release:validate` (or the PP equivalent).
- Config-dir migration: on first run, if `~/.config/airbnb-pp-cli/` does not exist and `~/.config/airbnb-vrbo-pp-cli/` does, rename it. Same for `~/.local/share/`. Log a one-time "migrated config from old name" line to stderr.
- Env vars: read `AIRBNB_PP_*` first, then fall back to `AIRBNB_VRBO_*` with a deprecation warning printed to stderr once per process. Document the deprecation in the README and announce removal in two releases.
- Skill rename: update the `cli-skills/pp-airbnb/SKILL.md` frontmatter `name: pp-airbnb`, update install/MCP installation snippets to point to the new module path. The SKILL.md is co-located with the CLI dir so it ships in the same PR.
- Update README.md with a top section explaining the rename, the migration, and the `pip install` / `go install` commands for the new module path.

**Patterns to follow:**
- Existing PP CLI structure under `library/<category>/<name>/` — match exactly.

**Test scenarios:**
- Happy path: a fresh install of `airbnb-pp-cli` runs `doctor` and reports OK with the new config dir at `~/.config/airbnb-pp-cli/`.
- Happy path: a user with an existing `~/.config/airbnb-vrbo-pp-cli/` runs `airbnb-pp-cli doctor` for the first time and the dir is renamed in place; the second run finds it at the new name.
- Edge case: a user with both old and new dirs (perhaps from a partial migration) runs the CLI; new dir wins, old dir is left intact, a warning is printed.
- Edge case: `AIRBNB_VRBO_FEEDBACK_ENDPOINT=...` is set but `AIRBNB_PP_FEEDBACK_ENDPOINT` is not; the CLI uses the legacy var and warns once.
- Happy path: `go install github.com/mvanhorn/printing-press-library/library/travel/airbnb-pp/cmd/airbnb-pp-cli@latest` produces a working binary named `airbnb-pp-cli`.
- Happy path: `claude mcp add airbnb-pp-mcp -- airbnb-pp-mcp` registers the renamed MCP server.
- Error path: `go install` against the old module path fails (the old path is gone) — release notes call out the new path.
- Integration: every test from U1–U6 still passes after the rename (paths in test files updated).

**Execution note:** Run the full test suite after the rename to catch any string-literal references to the old name that hand-edits missed. The PP regeneration path, if used, may overwrite hand-edits to generated files — confirm before relying on it.

**Verification:**
- `grep -r "airbnb-vrbo" library/travel/airbnb-pp/` returns only intentional references (e.g., legacy migration code paths, deprecation notices).
- `bun run release:validate` (or PP equivalent) passes.
- A fresh user can install and run the CLI under the new name without touching the legacy name.

---

- U8. **Documentation and manifest sync**

**Goal:** README, SKILL.md, and any cross-referencing docs in the repo reflect the renamed CLI and the VRBO-disabled posture.

**Requirements:** R15

**Dependencies:** U1, U7 (need final messaging from U1 and final names from U7)

**Files:**
- Modify: `library/travel/airbnb-pp/README.md`
- Modify: `library/travel/airbnb-pp/SKILL.md`
- Modify: `cli-skills/pp-airbnb/SKILL.md` (the user-installed skill — same content, possibly different format)
- Modify: top-level `library/travel/README.md` if it lists CLIs
- Modify: any `docs/` cross-references
- Test: `Test expectation: none -- documentation-only changes; covered by U1–U7 behavioral tests.`

**Approach:**
- Open with the rename announcement and migration steps. Note the VRBO quarantine prominently — users coming from the old surface need to see "VRBO is temporarily disabled" before they get confused by an error.
- Replace every code block install command, every command example using the old binary name, every config-dir reference.
- The "Unique Capabilities" section keeps the VRBO-related capabilities listed (match cross-platform, plan), but each entry is annotated with "(VRBO leg currently disabled — pending Akamai workaround)".
- The freshness contract paragraph stays as-is since `airbnb_wishlist` is the only currently-covered resource.
- Update the agent-mode example commands to use the new binary name.

**Verification:**
- README example commands run successfully (or error gracefully where they intentionally hit a quarantined surface).
- No SKILL.md or README references the old binary name except in the migration / deprecation section.

---

## System-Wide Impact

- **Interaction graph:** Every Cobra command file in `internal/cli/` is touched by either rename or behavior change. The `internal/cache/` and `internal/store/` packages gain new keys / writes from U2 and U5. The MCP server (`cmd/airbnb-vrbo-pp-mcp` → `cmd/airbnb-pp-mcp`) inherits the disabled-VRBO surface from U1.
- **Error propagation:** New `vrbo.Disabled()` sentinel and `ErrListingNotFound` propagate from source layer up to CLI layer with consistent exit codes. Backend-fallback errors propagate through `meta.fallback` rather than being swallowed.
- **State lifecycle risks:** Config-dir migration is the only persistent-state risk. Mitigation is in U7's edge-case scenarios. The local SQLite store gets new writes (U5's host-listings) — schema migration handled by the existing store-version field.
- **API surface parity:** The MCP server exposes the same tool surface as the CLI; quarantining VRBO at the CLI must mirror at the MCP boundary. U1's "every entry point" includes MCP tool handlers.
- **Integration coverage:** The full agent-mode contract (--agent → JSON on stdout, errors on stderr, exit codes per spec) needs to hold across renamed binary, disabled VRBO, and new error types. U3 + U4 together exercise this surface.
- **Unchanged invariants:** Airbnb scrape strategy, the `airbnb_wishlist` resource, the auth flow (`auth login --chrome`), the watch / wishlist diff features, and the freshness-contract semantics are explicitly unchanged.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| The `.printing-press.json` regeneration tool overwrites hand-edits during the rename, undoing U1–U6 changes. | Decide the regenerate-vs-hand-edit path first in U7. If regeneration is used, do it before U1–U6 so subsequent edits land on the regenerated tree. If hand-edits, freeze the generated files in this PR and document the divergence. |
| Existing users with the old config dir or legacy env vars hit confusing migration warnings. | One-time migration log line, dual-read env vars for one release, deprecation notice in README + release notes. |
| Renaming the Go module path breaks `go install` for users mid-upgrade. | Release notes explicitly call out the new install command. Old binary continues to work until the user reinstalls; nothing in the PR forces an upgrade. |
| The PP tool's catalog and per-skill regeneration is coupled to the old name in places we don't catch. | Run `release:validate` (or PP equivalent) as part of U7's verification; iterate until clean. |
| Test coverage gap after rename — string-literal references to the old name in test fixtures or mocks pass type-check but fail behavior. | Full grep + full test run after rename in U7. The execution note for U7 calls this out. |
| One PR is large enough that review fatigue masks bugs. | Order units U1 → U8 so each commit is independently reviewable; the rename (U7) is last and the most mechanical, making revert-to-just-bug-fixes feasible if review demands a split. |

---

## Documentation / Operational Notes

- Release notes call out: rename, install path change, env-var prefix change, VRBO quarantine, key bug fixes (cheapest default backend, date validation, feedback list).
- The `pp-airbnb-vrbo` skill in user installs needs a corresponding update to `pp-airbnb` after the publish PR merges. Document the user-side reinstall flow in the release notes.
- No monitoring or rollout flags — this is a CLI; release semantics are "user reinstalls".

---

## Sources & References

- Launch PR: [printing-press-library#217](https://github.com/mvanhorn/printing-press-library/pull/217) (`feat(airbnb-vrbo): add airbnb-vrbo`)
- Dogfood session log: in-conversation findings from 2026-05-03 — bug list with reproductions for B-V1–B-V3, B1–B16
- CLI source: `library/travel/airbnb-vrbo/` (renames to `library/travel/airbnb-pp/` in U7)
- SKILL.md (current): `library/travel/airbnb-vrbo/SKILL.md`
- Skill (user-installed): `cli-skills/pp-airbnb-vrbo/` (renames to `cli-skills/pp-airbnb/` in U7)
