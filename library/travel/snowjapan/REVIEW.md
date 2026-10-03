# Independent SnowJapan release review

2026-10-03 — one dedicated fresh-context gpt-6.1-sol MAX reviewer, reused through correction. No nested agents or Codex subprocesses.

**Overall assessment: PASS.** Phases 14–17 pass, and the required full live acceptance marker is green on the frozen source. Nine findings were resolved across three review rounds: one P1 stale packaged CLI and eight P2 issues concerning season completeness, ambiguity, bounded coverage scope, MCP comparison inputs/hints, source identity, catalog freshness and automatic get fallback. No remaining P1/P2 findings in the reviewed source or native artifacts.

The review assessed the travel-styles brief, source contracts, absorb manifest and brainstorm against the actual implementation, source facts, CLI/MCP outputs, tests and documents. The five documented planners exactly match the verified built set. README/SKILL/AGENTS accurately disclose source scope, local prerequisites, config paths, authentication and evidence states. The five cold-cache output-review samples and five populated final samples pass plausibility checks; additional independent populated, negative, empty and capped cases confirm the computations.

The source preserves the 478-row directory and 417-row completed 2025–2026 chart. Goryu terrain/installed-lift facts and its 151-day historical calendar span match the public source; a 2026-03-28–2026-04-05 interval yields nine overlapping calendar days. Kagura's separate access-base rows remain ambiguous. Uncaptured winters, missing endpoints and inconsistent dates remain distinct. Per-dataset observation ranges and freshness hints survive CLI and MCP. Exact SSR identities fail closed, typed IDs cannot become global flags, automatic fallback is network-only, and SQL row-close errors propagate.

The full Go suite passed, followed by focused regression sets after later source/CLI/MCP edits. The independent contradictory-SSR overlay now passes. Actual extracted final MCPB companions execute factual get, structured comparison and historical windows; flag-shaped IDs fail. Declared ordinary list/search MCP inputs produce the same factual fields as the CLI. Root search requires its term while resort search correctly accepts flags without a term. Both packaged binaries match the final stage hashes, closing the stale-companion blocker. Source-specific security findings are zero per the builder's final scan; generated-framework findings were triaged separately under the reserved-code rule.

The final full live marker records 135 passing cases, 85 skipped and 85 unverified. All five planner happy paths use an explicitly supplied disposable database and populated factual evidence, without dry runs or uncaptured-winter responses; changes has an available baseline. The Press version's special sync rule skips its matrix row, so that row is not claimed as exercised. The portable verification setup independently executes real public directory/season sync and two resort-detail captures, then removes its fixture. The installed Press 4.33.0 `CaptureSourceFingerprint` implementation confirms all 177 normalized source files match the marker, with zero drift.

## Assessed artifact hashes

| Artifact | SHA-256 |
|---|---|
| CLI | `b2b46b8b629cc4f2d33499317d17edd9b74e80564fda04a962babaf08f261701` |
| MCP | `5245da910e604336ba06caf5e237c044002c8b19b1829c0e3ac4ea041f3bdffc` |
| Darwin ARM64 MCPB | `5d54a8e04d6fd5db7747ab673eecc084859ba29cf9bdb2dea880dba7699ee9f9` |
| Normalized source fingerprint | `24b9e7dd555da90847bcfb88c33509fed243738cdd1a40646fbcf0cd2fe8bc45` |

## Scope and evidence

Historical endpoints do not prove continuous operation or future openings. Installed lifts do not establish current lift operation. October reports are preseason base/town observations; new snow is since the previous report. Popular-region membership filters remain excluded because no replay contract was verified. Source narratives, credentials and provider sessions are not retained. The exact Manza hostname correction was independently checked against the official Gunma directory without fetching the wrong host. Native Darwin ARM64 bundle execution was assessed; other platforms were not independently executed.

Builder evidence is under the run's `proofs/`, including `phase5-acceptance.json`, `matrix-actual-coverage.json`, `fixture-*.json`, `output-review-livecheck.json`, `populated-*-final.json` and `bundle-artifact-proof.json`. Independent factual outputs, final packaged MCP protocol samples, the normalized fingerprint check, artifact checks and detailed correction history are in `/private/tmp/snowjapan-independent-review-20261003/`. This verdict binds the hashes above; subsequent source changes require renewed acceptance.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---
