# Verification and operating limits

Walkerplus was built through Printing Press run `20260927-220621-b57bba4d`. The root Astra agent owns architecture and acceptance; Sol max workers implemented and tested the CLI. Original local artifacts and receipts are preserved. Publication packaging includes sanitized research and proofs; shared tool configuration is unchanged.

The publishing identity is `zjsng`. The initial PR used the wrong account. The identity correction preserved its reviewed runtime source; subsequent review fixes address coordinated closures and MCP JSON handling. [Account correction evidence](.manuscripts/20260927-220621-b57bba4d/proofs/account-correction.md) records the repair and original review history.

## Current verification

- `go test ./...`: 11 passing test packages, six additional packages without tests. Vet, CLI/all-package builds and reachable-only `govulncheck@v1.3.0` pass.
- Live E2E: eight tests plus 16 subtests pass across Tokyo, Kyoto, Hokkaido, Miyagi and Osaka, with actual date/location/category relevance, free/indoor evidence, opening/closing boundaries, city resolution and wrong-year/wrong-prefecture controls.
- Full Printing Press live matrix: 50/50 executed checks pass, zero failures, 40 separately disclosed framework/inapplicable skips. Five domain workflows pass happy-path and JSON checks.
- Independent regressions cover closure vs positive recurrence, weekday grammar, unsupported monthly rules, missing listing attributes, strict final filters, catalog JSON keys, coordinated closures, MCP JSON/error bounds, detail-budget selection/order, editions, deduplication, admission uncertainty and bounded requests/cache behavior.
- Publication validation passes manifest, acceptance, module path, dependency/vulnerability, vet/build, help/version, skill, customization and manuscript checks. Repository package/attribution/release/binary/generator-version/supply-chain checks passed before initial submission.

Historical shipcheck/scorecard and security records remain in the archive. The previously noted optional MCP HTTP timeout exposure was removed through spec-driven stdio-only generation. The current CLI does not include an HTTP MCP listener.

## Efficiency snapshot

Measured at `2026-09-28T02:38:56Z` on this machine. Discovery samples one listing page and returns at most three events; shortlist inspects at most three details. Cold uses an empty command-specific cache; warm repeats it. These are observations, not latency guarantees.

| Command | Cache | Events | Stdout bytes | HTTP attempts | Wall ms | Peak RSS MiB |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| search | cold | 3 | 6,986 | 1 | 56.33 | 27.16 |
| search | warm | 3 | 6,982 | 0 | 14.62 | 23.25 |
| shortlist | cold | 3 | 10,645 | 10 | 4528.17 | 31.80 |
| shortlist | warm | 3 | 10,631 | 0 | 22.79 | 28.47 |
| event | cold | 1 | 3,728 | 3 | 1034.98 | 29.91 |
| event | warm | 1 | 3,722 | 0 | 15.85 | 23.92 |

Measured public-module binary SHA-256: `4606126fa3ac61481410a6a9652bb85ba8fe4db455ec0eb3ed0c28430cc006f3`. Field selection previously reduced search output from 6,982 to 2,008 bytes (71.2%). A locally rebuilt bare-module binary has a different build-path hash but the same reviewed source behavior.

## Access and limits

Public Walkerplus HTTPS HTML is the only runtime source; no login, API key or browser is needed. Rebuild with Go 1.26.6+. Catalogs include 47 prefectures and 36 category labels; city routes resolve from current prefecture catalogs. Official organizer links are returned without crawling those sites.

Discovery is bounded sampling of an undocumented HTML contract. Month/day routes have no year selector; exact-year filtering prevents invented editions but cannot discover unpublished future events. Overall ranges and closure-only rules do not confirm daily activity. Unsupported holiday/monthly rules and approximate seasonal periods remain possible. Unknown admission remains null; raw source caveats and freshness are retained. Recheck the organizer before attendance.

## Evidence

- [Fresh full live acceptance](.manuscripts/20260927-220621-b57bba4d/proofs/phase5-acceptance.json)
- [Review follow-up](.manuscripts/20260927-220621-b57bba4d/proofs/account-correction-review.md)
- [Detail-budget review](.manuscripts/20260927-220621-b57bba4d/proofs/detail-priority-review.md)
- [Coordinated-closure regression](.manuscripts/20260927-220621-b57bba4d/proofs/conjunction-regression.log)
- [Independent regression evidence](.manuscripts/20260927-220621-b57bba4d/proofs/review-regressions.log)
- [Final integrated/live results](.manuscripts/20260927-220621-b57bba4d/proofs/corrected-review-checks.json)
- [Publication validator](.manuscripts/20260927-220621-b57bba4d/proofs/corrected-review-publish-validation.json)
- [Raw efficiency measurements](.manuscripts/20260927-220621-b57bba4d/proofs/efficiency-corrected-review/measurements.json)
- [Historical security review](.manuscripts/20260927-220621-b57bba4d/proofs/phase-4.95-findings.md)
