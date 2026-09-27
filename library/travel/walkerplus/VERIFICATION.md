# Verification and operating limits

The historical local build of this Walkerplus CLI was completed through Printing Press run `20260927-220621-b57bba4d`. Root gpt-6-astra owns architecture and acceptance; gpt-6-sol max workers implemented the commands, core and external tests with separate ownership. Shared tool configuration was unchanged. This record documents the historical local verification and stdio-only publication preparation; it does not assert a final merge or catalog availability. Local builds remain available.

## Access and coverage

Public HTTPS HTML from Walkerplus, with no API key, login, browser runtime or second event provider. The discovery catalog includes all 47 prefectures and 36 category labels. City routes resolve from the selected prefecture's current source catalog when needed. Official organizer URLs are returned as source links; the CLI does not crawl those sites.

`search` fetches listings only. `shortlist` enriches a bounded candidate set. `event` fetches one edition's main, data and price pages. Compact JSON preserves Japanese titles, stable IDs, source URLs, Asia/Tokyo dates, explicit unknowns and fetch freshness. `--select`/`--fields` reduce event fields while retaining coverage metadata. Source text and normalized/derived fields are distinguished in README/SKILL.

## Evidence

- Printing Press seven-leg shipcheck: PASS; mock verification 8/8, 100%, no critical failures. Earlier scorecard 80/100; stricter final live scorecard 76/100, with an exercised passing shortlist sample.
- Full live matrix: 50 executed checks passed, zero failed; 40 skips/unverified checks are separately disclosed in the acceptance report. Five event workflows passed happy-path/JSON checks. Skips concern positional-error applicability and inherited framework workflows.
- Supplemental real-source checks cover Tokyo, Kyoto, Hokkaido, Miyagi and Osaka, multiple event categories, actual date/location relevance, city code/slug/Japanese-name resolution, free/indoor evidence, opening/closing boundaries, deduplication and wrong-year exclusion.
- Deterministic coverage includes recurrence, exclusions, year boundaries, missing/mixed admission, cancellation/weather caveats, timeouts, retry/page/detail/concurrency bounds, cache freshness and eviction ownership. Final test results are linked below.
- Tools audit: no pending findings; one precise generated profile-list description accepted. PII audit: no findings within its documented detector scope. Security scan: zero unresolved hand-authored findings; 22 generated findings individually triaged in the code-review report.

The public package embeds the historical research and proofs at `.manuscripts/20260927-220621-b57bba4d/`. Original local pipeline receipts remain preserved separately and are excluded from the package.

## Efficiency snapshot

Measured on this machine at 2026-09-27T15:59:12Z. Each discovery command samples one listing page and returns at most three events; shortlist inspects at most three details. Event returns one edition. Cold means an empty command-specific cache; warm repeats the same query. Latency and memory are observations, not service guarantees.

| Command | Cache | Events | Stdout bytes | HTTP attempts | Wall ms | Peak RSS MiB |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| search | cold | 3 | 6,986 | 1 | 99.36 | 27.09 |
| search | warm | 3 | 6,982 | 0 | 16.07 | 23.14 |
| shortlist | cold | 3 | 10,645 | 10 | 4527.81 | 31.91 |
| shortlist | warm | 3 | 10,631 | 0 | 23.41 | 28.66 |
| event | cold | 1 | 3,728 | 3 | 1049.97 | 29.25 |
| event | warm | 1 | 3,722 | 0 | 25.12 | 23.53 |

Binary SHA-256: `0f2d1de53ee3f3472fcd6b5fff44137909e25a11446fc18b97307225da68d590`. Raw argv and metrics: `efficiency-final/measurements.json`. An earlier field-selection check reduced search stdout from 6,982 to 2,008 bytes (71.2%) with zero HTTP attempts on a warm cache.

## Limits

- Native month/day routes have no year selector. Exact-year local filtering prevents old editions from becoming current, but cannot discover unpublished future editions. Pagination and caps mean a shortlist is sampled coverage, not exhaustive Japan-wide inventory.
- Overall ranges are envelopes. Holiday-dependent recurrence, unknown schedules and approximate seasonal viewing remain possible, not confirmed daily activity. Source notices, opening hours, reservations and price caveats require reading before attendance.
- Unknown prices remain null; multiple price tiers retain raw text rather than inventing a single adult price. Strict free/indoor filters exclude unknown or conditional evidence.
- HTML is an undocumented source contract and can change. Failures are explicit; recheck source freshness with `--refresh`.
- MCP is compiled for stdio only by the explicit source spec. The generated HTTP listener and authentication/TLS flags are absent, removing the previously documented HTTP timeout exposure. Other historical generated audit findings remain documented; no external issue was posted.

## Final test and gate files

Final delivered-path verification from the workspace root: `go test ./...` passed all 12 test packages (five additional packages have no tests); `go vet ./...`, the CLI build and `go build ./...` all passed. Offline help exposes the five workflows; schema contains 60 selectable fields. The live marker was refreshed after the test-contract correction. The final locally rebuilt binary SHA-256 is `57a2b711872240eed302175c55df7b6c954b70d4c70ec3cbb9d5dc12cbe684cd`; the benchmark above identifies its separately built, behavior-identical staging binary.

- [Full live acceptance](.manuscripts/20260927-220621-b57bba4d/proofs/phase5-acceptance.json)
- [Integrated test output (includes resolved stale assertion)](.manuscripts/20260927-220621-b57bba4d/proofs/polish-go-test.log)
- [Corrected external package](.manuscripts/20260927-220621-b57bba4d/proofs/polish-acceptance-fixed.log)
- [Polish result](.manuscripts/20260927-220621-b57bba4d/proofs/20260928-fix-walkerplus-pp-cli-polish.md)
- [Security review](.manuscripts/20260927-220621-b57bba4d/proofs/phase-4.95-findings.md)
- [Raw final measurements](.manuscripts/20260927-220621-b57bba4d/proofs/efficiency-final/measurements.json)

- [Final delivered-path verification](.manuscripts/20260927-220621-b57bba4d/proofs/delivered-path-verification.json)
- [Final full test output](.manuscripts/20260927-220621-b57bba4d/proofs/delivered-go-test.log)
