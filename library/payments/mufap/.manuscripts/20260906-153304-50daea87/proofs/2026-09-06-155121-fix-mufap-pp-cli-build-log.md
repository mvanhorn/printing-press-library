Manifest transcendence rows: 10 planned, 0 built. Phase 3 will not pass until all 10 ship.

# MUFAP CLI — Phase 3 build log

## Priority 0 — foundation (DONE)
- `internal/mufap/parse.go` — header-driven HTML table parser. One parser serves all five daily
  tabs and the monthly page because every MUFAP table carries a real `<thead>` and every data row
  has exactly len(headers) `<td>` cells. Rows are matched on cell count, so the hidden sibling
  tables MUFAP concatenates into the same markup are skipped rather than mis-keyed.
- `internal/mufap/derive.go` — Percentile/Median, DeriveRate, DeriveDispersion, CheckAllocation.
- `internal/store/mufap_schema.go` — `mufap_obs(resource,date,row_key,payload,observed_at)` plus
  `mufap_coverage(resource,date,row_count,fetched_at)`. Observation rows and the coverage row are
  written in ONE transaction so the ledger can never claim a count the data does not have.
- `internal/cli/mufap_fetch.go` — the single place that owns MUFAP's four date encodings and both
  of its silent-failure shapes.
- `internal/cli/mufap_health.go` — doctor reachability fix (see machine bug 3).
- Tests: 9 test functions across `parse_test.go` + `derive_test.go`, all passing. They encode the
  measured 3-2026 invariant failure and the pre-2024 zero-percent case as real assertions.

## Transport — resolved WITHOUT Surf
`http_transport: browser-http` was set in the spec and silently ignored by the generator (machine
bug 1). That turned out not to matter: MUFAP needs no TLS impersonation at all. A header matrix
over plain Go stdlib showed the whole picture:

    GET  bare UA only ............ 403 challenge
    GET  UA + Accept: */* ........ 403 challenge
    GET  UA + browser Accept ..... 200  916 KB real table
    POST no XHR headers .......... 403 challenge
    POST + X-Requested-With/Origin/Referer ... 200 JSON

One unified header set clears every surface, so the fix went into the spec's `required_headers`
rather than into a hand-patched client — durable across regeneration.

## Machine bugs found (retro candidates, UNFILED)
1. `http_transport: browser-http` in an internal YAML spec is silently ignored by `generate` — no
   Surf dependency added, no warning emitted. The spec asked for a transport and got nothing.
2. `probe-reachability`'s stdlib probe omits a browser `Accept` header, so it classified MUFAP as
   `browser_http` when the site is really `standard_http` plus correct headers. The same
   misdiagnosis had been recorded against SBP on 4 Sep; both are header problems, not bot walls.
3. `doctor` reports an HTML-serving API as **"unreachable"** despite HTTP 200. The
   "expected JSON, API returned HTML" error is not a `*client.APIError`, so the doctor's switch
   falls through to its network-failure branch. This hits every `response_format: html` CLI.
   Worked around here via `mufapHealthGet`, which opts the probe into the HTML-response path.
   After the fix: `doctor` reports OK on all four checks.

## Priority 2 — the 10 transcendence commands
Fanned out one agent per command file under a strict shared contract (exact available APIs, the
verify-friendly RunE shape, the output-mode rule, the missing-mirror guard, the SQLite drain rule,
and the measured MUFAP domain facts), then statically reviewed each against that contract.
Status recorded on completion below.

## Round 1 result — 10/10 implemented, package compiled clean on first assembly
~141 KB of Go across the ten files, and `go build ./...` passed on the first try with no
cross-file collisions. The strict contract (exact available APIs, forbidden generic
package-level names, no concurrent `go build`) is what made a 10-way parallel edit of one Go
package safe.

Adversarial static review of each file against its own contract found **4 blockers, 17 majors,
27 minors** — all line-cited. The review earned its keep: three findings are defects that would
have produced wrong numbers rather than crashes, which is the failure class this project cares
about most.

Highest-value catches:
- **exposure (blocker)** — on context expiry the feeder goroutine dropped every un-dequeued
  job, and the month row was still emitted with `stocks_pkr_million: 0, fund_count: 0`. The
  shipped Example (`--from 2024-01 --to 2026-07`) is ~12,000 POSTs against a 60s default
  timeout, so the headline invocation would reliably have printed fabricated zero months for
  the exact series the command exists to produce.
- **backfill (blocker)** — the allocation month committed unconditionally, so a total-outage
  month was written with `row_count = 0` and rendered as "MUFAP published nothing". A fetch
  failure was being recorded as a legitimate empty, and stickily: the ledger then reported the
  month as done.
- **panel (blocker)** and **freshness (blocker)** — both keyed on `Fund Name`, which the run's
  own measurement had already shown is not unique (49 of 388 rows collide). panel undercounted
  `universe_width` by 12.6%; freshness emitted a blank fund identity on 4 of the 5 tabs,
  because its resolver did a containment test in the wrong direction ("Fund" does not contain
  "fund name").
- **backfill (major)** — a ragged carry could poison a later date in the same range: rows
  carried from date A marked date B fetched, so B's own explicit fetch was skipped and a
  3-row partial view permanently stood in for the ~300-row panel.

Shared resolvers were added to `mufap_fetch.go` so the fixes converge on one implementation
instead of ten: `MUFAPRowName` (probes both header spellings), `MUFAPRowKey`
(Sector|Category|Fund Name — 387 unique of 388), `MUFAPValidityColumn` (handles tab=payout's
"Payout Date"), and `MUFAPTabIsDateFiltered` (false for pricing/ter, which are reference data
returning 551 rows regardless of the requested date).

## Round 2 — fixes, and the bug that mattered most
Fanned out one fix agent per file against its own findings, then re-reviewed adversarially.
Blockers 4 -> 0, majors 17 -> 4. The four survivors were fixed by hand because three were
regressions the fixes themselves introduced, and fanning out again would have risked more:
  - backfill: the commit gate checked only TRANSPORT failures, so three silent paths still
    froze an outage into the ledger as "published nothing" -- an empty AMC roster, a roster
    with no funds, and (the real one) every allocation returning MUFAP's empty-200 shape,
    which is exactly what a rejected parameter or an expired session looks like on that
    endpoint. Gate now also refuses to commit a sampled run.
  - backfill: under the dogfood harness the command samples 1 AMC and 3 funds, and was
    committing that as a whole month into the operator's REAL data.db -- the verification
    matrix poisoning the mirror it was verifying. Verified fixed: dates_stored 0, month routed
    to dates_incomplete with "a sample is not a month".
  - universe: window_covered was false-by-construction, because LoadMUFAPObs already clips to
    [from,to] so the mirror's first/last date can never fall outside the window. Rewired to the
    fetch ledger, which is the only thing that knows a hole from a holiday, and taught to walk
    month-ends for the month-keyed resources instead of fabricating ~29 holes per month.
  - rates: --limit was spending its budget on holes, so a weekend pair could consume the whole
    window and print "no yields in the mirror" against a populated one.

### THE ONE THAT WOULD HAVE CORRUPTED THE RESEARCH
`ParseNumber` did not decode accounting negatives. MUFAP writes every negative in parentheses
and NEVER with a minus sign -- measured 2026-09-04, tab=returns: 96 of 388 YTD cells
parenthesised, 0 with a minus. So every LOSING fund parsed as "did not report" and silently
left the cross-section. The bias is not random; it removes exactly the left tail.

    equity YTD dispersion, 2026-09-03
      before:  fund_count 17 of 91   median +3.15
      after:   fund_count 91         median -3.70

The reported sign of the equity market was inverted. Money-market rows are almost never
parenthesised (2 of 120), so the short-rate proxy was untouched -- which is why the SBP-cycle
validation still held and would NOT have caught this. It was caught by printing a raw row and
reading it. Fixed in mufap.ParseNumber with regression cases for "(4.97)", "(1,236.28)",
"(3.10)%", "()" and "(N/A)".

## Phase 4 final
  verify PASS | validate-narrative PASS | dogfood PASS | workflow-verify PASS
  apify-audit PASS | verify-skill PASS | scorecard HOLD (live_api_verification unverified only)
  Scorecard 85/100 Grade A.  Live sample probe 10/10, 100%.
  Dead Code 1/5 -> 5/5 (4 unused generator helpers removed).
  MCP description quality 0/10 -> 5 thin descriptions resolved via the spec.

Also fixed, all real drift that would have shipped:
  - research.json examples used a resource key that never existed ("export daily",
    "coverage --resource daily"; the resource is "daily-returns"). Both failed with exit 2.
  - root.Short had drifted from narrative.headline.
  - export gained --format json: jsonl correctly emits nothing for an empty mirror, which left
    an agent with nothing to parse, so the json form answers with a provenance envelope
    {"resource":..,"from":..,"to":..,"rows":[],"count":0} -- an empty result that still says
    WHAT is empty.

## Machine bugs (retro candidates, UNFILED) — now 5
1. `http_transport: browser-http` silently ignored by generate.
2. `probe-reachability` stdlib probe omits a browser Accept header -> false browser_http.
3. `doctor` calls an HTML-serving API "unreachable" on HTTP 200.
4. dogfood's novel-feature depth check compares the advertised FULL PATH against Cobra's leaf
   Name(), so any novel feature nested under a parent reports
   'advertised as "verify allocation" but registered as "allocation"'. Any nested novel
   command trips this.
5. scorecard --live-check SIGBUSes intermittently when the CLI binary is rebuilt underneath it
   (mmap of a file replaced mid-read). Transient, but it surfaces as a bogus probe failure.
