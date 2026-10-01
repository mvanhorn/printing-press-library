# Tokyo Art Beat CLI — final evidence

Complete and locally promoted on 2026-10-01. The workspace root is a verified, buildable copy of the promoted source. No publication, PR, purchase, reservation, payment or account change occurred. All eight provider/domain review findings and the one serialized fixture issue are fixed. Exactly one fresh-context independent reviewer (gpt-6.1-sol, xhigh, fork_turns none) reviewed source, security, documentation and five passing output samples; subsequent rechecks used that same reviewer.

| Artifact | Canonical absolute path |
|---|---|
| Buildable source | `<workspace>` |
| CLI binary | `<workspace>/build/stage/bin/tokyo-art-beat-pp-cli` |
| MCP binary | `<workspace>/build/stage/bin/tokyo-art-beat-pp-mcp` |
| README / agent skill | `<workspace>/README.md`, `<workspace>/SKILL.md` |
| Promoted Press copy | `<workspace>/.press/library/tokyo-art-beat` |
| Preserved working copy | `<workspace>/.press/.runstate/tokyo-art-beat-cli-b3ad2de2/runs/20261001-020055-6892e92f/working/tokyo-art-beat-pp-cli` |
| Run state | `<workspace>/.press/.runstate/tokyo-art-beat-cli-b3ad2de2/runs/20261001-020055-6892e92f/state.json` |
| Receipt ledger | `<workspace>/.press/.runstate/tokyo-art-beat-cli-b3ad2de2/runs/20261001-020055-6892e92f/pipeline/phase-receipts.jsonl` |
| Archived manuscripts | `<workspace>/.press/manuscripts/tokyo-art-beat/20261001-020055-6892e92f` |
| Live domain results | `<workspace>/evidence/live` |
| Measurements | `<workspace>/evidence/measurements.json` |
| Full live matrix | `<workspace>/.press/.runstate/tokyo-art-beat-cli-b3ad2de2/runs/20261001-020055-6892e92f/proofs/live-dogfood.json` |
| Runner-owned acceptance | `<workspace>/.press/.runstate/tokyo-art-beat-cli-b3ad2de2/runs/20261001-020055-6892e92f/proofs/phase5-acceptance.json` |
| Independent review / hashes | `<workspace>/.press/.runstate/tokyo-art-beat-cli-b3ad2de2/runs/20261001-020055-6892e92f/proofs/independent-review.md`, `<workspace>/.press/.runstate/tokyo-art-beat-cli-b3ad2de2/runs/20261001-020055-6892e92f/proofs/source-hashes.json` |
| Promotion receipt | `<workspace>/.press/.runstate/tokyo-art-beat-cli-b3ad2de2/runs/20261001-020055-6892e92f/proofs/promotion.json` |

The inherited run is `20261001-020055-6892e92f`, resumed at entered `11-build-the-goat`. No `.press/.press-run.json` existed; the actual state/ledger were under this project's `.press/.runstate/tokyo-art-beat-cli-b3ad2de2`. The central `<press-home>/.runstate` had no matching Tokyo Art Beat scope. Recorded old PID 49413 was absent and Press classified the lock stale; supported `lock acquire` recovered it. The lock is released after promotion, and the final receipt is `21-next-steps completed → done`. Original research, receipts, browser capture and scaffold implementations remain preserved. Missing provenance/bundle metadata from the original network-interrupted generation was repaired through isolated sibling generation; no useful working source was regenerated over. Other seven CLI builds and shared/global config were untouched.

| Check | Final result |
|---|---|
| `go test -count=1 ./...` | PASS, all packages; deterministic regression assertions |
| `go vet ./...` | PASS |
| Root `go build ./...` | PASS; `<workspace>/evidence/root-build.log` |
| Canonical Press shipcheck | PASS, all seven legs; `<workspace>/.press/.runstate/tokyo-art-beat-cli-b3ad2de2/runs/20261001-020055-6892e92f/proofs/shipcheck.json` |
| Structural verify | 91.67% (11/12), PASS, 0 critical; mock mode |
| Scorecard | 77/100, grade B |
| Full Press live dogfood | 52/52 executed checks PASS, 0 failures; 14 explicit inapplicable/skipped probes |
| Live domain E2E | 29/29 content-level assertions PASS |
| Independent review | PASS, eight consequential fixes rechecked and final annotation hash cleared |
| Output plausibility | PASS, five eligible passing final-source samples |
| MCP stdio | PASS: initialize, tools/list, context; 11 focused read-only tools |
| Tools audit | 0 pending, 1 justified generated/unregistered platform leaf accepted |
| PII audit | No findings in declared phase-1 detector scope |
| Gosec | 0 unresolved hand-authored findings; 21 preserved generated/reserved scaffold findings triaged separately |
| Promotion | Press `lock promote` succeeded; source-bound acceptance gate passed |

Structural verification is not live correctness evidence. Its one low-score row probes the non-runnable `events` help group as a data command; actual leaves pass the domain/live matrix. The no-sync pipeline dimension is explicitly skipped because this product uses bounded response caching. The 14 skipped full-matrix probes are enumerated in live-dogfood.json (for example, no positional error probe); no no-auth credential skip was fabricated. Security scanner raw results and generated-scaffold triage are retained at `<workspace>/.press/.runstate/tokyo-art-beat-cli-b3ad2de2/runs/20261001-020055-6892e92f/proofs/gosec-after.json` and `<workspace>/.press/.runstate/tokyo-art-beat-cli-b3ad2de2/runs/20261001-020055-6892e92f/proofs/security-triage.json`; unused account/mirror handlers are absent from the runtime CLI and MCP trees.

Representative per-process measurements on macOS arm64, using `/usr/bin/time -l`:

| Scenario | Output bytes | Wall latency ms | Logical / wire requests | Peak RSS MiB |
|---|---:|---:|---:|---:|
| trip-uncached | 7733 | 7354 | 4 / 8 | 23.97 |
| trip-cached | 7723 | 21 | 0 / 0 | 18.27 |
| trip-offline | 7723 | 13 | 0 / 0 | 18.11 |
| nearby | 11365 | 10 | 0 / 0 | 20.02 |
| archive | 9423 | 9 | 0 / 0 | 18.16 |
| compare | 19099 | 9 | 0 / 0 | 18.78 |
| fields | 3568 | 9 | 0 / 0 | 18.03 |
| select | 109 | 9 | 0 / 0 | 18.12 |

Trip measurements use three cards with an area and date window. Uncached means `--fresh`; cached/offline repeat the exact query. One logical request usually entails two wire requests because the CDN redirects to its response cache. These are representative single runs, not statistical benchmarks. All 29 scenarios, exact arguments, timestamps, output sizes and per-process peak RSS are in measurements.json. Selected-envelope results omit stats by request; their zero request values in this table refer to the cached projection run, whose companion full-envelope cached measurement confirms zero network requests.

Implemented workflows cover geography/date/category/artist/venue exhibition discovery, bilingual venue/gallery/museum search/detail, finite catalogs, lazy edition inspection, venue schedules, starting/ending/overlapping trip windows, conservative day assessment, 2..4 edition comparison and bounded nearby ranking. Live assertions verify category/artist relevance, inclusive windows, stable ID/URL/Japanese names, pagination, archive year versus start year, malformed normalized dates, venue identity, fees, nearby radius/order/artist, exact-query cache modes, projections, invalid inputs and partial/all-failed comparisons. Source-listing fees and hours remain separate from venue defaults; source admission category/member conditions remain bilingual text. The last-admission phrase is preserved from notes when present; structured last-admission time remains null when unavailable.

The public provider is [Tokyo Art Beat EN](https://www.tokyoartbeat.com/en) and [Tokyo Art Beat JP](https://www.tokyoartbeat.com/). Read-only anonymous access to its undocumented published website feed worked. Public en-US/ja-JP fields are returned together; omitted translations remain null. Public MuPon indicators are included where supplied, with [membership conditions](https://www.tokyoartbeat.com/en/aboutSubscription) explicit; redemption and member-only content are excluded. No source access blocker remained within this approved scope. The protected content-types metadata route returned 401 during research and is excluded, not represented as an unavailable discovery endpoint.

Actual provider limitations are explicit: `[match]` artist filters, `[within]/[near]` coordinate filters and multi-key order can be silently ignored. Evidence is in research/resume-query-probes.json. Artist/distance predicates therefore filter bounded candidates locally; single source sort keys and per-page ID tiebreaks are disclosed. Scan size is separate from output limit. Source updates can move pagination, and capped/empty candidate results do not prove nationwide absence. Nearby reports straight-line distance, not walking time.

Date spans never establish actual open days. Weekly/exceptional/holiday/hidden closures, missing boundaries and unconfirmed ends are retained and produce conservative unknown assessments when needed. Publication status is not live opening status. Invalid normalized dates are null while original source values remain available. Legacy archive edition year is preserved separately from schedule start year. Source-provided official venue/exhibition URLs are handed off; ticket links are empty if absent. Listings never establish ticket inventory, reservation slots or sellout status. Unknown structured capabilities were not fabricated.

Runtime defaults are bounded: search 10/max 50, nearby at most 100 venue and 100 event candidates, local artist scan default 3/max 5 pages, sequential requests at a conservative 2/s ceiling, one retry, 15-second per-request and 60-second default command timeout (max 2m), 20 logical/60 wire request budget, 4 MiB body cap and namespaced private cache bounded to 128 files/25 MiB with one-hour TTL/seven-day eviction. `--offline` reads exact cached queries with explicit staleness. Failed live reads do not silently fall back to stale content. Errors are structured stdout plus stderr diagnostics; partial results preserve source failures.

The original discovery capture remains intact. The archived copy strips response bodies, cookies/auth headers and auth-shaped query values; vendor-anchored secret scan passed. No user credentials were used. Receipts remain in disposable run state and were not copied into manuscripts.
