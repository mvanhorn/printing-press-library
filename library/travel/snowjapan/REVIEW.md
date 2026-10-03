# Independent SnowJapan release review

2026-10-03 — one dedicated fresh-context gpt-6.1-sol MAX reviewer, reused through fixes and publication re-review. No nested agents or Codex subprocesses.

**Overall assessment: PASS.** All fifteen findings are resolved, including single-detail freshness scope. The same fixture confirms fresh/stale behavior through actual CLI, network fallback and typed MCP; collection-wide freshness remains correct. No P1/P2 findings remain. This verdict binds the final hashes below.

The review assessed the brief, source contracts, absorb manifest and brainstorm against implementation, public facts, CLI/MCP behavior, tests and documents. The five documented planners match the verified built set. Source evidence preserves the 478-row directory and 417-row completed 2025–2026 chart. Goryu's 151-day historical span and a nine-day overlap for 2026-03-28–2026-04-05 match source facts. Separate Kagura access-base rows remain ambiguous. Uncaptured winters, missing endpoints and inconsistent dates stay distinct.

Publication corrections select the newest compatible snapshot pair, require detail projections for offline get/network fallback, bound exact report capture to 1–4 distinct dated IDs, and stop declaration normalization before unrelated scripts. Saved-fact freshness uses actual observation times, so partial captures cannot hide older rows. Single-ID getters compute age after resolving the requested detail; unrelated old rows no longer produce false stale hints, while stale selected rows still warn. Identical before/after resort/report fixtures verify local CLI, automatic network fallback and typed MCP, plus unchanged collection oldest-row hints. Independent forced-network, quoted-data, newer-catalog-change and partial-report freshness boundaries pass. Exact SSR identities fail closed, typed IDs cannot become flags, and provider/schema failures do not trigger fallback.

The cache P1 was concretely reproduced: an open writer saw committed test peak 3000 while the old package returned 1676 and an empty change list. New source guards refuse active read journals, resolve symlinks, reject ambiguous hard links and pin one non-expiring SQL connection. Actual final packaged get/planners reject active WAL and recover the committed values after close. A real getter command with a settled path swap/restore returns original 1676; removing only pinning makes it return foreign 3000. Persistent replacement discards output.

Actual live report sync through canonical/symlink paths preserves an open canonical writer's unrelated committed observation and captures published 0/0 cm. Hard-linked writes reject with unchanged WAL bytes/hash and no added rows. Selected-path changes before/after save return errors without a success result or completion marker; post-save data stays solely in the original SQL database. New-cache creation uses `os.OpenRoot` with exclusive leaf creation and propagates file/parent close errors. Independent nested-cache and symlinked-parent creation preserve identity and 0600 permissions; actual native capture/readback retains the report projection and 0/0 cm. Direct/CTE/pragma writes through readers, cancellation, sidecars and URI-sensitive targets are checked. The affected store/CLI/MCP suites and independent actual-command overlays pass. Source-specific security findings are zero per the builder scan; emitted-framework findings were separately triaged.

Effective and embedded dependencies are `modernc.org/sqlite v1.46.2` and `modernc.org/libc v1.70.0`; an independent real `SELECT sqlite_version()` returns `3.51.3`, which includes the [official WAL-reset fix](https://www.sqlite.org/releaselog/3_51_3.html). The actual extracted final MCPB companions match staged binaries. Packaged MCP retains offline report/resort projections, preserves zero values and 12 installed lifts, rejects metadata-only fallback/flag-shaped IDs, and refuses active WAL before recovering its nine-day historical window after close. Cold-cache, populated, negative, empty and capped outputs pass plausibility checks. The renewed full live marker records 135 passing cases, 85 skipped and 85 unverified. All five planner happy paths use declared populated fixtures without dry runs or uncaptured winters; changes has a baseline. Sync's skipped matrix row is independently covered by real public fixture setup. All 183 normalized source files match the marker in working, promoted and publication copies.

## Assessed artifact hashes

| Artifact | SHA-256 |
|---|---|
| CLI | `29b2bd9641d5636af48ec41260d45ab8de673185e9815fcad1121a5991943feb` |
| MCP | `1241b91f5851a3a96507aff66806e9271b25e25f02bf4d328beab0713f5da7a7` |
| Darwin ARM64 MCPB | `2239c2870aef799a507e02fe7e56441a5a80adfcd35bf81c802d01342ade7d91` |
| Normalized source fingerprint | `de6fc16a807079b3063498430751d9fa6c020f321650e02fec663ed4639830b0` |

## Scope and evidence

Local reads require settled writers; active WAL/SHM/journal state deliberately fails closed. Hard-linked databases and URI-sensitive literal target names are unsupported. Historical endpoints do not establish continuous operation or future opening. Installed lifts do not establish current operations. October reports are preseason base/town observations; new snow is since the previous report. Popular-region filters remain excluded without a replay contract. Narratives, credentials and sessions are not retained. The Manza hostname correction was checked against the official Gunma directory without fetching the wrong host. Native Darwin ARM64 execution was assessed; other platforms were not independently executed.

Builder evidence is under the run's `proofs/`, including source-bound acceptance, populated planner, artifact, cache and normal-workflow proofs. Independent correction history is in `/private/tmp/snowjapan-independent-review-20261003/`; final cache audit/overlays and source/artifact/MCP proofs are in `wal-review/`, packaged WAL checks in `wal-final-package/`, and actual write-alias checks in `wal-sync-review/`. Subsequent source changes require renewed acceptance.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---
