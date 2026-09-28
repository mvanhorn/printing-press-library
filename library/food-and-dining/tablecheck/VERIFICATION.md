# Verification

Verified locally on 2026-09-27 UTC with Go 1.27.1 on macOS arm64 and Printing Press 4.32.5. Build requires Go 1.26.6+. The verification performed no booking or shared tool configuration change.

## Evidence

Saved reports: [live source checks](evidence/live-source-checks.json), [binary-owned acceptance](evidence/phase5-acceptance.json).

- `go test -race -count=1 ./...`, `go vet ./...`, CLI/MCP builds pass. Deterministic tests cover statuses, decimal amounts, Tokyo date conversion, retries, request/concurrency limits, cache provenance, malformed responses and partial failures.
- Printing Press shipcheck: all 7 legs pass; runtime verifier 27/27, score 83/100 (A). The generic workflow-manifest leg has no manifest and is informational; the focused live suite below verifies the real workflow.
- Full binary-owned live matrix: 87 mandatory checks pass, 0 fail. 53 auxiliary rows are skipped/unverified under harness rules (generated framework/local-write/non-ID/raw-fixture cases). All 8 approved planning leaves pass real happy-path and JSON checks.
- Follow-up publication verification: a two-day live scan matches JSON, CSV/plain rows and quiet identities, including mixed metadata and metadata-only selections; venue/course quiet output matches stable source IDs, and structured CSV/plain detail cells decode to the same values as JSON. Final cached JSON used zero requests and preserved fetch times. [Native-format evidence](.manuscripts/20260927-220537-e6065d2c/proofs/native-format-live.json).
- Replacement-publication review: sparse date windows, MCP shortlist input and terminal pagination are corrected. A fourteen-day live scan passed 18 assertions with four total HTTP attempts including an independent final-date check; its warm scan used zero requests. [Review and efficiency evidence](.manuscripts/20260927-220537-e6065d2c/proofs/publish-account-review-fixes.md).
- Focused source suite: 11 cases, 129 assertions, 11 HTTP attempts. It verifies geographic/cuisine/dinner-budget/date/party inputs, cursor pagination, stable venue identity, course prices/conditions, independent party calendars, default time-window behavior, booking URLs and partial scans.
- Gosec 2.26.1: zero unresolved custom-code findings after review. 21 generated framework findings are triaged in [security-triage.json](evidence/security-triage.json); they are not a claim that every generated path is security-audited. Tools audit has 0 pending findings; PII audit has 0 findings in its documented scope.

At the observed time, `sushi-tokyo81` (`67e657634474874e35785280`) on 2026-09-30 had 17:30 JST available for 2 people; 18:00 was unavailable. Party 20 had no available slot in the observed window. Both results matched independent source booleans. This is historical evidence, not current availability or a booking guarantee.

Course `68da546fcde865308c33e7f9` retained price `19800.0`, currency JPY, included tax and percent service-fee type. Exact fine print qualified the price as JPY 19,800–22,000 plus 10% service. The CLI leaves an absent structured charge amount/basis unknown and computes no total. Original cutoff timestamps are preserved exactly; independent responses differed at subsecond precision, so that comparison allows ≤1 second and records both values.

## Efficiency

One cold and one warm separate process per case; elapsed time includes startup. Cold cases use isolated empty caches, warm cases run immediately afterward. Measurements are observations, not statistical performance guarantees. All warm reads preserve original observation timestamps and report cache hits.

| Command | Output bytes cold/warm | HTTP attempts cold/warm | Wall ms cold/warm | Peak RSS MiB cold/warm |
| --- | ---: | ---: | ---: | ---: |
| search | 2,700/2,700 | 1/0 | 508/19 | 24.0/18.3 |
| venue | 5,253/5,253 | 1/0 | 451/19 | 24.2/18.3 |
| courses | 3,560/3,560 | 2/0 | 772/39 | 23.6/19.7 |
| check | 2,864/2,860 | 2/0 | 807/37 | 23.9/19.9 |
| scan | 10,776/10,763 | 4/0 | 1632/20 | 25.4/21.9 |

Search used 2 results; courses used 5; check used 10 slots; scan used 2 venues × 2 dates with 5 slots per row. The full benchmark used 12 HTTP attempts, including refresh. `--refresh` on check made 2 requests and replaced the observation timestamp. RSS comes from `/usr/bin/time -l` in bytes, converted above to MiB. [Raw measurements](evidence/benchmark.json) record response bytes and CLI-reported latency as well.

## Access and limits

The verified consumer reads use public TableCheck HTTPS endpoints without login, cookies or API keys. No paid partner API was used. The separate [official partner API](https://tablecheck.atlassian.net/wiki/spaces/API/pages/44729064/Request+API+Access) requires approval and supplies pricing during application. Undocumented consumer endpoints have no published fixed entitlement or SLA and may change.

Coverage is TableCheck's Japan listings and the course/calendar data each venue exposes. Search requires coordinates; cuisine lookup provides stable keys. Search budgets are venue dinner averages, not all-in course prices. Calendar results are venue-level and do not bind a named course. The source returns a time window around an anchor: the default is 18:00 local; use `--time` for another period and an exact-time check. Coverage explicitly says `full_day:false`.

Explicit false is unavailable, not proof of sold out. Closed, missing inventory, unfamiliar source modes and failures remain distinct. Request, waitlist, sold-out and unpublished modes cannot all be reliably established from the verified read surface; unfamiliar modes preserve their source status and remain unknown. Static policies never become live inventory. Additional course, eligibility and cancellation conditions may appear only during human booking handoff.

Defaults: 10 results, maximum 50; scans at most 5 venues × 14 inclusive dates; at most 20 HTTP attempts per command, one retry, concurrency two, timeout 10s (maximum 30s), and a 4 MiB response cap. Cache lifetimes: availability 30s, search 5m, venue/menu 1h, cuisines 1 day. Refresh bypasses local reads only.

## Reproduce

```bash
go test -race -count=1 ./...
go vet ./...
go build -o tablecheck-pp-cli ./cmd/tablecheck-pp-cli
python3 scripts/planning-live-check.py --run-live --binary ./tablecheck-pp-cli --output-dir /tmp/tablecheck-live
python3 scripts/planning-bench-check.py --run-live --binary ./tablecheck-pp-cli --output-dir /tmp/tablecheck-bench
```

Both live scripts enforce 40-attempt ceilings and require explicit opt-in. Their default test date is Tokyo today plus two days; a venue may change inventory, so live mismatches should be investigated against saved source evidence.

Sanitized research and proofs are packaged under `.manuscripts/20260927-220537-e6065d2c/`. Full operational transcripts and the closed 21-phase receipt ledger remain in the original local run archive.
