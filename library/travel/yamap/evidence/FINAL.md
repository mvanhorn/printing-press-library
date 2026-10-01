# YAMAP CLI final evidence

This is the local-generation report retained for provenance. Publication subsequently qualified the Go module path, regenerated public install instructions, omitted private paths/raw captures, and reran validation, the full live matrix, and 25 source-comparison E2E cases on the public package. Current publication proofs are in `.manuscripts/20261001-020507-6320344e/proofs/`. Historical log filenames below refer to retained local generation evidence; raw logs are intentionally omitted from the public bundle.

Run: `20261001-020507-6320344e`. Research and verification date: 2026-10-01 Asia/Singapore. Sole builder; exactly one independent fresh-context reviewer, reused for verification. No publishing, PR, purchase, booking, account mutation or shared configuration change.

## Outcome and canonical paths

The agreed public hiking discovery scope is implemented, buildable, independently reviewed and successfully promoted to the local library. The delivered workspace builds; all 182 acceptance source files are byte-identical across project/staging/library; the promoted binary passed a fresh anonymous read. See `promotion.json` beside this report. No substantive independent review finding remains.

| Artifact | Canonical path |
|---|---|
| Delivered project | `<source-checkout>` |
| Isolated staging source | `<private-runstate>` |
| Local library | `<local-library>` |
| Archived research/proofs/discovery | `<archived-manuscripts>` |
| Disposable receipt ledger | `<private-runstate>` |

Project `README.md`, `SKILL.md`, `AGENTS.md`, Go source, tests, spec and both CLI/MCP binaries are local deliverables. Go 1.26.6+ is required; verified with Go 1.27.1. Build with `go build -o yamap-pp-cli ./cmd/yamap-pp-cli`. The local Go module is deliberately unpublished. The receipt ledger stays in runstate and is not a manuscript. Runtime databases/cache and the stale initial MCP bundle are isolated from the deliverable. The reviewed MCP binary is current; no stale bundle is distributed.

## Source access, costs and scope

Anonymous website HTTP requests to `yamap.com` returned AWS WAF HTTP 202 empty responses. Fresh Chrome rendered the public website; its first-party page asset inventory exposed `api.yamap.com/v6`. A raw CDP permission was dismissed and no workaround used. Plain HTTP replay of that API returned public JSON with Japanese Accept-Language and a Mozilla-leading User-Agent. No cookies, token, credentials or paid account were used. These are undocumented first-party website endpoints, so availability and schemas can change. A future challenge or restriction is surfaced as a failure, never fabricated empty data.

At research time, [YAMAP Premium](https://yamap.com/premium) advertised 5,700 JPY/year or 780 JPY/month. The shipped scope incurs no provider subscription fee. GPX downloads, personal account exports and Premium [multi-landmark operations](https://help.yamap.com/hc/ja/articles/30703212737561) are excluded. Public access was exhausted before selecting the focused scope. No external authority, weather, transit or other travel provider is integrated. Community [yamap-export](https://github.com/akiyama709/yamap-export) informed header/access research; its bulk archival scope was explicitly excluded in `ABSORB.md`.

## Shipped functionality and evidence boundaries

| Commands | Shipped behavior | Interpretation limits |
|---|---|---|
| `mountains search/get/routes/reports` | Japanese names, source IDs, prefectures, canonical URLs, elevation where public; mountain-specific courses/reports | Duplicate names remain separate; broad search hits are not proof of traversal |
| `routes search/get` | Planned reference courses, source duration/distance/elevation, map IDs and publisher route flags | Standard course time is an estimate; false closed/dashed flags do not establish open or safe |
| `routes compare COURSE REPORT` | Separate planned metrics and explicitly recorded trip metrics | Planned or unknown-status activity detail is rejected; no track equivalence, pace or safety inference |
| `reports search/get` | Lazy activity summaries/details, trip date versus publication/update/fetch time, bounded source text | Summary planned status may be unknown; missing fields are null |
| `reports recent` | Trip-date filtering and sorting within a bounded candidate scan; known planned/future activities excluded | Missing planned status remains unknown; seasonal evidence remains dated; not exhaustive recency coverage |
| `reports observations` | Contributor text, trip date, source link and evidence class | Contributor observations are not authoritative advice or official closure verification |
| `maps search/get/coverage` | Map-area bounds, source version/deprecation and attributed publisher cautions where present | Bounds are not trail/track overlap, navigability, offline tiles or complete closure coverage |
| `inventory status` | Exact cached request inventory and freshness without a source crawl | Cache is a request window, not a full offline inventory |

Activity detail prefers the regularized `activity_whole_section`; list summaries use legacy activity aggregates and identify that source. Live activity 51497803 had detail distance 5,598 m versus legacy 5,610.66 m, and elapsed 5,366 s versus legacy 5,400 s. Those are separate source definitions, not rounding into a single invented total. Detail also supplied active 5,085 s and rest 281 s. Per [YAMAP time definitions](https://help.yamap.com/hc/ja/articles/900005534146), elapsed includes rest; automatic stops of at least three minutes contribute to rest. Source active/non-rest time is not asserted to be exact moving time; `moving_seconds` remains null.

Publisher map cautions retain source attribution/date, capped at five notices of 600 characters. Linked authorities are not fetched or independently verified. All focused output states unknown official closure coverage and unknown safety assessment. No command certifies a route safe or open from user reports, empty results or a false publisher flag.

## Agent interface and operational bounds

Focused commands default to compact JSON `{meta,results}`; `--select` projects row/dotted fields while retaining provenance and coverage. Units are meters/seconds; UTC timestamps and Japan trip dates are explicit. Null distinguishes unavailable fields from valid zero and from a source-provided empty string. IDs are strings and canonical links stay on YAMAP. Diagnostics/errors go to stderr. Invalid IDs, foreign-provider URLs, oversized/negative bounds and unsupported modes fail predictably.

Default search is one page/five results; limit 1–20 and at most five pages. Recent scans one page of twenty candidates by default, at most five pages. API activity totals can cap at 10,000; outputs preserve partial source coverage and candidate-scan limits. List commands perform no detail/photo/geometry fanout. Planned courses and activity logs use distinct endpoint contracts. Course text filtering uses the source `name` parameter: live research established that `keyword` was silently ignored, so it is not used.

Focused reads are serial, at most two requests/second, two attempts for transient 429/5xx, 15-second per-request timeout, 30-second default total command budget (positive configurable maximum 2m) and 4 MiB response cap. Long Retry-After fails promptly. Cache TTL is 15 minutes, capped at 64 entries/16 MiB with private atomic confined files. `--refresh` refreshes the exact request; `--no-cache` bypasses persistence; `--data-source local` serves only an exact cached request with stale metadata. Cache misses fail explicitly; upstream failures do not silently fall back to old data. Accepted cache age is positive and at most 24h.

Generated compatibility `source-*` mirrors are hidden from normal help and raw MCP endpoints are disabled. Retained compatibility page/per flags are bounded. Generated sync/archive use only bounded source-course windows, preserve source terminal pagination and explicitly identify partial coverage at a page cap. Focused commands disable automatic learning; generic local framework tools remain separately available. JSON is the focused format; CSV/plain/quiet are rejected with usage exit 2. Exit codes and recovery steps are in README/SKILL.

## Verification and independent review

| Check | Result | Evidence |
|---|---|---|
| Fresh first-party correctness/query relevance E2E | 25 cases pass; source IDs/names/metrics compared to separate fresh HTTP reads | `live-e2e.json`, `live-e2e-final.log`, project `tools/live_e2e.py` |
| Full Press live matrix | 218 mandatory checks pass; zero failures | `live-dogfood.json`, binary-owned `phase5-acceptance.json` |
| Optional/generic matrix checks | 122 skipped/unverified; not claimed as verified | Runner classifications in `live-dogfood.json` |
| Source workflow verification | Six live source workflows pass | Project `workflow-verify-report.json`, `workflow_verify.yaml` |
| Final shipcheck | All seven legs pass | `shipcheck-polish.json` |
| Structural/mock verification | 46/47, 97.87%, zero critical failures; one reports-parent classification failure | Manifest `.printing-press.json`; distinct from live source verification |
| Scorecard | 88/100, grade A | `.printing-press.json` |
| Go tests/vet | Full suite and final focused interface regressions pass; vet clean | `all-tests-final.log`, `interface-fix-tests.log`, `vet-final.log` |
| Vulnerability scan | Generation/build scan passed | `build-log.md` |
| PII audit | Zero findings | `pii-final.json` (`null`, exit 0) |
| Independent review | PASS after eight reproduced findings were fixed, no remaining substantive findings | `review-round1.md`, `review-fixes.diff`, `review-final.md` |

The sole reviewer was `gpt-6.1-sol` at xhigh with no conversation inheritance and no edits. Six original findings concerned planned/unknown comparisons, source pagination, archive resources, raw MCP bounds, missing text and root usage/path handling. Two subsequent cascades concerned generated string-valued pagination flags and terminal page fallback. All were fixed and verified by the same reviewer. Its final result: terminal pages finish after exactly two requests; larger budgets make no extra request; capped sync/archive remain explicitly partial; no substantive finding remains. The later help-example/dry-run interface adjustments passed the full live matrix.

Deterministic fixtures cover consequential parsing/domain/state logic: missing/zero/text values, metric definitions, trip-date freshness, planned versus recorded comparisons, ID/URL validation, cache confinement/freshness, throttling/budgets and terminal/capped pagination. Reviewer reproductions also used controlled cache/HTTP fixtures. Those fixtures are never represented as live source verification. Live evidence actually called anonymous first-party endpoints. Initial failed interface acceptance reports are retained as `phase5-acceptance.failed-interface.json` and `live-dogfood.failed-interface.json`; the missing group examples and compatibility dry-run JSON were fixed before the full rerun. The pass marker was produced by the Press binary and was not hand-authored.

Gosec v2.26.1: zero remaining findings in handwritten hiking/focused-command code. Seven generated framework signals remain, reviewed as intended local filesystem operations: G703 `teach.go:210` scrubbed local-log write; G304 `teach_playbook.go:376` supplied input file; `teach.go:127,150` local logs; `feedback.go:66,204` local feedback files; `export.go:213` explicitly selected output file. They are not reported as a clean whole-repository scan. See `gosec-after.json`. The two tools-audit short-description warnings are accurate local list commands (`platform_client.go:517`, `teach.go:868`). Dead-code dry-run candidates include active additive callbacks and framework extension hooks; blindly deleting them would break functionality. No substantive polish issue remains.

## Measured cost and resource use

Representative five-result `reports search 高尾山` samples, measured with macOS `/usr/bin/time -l`. Peak resident set values are bytes. These are individual measurements, not latency guarantees. Metadata bytes vary slightly with cache status.

| Mode | Output bytes | Upstream bytes | HTTP requests | Cache hits | Wall latency ms | Peak RSS bytes |
|---|---:|---:|---:|---:|---:|---:|
| Explicit refresh (uncached) | 4,416 | 54,658 | 1 | 0 | 938.06 | 25,493,504 |
| Cached exact request | 4,411 | 0 | 0 | 1 | 15.01 | 19,759,104 |
| Cached `--select id,title,url` | 1,642 | 0 | 0 | 1 | 11.25 | 20,611,072 |
| Offline exact cached request | 4,411 | 0 | 0 | 1 | 10.93 | 21,102,592 |

The full compact response was approximately 92% smaller than the upstream JSON; projection reduced the cached response another 63%. Lazy single-detail requests remained one upstream request. The report includes per-case output size, request count, latency and peak memory for all 25 E2E cases.

## Press completion and remaining limits

The Press run uses isolated state and receipts through local promotion/Done. The installed 4.32.5 binary had an outdated unpublished-module install checker; an isolated 4.32.5 tool was built from an existing local fix, with shared source/binary untouched. Its absolute path is recorded in `.press-run.json`. Shipcheck, live acceptance and promotion use that same binary. The original binary path is retained for traceability. No publishing readiness is implied and no global install was performed.

Unsupported/unverified coverage is explicit: membership/private/deleted resources, official closure verification, personalized safety advice, tracks/GPX/tiles/offline navigation, complete source inventories and guaranteed future access to an undocumented API. These are outside the agreed shipped scope. Map cautions are bounded and linked authorities remain unverified. A recent text search can mention a mountain without traversing it. Current-condition conclusions require independent authoritative evidence; this tool preserves contributor observations and dates.

Archived proof files exclude runtime cache/database directories. Discovery JSON and research are anonymous public evidence; no authentication/session state was captured. Disposable receipts stay in runstate. The user already selected local completion and prohibited publishing, so the final next-step receipt records Done without opening a menu or contacting another agent for routine approval.
