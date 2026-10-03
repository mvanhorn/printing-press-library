# Independent SnowJapan release review

2026-10-03 — one dedicated fresh-context gpt-6.1-sol MAX reviewer, reused through correction and publication re-review. No nested agents or Codex subprocesses.

**Overall assessment: PASS.** Thirteen findings are resolved: nine from the original independent review, three valid Greptile publication findings, and one further independent P2 concerning partial-report freshness. No P1/P2 findings remain in the reviewed source or native artifacts. This renewed verdict binds the source and artifact hashes below.

The review assessed the travel-styles brief, source contracts, absorb manifest and brainstorm against the actual implementation, source facts, CLI/MCP outputs, tests and documents. The five documented planners match the verified built set. README/SKILL/AGENTS accurately disclose source scope, local prerequisites, configuration paths, authentication and evidence states. Cold-cache and populated planner samples pass plausibility checks; independent populated, negative, empty and capped cases confirm the computations.

The source preserves the 478-row directory and 417-row completed 2025–2026 chart. Goryu terrain/installed-lift facts and its 151-day historical calendar span match the public source; a 2026-03-28–2026-04-05 interval yields nine overlapping calendar days. Kagura's separate access-base rows remain ambiguous. Uncaptured winters, missing endpoints and inconsistent dates remain distinct. Exact SSR identities fail closed, typed IDs cannot become global flags, fallback is network-only, and SQL row-close errors propagate.

The renewed review verifies the publication fixes: changes selects the newest compatible saved pair instead of preferring older detail; local get and automatic fallback require `detail-v1`/`report-observations-v1`; declaration normalization stops at the declared JSON container, preserving quoted strings and rejecting expressions within the data. Exact report observations can be captured using 1–4 distinct dated IDs. List-only rows fail with `detail_not_captured`, and docs disclose that later list sync can replace a saved detail row.

The additional report-freshness finding is also resolved. A fresh partial capture no longer masks older saved observations behind a new resource sync timestamp. Independent overlay checks prove the two-hour-old Niseko/fresh Hakuba case emits a stale hint, forced network failure rejects list-only fallbacks while preserving published zero for valid report details, and an actual changes command exposes a newer catalog change from 100 to 300 over older unchanged detail.

The Go suite passed during review, with relevant focused regression sets after subsequent corrections. Source/store/CLI regressions, the contradictory-SSR overlay and renewed independent boundary overlays pass. The actual final extracted MCPB executes offline resort/report details and typed MCP network fallback with the same factual projections; report new/season snowfall remains 0/0 cm, and Goryu retains 12 installed lifts. Metadata-only report/catalog rows and flag-shaped IDs fail. Packaged historical windows still return nine days. Declared ordinary collection MCP inputs retain CLI factual fields. Both embedded companions match the final staged binaries. Source-specific security findings are zero per the builder scan; emitted-framework findings were separately triaged.

The renewed full live marker records 135 passing cases, 85 skipped and 85 unverified. All five planner happy paths use a declared disposable database and populated factual evidence, without dry runs or uncaptured-winter responses; changes has an available baseline. This Press version skips sync's matrix row, so that row is not claimed as exercised. The portable setup independently performs real public directory/season sync, two resort-detail captures and an exact report capture with offline detail checks, then removes its fixture. The installed Press 4.33.0 `CaptureSourceFingerprint` implementation confirms all 177 normalized source files match the final marker, with zero drift in the working, promoted and publication copies.

## Assessed artifact hashes

| Artifact | SHA-256 |
|---|---|
| CLI | `e38413ea15a6d5a33150a9521d6ce4819705cb3803734c4e9949b8ddbb25aaf4` |
| MCP | `cdc10f511ce6f394870b0b62aac96006f671f3176508cec9b87a36776be1fea5` |
| Darwin ARM64 MCPB | `60ab7c71a354549a6af290197e84d3c81aed312cd8cc78905b583806f5f4d8c6` |
| Normalized source fingerprint | `1e57f91e9e2d01440c93d81ba28379ee3bb2a7357d79ac135f9d070c120cdd48` |

## Scope and evidence

Historical endpoints do not prove continuous operation or future openings. Installed lifts do not establish current lift operation. October reports are preseason base/town observations; new snow is since the previous report. Popular-region filters remain excluded because no replay contract was verified. Source narratives, credentials and provider sessions are not retained. The exact Manza hostname correction was checked against the official Gunma directory without fetching the wrong host. Native Darwin ARM64 execution was assessed; other platforms were not independently executed.

Builder evidence is under the run's `proofs/`, including `phase5-acceptance.json`, `matrix-actual-coverage.json`, populated planner samples and `bundle-artifact-proof.json`. Independent correction history and factual outputs are in `/private/tmp/snowjapan-independent-review-20261003/`; renewed overlays, packaged protocol samples and final source/artifact proofs are in its `recheck/` directory. Subsequent source changes require renewed acceptance.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---
