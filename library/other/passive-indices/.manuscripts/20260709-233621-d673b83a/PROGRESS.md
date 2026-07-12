# passive-indices-pp-cli — Run Progress (for context resume)

## Run identity
- RUN_ID: `20260709-233621-d673b83a`
- RUN dir: `/Users/mayanklavania/printing-press/.runstate/cli-printing-press-00dee511/runs/20260709-233621-d673b83a`
- CLI_WORK_DIR: `$RUN/working/passive-indices-pp-cli`
- Binary (built): `$CLI_WORK_DIR/passive-indices-pp-cli`
- PRINTING_PRESS_BIN: `/Users/mayanklavania/projects/cli-printing-press/cli-printing-press` (repo-mode local build, v4.28.0)
- PRESS_LIBRARY: `/Users/mayanklavania/printing-press/library` — **PROMOTED** to `library/passive-indices` (Phase 5.6 done).
- Lock: released by `lock promote`.
- Model: session started Opus 4.8, briefly touched Sonnet 5, back on Opus 4.8. Two unrelated transient infra outages occurred (Bash safety classifier once; write-tool classifier once) — both resolved by retrying on a later turn. Not a recurring issue, no special handling needed going forward.

## What this CLI is
Combo CLI unifying two complementary Indian passive-investing data sources:
- **niftyindices.com** (PRIMARY, per user) — NSE's official index provider. Live levels + constituents + full historical (OHLC/TRI/PE-PB-DivYield). All no-auth.
- **indiapassivefunds.com** (SECONDARY/complementary) — ETFs & index funds that track those indices. Full REST API, credential-less runtime-minted Bearer token.
- **bseindices.com** — explicitly deprioritized/out of scope this run per user instruction.

Source priority confirmed by user: niftyindices leads; indiapassivefunds is complementary (indices vs. the funds tracking them); bseindices deferred.

## Pipeline phase status
- [x] Phase 0 (resolve/reuse) — no prior research, no catalog entry, no library/lock conflicts, no public-library registry/blocked-journal match.
- [x] Phase 0.5 (API key gate) — skipped, no user auth needed for either source.
- [x] Phase 1 (research brief) — written and iterated twice (see Key Discoveries below for the two scope-changing corrections).
- [x] Phase 1.6/1.7 (browser-sniff gate) — approved by user, but actual discovery was done via direct HTTP/JS-bundle-mining rather than a live browser (agent-browser hit `ERR_HTTP2_PROTOCOL_ERROR` against indiapassivefunds' Chrome-for-testing fingerprint; pivoted to curl + JS bundle extraction, which fully succeeded). Marker file written: `browser-browser-sniff-gate.json`.
- [x] Phase 1.5 (absorb + novel-features subagent) — subagent ran once cleanly (agentId a8a094b2c245be4bb), returned 8 survivors. **NOTE:** a stray "still working?" check-in message I sent to the same agent later caused it to silently redo the whole brainstorm from scratch with DIFFERENT personas/commands — that second output was correctly discarded; only the FIRST result is reflected in the manifest. Do not re-run this subagent.
- [x] Phase Gate 1.5 — user approved full manifest (13 absorbed + 9 novel = 22 features), after two rounds of scope revision (see Key Discoveries).
- [x] Phase 1.9 (reachability) — effectively satisfied via direct verification during research; not run as a separate mechanical gate but every endpoint was hit live and confirmed working before generation.
- [x] Phase 2 (generate) — ran clean, all 8 quality gates passed. Spec: `research/passive-indices-spec.yaml` (only 2 generated endpoints: `index live`, `index constituents`; everything else hand-coded — see below).
- [x] Phase 3 (build) — DONE. All 22 manifest commands have real implementations (not stubs) and build clean. Completion Gate passed (all commands resolve via `--help`, exit 0). Build log: `proofs/20260711-140207-fix-passive-indices-pp-cli-build-log.md`.
- [x] Phase 4 (shipcheck) — DONE, verdict PASS (7/7 legs). One real bug found and fixed along the way (see SESSION UPDATE 2026-07-11 below).
- [ ] Phase 4.7-4.95 (sync-param-drop, agentic SKILL review, README/SKILL audit, output review, local code review) — NOT STARTED.
- [ ] Phase 5 (dogfood/live smoke) — NOT STARTED. No API key needed for either source; this should be MANDATORY full dogfood per the skill (no-auth APIs are always testable). NOTE: indiapassivefunds.com is currently showing live instability (timeouts, 500/502s) — see SESSION UPDATE 2026-07-11; expect some flaky live-smoke results from that source and don't chase them as CLI bugs unless they reproduce against our own request construction.
- [x] Phase 5.5 (polish) — DONE. `printing-press-polish` skill run, scorecard 76→77/100, `ship_recommendation: ship`, `further_polish_recommended: no`.
- [x] Phase 5.6 (promote to library) — DONE. `lock promote` succeeded, `library/passive-indices`, lock released.
- [ ] Phase 6 (next steps / publish offer) — NOT STARTED. Manuscripts not yet archived to `$PRESS_MANUSCRIPTS/`.

## Why the spec only covers 2 endpoints (everything else is hand-coded)
Documented in the absorb manifest's "Auth note": indiapassivefunds' auth is a credential-less runtime-minted Bearer token minted on a DIFFERENT host (`www.indiapassivefunds.com`) than the data API (`data.indiapassivefunds.com`) — doesn't fit any spec `auth.type`. niftyindices' historical endpoints need a `cinfo` field containing an escaped JSON string composed from other params — not expressible in the spec body DSL. Per AGENTS.md's custom-auth-flow pattern, the entire fund layer + niftyindices historical layer is hand-written with sibling clients.

Spec file (`research/passive-indices-spec.yaml`) only declares:
- `index live` → GET https://iislliveblob.niftyindices.com/jsonfiles/LiveIndicesWatch.json (no auth)
- `index constituents <slug>` → GET https://www.niftyindices.com/IndexConstituent/ind_{slug}list.csv (no auth)

## Sibling client packages (hand-written)
- `internal/niftyindices/client.go` — historical OHLC/TRI/PE-PB-DivYield via `POST https://www.niftyindices.com/BackPage/{method}` (NOT `/Backpage.aspx/` — that's gated, see Key Discoveries). Also has `Constituents()` (CSV) and `LiveWatch()` (duplicates the generated endpoint but as a typed Go call for cross-command joins) and `Slugify()`.
- `internal/indiapassivefunds/client.go` — token mint (`POST /pages/api/login`, body `{}`, no creds) + cached Bearer attach + retry-once-on-401. Typed methods: `Dashboard`, `ScreenerFilters`, `SymbolLookup`, `FundDetail` (deep nested parser — see Key Discoveries), `NFO`, `Screen`, `TimeSeries`, `FundCompare`, `MarketRankings`. Plus `FindUnderlyingIndexValue` and `FindAMCValue` — critical helpers that resolve human names (e.g. "NIFTY 50", "HDFC") against server-enumerated `{text,value}` taxonomies from `screeners/filters`. **Both AMC and underlyingIndex screener params MUST go through these resolvers — passing raw strings causes `"Internal Error"` from the upstream API.**

## Command inventory (all 22 manifest features + framework)
All under `internal/cli/`, wired into `index.go` (parent) and `fund.go` (parent) and `root.go` (for top-level `compare`).

**Generated (2):** `index live`, `index constituents` (from spec.yaml, standard generated code — not touched).

**Hand-coded absorbed (11):** `index_history.go`, `index_tri.go`, `index_valuation.go`, `index_list.go`, `fund_get.go`, `fund_search.go`, `fund_screen.go`, `fund_timeseries.go`, `fund_rankings.go`, `fund_compare_list.go` (multi-fund `fund compare`), plus the shared `siblingclients.go` + `hybrid_store.go` helpers.

**Hand-coded novel/transcendence (9, all scored ≥5/10 by subagent):**
1. `index_funds.go` — `index funds <index>` — join via `resolveIndexTrackers()` in `index_fund_join.go` (uses `FindUnderlyingIndexValue` + `Screen`)
2. `index_tracking.go` — `index tracking <index>` — ranks by expense ratio, undisclosed(0) sorted last
3. `index_cheapest_tracker.go` — `index cheapest-tracker <index>` — excludes undisclosed(0) entries from winner, reports them separately as `undisclosed_expense_ratio`
4. `index_constituents_diff.go` — `index constituents-diff <index> --since <dur>` — self-building snapshot history in generic `resources` table (resourceType `index_constituent_snapshot`, id = `<slug>:<RFC3339>`), first run always says "no baseline yet"
5. `index_sectors.go` — `index sectors <index>` — COUNT-based industry breakdown (constituent CSV has NO weight column, only Company/Industry/Symbol/Series/ISIN — documented correction, see Key Discoveries)
6. `fund_nfo_tracking.go` — `fund nfo tracking <index>` — best-effort fund-NAME substring match (NFO listing has no underlying-index field — documented caveat in Long description)
7. `fund_raw.go` — `fund raw <schemeId>` — `decodeFieldCodesDeep()` recursively resolves field codes (f_29 etc) to displayNames throughout the ENTIRE nested raw response, not just top level
8. `compare.go` (root-level) — `compare <schemeId> <index>` — fund detail + live index quote + constituent sample side by side
9. `index_tracking_error.go` — `index tracking-error <schemeId> --tenure` — uses indiapassivefunds' OWN disclosed monthly Tracking Error/Difference ratios (real reported figures — see Key Discoveries), NOT a synthesized NAV-vs-index computation

## Key Discoveries (things a fresh context MUST know to avoid redoing wrong work)

1. **niftyindices historical endpoints: path is `/BackPage/` (capital P, NO `.aspx`), not `/Backpage.aspx/`.** The `.aspx` variant 302-redirects to Sitefinity login (`WWW-Authenticate: Bearer`) — genuinely gated, legacy/deprecated. User supplied this fix mid-run after I'd initially given up on historical data as a "documented gap". This is why the manifest went from 18→22 features across two revisions — DO NOT re-introduce a "historical data is gated" gap; it is NOT gated via the correct path.

2. **Constituent CSV has NO weight column.** Columns are: `Company Name, Industry, Symbol, Series, ISIN Code`. No weight/percentage field exists anywhere in this endpoint. `index sectors` is therefore COUNT-based (share of constituents per industry), not weight-based. Do not claim weight-based aggregation anywhere in docs/README.

3. **indiapassivefunds' `funddetail` response is deeply, inconsistently nested** — NOT a flat field-coded list like other endpoints (`nfo`, `screeners`, `symbollookup` ARE flat `{columns, data}` envelopes — only `funddetail` is special). Sections: `header`, `funddescription` (mid-array embeds `section1`/`section2` sub-objects with their OWN columns/data), `fundamentals` (label/value/asof triplets `{f_01,f_02,f_03}`, NOT field-coded), `ratios` (real monthly-disclosed Tracking Error/Tracking Difference/Total Expense Ratio — genuinely valuable, don't recompute this), `assetholding`, `sectorholding`, `portfolio`, `similarfunds`, `filterdata`. The `FundDetail()` client method has bespoke parsers for each (`firstDataRow`, `fieldByDisplayName`, `labelValueTriplets`, `latestRatiosRow`, `deepFindBenchmarkIndex`, `namedPercentRows`, `parseSimilarFunds`).

4. **`underlyingIndex` and `amc` screener params are server-enumerated `{text,value}` taxonomies from `screeners/filters`, NOT free-text strings.** e.g. "NIFTY 50" → look up "Nifty 50 TRI" → value `320`. Passing a raw name string causes silent `"Internal Error"` from upstream. `FindUnderlyingIndexValue` and `FindAMCValue` in the client handle this. **If you add any NEW screener-filter-based command, check `screeners/filters` first — do not assume a param is free text.**

5. **A `0` expense_ratio from `funddetail.ratios` means "not yet disclosed", not "free".** Fixed bug: `index cheapest-tracker` was initially picking undisclosed(0%) funds as "cheapest". Now excludes them from the winner and reports separately. `index tracking` sorts them to the end instead of first. **Apply this same "0 = undisclosed" caution to any other ratio-derived ranking you might add.**

6. **`fund` NFO listing has no clean underlying-index field** — only Name/CategoryName/SchemeType/Riskometer/MinSubscription/OpeningDate/ClosingDate/fund_id. `fund nfo tracking` uses fund-name substring matching as a documented best-effort fallback (Indian index funds/ETFs near-universally embed the tracked index name in their fund name, e.g. "Mirae Asset Nifty200 Momentum 30 Plus...").

7. **`market rankings --command <value>`: no working enum value was found.** Tried `topAUM`, `aum`, `AUM`, `top-aum`, `topaum`, `nav`, `most_active`, `gainers`, `losers` (last two timed out, possibly rate-limited, not confirmed working). Left as a REQUIRED user-supplied flag with no default — documented honestly, not guessed. This is the one absorbed feature (#8, `fund rankings`) that may need a real working example found before Phase 4/5 dogfood, or should be flagged in shipcheck as needing a `pp:happy-args` annotation once a working value is found, or accepted as a documented limitation.

8. **agent-browser could not load indiapassivefunds.com** (`ERR_HTTP2_PROTOCOL_ERROR` via Chrome-for-testing) — pivoted to direct `curl --http2`/`--http1.1` + JS bundle mining (`_next/static/chunks/*.js`) instead. This is why discovery evidence in the brief cites curl probes and bundle greps rather than a HAR capture. This is fine/expected — browser-sniff gate was approved but the actual successful method was direct API reverse-engineering, which the gate's disclosure language covers ("agent may run browser-use, agent-browser, ask for HAR... or fall back").

## Known Fixed Bugs (already resolved, don't re-fix)
1. `index cheapest-tracker`/`index tracking`: undisclosed (0%) expense ratio ranking as cheapest → FIXED (excluded/sorted-last).
2. `fund screen --amc "HDFC"` → `"Internal Error"` because AMC needs numeric taxonomy value, not raw string → FIXED (`FindAMCValue` resolver added, `ScreenParams.AMC` changed `string`→`any`).
3. `fund rankings --command topAUM` (the documented example) → upstream `"Requested ranking doesnt exist"`. No working `--command` value could be found (JS-chunk mining hung indefinitely against indiapassivefunds — same anomalous-for-tooling behavior as Key Discovery 8, confirmed again via direct curl `--max-time` timeouts on `_next/static/chunks/*.js`). FIXED the misleading part only: help text/example no longer claims `topAUM` works; now documents that indiapassivefunds does not publish its accepted ranking values and a live rejection is the real API's answer, not a CLI bug. The flag itself stays required with no default (unchanged design).
4. **MACHINE BUG (generator-level, fixed in the main cli-printing-press repo, not just this printed CLI):** `which_test.go.tmpl`'s `TestWhichIndex_ExistsAndIsWellFormed` asserted `root.Find(strings.Fields(e.Command))` resolves with zero remaining args — but `NovelFeature.Command` is documented/rendered elsewhere (README/SKILL templates) to legitimately include usage placeholders like `<index>`, `--since <date>`, `--tenure 1y`. Any generated CLI with a novel feature taking positional args or flags would fail this test. Fixed by adding a `whichCommandPath()` helper to `which_test.go.tmpl` that strips trailing placeholder/flag tokens before calling `root.Find`, so it validates the resolvable command prefix instead of the literal doc string. Applied to both `internal/generator/templates/which_test.go.tmpl` (source of truth) and the already-generated `$CLI_WORK_DIR/internal/cli/which_test.go` (kept in sync manually rather than re-running `generate --force`). Verified: `go test ./...` passes in both the printed CLI and confirmed via `scripts/golden.sh verify` (32/32 pass, no regression) in the main repo. **This fix has NOT been committed in the main repo yet — still working tree changes.**

## Live smoke-test results so far (all against real APIs, no mocking)
✅ Working confirmed: `index list`, `index funds`, `fund get`, `index tracking-error`, `index cheapest-tracker` (post-fix), `index sectors`, `fund nfo tracking`, `fund raw`, `compare`, `index history`, `index tri`, `index valuation`, `index constituents-diff` (first-run "no baseline" path only — the diff-found path is UNTESTED, needs a second run after `--since` elapses or a shorter `--since` value), `fund search` (returned `[]` for "gold" — confirmed via direct curl this is a genuine zero-match, not a bug), `fund screen --amc "HDFC"` (post-fix), `index tracking` (full ranked list, ascending by expense ratio, confirmed working), `fund timeseries` (raw pass-through is correct — plain field names, not the columns/data field-code pattern), `fund compare` (3-fund multi-compare confirmed working), `fund screen --underlying-index` (re-verified post-AMC-fix, still works).

❌ NOT yet smoke-tested: `index constituents-diff`'s diff-found path (needs a second run over time — deferred to Phase 5 dogfood or later).
⚠️ Known limitation (documented in help text, not a bug to fix): `fund rankings --command <value>` has no confirmed-working value; upstream API doesn't publish its enum and JS-chunk mining to find it consistently times out.

## SESSION UPDATE (2026-07-10): Phase 3 Completion Gate done, plus a real incident + 2 machine-level bugs found and fixed

**All 21 manifest commands now resolve cleanly via `--help`** (verified individually — remember zsh does NOT word-split unquoted `$var`, use a zsh array `${(z)c}` or bash for loop testing).

**MACHINE-LEVEL INCIDENT (important, read before touching dogfood again):** Running `cli-printing-press dogfood` triggered its auto-doc-sync feature, which computes `novel_features_built` by matching each `research.json` novel feature's `Command` string (e.g. `"index funds <index>"`) against the real Cobra tree. The matcher's `commandPath()` helper (`internal/pipeline/dogfood.go`) stripped flag tokens (`-`-prefixed) but NOT placeholder tokens (`<...>`-prefixed), so EVERY novel feature failed to match, `novel_features_built` was written as empty, and dogfood auto-synced (wiped) `which.go`, `root.go` Highlights, README.md Unique Features, SKILL.md Unique Capabilities, `tools.go` MCP capabilities, and `.printing-press.json` novel_features — all down to zero. This is the SAME bug class as `which_test.go.tmpl`'s test (see below), just in a different, more destructive place (auto-write instead of read-only test assertion) — I hit both in the same session before realizing they were one root cause.
- **Recovery:** no git repo in `$CLI_WORK_DIR`, so recovery was manual reconstruction (I had the original which.go content in my own context from earlier in the session). Fixed the actual bug (below), reran dogfood, and it correctly re-synced everything with 9/9 features found — restoring all wiped files correctly, including my wording fixes.
- **Lesson for next time:** if `dogfood` output ever says `N/N novel features missing` when you know they're built and resolve fine via `--help`, STOP before it writes anything — that's a detection bug, not a real gap. Check `commandPath()` / `matchNovelFeature()` in the generator first.

**MACHINE FIX #1** — `internal/pipeline/dogfood.go:856` `commandPath()`: added `|| strings.HasPrefix(t, "<")` to the token-stopping condition (previously only stopped at `-`-prefixed flag tokens). This is the root cause of the incident above. Added regression test `TestCommandPathStripsPlaceholdersAndFlags` in `dogfood_test.go`.

**MACHINE FIX #2** — `internal/generator/templates/which_test.go.tmpl`: `TestWhichIndex_ExistsAndIsWellFormed` had the identical bug independently (asserted `root.Find(strings.Fields(e.Command))` resolves with zero remaining args, which is impossible for any novel feature with a positional arg/flag documented in its `Command` string — a legitimate, template-sanctioned convention per `readme.md.tmpl`/`skill.md.tmpl` rendering `.Command` verbatim with placeholders). Added a `whichCommandPath()` helper to the test template that strips placeholder/flag tokens before calling `root.Find`. Applied to both the template (source of truth) and manually kept the already-generated `$CLI_WORK_DIR/internal/cli/which_test.go` in sync (didn't re-run `generate --force` to avoid disturbing hand-coded Phase 3 work).

**Verification of both machine fixes:** `go build`, `go test ./...` (full, not scoped), and `scripts/golden.sh verify` (32/32 pass) all clean in the main `cli-printing-press` repo. **Both fixes are UNCOMMITTED working-tree changes in `/Users/mayanklavania/projects/cli-printing-press`** — not committed, not pushed. Recommend committing separately from any passive-indices-pp-cli work (they're generalizable machine fixes, unrelated in spirit to this one CLI).

**IMPORTANT — unrelated pre-existing WIP discovered in the main repo:** while investigating, found (and briefly stashed/restored) two OTHER uncommitted working-tree changes not from this session: `internal/generator/generator.go` (`exampleLine` kebab-casing fix) and `internal/pipeline/live_dogfood.go` (`joinShellContinuationLines` for multi-line shell examples). These are NOT mine and I did not evaluate or fix them. They currently cause `scripts/golden.sh verify` to report `missing artifact for generate-golden-api-rich-auth: printing-press-rich-auth/internal/cli/auth.go` (1 failing case) when present alongside my 2 fixes — confirmed via isolation (stashing them out made golden pass 32/32; my fixes alone are clean). **Flag this to the user before committing anything in the main repo** — don't silently commit or discard this other WIP.

**Doc-accuracy fixes (this CLI only):** Found and fixed stale "constituent weights" language left over from before the "no weight column in the CSV" discovery (Key Discovery 2) — it had leaked into `index_constituents.go`'s generated Short text, `index_constituents_diff.go`'s Short text, `passive-indices-spec.yaml`, and `research.json` (novel feature descriptions/rationale, `when_to_use`, a recipe explanation). Fixed all of them to say "count-based" / "constituent list (additions/removals)" instead of "weights". Also fixed the misleading `fund rankings --command topAUM` example (upstream rejects it; API doesn't publish its accepted values) — help text now says so honestly instead of presenting a non-working example.

**Reimplementation-check false positives (fixed):** dogfood's `reimplementation_check` flagged 6/9 novel features as "hand-rolled response: no API client call" because its structural heuristic only recognizes calls to the generated `internal/client` package, not hand-written sibling packages (`internal/niftyindices`, `internal/indiapassivefunds`) called through a package-local `newXClient()` helper. This is a known, documented gap with a sanctioned escape hatch (`// pp:client-call`, see `AGENTS.md` Anti-reimplementation section and `skills/printing-press/references/aggregator-pattern.md`) — NOT something to fix in the generator. Added the marker to `index_funds.go`, `index_cheapest_tracker.go`, `fund_raw.go`, `index_tracking_error.go`, `fund_nfo_tracking.go`, `index_tracking.go`.

**New tests added (closes the `test_presence` gap):** `internal/niftyindices/client_test.go` (Slugify, truncate, cinfoBody double-JSON-encoding shape) and `internal/indiapassivefunds/client_test.go` (15 test functions covering `Decode`, `parseListEnvelope`, `FindUnderlyingIndexValue`/`FindAMCValue` matching logic, and all the `funddetail` deep-parsing helpers: `firstDataRow`, `fieldByDisplayName`, `rawToString`/`rawToFloat`, `deepFindBenchmarkIndex`, `labelValueTriplets`, `latestRatiosRow`, `namedPercentRows`, `parseSimilarFunds`). No testify — matched this CLI's existing stdlib-only test convention (which_test.go).

**Final dogfood state:** `novel_features_check: 9/9 found`. `reimplementation_check: 9/9 exempted (1 store, 6 client-directive, 2 clean)`. `test_presence: clean, no missing_tests`. `verdict: WARN` — sole remaining issue is `defaultSyncResources empty` (generated `sync.go`, DO NOT EDIT — a legitimate generator default for a CLI whose only 2 spec-declared endpoints are a live-snapshot blob and a param-required CSV, neither a natural bulk-sync target; accepted as a documented, non-blocking design characteristic per AGENTS.md "don't change the machine for one CLI's edge case").

## SESSION UPDATE (2026-07-11)

**Build log written:** `proofs/20260711-140207-fix-passive-indices-pp-cli-build-log.md`.

**Phase 4 shipcheck: ran, found and fixed one real bug, now PASS (7/7 legs).**

First run FAILED on `validate-narrative`: the quickstart example
`sync --resources indices,funds` errored with `unknown sync resource
"indices"` (exit 1, both resources errored). Root cause: `knownSyncResourceNames()`
/ `syncResourcePath()` in the generated `sync.go` only recognize a single
resource, `"index"` (singular — maps to niftyindices' `LiveIndicesWatch.json`
snapshot). There is no `"funds"` sync resource at all — indiapassivefunds
data is never bulk-synced; every fund-layer novel command calls
indiapassivefunds live per-invocation. The quickstart's plural
`indices,funds` was simply wrong on both counts. Fixed in `research.json` and
`README.md`: `sync --resources index`, comment corrected to "Populate the
local store with NSE's index list (fund data is fetched live per command,
not synced)". Re-ran shipcheck: PASS 7/7.

**Found but NOT fixed (out of scope, flagging as a retro candidate):**
`sync.go`'s own `--help` Examples block still shows a generic
`sync --resources channels,messages` line. Traced to
`internal/generator/templates/sync.go.tmpl:160` (and the graphql variant) in
the main repo — a hardcoded generic placeholder example, not derived from
the actual spec's resource names. Cosmetic only (doesn't break anything,
`sync.go` is a DO-NOT-EDIT generated file so not hand-patched here), but
worth a machine-level fix later: the template should either omit a
resource-name example when it can't derive a real one, or use the actual
first declared resource name.

**Scorecard's live Sample Output Probe (5/9 pass) surfaced indiapassivefunds.com
live instability, not a CLI bug:** "Tracking fidelity report" / "Cheapest
tracker finder" timed out after 10s; "Field-code decoder" / "Rolling tracking
error" got upstream `Internal Error` (HTTP 5). Reproduced directly:
`fund search "nifty 50" --json` alternately returned `[]` (empty, valid
JSON), a `token mint returned HTTP 500` wrapping an upstream Axios `502`
(indiapassivefunds' own backend erroring, leaking their Next.js server file
paths in the error body), and eventually `context deadline exceeded` calling
`data.indiapassivefunds.com/api/v1/etf/symbollookup` directly. This matches
the SAME site's documented anomalous-for-tooling behavior already recorded
above (Key Discovery 8 / the `fund rankings` JS-chunk-mining timeouts) — not
a new finding, not our request construction. Per prior session's own
decision, not over-investing further here. **For Phase 5 dogfood: expect
similar flakiness from indiapassivefunds-backed commands; treat timeouts/5xx
from that host as the site's current instability, not a regression, unless
the SAME request reproducibly fails against a healthy response elsewhere
(e.g., a malformed request we're sending).**

**Doc audit (README/SKILL) done:** swept for stale "weight"/`topAUM`/
`channels,messages` residue (none left — the "weight" hits found are the
already-correct post-fix wording). Fixed one boilerplate-troubleshooting
line in `README.md` ("Run the `list` command to see available items" — this
CLI has no `list` command; replaced with `index live` / `fund search`
pointers). The generic `list`-command line is itself template boilerplate
(`readme.md.tmpl:652-653`, same class as the `sync.go.tmpl` placeholder
above) — a second retro candidate, not fixed at the template level this
session.

**Phase 5 live dogfood (mandatory, `--live --level full`): first run FAILED
7/111 tests, all real bugs or expected limitations — fixed 4, accepted 1
(known), left the substring-match one as correct-as-is:**

1. **`index history` / `index tri` / `index valuation` error_path FAIL** — an
   unknown index name silently returned `[]` with exit 0 instead of erroring.
   Root cause: niftyindices' BackPage historical endpoints return HTTP 200 +
   empty array for both an unknown index name AND a valid name with no data
   in range — no way to tell them apart from that response alone. **Fixed**
   in `internal/niftyindices/client.go`: added `rejectIfUnknownIndex()`,
   called only when a fetch returns zero rows, which cross-checks the name
   against the live `LiveWatch()` index list (real data, no hardcoded list)
   and returns a real error only when the name genuinely isn't published.
   Wired into `History`, `TRI`, `Valuation`. Verified: bogus name -> exit 5
   with a clear message; valid name still returns real data (no regression).
2. **`fund timeseries` error_path FAIL** — a non-numeric/invalid schemeId
   upstream-returns `{"status":false,"message":"Internal Error","response":null}`
   but the CLI printed it as if successful (exit 0). Root cause:
   `Client.TimeSeries` in `internal/indiapassivefunds/client.go` returned the
   raw envelope unchecked, unlike every other envelope-returning method in
   that file (`parseListEnvelope`, `ScreenerFilters`, etc.), which all check
   `status` first. **Fixed:** added the same status check; still returns the
   full raw envelope (header/types/period) on success since the command
   prints it whole, but now rejects `status:false` as an error. Verified:
   bogus id -> exit 5 with the upstream message; valid id unchanged.
3. **`fund nfo tracking` error_path FAIL — assessed as correct-as-is, not a
   bug.** This command does a fund-name substring match (NFO listings have
   no underlying-index field), so an unknown index and a known index with
   zero upcoming NFOs are genuinely indistinguishable — exit 0 with an empty
   `matching_nfos` array is the honest answer either way. **What WAS wrong:**
   `research.json`'s rationale text claimed a field-level join ("Filters...
   by its underlying-index field") that contradicts the actual substring-match
   implementation (and the command's own `match_method` string). Fixed the
   rationale text in `research.json` (2 occurrences) to describe the real
   substring-match approach instead of a nonexistent field join. Also
   annotated the command with `cmd.Annotations["pp:no-error-path-probe"] =
   "true"` (the sanctioned dogfood opt-out for exactly this "HTTP 200 +
   empty success envelope for unknown input" shape — see
   `skills/printing-press/SKILL.md`'s "Dogfood error-path opt-out" section),
   so the live matrix stops treating this as a failure.
4. **`fund rankings happy_path`/`json_fidelity` — RESOLVED by removing the
   command entirely, per explicit user decision.** Root problem:
   indiapassivefunds' `marketrankings` endpoint requires a `--command` enum
   value it never documents anywhere (confirmed unfindable across two
   sessions, including JS-bundle mining that timed out against the live
   site). No code fix can supply a value that doesn't exist, and the
   generator's live-dogfood harness has no sanctioned "no example exists"
   escape hatch for happy_path (only `pp:no-error-path-probe` for
   `error_path`, and an HTTP-400/422-specific fixture-skip that doesn't match
   this API's HTTP-200-with-app-level-`status:false` failure shape — both
   confirmed by reading `internal/pipeline/live_dogfood.go` directly). I
   first tried softening the command's `Example:` text from a fake runnable
   value to a `#`-prefixed comment (avoids sending literal garbage like
   `<ranking-command>` to the live API) — this cut the failure count from
   2 to 1 but couldn't reach 0 (the harness's own "missing runnable example"
   check still counts as a happy_path failure). Presented the tradeoff to
   the user directly (manually write a pass marker with documented rationale
   vs. drop the feature vs. stop for review) — **user chose to drop the
   feature.** Removed: `internal/cli/fund_rankings.go` (deleted),
   `newFundRankingsCmd` registration in `fund.go`, and the unused
   `MarketRankingsParams`/`MarketRankings` in
   `internal/indiapassivefunds/client.go`. Confirmed no test files, README,
   SKILL.md, or manifest referenced it (it never surfaced past its own
   command file + the `fund.go` registration). `go build`/`go vet`/`go test
   ./...` all clean after removal. **This drops the CLI from 22 to 21
   manifest features (13→12 absorbed + 9 novel unchanged).**

**Final live dogfood result after the fix: 107/107 passed, 0 failed, 0
skipped-as-failure. `phase5-acceptance.json` written with `status: "pass"`.**

## Phase 5.6: promoted

`lock promote --cli passive-indices-pp-cli --dir "$CLI_WORK_DIR"` succeeded:
`library_dir: /Users/mayanklavania/printing-press/library/passive-indices`,
`promoted: true`. Lock released.

All 4 code fixes verified: `go build` clean, `go test ./...` clean (all
packages), and manual live re-checks of both bogus and valid inputs for each
fixed command show no regressions.

**Re-ran the full live dogfood matrix: 108/111 passed, 3 failed (down from
7) — both remaining failures confirmed accepted, not bugs:**
- `fund rankings happy_path` + `json_fidelity` (2 failures, exit 5) — the
  already-documented enum-value limitation (indiapassivefunds doesn't
  publish accepted `--command` values).
- `fund nfo tracking error_path` (1 failure) — confirmed by direct
  re-invocation still exits 0 with an empty `matching_nfos` array for a
  bogus index name. This is correct-as-is per the analysis above: fuzzy
  substring matching cannot distinguish "index doesn't exist" from "index
  exists, zero upcoming NFOs" — both are honestly empty results, not an
  error condition. Not fixed; documented here as the reason this one live
  test is expected to always show non-zero-exit-required as unmet.

`phase5-acceptance.json` written: `status: fail`, `tests_passed: 108`,
`tests_failed: 3`, `failure_summary.commands: ["fund nfo tracking", "fund
rankings"]`. Both are understood, accepted limitations (not deferred bugs),
consistent with proceeding to Phase 5.5.

**Still open from prior session, unchanged:**
- Unrelated pre-existing WIP in the main repo (`generator.go`, `live_dogfood.go`)
  — still untouched, still not evaluated, still flagged for the user before
  anyone commits in that repo.
- The 2 machine-level dogfood fixes (`dogfood.go` commandPath,
  `which_test.go.tmpl`) — still uncommitted, validated, awaiting a commit
  decision.
- Two more machine-level retro candidates found this session (both cosmetic,
  neither fixed at the template level): the hardcoded `sync --resources
  channels,messages` example (`sync.go.tmpl`/`graphql_sync.go.tmpl`), and the
  hardcoded `Run the \`list\` command` troubleshooting line
  (`readme.md.tmpl:652-653`) that assumes every CLI has a `list` command.

- [x] Phase 5 (dogfood/live smoke) — DONE. Final state: 107/107 live tests
  pass, 0 failed, after removing `fund rankings` (see below) — the CLI now
  ships with 21 of the original 22 manifest features (12 absorbed + 9 novel).

## Immediate next steps (in order)
1. Archive manuscripts to `$PRESS_MANUSCRIPTS/passive-indices/$RUN_ID/`.
2. Phase 6 next-steps menu (ship-path, since no hold conditions expected;
   mention the manifest dropped to 21 features and why).
3. Before wrapping the whole run: surface the still-open main-repo items to
   the user (unrelated WIP in `generator.go`/`live_dogfood.go`, 2 uncommitted
   machine fixes in `dogfood.go`/`which_test.go.tmpl`, 2 new cosmetic retro
   candidates in `sync.go.tmpl`/`readme.md.tmpl`) — do not silently commit or
   discard any of it.

## Files to read on resume (in priority order)
1. This file.
2. `$RUN/research/2026-07-09-233621-feat-passive-indices-pp-cli-brief.md` — final research brief (post both revisions).
3. `$RUN/research/2026-07-09-233621-feat-passive-indices-pp-cli-absorb-manifest.md` — final approved manifest (22 features).
4. `$RUN/research.json` — narrative/novel_features source of truth for README/SKILL generation.
5. `$RUN/discovery/discovery-notes.md` — raw endpoint discovery notes (final, post-BackPage-fix version).
6. `internal/indiapassivefunds/client.go` and `internal/niftyindices/client.go` — the two sibling clients, read before touching any command that uses them.
