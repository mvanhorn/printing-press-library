# Weathernews CLI final evidence

Outcome: buildable, independently reviewed public Japan weather/seasonal evidence CLI; local promotion completed successfully (promotion.log); source parity and provenance are recorded in the retained evidence. The generation run ended with local promotion. This publication session was separately authorized; no paid accounts, purchases or bookings are required. Sole builder plus exactly one fresh-context gpt-6.1-sol/xhigh reviewer, reused to verify fixes. No global config changes.

## Public module

`github.com/mvanhorn/printing-press-library/library/travel/weathernews`. Confidential local development paths are omitted from this package.

## Shipped behavior

- First-party Japanese place resolution: source identity/URLs, ambiguous results and coordinates preserved; elevation unavailable is null.
- Bounded hourly/daily forecasts, explicit actual horizon, JST validity, units and separate observation. Public forecast issue time remains null. Caller thresholds compared independently across 2–5 candidates, with no universal score.
- Sakura/koyo summaries, bounded literal query matches and pagination, one-page inventory retrieval and lazy one-spot detail. Seasonal report date/age differs from fetch date/age; predictions and historical normals remain separate. Season year and ended status are source-derived. Explicit source years cannot be reassigned to a different title year.
- Compact JSON by default, validated dotted projection including empty collections/null parents, agent envelopes and selected CSV/plain fields. Unknown paths fail with exit 2 before output. Bounded cache/retries/timeout/serial requests and explicit refresh; fresh local cache mode makes no network request. Typed usage/not-found/access/upstream/rate-limit failures and partial comparisons are explicit.

Public scope is verified without credentials. Member radar/gated products and commercial WxTech are excluded; no subscription is silently required. Source access and coverage were researched before implementation, using current first-party pages and anonymous browser/direct-HTTP evidence. API contracts are undocumented and fail closed on schema/identity drift.

## Live verification

Generation focused matrix: 11 of 12 cases passed; the mountain-place case failed because a legitimate first-party mountain URL was rejected. The publication session fixed this parser defect and reran all 12 cases with fresh requests successfully; see `publish-focused-live/summary.json` and `publish-mountain-source-agreement.json`. Independent agreement/relevance checks: 7 passed against actual live-captured HTTP bodies, decoded separately in Python. Live chain: seasonal search → actual source ID → lazy detail → weather at returned coordinates. Edge/cache/pagination checks: 13 passed. See `live/`, `edges/` and `scripts/` for exact arguments and results.

Press full matrix: **113/113 executed checks passed, 0 failures**. 97 framework/local mutation or non-fixtured cases were skipped/unverified by the runner; they are not claimed as live source verification. All six focused command happy paths, source-helper happy paths, JSON modes and error/dry-run paths passed. `phase5-acceptance.json` is binary-written and source-fingerprinted, never hand-authored.

Canonical shipcheck: 7/7 legs passed; structural scorecard **80/100 Grade A**, and 5/5 representative novel command samples passed. Workflow-verify's generic manifest leg reported no manifest; the separate real live chain above supplies concrete workflow evidence. `go test -count=1 ./...`, targeted post-fix CLI tests, build and `go vet ./...` passed. See `shipcheck.log`, `go-test.log`, `final-cli-tests.log`, `fix-tests.log` and `go-vet.log`.

## Runtime measurements

Measured on macOS arm64 with `/usr/bin/time -l`, isolated `--home`, `--no-learn`, one real uncached read and a populated cache read. Wall latency includes process startup. Uncached means `--no-cache`; cache population was a separate explicit refresh. These are representative measurements, not performance guarantees.

| Case | Mode | JSON bytes | Requests | Wall ms | Peak MiB |
|---|---|---:|---:|---:|---:|
| forecast | uncached | 3719 | 1 | 292 | 23.2 |
| forecast | cached | 3719 | 0 | 19 | 18.7 |
| inventory | uncached | 1825 | 1 | 66 | 26.4 |
| inventory | cached | 1825 | 0 | 19 | 23.0 |
| nationwide | uncached | 3360 | 1 | 1121 | 40.2 |
| nationwide | cached | 3360 | 0 | 61 | 35.4 |

Full metrics include downloaded bytes, cache hits and in-command latency in `performance/summary.json`; exact stdout/stderr captures are adjacent. Source inventory retrieval can be much larger than the bounded summaries (national sakura inventory about 1.75 MB), while detail remains lazy.

## Independent review and fixes

Three verified P2 findings fixed: invalid projection success; fabricated identity from missing/invalid source fields; explicit prior-year date relabeling. Follow-up P3 selected CSV/plain header regression fixed. Source identity and date-year failures were reproduced through synthetic mutations of real cached bodies, separate from live success evidence. Reviewer independently verified original reproductions, all six output schemas, fresh source reads and tests. Later raw-helper fixture delimiters and clean JSON dry-run metadata were also verified by the same reviewer. Final clearance: **no remaining independently verified findings**. See `independent-review.md` and the two `review-*-regression.json` proofs.

## Polish and limits

Zero unresolved gosec findings in hand-authored source. 32 generated framework findings remain disclosed in `gosec.json` as template candidates; no broad framework rewrite was performed. PII audit is clean. Two generated thin help descriptions were individually accepted with rationale; tools audit has no pending findings. Generic unused sync/helper and per-file wrapper-regex warnings are explained in `polish.md`; all actual provider HTTP passes through the bounded shared client with typed 429 errors.

As checked on 2026-10-01 JST: 2026 sakura updates explicitly ended, with next-season publication expected by the source in February 2027. Active next-season sakura predictions cannot be live-verified before publication; deterministic date/state tests cover parsing, and the CLI reports unavailable/ended states today. Current koyo reports and peak predictions are live verified. Unknown source issue times, elevations and unavailable dates remain null/explicit; no normal or fixture substitutes for a forecast.

## Reproduction and provenance

Build from project or library: `go build -o weathernews-pp-cli ./cmd/weathernews-pp-cli`. Required Go: 1.26.6+. Main focused source: `internal/evidence/`, adapter/schema projection: `internal/cli/travel_evidence.go` and `evidence_projection.go`. README, help and SKILL provide working recipes. Task-local cache captures remain Git-ignored; raw personal report/photo SSR streams are omitted from archives.

First-party sources: [home](https://weathernews.jp/), [sakura](https://weathernews.jp/sakura/), [foliage](https://weathernews.jp/koyo/), [Kyoto foliage](https://weathernews.jp/koyo/area/kyoto/), [koyo spot 26102](https://weathernews.jp/koyo/spot/26102/), [sakura spot 384](https://weathernews.jp/sakura/spot/384/), [member coverage](https://weathernews.jp/news/202308/210085/), [separate commercial API requirements](https://wxtech.weathernews.com/products/data/api/en/mcp/). Exact replay endpoints and discovery provenance are in research/source specs, browser report and capture index in the archived manuscript.

Final promoted-binary fresh public read passed (`promoted-smoke.json`); project/library provider source parity passed (`source-parity.json`). Receipt ledger closed at 21-next-steps with local-only completion.

## Publication verification (2026-10-01)

The publish review found and fixed two consequential defects: legitimate first-party mountain resolver paths were rejected, and bounded cache eviction could delete unrelated JSON in a caller-selected cache directory. Both regressions were reproduced before the fix and now pass. Mountain identities/coordinates remain source-derived; unavailable elevation remains null. Cache eviction operates only on regular SHA256-named cache JSON files.

Canonical install instructions were regenerated from the Printing Press emitter. Code customizations are indexed in `.printing-press-patches/weathernews-public-travel-evidence.json`. Public packaging excludes mutable runtime/workflow homes, raw dogfood transcripts and confidential local-path records.

Fresh full publish dogfood: **105/105 executed checks passed**; 97 skipped/unverified framework or non-fixtured checks remain explicit. All 12 focused cases reran with `--refresh` against first-party sources and passed. A separate decoder checked all three 高尾山 source rows for names, URLs and exact coordinates. Acceptance was regenerated by the binary and synchronized to both embedded and archived proofs; fingerprints normalize module names/imports for publish rewriting. See `publish-phase5-acceptance.json`, `publish-validation.json`, `publish-domain-tests.log` and `publish-focused-live/`.
