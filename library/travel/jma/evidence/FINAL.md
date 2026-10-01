# Final JMA CLI evidence

Outcome: completed focused source, buildable standalone Go CLI and MCP companion. Public JMA access verified without credentials, fees, browser residency or third-party weather integration. No publishing/PR, purchases, bookings or unrelated writes. Baseline empty project; no competing JMA lock/build existed. Global configuration remained unchanged.

## Canonical paths

- Project: `<source-project>`
- Staging: `<build-run>/working/jma-pp-cli`
- Local library: `<local-library>`
- Manuscripts: `<archived-run>`
- Run receipt ledger: `<build-run>/pipeline/phase-receipts.jsonl`
- Press binary: `<build-run>/tools/cli-printing-press` (isolated copied corrected4.32.5; global binary unchanged)

**Local promotion succeeded** (`promotion.json`). The unrelated public registry capture was excluded from the deliverable; the final PII audit is clean. Source fingerprint and source-bound acceptance are in phase5-acceptance.json; original runner marker is embedded in staging/library .manuscripts. Receipt closure is in the run ledger.

## Verification

- Canonical shipcheck: PASS / exit0, all seven legs. Scorecard 81/100. `shipcheck.json` is the current authoritative gate; earlier rerun files preserve failed diagnostics, not final status.
- Printing Press full live matrix: 74/74 passed, zero failures. 56 extra framework/nonapplicable probes skipped; these are not claimed as live verified weather features. `dogfood-live.json` and `phase5-acceptance.json` are runner-generated, never hand-edited.
- Actual focused live E2E:14 cases passed (`live/matrix.json`), including source inventory refresh, Tokyo/Osaka forecasts, Hokkaido/Amami office aliases, north-Izu broader weekly region/station, current coastal warnings/lifted events, no-warning municipality, inland wave applicability, both active cyclones and empty-result projection. Additional live doctor and empty warning-detail projection passed.
-46 source-correctness checks compare real normalized CLI outputs to captured first-party JSON (`source-correctness.json`): source IDs/issue times/values, independent warning product statuses, analysis versus forecast, coherent cyclone issue/valid joins, pressure and radius units. Captures are genuine network responses. Source data can change; run live_verify.py before source_correctness.py when reproducing.
- MCP:9 focused runtime-enumerated tools and3 real calls passed (`mcp-verification.json`); typed raw endpoint bypass removed. Explicitly synthetic incomplete projection proof is separately labeled under evidence/mcp. Independent reviewer also verified two-call concurrency cap and rejected third call.
- Full Go suite and vet passed (`tests-final.log`, `vet-final.log`). Consequential deterministic tests cover source resolution/eligibility, applicability/completeness/lifecycle, extrema buckets, missing/invalid numbers, cache expiry, typhoon issue join, nulls and projection. Synthetic fixtures are never represented as live observations.
- Exactly one fresh independent gpt-6.1-sol xhigh reviewer; all findings fixed and independently reverified (`REVIEW.md`, `REVIEW-FINAL.md`). No implementation/planning agents were spawned.
- Pinned gosec completed, zero unresolved authored findings; generated-framework findings and narrow trusted-path suppressions explained in SECURITY.md. PII audit clean. MCP HTTP header/idle timeouts and child concurrency bounded.

## Resource measurements

Measured sequentially with macOS `/usr/bin/time -l` on real responses. Uncached means forced refresh that saves cache, followed immediately by a cached call. Values are point-in-time measurements, not promises; source payloads and latency change. Inventory refresh uses8 bounded sequential requests and stores its snapshot separately.

| Command | Cache mode | stdout bytes | requests | latency ms | peak RSS MiB |
|---|---|---:|---:|---:|---:|
| forecast | uncached-forced-refresh | 5611 | 1 | 45 | 24.59 |
| forecast | cached | 5609 | 0 | 3 | 20.36 |
| warnings | uncached-forced-refresh | 4604 | 1 | 40 | 25.98 |
| warnings | cached | 4602 | 0 | 3 | 20.69 |
| typhoon | uncached-forced-refresh | 5855 | 3 | 513 | 23.95 |
| typhoon | cached | 5850 | 0 | 0 | 18.0 |
| typhoon-list | uncached-forced-refresh | 1046 | 1 | 34 | 22.05 |
| typhoon-list | cached | 1044 | 0 | 0 | 18.02 |

Complete raw measurements and method: benchmark.json / benchmark/*.time. Requests are bounded per command and paced max4/s; body2MiB; two attempts; ten seconds/attempt;45s default command budget max60s; cache128payloads/32MiB; discovery20 default/max100; MCP two child calls. Cached calls have zero source HTTP requests. Final source preserves units/JST/provenance under field projection.

## Scope and source limits

All five approved focused improvements shipped: source resolution, warning completeness, coherent cyclone detail, lazy discovery, explicit atomic inventory refresh. No stubs or blocked approved functionality. Runtime uses current r8 warnings; the legacy200 endpoint was observed frozen at May28 and is excluded. Weekly eligibility and no-wave/tide applicability come from JMA first-party catalogs. Daily extrema follow JMA frontend issue buckets/indices.

Warning coverage is partial by agreed design: municipality weather warnings; joint river flood bulletins/coastal-zone supplements excluded. Explicit not-applicable differs from no-warning and missing/unknown/failed retrieval. Incomplete semantics return exit5 and retain JSON evidence; transport failures return4. Warning expiry is unknown, issue age is not expiry, and no result implies personal safety clearance. Typhoon forecast circles express70% probability of the center at the specified time, not storm size or a guaranteed route; analysis/estimate/forecast remain distinct. The unversioned first-party website contract can change; incompatible data fails predictably.

Authoritative source URLs, access/format research, decisions and attribution: RESEARCH.md. Concise user/agent workflows: README.md and SKILL.md. Original source assets: evidence/source; current static source catalogs: internal/jma/catalog.

Completion: receipt ledger closes at21-next-steps ->done. Local library source hashes match the canonical project Go files byte-for-byte; `go build ./...` passed in the library as a standalone module (`library-build.log`). No remaining required work or approval.
