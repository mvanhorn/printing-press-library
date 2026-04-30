---
title: "fix: Stop emitting Float64Var for cursor/page/timestamp pagination flags in generated CLIs"
type: fix
status: active
date: 2026-04-30
---

# fix: Stop emitting Float64Var for cursor/page/timestamp pagination flags in generated CLIs

## Overview

Generated Printing Press CLIs declare cursor, page, and timestamp pagination flags as `Float64Var`. Go's default float formatter renders any value at or above ~10^6 in scientific notation (`1.740168616e+09`), and the upstream APIs reject those strings as invalid integer cursors. The bug stays hidden during page 1 and during local testing with small cursors (which round-trip as `30`, `60`, …), then breaks on every paginating call where the server returns a real Unix-second or Unix-millisecond cursor.

This plan fixes the root cause in `cli-printing-press` (the generator) and re-emits the affected CLI in `printing-press-library`, with the scrape-creators CLI as the demonstration target. A secondary `client.go.tmpl` change cleans up a misleading dry-run formatter that prints `?` before every query parameter.

This is a two-repo change. The generator PR is the durable fix; the library PR is the regenerated artifact.

---

## Problem Frame

Observed during a TikTok follow-graph pull on 2026-04-30:

1. `scrape-creators-pp-cli tiktok user-following --handle mattvanhorn24 --min-time 1740168616` returns
   `HTTP 500: "Request failed with status code 400"`.
2. `--dry-run` shows the param encoded as `min_time=1.740168616e+09`.
3. The same URL with `min_time=1740168616` returns 200 from the upstream API.
4. The flag is declared `cmd.Flags().Float64Var(&flagMinTime, "min-time", 0.0, ...)` in `library/developer-tools/scrape-creators/internal/cli/tiktok_list-user-3.go:87`. Go's `fmt.Sprintf("%v", float64)` switches to scientific notation around magnitude 10^6.

The bug is invisible until pagination crosses the scientific-notation boundary, which it always does for Unix-second timestamps and reliably does for Unix-millisecond cursors. Dry-run testing with small literals (`--cursor 30`) hides the bug entirely.

The same flag-type mismatch affects 19 generated declarations across the scrape-creators CLI and 257 non-rate-limit `Float64Var` declarations across 171 generated files in `library/`. All will be remediated when the generator is fixed and the affected CLIs are regenerated; this plan ships the scrape-creators slice and leaves a follow-up unit for the rest of the library.

A secondary cosmetic bug in the same area: the dry-run printer in the generated `internal/client/client.go` emits `?key=value` for every parameter, producing output that looks like `?handle=x ?min_time=...`. The actual HTTP request assembles correctly with `&`, but the printed preview misleads anyone debugging a paginated failure into suspecting URL malformation.

---

## Requirements Trace

- R1. Generated CLIs must encode integer-valued query parameters (cursor, page, min_time, max_time, offset) as integers, never as scientific-notation floats.
- R2. The fix must hold for every generated CLI in `printing-press-library`, not just scrape-creators — i.e., the change must live in the generator, not as a hand patch.
- R3. Dry-run output must show a syntactically faithful preview of the URL: one `?` separator before the first parameter and `&` between subsequent parameters.
- R4. The scrape-creators CLI must paginate `tiktok user-following`, `tiktok user-followers`, and at least one cursor-based endpoint end-to-end against the live API after regeneration.
- R5. Generator behavior is regression-tested: a generator-level test fails if the OpenAPI `number`/`float` parameter type ever maps back to `Float64Var` for the param-name classes covered here.
- R6. No breaking change to existing scripts that pass integer literals (`--min-time 1740168616`) — string-typed cobra flags accept any token.

---

## Scope Boundaries

- Local-side `--rate-limit float` flags stay `Float64Var`. They are throttle ratios, not query parameters, and never feed into URL encoding.
- `tiktok spikes --threshold float` stays `Float64Var`. It is a real engagement-rate multiplier, not a pagination cursor.
- Upstream proxy error mapping (the 400→500 wrapping that surfaces "Request failed with status code 400") is owned by Scrape Creators; this plan does not attempt to change it.

### Deferred to Follow-Up Work

- Regeneration of the other 170 generated CLIs in `library/` carrying the same flag-type bug: a separate plan and PR after the scrape-creators slice merges. The fix is identical (regenerate); each CLI just needs its own smoke check. Tracked in a follow-up plan, not this one.
- Replacing every existing `--cursor`/`--page`/`--min-time`/`--max-time`/`--offset` numeric default that scripts may currently pass as `0` — string default `""` is functionally equivalent because the generated client only includes the param when non-empty, but a downstream caller relying on Go's float zero-value semantics would need to switch to comparing against `""`.

---

## Context & Research

### Relevant Code and Patterns

**Generator (`cli-printing-press`):**
- `internal/generator/generator.go:2201` — `cobraFlagFunc(t string)` switch that maps `"float" → "Float64Var"`. This is the root mapping to change.
- `internal/generator/generator.go:2086` — `isIDParam(name string)` — existing precedent for name-based type override (IDs that look numeric get `StringVar` regardless of declared spec type). This is the pattern to mirror for cursor/timestamp params.
- `internal/generator/generator.go:2227` — `cobraFlagFuncForParam(name, t string)` — the per-param wrapper that already gates ID overrides; cursor/timestamp override belongs here.
- `internal/generator/templates/client.go.tmpl:530-540` — dry-run printer that hardcodes `?` for each param. Needs first-iteration vs subsequent-iteration distinction.
- `internal/generator/templates/command_endpoint.go.tmpl:530`, `internal/generator/templates/command_promoted.go.tmpl:225` — call sites that invoke `cobraFlagFuncForParam` for endpoint params.
- `internal/generator/generator_test.go` — existing generator test file; new test cases extend this rather than create a new one.

**Library (`printing-press-library`):**
- `library/developer-tools/scrape-creators/internal/cli/tiktok_list-user-3.go:87` — canonical example of the buggy declaration (`user-following --min-time`).
- `library/developer-tools/scrape-creators/internal/cli/tiktok_list-user-2.go:89` — sibling case (`user-followers --min-time`).
- 17 additional declarations across 18 files in `library/developer-tools/scrape-creators/internal/cli/` (cursor + page variants).
- `library/developer-tools/scrape-creators/internal/client/client.go:341-348` — generated dry-run printer that exhibits the cosmetic bug.
- The "DO NOT EDIT" header on every file: hand-patching the library is explicitly forbidden by the existing generator contract.

### Institutional Learnings

- `docs/solutions/` was scanned for existing learnings on Float64 / pagination / cobra flag typing — none found. This is the first documented occurrence of the class.
- Existing convention from `isIDParam`: name-based overrides are how the generator handles "spec says number but runtime needs string" mismatches. Add a parallel `isCursorParam` rather than blanket-converting all floats.

### External References

- Go `strconv.FormatFloat` documentation: with `'g'` verb (which `fmt.Sprintf("%v", float64)` uses), values with magnitude ≥ 10^6 switch to scientific notation. This is the deterministic boundary that makes the bug appear at exactly the values pagination needs to handle.
- Cobra `StringVar` accepts any token, integer literals included — the migration from `Float64Var` to `StringVar` is non-breaking for callers passing `--cursor 1740168616`.

---

## Key Technical Decisions

- **Override at the generator, not the spec**: The OpenAPI specs on file declare cursor/timestamp params as `number` because that is what the upstream APIs document. Rewriting every spec would mean fighting the source of truth on every regeneration. Override in the generator's name-based classifier instead, mirroring how `isIDParam` already handles a parallel mismatch.
- **Use `StringVar`, not `Int64Var`**: Some endpoints return cursors as opaque base64 tokens (`next_page_token: "eyJtYXhfY3Vyc29yI..."`), some return numeric strings, some return integers. `StringVar` handles all three uniformly and matches the pattern already used by `tiktok profile-videos --max-cursor string` (which works correctly today).
- **Name-based classifier**: Add `isCursorParam(name)` covering `cursor`, `min_time`, `max_time`, `min_cursor`, `max_cursor`, `next_cursor`, `page`, `page_token`, `next_page_token`, `offset`. Keep the list narrow and obvious; do not blanket-convert every numeric param.
- **Default value adjustment**: When the override flips a param from `Float64Var` to `StringVar`, the generated default changes from `0.0` to `""`. The downstream `if flagMinTime != 0.0` guard becomes `if flagMinTime != ""`, which is the correct way to detect "user didn't pass a value." The template already uses `defaultValForParam` and `cobraFlagFuncForParam` together, so they will agree if both consult the same classifier.
- **Dry-run printer**: Range over `params` in deterministic order so subsequent fixes (e.g., snapshot tests) are stable. The current map iteration is non-deterministic; the new printer should sort keys before printing.

---

## Open Questions

### Resolved During Planning

- _Should the fix live in the generator or the library?_ Generator. The library files carry "DO NOT EDIT" headers and a hand patch will be wiped on the next regeneration. User confirmed via planning question.
- _StringVar or Int64Var for cursor types?_ StringVar — handles opaque tokens uniformly and matches the existing `tiktok profile-videos --max-cursor` precedent.
- _Does this break callers?_ No. Cobra string flags accept integer literals. Scripts passing `--min-time 1740168616` continue to work.

### Deferred to Implementation

- Whether the per-CLI regen produces any unrelated diff drift (e.g., reformatted comments, version bumps in client.go) that should be split into its own commit. Decide while reviewing the regen output.
- Whether the dry-run printer's deterministic key ordering should be alphabetical or insertion-preserving. Either is acceptable; pick whichever the existing template style favors when implementing.

---

## Implementation Units

- [ ] U1. **Add `isCursorParam` classifier and route through `cobraFlagFuncForParam`**

**Goal:** Introduce a name-based override that returns `StringVar` for cursor, page, and timestamp pagination params regardless of the spec's declared `number`/`float` type, mirroring the existing `isIDParam` pattern.

**Requirements:** R1, R2, R5

**Dependencies:** None

**Files:**
- Modify: `internal/generator/generator.go` (in `cli-printing-press`)
- Test: `internal/generator/generator_test.go` (in `cli-printing-press`)

**Approach:**
- Add `isCursorParam(name string) bool` next to `isIDParam`. Match suffix/exact comparisons against the documented set: `cursor`, `min_cursor`, `max_cursor`, `next_cursor`, `page`, `page_token`, `next_page_token`, `min_time`, `max_time`, `offset`.
- Extend `cobraFlagFuncForParam(name, t string)` so a hit on `isCursorParam` returns `"StringVar"` regardless of `t`.
- Extend `goTypeForParam(name, t string)` so the same names return `"string"`. The flag function and the field type must agree, otherwise the generated code won't compile.
- Mirror the change anywhere `defaultValForParam` decides default values, so the StringVar's default becomes `""` instead of `0.0`.

**Patterns to follow:**
- `isIDParam` and its routing through `cobraFlagFuncForParam` and `goTypeForParam` — same shape, different name list.

**Test scenarios:**
- Happy path: `cobraFlagFuncForParam("cursor", "number")` returns `"StringVar"`.
- Happy path: `cobraFlagFuncForParam("min_time", "integer")` returns `"StringVar"`.
- Happy path: `cobraFlagFuncForParam("page", "number")` returns `"StringVar"`.
- Edge case: `cobraFlagFuncForParam("page_size", "integer")` returns `"IntVar"` — `page_size` is not a cursor; only the documented names override.
- Edge case: `cobraFlagFuncForParam("threshold", "number")` returns `"Float64Var"` — non-cursor floats still map to float.
- Edge case: `goTypeForParam("min_time", "number")` returns `"string"` — the field type matches the flag function.
- Regression: `cobraFlagFuncForParam("user_id", "integer")` still returns `"StringVar"` via `isIDParam` — the new classifier does not stomp the existing one.

**Verification:**
- New generator tests pass.
- Re-running the generator for any CLI with cursor/page/min_time params emits `StringVar` with default `""` and `string` field types in the resulting Go file.

---

- [ ] U2. **Fix dry-run query-string formatter in `client.go.tmpl`**

**Goal:** Print one `?` before the first query parameter and `&` between subsequent ones, so the dry-run preview is a faithful representation of the actual URL.

**Requirements:** R3

**Dependencies:** None (independent of U1; either can land first)

**Files:**
- Modify: `internal/generator/templates/client.go.tmpl` (in `cli-printing-press`)

**Approach:**
- Sort `params` keys before printing so output is deterministic.
- Track whether any param has been printed; emit `?` for the first, `&` for subsequent.
- Apply the same pattern to the auth-as-query-param sub-block (the `{{if and .Auth .Auth.In (eq .Auth.In "query")}}` branch in the template) so the auth line correctly uses `&` when other params preceded it.

**Patterns to follow:**
- Existing `path + "?" + strings.Join(parts, "&")` pattern at template line 87 — same idea, applied to the printer.

**Test scenarios:**
- Snapshot: dry-run of a single-param request prints exactly one `  ?key=value` line.
- Snapshot: dry-run of a two-param request prints `  ?<first>=...` then `  &<second>=...` in deterministic order.
- Snapshot: dry-run of a request with query-position auth prints `&<auth_param>=...` when other params preceded it.
- Edge case: dry-run of a zero-param request prints no `?`/`&` line at all.

**Verification:**
- Snapshot test in the generator's template-render tests asserts the new output shape.
- Regenerating any CLI and running `--dry-run` with two flags shows `?...` then `&...`.

---

- [ ] U3. **Regenerate scrape-creators CLI in `printing-press-library` and verify pagination end-to-end**

**Goal:** Apply U1 and U2 to the live scrape-creators CLI by running the generator, commit the regenerated diff, and prove pagination works against the live API.

**Requirements:** R1, R2, R3, R4, R6

**Dependencies:** U1, U2

**Files:**
- Modify (regenerated): `library/developer-tools/scrape-creators/internal/cli/tiktok_list-user-3.go`, `library/developer-tools/scrape-creators/internal/cli/tiktok_list-user-2.go`, and the other 17 cli files identified in the float-flag inventory below.
- Modify (regenerated): `library/developer-tools/scrape-creators/internal/client/client.go`
- Test: `library/developer-tools/scrape-creators/internal/cli/tiktok_pagination_test.go` (new) — integration smoke test using the live API or a recorded fixture.

**Approach:**
- Run the same regeneration command the user already uses for scrape-creators (the generator's standard entry point against the existing `.printing-press.json`).
- Commit the regenerated files in one commit so the diff is reviewable as "regen output of U1+U2."
- Spot-check the diff: every formerly-`Float64Var` cursor/page/timestamp flag should now be `StringVar`, every formerly-`float64` field should now be `string`, every default `0.0` should be `""`, and `client.go` dry-run should use `&`.

**Patterns to follow:**
- Existing regen workflow used in prior plans under `library/developer-tools/`.

**Test scenarios:**
- Integration: `tiktok user-following --handle mattvanhorn24` paginates past page 1 with a real `min_time` cursor and returns ≥ 2 distinct pages with `has_more=false` on the last page.
- Integration: `tiktok user-followers --handle mattvanhorn24 --min-time <large>` returns 200, not 500.
- Integration: `tiktok video-comments --url <known-url> --cursor <real>` paginates past page 1.
- Edge case: `--min-time 0` (zero) is not sent as a query param (string-empty guard works).
- Edge case: `--min-time 30` (small int as string) round-trips correctly.
- Snapshot: `--dry-run` output shows `?` then `&`, not `?` then `?`.
- Regression: `tiktok profile-videos --max-cursor <ms-timestamp>` (already string-typed, already works) is not affected by the regeneration.

**Verification:**
- The full follow-graph fetch reproduced from the original bug report (paginating to ≥ 13 pages and recovering ≥ 260 entries) works through the CLI rather than requiring the direct-API workaround.
- `go build ./...` and `go vet ./...` clean for the regenerated package.

---

- [ ] U4. **Open paired PRs and link them**

**Goal:** Ship the fix as two reviewable PRs that reference each other, so a reviewer sees the generator change first and the library regen as the consequence.

**Requirements:** R2

**Dependencies:** U1, U2, U3

**Files:**
- New PR 1 against `cli-printing-press` containing U1 and U2.
- New PR 2 against `printing-press-library` containing U3.

**Approach:**
- PR 1 title: `fix: stop emitting Float64Var for cursor/page/timestamp pagination flags` against `cli-printing-press`. Body links to this plan and includes the encoding-boundary repro.
- PR 2 title: `fix: regenerate scrape-creators CLI for cursor/page string-flag fix` against `printing-press-library`. Body links PR 1 and notes that the change is purely the regen output, no hand edits.
- In PR 2, paste the before/after dry-run output for one cursor flag and the integration test result so reviewers can see the bug and the fix without running it themselves.
- Note in PR 2 that 170 other generated CLIs in `library/` carry the same flag-type bug and a follow-up regen PR will batch them.

**Test expectation:** none — this unit is PR mechanics, not behavioral.

**Verification:**
- Both PRs link to each other in their descriptions.
- PR 1 is mergeable on its own; PR 2 is mergeable once PR 1 is merged and the generator binary is rebuilt.

---

## Affected scrape-creators files (regenerated by U3)

This list is the inventory U3 should produce as a regenerated diff. If the regen output diverges from this list (more files affected, fewer, or different), investigate before committing.

| File | Flag | Param name |
|------|------|------------|
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-user-3.go` | `--min-time` | user-following |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-user-2.go` | `--min-time` | user-followers |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-search.go` | `--cursor` | search-keyword |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-search-2.go` | `--cursor` | search-hashtag |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-search-3.go` | `--cursor` | search-top |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-search-4.go` | `--cursor` | search-users |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-song-2.go` | `--cursor` | song-videos |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-video.go` | `--cursor` | video-comment-replies |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-video-2.go` | `--cursor` | video-comments |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-videos.go` | `--page` | videos-popular |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-songs.go` | `--page` | songs-popular |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-hashtags.go` | `--page` | hashtags-popular |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list.go` | `--page` | creators-popular |
| `library/developer-tools/scrape-creators/internal/cli/tiktok_list-shop-3.go` | `--page` | shop-search |
| `library/developer-tools/scrape-creators/internal/cli/google_list-search.go` | `--page` | google search |
| `library/developer-tools/scrape-creators/internal/cli/instagram_list-reels.go` | `--page` | instagram reels-search |
| `library/developer-tools/scrape-creators/internal/cli/linkedin_list-company-2.go` | `--page` | linkedin company-posts |
| `library/developer-tools/scrape-creators/internal/cli/promoted_tiktok.go` | `--page` | tiktok-related promoted call |
| `library/developer-tools/scrape-creators/internal/client/client.go` | n/a | dry-run printer |

---

## System-Wide Impact

- **Interaction graph:** The generator change ripples to every CLI in `library/` on regeneration. This plan only regenerates scrape-creators; other CLIs continue to carry the bug until their own regen.
- **Error propagation:** The fix removes a silent client-side encoding error. Upstream proxy errors (the 400→500 wrap) are unchanged; users will now see the real upstream errors when they happen, not the fake-server errors caused by malformed cursors.
- **State lifecycle risks:** None. The change is purely flag-typing and printer formatting; no persistent state.
- **API surface parity:** Cobra string flags accept integer literals, so callers passing `--min-time 1740168616` see no change in invocation. Callers reading the flag's Go type inside the same binary do not exist (these are CLI-only flags).
- **Unchanged invariants:** `--rate-limit float`, `--threshold float`, and any non-cursor numeric param continue to use float. The HTTP request-assembly path in `client.go` is unchanged; only the dry-run *printer* format changes.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Regenerating scrape-creators surfaces unrelated diff drift (comments, copyright years, formatting changes from a generator that's moved on since the last regen) | U3 commits the regen as a single commit; review-time check separates intentional change from drift; if drift is significant, split into "unrelated regen drift" and "U1/U2 effect" commits before opening the PR |
| `isCursorParam`'s name list misses an endpoint that uses an unconventional cursor param name | The list is narrow on purpose; any escape gets caught in U3's integration spot-checks. If found, add to the list and re-test |
| Some downstream code depends on the float zero-value to detect "not set" | Searched the generated codebase; the only consumer is the `if flag != 0.0` guard inside the same generated file, which becomes `if flag != ""` automatically when the type changes. No external consumer reads these flag values directly |
| Default-value template (`defaultValForParam`) and flag-function template (`cobraFlagFuncForParam`) disagree about a cursor param, producing code that doesn't compile | U1 mandates that both consult the same classifier; the generator's existing test suite + a clean `go build` after regen catches any disagreement immediately |
| Other 170 affected CLIs ship before they're regenerated, leaving them visibly broken on cursor pagination | They are no more broken than they are today; the follow-up plan handles them. Note explicitly in PR 2's description so users know the scope |

---

## Documentation / Operational Notes

- The fix is invisible to end users who only use page 1 of any endpoint. Most TikTok scraping flows involve pagination, so most real users will benefit.
- After PR 1 lands and the user updates their local `cli-printing-press` install, any future `pp install` of an existing or new CLI will pick up the fix automatically. The library PR 2 unblocks the scrape-creators CLI immediately for users who don't regenerate locally.
- A short note in `cli-printing-press` `CHANGELOG.md` (if present) under "fixed" is sufficient. No public API change.

---

## Sources & References

- Original bug observation and CLI debugging session: 2026-04-30 conversation with the user, paginating `tiktok user-following` for `@mattvanhorn24`.
- Encoding-boundary probe results (small ints render literally, ≥ 10^6 flips to scientific notation): documented in the conversation under "Probe: float-encoding boundary."
- Live-API confirmation that the URL difference between `min_time=1.740168616e+09` and `min_time=1740168616` is the entire problem: documented in the same probe.
- Existing generator pattern that this change mirrors: `isIDParam` and its use sites in `internal/generator/generator.go`.
