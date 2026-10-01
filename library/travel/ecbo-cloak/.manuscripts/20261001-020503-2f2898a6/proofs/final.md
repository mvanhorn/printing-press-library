# ecbo cloak CLI — final build evidence

Outcome: complete, verified and locally promoted. No public publishing, PRs, purchases, bookings, payments or account changes. Global/shared configuration and installations unchanged. Exactly one independent reviewer was spawned and reused; all research/planning/implementation/fixes performed by the sole builder.

## Canonical paths

- Editable project: `<project-dir>`
- Staging: `<press-home>/.runstate/ecbo-cloak-cli-70fcbbc6/runs/20261001-020503-2f2898a6/working/ecbo-cloak-pp-cli`
- Promoted local library: `<press-home>/library/ecbo-cloak`
- Manuscripts: `<press-home>/manuscripts/ecbo-cloak/20261001-020503-2f2898a6`
- Receipt ledger: `<press-home>/.runstate/ecbo-cloak-cli-70fcbbc6/runs/20261001-020503-2f2898a6/pipeline/phase-receipts.jsonl`
- Isolated Press binary: `<press-home>/.runstate/ecbo-cloak-cli-70fcbbc6/runs/20261001-020503-2f2898a6/tool-bin/cli-printing-press`
- Artifact integrity: [artifacts.json](artifacts.json) verifies workspace/promoted root binaries and exact bundle/staged binary equality.
- Executables: `ecbo-cloak-pp-cli`, `ecbo-cloak-pp-mcp`; MCP bundle in `build/`.

## Shipped scope

| Workflow | Behavior / evidence |
|---|---|
| facilities near | Japan coordinates, bounded nearest window, Japanese/English identities, radius/name filter, local paging/projection; Tokyo/Kyoto/Osaka live relevance checked |
| facilities get | Lazy legacy-ID → UUID resolution, canonical URL, Japanese identity, address/coordinates, weekly/holiday hours, null independent cutoffs, overnight/same-day/holiday flags, per-size daily prices, raw restrictions/dimensions and source ratio timestamp |
| offer inspect | Exact Asia/Tokyo minute-precision from/to and small/large counts, anonymous live price and validator; validation differs from price, no slot held or remaining-count inference |
| inventory refresh/list | Explicit single bounded snapshot (50 source-window records), bounded display, offline name filtering; no implicit nationwide sync |
| Agent interface | Compact JSON, explicit nulls/currency/price basis/timezone/km, selected fields and observation/cache metadata, stderr failures, typed exits; stdio MCP with cache/home destinations controlled by operator |

Only `cloak.ecbo.io`, `api.ecbo.io`, `search.ecbo.io` runtime source operations. Public GET discovery/detail and first-party POST price/validate are read-only and require no user credentials or paid account. Source JavaScript contract and direct HTTP/SSR replay inspected before implementation. Public language routes verified; zh-CN web pages redirect to ja and booking URLs normalize accordingly.

## Verification

- Full `go test -count=1 ./...`, `go vet ./...`, build CLI/MCP: pass with isolated caches. Initial default-sandbox TCP-listener denials were infrastructure failures; permitted reruns passed.
- Printing Press final shipcheck: all seven legs pass, persisted score 80/A; structural mock verify 9/9 (100%). [Full live matrix](live-dogfood.json): 53 evaluated checks pass, zero failures, 14 expected skips, all shipping features have actual happy-path coverage. [Source-bound marker](phase5-acceptance.json) is tool-generated, never edited.
- [Live E2E summary](live-summary.json): 23 actual public-source cases, including paging/query relevance, 3 regions, source filters, valid offer, ordinary/24-hour overnight, outside-hours/count rejection, legacy/JR restrictions, cache/no-cache, local inventory, invalid input and missing facility. Independent raw quote and validator compared: 1300 JPY, valid=true for HATCH small=1/large=1 on the recorded interval. 24-hour overnight quote 1600 JPY validated true. These are live observations, not fixture claims.
- Deterministic consequential tests cover time/count/ID validation, cache ownership, source-shape rejection, typed outage failures, recognized domain rejection, corrupt snapshot/projection behavior, home normalization, MCP filesystem boundary, and structured request previews. Fixtures are synthetic only in these tests.
- [Independent review](review-findings.md): one fresh-context gpt-6.1-sol xhigh reviewer, same reviewer reused after fixes. Original source/state/MCP findings plus home/preview corrections cleared. Final verdict: no actionable findings; independently repeated live reads and full tests/vet.
- [Security scan](gosec-final.json): zero unresolved hand-authored findings; generated framework candidates retained in scan, not disguised as user-facing source failures. [Tools decisions](tools-audit-decisions.md) explain static false positives; strict PII audit has no pending findings.

## Measurements

Actual `/usr/bin/time -l` peak RSS, process wall latency, exact UTF-8 stdout bytes and counted wire requests. Cached detail/discovery remain timestamped observations. Quotes/validation are always live. Full data in [metrics.json](metrics.json).

| Case | Output bytes | Requests | Wall ms | Peak RSS MiB |
|---|---:|---:|---:|---:|
| near_uncached | 5334 | 1 | 819 | 23.3 |
| near_cached | 5331 | 0 | 21 | 18.3 |
| near_projected | 1836 | 0 | 13 | 18.5 |
| detail_uncached | 4945 | 2 | 531 | 22.5 |
| detail_cached | 4941 | 0 | 20 | 17.7 |
| offer_uncached | 5851 | 4 | 1197 | 23.7 |
| offer_valid | 5848 | 2 | 532 | 22.0 |
| inventory_refresh | 3579 | 1 | 588 | 23.5 |
| inventory_offline | 3573 | 0 | 20 | 17.8 |

Defaults: 10 displayed results, 50-hit source window, 5 km radius, sequential requests at 3/sec, one retry for 429/5xx, 15s whole-command deadline (1s–60s override), 2 MiB response/request-preview limit, 256 owned cache-response files. Detail TTL 1h, discovery TTL 5m; inventory explicit refresh only. No resident browser runtime.

## Public-source limits and missingness

A listed facility/max item count/raw ratio is not confirmed remaining capacity. Exact legacy remaining-count endpoint returned 401 and is excluded. Validator acceptance is a fresh observation for requested interval/counts, without capacity reservation or later guarantee. Structured independent acceptance/pickup cutoffs are absent and stay null; restrictions may contain pickup instructions. Weekly clocks are preserved across midnight; authoritative validator governs exact interval suitability. Source language variants disagree at exactly 45 cm and about calendar/business-day wording; category policy uncertainty is explicit and requested totals come directly from source. Facility restrictions override general policy, including special station slot counts. Source total can exceed its 50-hit nearest window; local name filters/pagination never claim Japan-wide completeness. No text geocoder or other provider integrated.

## Printing Press handling

Original installed v4.32.5 cannot genuinely cover local-write happy paths. Existing corrected source was compiled into this run’s isolated tool-bin; shared source, binary and config were untouched. Local-write matrix uses an existing confined fixture home, accurate MCP local-write annotations and the explicit runner switch; provider operations remain read-only. Dry-run legs describe requests with planned/dry_run metadata, not provider results. Rebuilt final CLI/MCP/bundle are required before promotion. Receipts sequence every phase; ordinary scope/briefing gates used the user’s explicit preauthorization. Extra skill planner/output-polish agents were excluded by the exactly-one-reviewer instruction. No acceptance marker fabricated or hand-edited.

Observed proof time: 2026-09-30T20:13:09.544122+00:00. Local promotion succeeded; Printing Press validated the normalized source fingerprint, refreshed final artifacts, and released this CLI’s lock.
