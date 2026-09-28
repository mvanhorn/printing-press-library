# Verification and limits

Checked on 2026-09-27 UTC (September 27–28 in Japan) against Jalan's anonymous Japanese accommodation pages. This local CLI requires no API key or paid data service. Availability and prices are observations, not booking guarantees.

## Live source checks

The final source-checked runner (`live-e2e-acceptance.json`) passed 37/37 cases: 42 CLI invocations, 16 CLI upstream requests and 10 independent read-only source requests. It covered a regional ryokan (385995), an onsen property (371898), an urban hotel discovered through the source filter (379411), dated offers, exact plan/room pairs, two-room family occupancy, meals/smoking filters, logical pagination with an explicit shared cache, bounded comparisons and cache freshness.

Facts were checked against independently fetched source DOM: property identity, Japanese names, access/amenity text, all seven review category labels, room/plan IDs, meals, smoking, charge totals, fees and source query echoes. Property 371898's shared onsen does not make its room outdoor baths hot springs: the explicit negative source statement is preserved. Plan 03912759/room 0576806 at property 385995 showed a 74,800 JPY one-night, two-adult base quote; points and conditional coupons remain separate, and extra bathing tax prevents asserting a final payable total. These are dated observations, not current price promises.

A requested exact plan on 2026-12-31 returned an undated reference-price fallback; the CLI reported unavailable dated inventory and kept that reference separate. The 2027-09-20 offers page reported no matching plans or stopped bookings; the CLI reported `no_matches`, not a destination-wide sold-out claim.

## Deterministic coverage

Source-derived fixtures and controlled HTTP tests protect bath distinctions and negative evidence, Japanese extraction, identity, per-room child categories, two-night quote units, cancellation terms, query echo checks, pagination boundaries, unsupported continuations, partial/all-failure behavior, timeouts, 429/retry bounds, body limits, cache age and JSON/error output. Encoding regressions cover Windows-31J, document versus script charset and invalid UTF-8.

Not all source conditions were forced live: actual upstream 429s, timeouts, malformed markup, cross-page partial failure and all child-category combinations are deterministic-only. Fixtures do not establish current inventory. Two-night pricing has a captured live source observation plus a deterministic parser test; it is not a row in the final automated live runner.

## Efficiency

The bounded benchmark measures stdout bytes, whole-process wall latency, upstream attempts and per-process peak RSS (`wait4`), with a fresh isolated home and explicit warm reuse. Cold and warm commands preserve the same query, extracted facts and original observation timestamps. These successful pairs were gathered across two runs, not one uninterrupted benchmark. Earlier failed runs are retained alongside the successful observations; see `efficiency-summary.json` and its linked source proofs. Measurements are single observations on this host, not percentile guarantees.

| Command | Bytes cold / warm | Requests cold / warm | Wall ms cold / warm | Peak RSS MiB cold / warm |
|---|---:|---:|---:|---:|
| search-limit5 | 12,021 / 12,019 | 1 / 0 | 3051 / 58 | 34.0 / 27.9 |
| offers-limit5 | 14,211 / 14,209 | 1 / 0 | 3232 / 33 | 30.0 / 25.1 |
| exact-plan | 10,949 / 10,947 | 1 / 0 | 617 / 34 | 25.0 / 21.0 |
| compare-two-dates | 30,313 / 30,314 | 2 / 0 | 2226 / 55 | 31.3 / 27.6 |

The property field-selection check reduced cached stdout from 4,583 to 864 bytes (81%) while retaining shared metadata. The last source-checked suite repeated that check; per-run values remain in its proof.

## Source limitations

- Public markup can change or return an unrecognized variant. Live benchmarking observed one encoding failure and intermittent offers/comparison parse failures. A proven charset-selection defect was fixed; the failed live bodies were not retained, so those original variants' exact causes remain unconfirmed. Later direct captures parsed correctly. Errors remain explicit; successful alternatives survive partial failures.
- Coverage is the fetched subset. Native search pages are separate observations; use explicit cache reuse for repeatable slices within one page. No atomic cross-page snapshot or global cheapest-stay guarantee is made.
- English support consists of curated aliases for 47 prefectures and eight additional areas, not arbitrary translation. Some area aliases cover a wider region than their familiar place name.
- Room parties must be identical. Exact adults 1–8, each child category 0–5, rooms 1–10 and nights 1–9 are bounded; the source's 9-or-more adult bucket is rejected. The 365-day date window is a client bound, not released inventory.
- Unknown fees, coupon eligibility, room attributes or cancellation facts remain explicit. Recheck terms and availability at the canonical Jalan handoff URL.
- No account access, bookings, payments, cancellations, other travel providers or legacy API integration. Legacy registration is closed; existing-key operation was not credential-verified. The Korean-widget MCP lacks the required party and pagination coverage and is excluded.

## Reproduce locally

```bash
go build -o bin/jalan-pp-cli ./cmd/jalan-pp-cli
go test -count=1 ./...
go vet ./...
python3 scripts/live-e2e.py --cli bin/jalan-pp-cli --output /tmp/jalan-live-e2e.json
python3 scripts/measure.py --cli bin/jalan-pp-cli --output /tmp/jalan-efficiency.json
```

Fixtures use 2026-11-10 and nearby dates. Update the runner's fixture dates/plan identities when the booking horizon changes, and validate new facts against the source. A future source access failure is not a valid reason to treat an old fixture as live evidence.

Run ID: `20260927-223649-6dd0cc14`. Final proofs are archived with this checkout in `.manuscripts/20260927-223649-6dd0cc14/proofs/`; full local research state remains under `.printing-press/`.

## Static and framework checks

`go test -count=1 ./...` and `go vet ./...` pass. Focused metadata tests also pass after the final live-matrix fixture changes. Printing Press shipcheck passes all seven legs; its structural verify is 26/26 (100%) and scorecard is 80/100 (A). Five approved feature samples passed live output review. These structural scores supplement the independent source assertions; they are not source-correctness claims by themselves.

The pinned security scan reports zero unresolved findings in custom Jalan code after seven narrowly scoped cleanup/false-positive fixes. It still reports 29 findings in generated framework code; these are recorded individually in `gosec-triage.json` and were not hand-patched in generator-owned files. This is not a claim that the whole framework is security-clean. Tool-description audit has zero pending findings (two concise generated descriptions accepted), and the PII detector reports no findings within its documented scope.

The full matrix first found missing help examples and malformed fixture annotations; those were fixed. A later run passed 100/101 and exposed an intermittent offers-page parse failure. Five bounded direct diagnostic reads then all returned 54 valid offers; no failed body was captured and no speculative parser relaxation was made. Final matrix fixtures opt into a five-minute cache only for reusing a fresh isolated observation during JSON-format checks. Ordinary command defaults remain fresh.

## Final acceptance and artifacts

The final binary-owned full matrix passed **101/101 executed checks**, with 78 framework, mutating, fixture-less or inapplicable checks skipped. All approved stay features have successful happy-path and JSON-fidelity checks; there is no hollow approved feature. The independent source-checked suite passed **37/37**. Both the machine-owned acceptance marker and unsuccessful earlier attempts remain in the archived proof set.

- [Acceptance report](../.manuscripts/20260927-223649-6dd0cc14/proofs/jalan-acceptance.md)
- [Machine-owned gate](../.manuscripts/20260927-223649-6dd0cc14/proofs/phase5-acceptance.json)
- [Source-checked cases](../.manuscripts/20260927-223649-6dd0cc14/proofs/live-e2e-acceptance.json)
- [Efficiency observations](../.manuscripts/20260927-223649-6dd0cc14/proofs/efficiency-summary.json)
- [Shipcheck](../.manuscripts/20260927-223649-6dd0cc14/proofs/shipcheck-final.json)
- [Security triage](../.manuscripts/20260927-223649-6dd0cc14/proofs/gosec-triage.json)

The delivered workspace root was rebuilt successfully and its CLI/Jalan package tests passed. All 179 source files tracked by the acceptance marker are byte-identical to the accepted working tree. The acceptance marker itself is unmodified. See [delivery integrity](../.manuscripts/20260927-223649-6dd0cc14/proofs/delivery-integrity.json) and [delivered tests](../.manuscripts/20260927-223649-6dd0cc14/proofs/delivered-tests.log).

## Publication review regressions

The publication review found that generic output routing dropped partial results from MCP and explicit delivery sinks. Focused real-subprocess MCP tests and Execute-level file/webhook tests now verify preservation of observations and fetch failures, exit code 8 / MCP error state, one delivery, unchanged ordinary errors and explicit output bounds. Failed sinks produce one actionable structured diagnostic while stdout retains the partial facts. These induced failures are deterministic tests, not claims that a live Jalan outage was forced. See the publication-review verification proof for the renewed live acceptance.

## Corrected-account publication review

The replacement publication preserves the original scope and prior partial-output repairs. Regression tests cover three additional review findings: MCP rejects caller-chosen cache directories before CLI execution; explicit Japanese room-bathroom negations retain negative evidence and do not erase an independently documented outdoor bath; comparison field selection traverses nested offers while retaining alternative identity, query, freshness, coverage and failures. The added bathroom phrases and blocked-path attacks are deterministic fixture cases.

The live runner also checks date and exact-plan comparisons with full and selected output from the same cached observations. It checks offer IDs, complete price objects, order and counts, observation timestamps, reduced output size, and zero upstream requests on warm calls. The latest publication acceptance records the result on the final source; earlier performance and generation artifacts remain historical observations.

Search-page HTTP observations are not atomic: separate requests can change the native property-card set even with the same dated query. The pagination acceptance therefore checks the exact HTML from its fresh CLI observation with an independent parser, binding the cache entry's URL and timestamp to the response and verifying the matching property's price. Other source witnesses remain separate HTTP requests using the CLI's anonymous header contract. Earlier failed cross-observation checks remain recorded; they are not presented as passes.
