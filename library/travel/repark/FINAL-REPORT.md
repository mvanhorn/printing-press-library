# Repark CLI final report

Status: **ship — built, independently reviewed, live-tested and atomically promoted locally.** GitHub publication remains separate, as authorized by the batch brief. No payment, reservation or account action was performed.

Source repository: `/Users/zjsng/Projects/Personal/Coding/repark-cli`

Promoted library: `/Users/zjsng/Projects/Personal/Coding/.japan-cli-builds/batch3-press/repark/library/repark`

Fresh host executable: `build/stage/bin/repark-pp-cli` beneath the promoted library. Fresh MCP executable and Darwin ARM64 bundle are in `build/stage/bin/repark-pp-mcp` and `build/repark-pp-mcp-darwin-arm64.mcpb`.

Archived manuscripts: `/Users/zjsng/Projects/Personal/Coding/.japan-cli-builds/batch3-press/repark/manuscripts/repark/20261002-224755-65ef3fef`

Run ID: `20261002-224755-65ef3fef`. Printing Press v4.32.5 was used for actual generation, checks, binary-owned acceptance and atomic promotion, with an isolated home. The build lock is released. Phase receipts reach completion through phase 21. Exactly one fresh-context gpt-6.1-sol MAX reviewer was used and reused for closure; no extra agents or codex exec.

## Delivered behavior

`parking search` resolves a supplied Japanese place/station/area through the provider, with explicit refinement results when no unique anchor is established. `parking nearby` accepts explicit WGS84 coordinates or canonical source lot coordinates. `parking detail`, `parking compare` and `parking quote` expose source facts, comparison and verified source fee simulation for an explicit bay and JST interval. `parking capabilities` states supported paths, bounds and unknowns. All five approved capabilities are built.

Live occupancy categories remain separate from declared height/length/width/weight checks and remaining-bay suitability. Hours, capacity, day/night/day-type rate groups, raw maximum-charge rules, repeating versus one-time applicability, overnight and calendar boundaries, provider timestamps and nullable tax statements are preserved. Array and non-contiguous indexed charge groups are supported. Bay-specific price exceptions stay unparsed/nullable instead of becoming a false universal cap. No local complete total is invented.

Runtime uses same-provider public HTTP without a browser or credentials. Native Chrome source behavior was inspected first; retained capture evidence distinguishes the actual document observation from subsequent direct public HTTP contract verification. It includes source freeword resolution, marker-window JSON, lot detail and fee-simulation form/POST. No driver location is inferred.

## Verification

| Gate | Result |
|---|---|
| Canonical final shipcheck | Seven legs PASS, exit 0 |
| Runtime verification | 31/31, 100%, zero critical failures |
| Final full live matrix | 113/113 active rows PASS, zero failures; 100 explicit framework/safety skips |
| Approved feature samples | Five evaluated, five PASS, none hollow |
| Primary workflow | Source discovery → detail → provider quote PASS |
| Independent SKILL/docs/output/code review | PASS, all findings closed |
| Scorecard | 91/100, grade A |
| Go tests and vet | PASS on final source |
| Dependency vulnerability scan | Pinned govulncheck v1.3.0: no vulnerabilities |
| Tool audit | No pending findings; two precise generated framework Shorts accepted |
| PII audit | Shipping tree clean; archive has two justified public-station URL false positives accepted |
| Promotion proof | Fresh promoted executable fetched REP0022209 successfully; lock released |

The live matrix's six BLOCKED_FIXTURE skips concern generated local `learnings confirm/reject`, not parking/source capabilities. They are preserved as machine-gap evidence. Gosec v2.26.1 completed and retained 30 findings in byte-identical initial generated framework files, with zero hand-written Repark findings; this is not an all-clean raw scanner claim. Full triage and local retro candidates are archived. Reserved cliutil/MCP cobratree code was not patched.

Measured live Tokyo Station search, three lots: 1.233 seconds, 10,478 stdout bytes and 27.06 MiB peak RSS. Source quotes took about 9–13 seconds in earlier checks. Explicit bounds: 50–2000 m radius, 1–50 output lots, 1–1000 scanned rows, 2 MiB response bodies, 20-second per-request and 60-second command limits, no automatic retries, default two source requests per second, typed throttling. Snapshots require an explicit validated map window through a flag or `REPARK_SYNC_RANGE`; no default location is preset.

Known source boundaries: exact free-space count and occupancy measurement time are unknown; available/crowded can represent compact or restricted remaining bays; remaining-bay fit and coverage are unguaranteed; provider timestamp semantics/timezone and unspecified tax inclusion remain unknown; quotes use current source tariffs, exclude partner discounts and do not guarantee final billing. These are disclosed in output and docs.

Key evidence: archived `proofs/repark-acceptance.md`, `phase5-acceptance.json`, `dogfood-live-full-final.json`, `shipcheck-post-dogfood.json`, `repark-polish.md`, `independent-review.md`, `gosec-triage.md`, `machine-gaps.md`, `performance-search-summary.json`, `promote.txt` and `promoted-live-detail.json`. Native/public HTTP discovery evidence is under archived `discovery/`. HAR authentication/cookies/query secrets and response bodies were stripped; no authenticated session state was retained.

Next-step selection: **Done with local build/promotion**, preapproved by the batch brief. Publication can use the separate Printing Press publish workflow when requested.
