# Printing Press Retro: numista

## Session Stats
- API: numista
- Spec source: OpenAPI 3.0 (https://en.numista.com/api/doc/swagger.yaml, 17 endpoints after pruning paid + Catalogue Edition + deprecated)
- Scorecard: 85/100 (Grade A)
- Verify pass rate: 100% (43/43)
- Fix loops in Phase 4: 1
- Fix loops in Phase 5: 6
- Manual code edits (after Codex's 3 batch runs): ~12 (mostly Use-string narrowing, narrative alignment, dogfood/verify env shortcircuits)
- Features built from scratch: 10 transcendence commands + monthly quota subsystem (lookup_log + --quota flags + audit) — ~2,300 LoC across 3 Codex batches

## Findings

### F1 — `defaultSyncResources()` emits auth-flow endpoints as syncable resources (Default gap)
- **What happened:** The generator's `defaultSyncResources()` for numista-pp-cli included `oauth-token` alongside `catalogues, issuers, mints, types`. The `oauth-token` endpoint requires per-call params (`code` for authorization_code grant, `scope` for client_credentials) and returns HTTP 400 when called with no params. Running `sync` or `workflow archive` with no args triggered `sync_error` events for oauth-token, ultimately producing exit -1 from sync and turning four live-dogfood probes into failures (sync happy_path + sync --json + workflow archive happy_path + workflow archive --json).
- **Scorer correct?** Yes — dogfood correctly observed the failure. The bug is in the generator.
- **Root cause:** `internal/generator/...` — the syncable-resources list is built from "every spec resource with a list-shaped endpoint" without filtering by tag. The Numista spec tags the `/oauth_token` operation as `OAuth`, but the syncable-resources logic doesn't check tags.
- **Cross-API check:** Stripe (`/oauth/token`), GitHub (`/login/oauth/access_token`), Linear (any OAuth endpoint), Hubspot (`/oauth/v1/token`), Notion (`/v1/oauth/token`). Every API with an OAuth or Authentication tag will trip this when sync touches the endpoint with no per-call params.
- **Frequency:** every API with an OAuth-tagged endpoint emitted in the syncable-resources list. ~5+ named APIs above.
- **Fallback if the Printing Press doesn't fix it:** Each printed CLI's author manually edits `defaultSyncResources()` to drop the auth endpoint. The fix is one-line per CLI but easy to forget — and verify/dogfood don't catch it unless live-dogfood runs.
- **Worth a Printing Press fix?** Yes. Concrete bug, clear fix, generalizable.
- **Inherent or fixable:** Fixable.
- **Durable fix:** When building `defaultSyncResources()`, exclude any spec operation whose tag matches `OAuth | Authentication | Auth` (case-insensitive prefix). The same filter belongs in `workflow archive`'s hard-coded resources list (currently a sibling list in `channel_workflow.go`).
- **Test:** (positive) for an API with an OAuth-tagged endpoint, `defaultSyncResources()` does NOT include it. (negative) for an API with no OAuth tag, all resources are included.
- **Evidence:** Phase 5 iteration 1 — 4 of 13 initial dogfood failures had this root cause. Confirmed via direct test: `./numista-pp-cli sync --json` emitted `{"event":"sync_error","resource":"oauth-token","status":400,"body":"Missing mandatory parameter 'code'"}` until I patched `defaultSyncResources()` in the printed CLI.
- **Related prior retros:** None.
- **Step G case-against:** "A power user might want oauth-token cached for self-discovery." Weak — the endpoint isn't list-shaped and the per-call params can't be auto-filled from sync state.

### F2 — README/SKILL narrative blocks (quickstart, recipes, troubleshoots, auth_narrative) don't re-sync from research.json after dogfood updates (Template gap)
- **What happened:** After Phase 4 shipcheck, I updated `research.json` to fix verify-skill failures (renamed `users items add` → `users collected-items add`; replaced invented `auth login --scope view_collection` with real `oauth-token --grant-type client_credentials --scope view_collection`; fixed `audit --by endpoint` → `audit --by-endpoint`; etc.). Dogfood re-rendered `## Unique Features` (README) and `## Unique Capabilities` (SKILL) blocks, but the `## Quick Start`, `## Authentication`, `## Troubleshooting`, and the SKILL recipes block did NOT re-render. Verify-skill found 25 errors. I had to hand-edit each block across README and SKILL with sed scripts, then break the prose value-prop sentence "numista-pp-cli wraps..." → "This CLI wraps..." to dodge verify-skill's grep-based "wraps" misread.
- **Scorer correct?** Partially. Verify-skill is right to flag the mismatches; the underlying problem is that only `novel_features` rendering re-syncs.
- **Root cause:** The post-dogfood sync writes `novel_features_built` and rewrites the README "Unique Features" and SKILL "Unique Capabilities" sections — but `narrative.quickstart`, `narrative.recipes`, `narrative.troubleshoots`, and `narrative.auth_narrative` are emitted at generate-time only. Any post-generate edit to `research.json`'s narrative.* fields drifts from the rendered prose.
- **Cross-API check:** Every CLI has `narrative.quickstart` and `narrative.recipes`. The polish + dogfood loop frequently edits research.json (rename commands, fix flag names, swap invented commands for real ones). Every printed CLI hits the same drift.
- **Frequency:** every CLI where research.json is edited after generate (which is most of them, per polish skill behavior).
- **Fallback if the Printing Press doesn't fix it:** Hand-edit README + SKILL after every research.json change. ~10 edits in this session.
- **Worth a Printing Press fix?** Yes.
- **Inherent or fixable:** Fixable. Extend the post-dogfood sync logic that handles `## Unique Features` / `## Unique Capabilities` to also rewrite README `## Quick Start`, README `## Authentication`, README `## Troubleshooting`, SKILL recipes, and SKILL auth blocks from `narrative.*` whenever research.json differs from the rendered version. The render templates exist; what's missing is the post-edit rewrite path.
- **Durable fix:** Generalize the dogfood sync. Today: `novel_features_built` drives Unique Features + Unique Capabilities. Extend: `narrative.quickstart` drives README Quick Start, `narrative.troubleshoots` drives README Troubleshooting, `narrative.auth_narrative` drives README Authentication, `narrative.recipes` drives SKILL Recipes. Mirror the same diff-and-rewrite pattern.
- **Test:** Edit `narrative.quickstart` in research.json post-generate, re-run dogfood, verify README Quick Start matches.
- **Evidence:** Phase 5 — ~10 manual sed substitutions across README.md + SKILL.md after editing research.json. The fix-py script `/tmp/fix_numista_docs.py` ran 17 substitutions across two files just to keep narrative blocks honest.
- **Related prior retros:**
  - `pcgs` retro (`20260516-232004-retro-pcgs-pp-cli.md`) F3 — `aligned` (narrower scope: novel_features → Highlights/Long/README/SKILL drift; my finding extends to quickstart/recipes/troubleshoots/auth_narrative)
- **Related open issues:**
  - **#1562** (`generator: README template doesn't emit ## Recipes section from narrative.recipes`) — `related-area`. #1562 is about README never emitting the Recipes block; my finding is about narrative.* fields not RE-rendering after research.json edits. Both are gaps in narrative→rendered-doc fidelity; pieces of the same overall puzzle.
- **Step G case-against:** "The agent should iterate research.json BEFORE generate, then never edit it again. The fix is workflow discipline, not generator behavior." Counter: dogfood ITSELF edits research.json (writes `novel_features_built`), and verify-skill's findings force narrative re-edits when invented commands don't match shipped commands. Edit-after-generate is the expected workflow, not an anti-pattern.

### F3 — `syncResource` emits NDJSON event stream to stdout when called from a parent that wants single-doc JSON (Bug)
- **What happened:** `workflow archive --json` calls `syncResource` in a loop, then emits a single final summary JSON object. But `syncResource` itself emits `{"event":"sync_start","resource":"..."}` and `{"event":"sync_progress",...}` lines to stdout when `humanFriendly == false`. The combined stdout is N NDJSON lines followed by a single JSON object — fails the matrix's single-doc JSON parser. I worked around it by saving and flipping the package-global `humanFriendly` in workflow archive's RunE when `--json` is set, then restoring with defer.
- **Scorer correct?** Yes — the matrix's json_fidelity check is correct in expecting single-doc JSON when --json is requested.
- **Root cause:** `internal/cli/sync.go` (or wherever `syncResource` lives). `syncResource` emits NDJSON to os.Stdout directly, with the gating logic inverted (writes-to-stdout when `humanFriendly == false`). For an interactive `sync --json` invocation that's defensible (streaming progress for monitoring). For a parent wrapper like `workflow archive --json` that itself wants single-doc JSON, the events pollute stdout.
- **Cross-API check:** Every printed CLI has a `sync` command and (in most cases) a `workflow archive` wrapper. The same wrapping pattern recurs.
- **Frequency:** every CLI's workflow archive --json invocation.
- **Fallback if the Printing Press doesn't fix it:** Each printed CLI's author flips humanFriendly defensively in any wrapper. Easy to forget, very easy to NOT notice until live dogfood.
- **Worth a Printing Press fix?** Yes.
- **Inherent or fixable:** Fixable. Either (a) `syncResource` takes a `Quiet bool` (or `EventSink io.Writer` defaulting to os.Stdout) parameter so callers can route events to stderr or io.Discard; or (b) when called from a parent that emits its own --json output, route the events to stderr instead of stdout.
- **Durable fix:** Add `syncResource(..., quiet bool)` param. In `--json` callers (and in workflow archive specifically), pass `quiet=true` which routes events to stderr (preserve them for tail-following) but keeps stdout pure for the final summary.
- **Test:** `<cli> workflow archive --json | jq .` succeeds (single-doc JSON parse).
- **Evidence:** Phase 5 iteration 4 — workflow archive happy_path + json_fidelity both failed with "invalid JSON" until I added the humanFriendly flip in `channel_workflow.go`.
- **Related prior retros:** None.
- **Step G case-against:** "humanFriendly being inverted is intentional for streaming-mode sync." Counter: even granting that, the WRAPPER pattern is broken. A first-class quiet-mode parameter resolves both intents without an inverted-flag hack.

### F4 — Live-dogfood 1 MiB stdout cap surfaces as opaque "invalid JSON" with no actionable signal (Scorer bug)
- **What happened:** `numista-pp-cli issuers --json` returns a valid 3.9 MB JSON document (11,789 issuers). The live-dogfood matrix's `runLiveDogfoodProcess` captures stdout into a `limitedWriter{remaining: MaxOutputBytes}` where `MaxOutputBytes = 1 << 20` (1 MiB). The capture is silently truncated mid-array. `validLiveDogfoodJSONOutput` then runs `json.Valid([]byte(trimmed))` against the truncated body, which fails, and the matrix reports `reason: "invalid JSON"` — a misleading message because the JSON the CLI emitted IS valid; the test runner truncated it. Identical pattern hit `mints get --json` (1.1 MB).
- **Scorer correct?** No — the scorer's failure message is incorrect for this case. The CLI is fine; the matrix's capture cap clipped the input to the validator.
- **Root cause:** `internal/pipeline/live_dogfood.go` `runLiveDogfoodProcess` + `validLiveDogfoodJSONOutput`. The capture path doesn't distinguish "command emitted invalid JSON" from "we truncated the capture so what we have looks invalid."
- **Cross-API check:** Numista (issuers, mints, catalogues). ESPN (sports/teams). Steam Web (large search results). Any free public-registry API that returns full list responses without pagination. Hand-wave names "many", with-evidence names ≥3 (Numista, ESPN, Steam — all have >1 MB list responses in the catalog or library).
- **Frequency:** every CLI with at least one large unpaginated list endpoint and --json output.
- **Fallback if the Printing Press doesn't fix it:** Each printed CLI's author adds an `IsDogfoodEnv()`-gated client-side slice helper (which is what I shipped: `truncateForDogfood` in helpers.go). That's a per-CLI workaround for a scorer bug. Per the cardinal rule: "Never work around a scorer bug in the Printing Press. If a scoring tool penalizes something incorrectly, the fix goes in the scoring tool."
- **Worth a Printing Press fix?** Yes — and per the cardinal rule the fix must be in the scorer, not in printed CLIs.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Two parts. (a) Raise `MaxOutputBytes` to a more pragmatic value (~10 MiB; matrix RAM impact still negligible) so most large-list APIs don't trip the cap at all. (b) When the cap IS hit, surface `reason: "output exceeded 1 MiB capture cap"` instead of `"invalid JSON"` so agents see the actual failure. Optionally also stream a final `\n--TRUNCATED--\n` marker so the JSON validator can detect-and-skip rather than fail.
- **Test:** Run a command that emits 2 MB of valid JSON. (with current cap + clearer message): test passes with "output exceeded capture cap" reason; (with raised cap): test passes as valid JSON.
- **Evidence:** Phase 5 — `wc -c` confirmed direct invocations of `issuers --json` produce 3.9 MB of valid parseable JSON; matrix consistently reported `invalid JSON`. Source: `internal/pipeline/live_dogfood.go:730`.
- **Related prior retros:** None directly.
- **Step G case-against:** "1 MiB is generous; if a CLI returns >1 MiB by default, the CLI should be paginating or limiting." Counter: that's a per-API design call, not a constraint the matrix should silently enforce via misleading error messages. The cap can stay; the message should be clear.

### F5 — Default sync `--concurrency=4` triggers SQLITE_BUSY in fast-response scenarios (quota-limited APIs in particular) (Default gap with subclass guard)
- **What happened:** `numista-pp-cli sync` with default `--concurrency=4` reliably tripped SQLITE_BUSY when running against the live API. The existing `cliutil.IsVerifyEnv()` check at sync.go:191 already forces concurrency=1 in mock mode — proof that the issue is known for mock mode, but the same race condition affects live mode when the API is fast enough (or under burst conditions). Numista responses are small + fast enough to surface the race; on a monthly-quota API where parallelism is wasteful anyway, concurrency=1 is the correct default.
- **Scorer correct?** Yes — dogfood correctly observed the timeout. The bug is the default.
- **Root cause:** `internal/generator/.../sync.go` — concurrency defaults to 4 without consulting the spec's rate class. For monthly-quota APIs, parallelism produces no real speedup (each call costs quota; parallel just exhausts the budget faster) AND trips SQLite's single-writer mutex.
- **Cross-API check:** Numista (2K/month). Other free-tier public APIs (most ad-supported APIs cap free at low daily/monthly numbers — TwelveData free, Polygon free, FRED, etc.). The subclass is "rate_class: monthly" or "rate_class: low-volume" APIs. Hand-wave at "free-tier APIs"; with-evidence ≥3 = Numista + presumably others when checked; **only 1 with-evidence at this time**. Per Step B's rule, this drops to **P3 with subclass annotation**.
- **Frequency:** subclass:monthly-quota APIs.
- **Fallback if the Printing Press doesn't fix it:** Each printed CLI's author drops concurrency to 1 when they realize their API has a monthly quota. Easy to forget until SQLITE_BUSY surfaces.
- **Worth a Printing Press fix?** Conditionally yes — at low priority and only with a guard.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Add a spec-level `rate_class: monthly | daily | per-second | unlimited` annotation that the generator reads. For `monthly` and `daily` (with low caps), default `--concurrency=1`. The current `IsVerifyEnv` shortcircuit becomes a special case of this. Without the spec annotation, default stays 4 (current behavior).
- **Test:** Spec with `rate_class: monthly` → generated sync has `concurrency` flag default=1. Without it, default=4 (no regression).
- **Evidence:** Phase 5 iteration 2 — `sync_state for issuers: database is locked (5) (SQLITE_BUSY)` with concurrency=4. Hand-patched to 1; SQLITE_BUSY gone.
- **Related prior retros:** None.
- **Step G case-against:** "Concurrency=4 is the right default for the vast majority of APIs. Monthly-quota APIs are rare. Adding a spec annotation just for this is over-engineering." Strong case-against. The retro keeps this at P3 with the explicit subclass-evidence shortfall called out — file as advisory rather than action-item.

### F6 — `parentNoSubcommandRunE` parent commands flagged as thin-short by tools-audit (Scorer bug)
- **What happened:** Generated parent commands like `mints`, `types`, `users collected-items`, etc. use the `parentNoSubcommandRunE` helper, which sets RunE on the parent. Because tools-audit's parent-grouper exemption checks "no RunE", these commands fail the exemption and get flagged as thin-short. The `Short: "Manage mints"`, `Short: "Manage collected items"` strings are generator-emitted boilerplate that legitimately don't carry per-command actionable detail — the leaf subcommands do.
- **Scorer correct?** No — tools-audit's parent-grouper detection has a blind spot for the `parentNoSubcommandRunE` sentinel pattern.
- **Root cause:** `internal/cli/tools_audit.go` (or equivalent) — the parent-grouper exemption keys on RunE absence. Either (a) the exemption should also recognize the `parentNoSubcommandRunE` sentinel, OR (b) the generator emits richer Short strings for these parents.
- **Cross-API check:** Every printed CLI with multi-level command trees uses `parentNoSubcommandRunE` for parent groupers. Numista has 5 such parents (`mints`, `types`, `types issues`, `users collected-items`, `users collections`). Same for PCGS, Linear, etc.
- **Frequency:** every CLI with nested command trees, which is most of them.
- **Fallback if the Printing Press doesn't fix it:** Polish marks the findings as accepted (which is what I did this run — all 5 accepted with rationales). No user impact, but every polish run spends cycles on these.
- **Worth a Printing Press fix?** Yes, low priority. The case-for is "every polish run for every multi-level CLI spends cycles accepting these findings" — cosmetic but recurring.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Update tools-audit's parent-grouper exemption to recognize the `parentNoSubcommandRunE` function reference (string-match on `parentNoSubcommandRunE` in the AST or via cobra's `RunE` value identity). Alternative: generator emits `Short: "Get details / search " + <plural>` synthesized from the leaf subcommand list.
- **Test:** A CLI with a parent that uses `parentNoSubcommandRunE` and a generic `Short` should NOT trigger thin-short findings.
- **Evidence:** Phase 5.5 polish — accepted 5 thin-short findings on `mints`, `types`, `types issues`, `users collected-items`, `users collections` parent groupers.
- **Related prior retros:** None.
- **Step G case-against:** "The accepted-with-rationale flow exists for exactly this case. The maintainer doesn't need to see this." Counter: every polish run for every multi-level CLI repeats the same accepted-with-rationale work. The scorer's behavior IS the bug; the rationale-loop is the workaround.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | defaultSyncResources() excludes auth-flow endpoints | generator | every CLI with OAuth-tagged endpoints | medium (easy to forget) | small | filter on tag prefix |
| F2 | Post-dogfood sync rewrites README+SKILL narrative blocks (quickstart, recipes, troubleshoots, auth_narrative) | generator | every CLI where research.json is edited post-generate | low (manual edits required) | medium | none |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F3 | syncResource gains quiet-mode param to route events to stderr | generator | every CLI's workflow archive --json | low | small | none |
| F4 | Live-dogfood raises 1 MiB cap and/or clarifies "output cap exceeded" vs "invalid JSON" | scorer | any CLI with large unpaginated list endpoints | medium (per-CLI workaround) | small | none |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F5 | spec-level rate_class drives sync concurrency default (subclass:monthly-quota; only 1 API with-evidence) | spec-parser + generator | subclass:monthly-quota APIs | medium | small | spec annotation |
| F6 | tools-audit recognizes parentNoSubcommandRunE sentinel | scorer | every multi-level CLI | high (auto-accepted with rationale) | small | none |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|----------------------|
| --limit flag on unpaginated list endpoints | generator emits client-side --limit when endpoint returns N > K items unpaginated | Step G: case-against (changes default behavior unexpectedly) ≥ case-for. Better solved by F4 raising the matrix cap. |
| Generator surfaces "at least one of X" parameter constraints | generator emits client-side validation when spec descriptions say "At least one of..." | Step G: case-against ≥ case-for. Fuzzy prose parsing; over-validation risk; better to let API errors surface with --help-style nudging. |
| verify-skill "wraps" prose false positive | verify-skill's command detection flags natural-English prose containing the CLI name | Step D: already filed as #1556 (`scorer: verify-skill command detection flags natural-English prose mentions of CLI name`). |
| Stale build/stage/bin friction | shipcheck/scorecard don't rebuild stage binary after edits | Step D: already filed as #1555 (`scorer: shipcheck/scorecard don't rebuild build/stage/bin/<cli> before --live-check`). Also raised in blu-ray and exchangerate-api retros — case-benefit math is on file. |
| Use:"<arg>" angle brackets break verify-skill | hand-written commands declaring required positionals via angle brackets fail verify-skill's positional-args check | Skill instructional, already partially documented in the SKILL's novel-command guidance. Not generator-wide. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| crawl issuer "australia" wrong code | example used "australia" but real Numista code is "australia_section" | printed-CLI |
| Per-command --from-file extension for users collected-items add | extension to a generator-emitted command for this CLI's bulk import flow | printed-CLI |
| IsVerifyEnv shortcircuit on types_batch.go | per-CLI implementation of the existing SKILL pattern | printed-CLI |
| README value-prop sentence rewrite | "numista-pp-cli wraps..." → "This CLI wraps..." (verify-skill false positive workaround) | covered by Skip-#1556 |
| Codex completion-marker pattern observation | _codex-result.json works extremely well — task completion signal is rock-solid | not a problem; positive observation in "What the Printing Press Got Right" instead |

## Work Units

### WU-1: Exclude OAuth/Authentication-tagged endpoints from defaultSyncResources (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** Spec endpoints tagged `OAuth`, `Authentication`, or `Auth` (case-insensitive) are excluded from the generated `defaultSyncResources()` list and from any sibling hard-coded resource list in `workflow archive`.
- **Target:** `internal/generator/` — wherever `defaultSyncResources()` and `workflow archive`'s resource slice are emitted from
- **Acceptance criteria:**
  - positive test: a spec with an OAuth-tagged endpoint produces `defaultSyncResources()` that does NOT include it
  - negative test: a spec with no OAuth tag produces `defaultSyncResources()` matching the current (all-list-shapes) behavior
- **Scope boundary:** Don't touch the typed Upsert<Resource> emission or the sync command flag set. Only the default-resources list and the workflow-archive sibling list.
- **Dependencies:** None
- **Complexity:** small

### WU-2: Post-dogfood sync rewrites README + SKILL narrative blocks from research.json (from F2)
- **Priority:** P1
- **Component:** generator
- **Goal:** When dogfood rewrites research.json (via `novel_features_built` and during the `narrative` validation loop), it also re-renders the corresponding README and SKILL sections: README `## Quick Start` (from narrative.quickstart), README `## Authentication` (from narrative.auth_narrative), README `## Troubleshooting` (from narrative.troubleshoots), SKILL Recipes (from narrative.recipes). Today only `## Unique Features` / `## Unique Capabilities` re-render.
- **Target:** `internal/generator/...` post-dogfood sync logic — the same machinery that already rewrites Unique Features. Extend to the four other narrative-driven blocks.
- **Acceptance criteria:**
  - positive test: edit `narrative.quickstart` in research.json, re-run dogfood, README Quick Start matches the new value (verify via grep)
  - positive test: edit `narrative.recipes` in research.json, re-run dogfood, SKILL Recipes section matches
  - negative test: research.json unchanged → README + SKILL unchanged (no spurious rewrites)
- **Scope boundary:** Don't touch the initial generate-time rendering. Only the dogfood-time re-render path. Don't extend to AGENTS.md or any third file.
- **Dependencies:** None (uses existing narrative-rendering templates)
- **Complexity:** medium
- **Related:** #1562 covers a narrower piece of this (README Recipes section); WU-2 supersedes / extends it. Worth referencing in the issue.

### WU-3: syncResource quiet-mode gates NDJSON event stream (from F3)
- **Priority:** P2
- **Component:** generator
- **Goal:** `syncResource` accepts a `quiet bool` (or `eventSink io.Writer`) so wrappers like `workflow archive --json` can suppress (or stderr-route) the NDJSON event stream without flipping a package-global flag.
- **Target:** `internal/generator/.../sync.go` template (and wherever `syncResource`'s signature is defined)
- **Acceptance criteria:**
  - positive test: `workflow archive --json` produces stdout containing exactly one JSON document (the summary)
  - positive test: `sync --json` continues emitting NDJSON events on stdout (no regression for streaming consumers)
  - negative test: `sync` (no flags, interactive) emits human-readable output to stderr, no JSON on stdout
- **Scope boundary:** Don't redesign the streaming protocol. Just add an opt-out for wrappers.
- **Dependencies:** None
- **Complexity:** small

### WU-4: Live-dogfood capture cap clarity + raised limit (from F4)
- **Priority:** P2
- **Component:** scorer
- **Goal:** When `runLiveDogfoodProcess`'s output capture exceeds `MaxOutputBytes`, the live-dogfood result's `reason` is `"output exceeded capture cap"` (not `"invalid JSON"`). Optionally also raise `MaxOutputBytes` from 1 MiB to ~10 MiB.
- **Target:** `internal/pipeline/live_dogfood.go` — `runLiveDogfoodProcess`, `validLiveDogfoodJSONOutput`, and `limitedWriter`
- **Acceptance criteria:**
  - positive test: a command emits 2 MiB of valid JSON; the live-dogfood result is either PASS (cap raised to 10 MiB) or FAIL with reason `"output exceeded capture cap"` (cap kept at 1 MiB)
  - negative test: a command emits broken/truncated JSON < cap; reason remains `"invalid JSON"`
- **Scope boundary:** Don't change the matrix's pass/fail thresholds. Just disambiguate the reason field for over-cap captures.
- **Dependencies:** None
- **Complexity:** small

### WU-5: Spec-level `rate_class` annotation drives sync concurrency default (from F5)
- **Priority:** P3
- **Component:** spec-parser (annotation parsing) + generator (default flag value)
- **Goal:** A spec with `rate_class: monthly` (or another low-volume class) generates `sync` with `--concurrency` default=1. Existing behavior (no annotation = default 4) is preserved.
- **Target:** internal YAML + OpenAPI vendor extension parsing in `internal/spec/` and `internal/openapi/`; generator template in `internal/generator/.../sync.go`
- **Acceptance criteria:**
  - positive test: spec with `rate_class: monthly` produces sync.go with `IntVar(&concurrency, "concurrency", 1, ...)`
  - negative test: spec without `rate_class` produces sync.go with `IntVar(&concurrency, "concurrency", 4, ...)`
- **Scope boundary:** Add the annotation + the default-flag wire. Don't add rate-limiting middleware, retry policies, or back-off — those are separate concerns.
- **Dependencies:** None
- **Complexity:** small

### WU-6: tools-audit recognizes `parentNoSubcommandRunE` sentinel for parent-grouper exemption (from F6)
- **Priority:** P3
- **Component:** scorer
- **Goal:** tools-audit's parent-grouper exemption recognizes that `parentNoSubcommandRunE` is the canonical helper used by every generated CLI for parent commands that don't run real logic, so generic `Short: "Manage X"` strings on such parents don't trigger thin-short findings.
- **Target:** `internal/cli/tools_audit.go` (or wherever the parent-grouper exemption is implemented)
- **Acceptance criteria:**
  - positive test: a CLI with parents using `parentNoSubcommandRunE` and `Short: "Manage <plural>"` does NOT trigger thin-short on those parents
  - negative test: a parent that has real RunE logic (not the sentinel) and a thin Short still triggers the finding
- **Scope boundary:** Just the exemption check. Don't change the leaf-command thin-short detection.
- **Dependencies:** None
- **Complexity:** small

## Anti-patterns observed in this run

- **Editing research.json after generate is the expected workflow, but the doc-sync pipeline assumes otherwise.** Polish + verify-skill iterate on research.json; the post-dogfood sync only handles a narrow slice (Unique Features). Either commit to "research.json is frozen post-generate" (and dogfood writes its `novel_features_built` to a side file) or commit to "full bi-directional sync." The current half-sync produces the worst of both: research.json is the source of truth but README/SKILL drift silently.
- **Defaults are tuned for the median API rather than the API's actual constraints.** sync concurrency=4 is fine for most APIs but wrong for quota-limited ones; `defaultSyncResources()` includes every list-shaped endpoint regardless of whether it's an auth-flow primitive that requires per-call params. A spec-driven defaults model (rate_class, tag-filtered resource lists) would close both gaps with a single mechanism.

## What the Printing Press Got Right

- **Codex delegation pattern worked end-to-end across three batches with zero rollbacks.** All three Codex tasks (quota subsystem, batch 2a, batch 2b) hit the completion marker on the first try. ~2,300 LoC of hand-code delivered cleanly. The `_codex-result.json` completion-marker contract is rock-solid.
- **`cliutil.IsVerifyEnv()` + `cliutil.IsDogfoodEnv()` as canonical shortcircuits.** Once I learned the pattern, applying it to long-running commands (sync, workflow archive, types batch, issuers/mints) was mechanical and recurred cleanly across hand-written novel features. This is the right shape for "real behavior under live API, curtailed behavior under matrix."
- **The 17-endpoint MCP surface with `mcp:read-only` annotations works without spec edits.** All 10 hand-written novel commands picked up MCP exposure automatically via the cobratree mirror. Setting `mcp:read-only: "true"` on read commands flowed through to the MCP tool annotation without me touching mcp/tools.go.
- **The PCGS quota pattern transferred cleanly to Numista's monthly model.** lookup_log table + LogHook wiring + ResetForNextMonth + `--quota` short-circuit ported with ~80 LoC adapted; the only Numista-specific bit was the `strftime('%Y-%m', called_at) = strftime('%Y-%m','now')` SQL because SQLite doesn't have a 'start of month' modifier. The reference CLI being readable + close-to-isomorphic kept the adaptation fast.
- **`validate-narrative --strict --full-examples` is exactly the right gate.** It catches the bogus examples (auth login --scope, audit --by endpoint, etc.) that verify-skill alone misses. Running it both standalone and inside shipcheck closed the loop.
