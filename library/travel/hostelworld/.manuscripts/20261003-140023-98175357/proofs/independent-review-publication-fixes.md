# Independent review — publication fixes

**PASS — 2026-10-03.** All four publication defects are resolved in the frozen working source. No additional actionable finding in the reviewed changes. This is the existing dedicated independent reviewer's focused re-review; no delegation, source edits or publication actions were performed. Previous broad clearance stands for unchanged paths.

Source: `working/hostelworld-pp-cli` in run `20261003-140023-98175357`. Reviewed the platform-scoped cache/save/saved/search/status paths, typed MCP search/SQL/context, retention transaction, date validation, cancellation parser and discovery disclosure, associated tests, README/SKILL guidance and patch record.

## Findings resolved

| Defect | Resolution and independent evidence |
|---|---|
| Profile cache escape | `internal/cli/hostelworld_planning.go:189` resolves a verified session's exact `Paths.DataFile`; missing verification/path and conflicting overrides fail closed. Save, saved reads, local search, explicit legacy sync and typed MCP search/SQL use this resolver. Workflow status reports scoped local counts and stale/manual cache semantics. MCP context reports the selected data/config/cache/state paths. Two synthetic verified profile sessions backed by separate real SQLite files were exercised: save/read, actual CLI search/status, typed MCP search/SQL/context return only their own profile's data. Unverified handlers reject rather than fall back; conflicting overrides reject. These are isolated synthetic profiles, not authenticated user accounts. |
| Unbounded/orphan FTS retention | `internal/cli/hostelworld_planning.go:221` prunes resources and orphan planning FTS rows in one transaction, deleting FTS by its own rowid. After 205 saves, both planning resources and planning FTS contain exactly 200; old and seeded orphan terms are absent, latest terms remain and unrelated resource/FTS rows survive. An independent off-tree overlay test forces FTS failure after the resource deletion step: prune errors and all 205 resources remain, proving rollback. |
| Same-day source date rejected | `internal/hostelworld/planning.go:30` uses the conservative UTC−12 calendar boundary when destination timezone is unknown. UTC 00:30 accepts the preceding calendar day because it is still current somewhere; an older date rejects. UTC 20:30 rejects the preceding day and accepts the current day. Existing stay-length, party and date-consistency bounds remain. This accepts a query for source evaluation; it does not promise same-day booking eligibility. |
| Availability signal mistaken for refundable rate | `internal/hostelworld/planning.go:329` establishes cancellation per rate. The unmodified observed deposit fixture is `conditional`, with basis `payment_requires_optional_flexible_booking`, despite the availability-wide free flag/deadline. Strict `--free-cancellation` yields `no_matching_offers` and an empty array. Wide source signal and deadline remain separately disclosed with `availability_response` scope. Contradictory non-refundable text beyond the 1600-character output cap is still classified from the full description. Positive per-rate synthetic-field tests establish parser semantics only; they do not establish current source support. |

Discovery now explicitly describes its weaker property-summary scope in flag help, response envelope and every row (`internal/cli/hostels_search.go:77,82,86`). Ordinary `--agent` preserves this scope; it does not imply a selected refundable rate. Optional-save MCP tools retain `readOnly:false`, `destructive:false`, `openWorld:true`.

## Independent validation

- Current-source targeted CLI/MCP/domain regressions passed, including profile isolation, retention, date boundaries, source units and cancellation cases. Additional temporary off-tree Go overlays exercised actual typed MCP handlers across both profile databases, retention rollback, and the unmodified actual deposit fixture. No repository tests or implementation were modified by the reviewer.
- Extracted the final MCPB and independently exercised its CLI and MCP against an isolated loopback server serving sanitized observed source fixtures. All seven checks in [independent-review-publication-fixes-runtime.json](independent-review-publication-fixes-runtime.json) pass.
- Packaged ordinary `--agent` retains dorm source **16910 per bed / three-night stay**, derived **33820 for two guests**, and private source **44808.48 per room / three-night stay**, with the same party total for two guests and explicit occupancy-slot nightly basis. Dates, currency, quantity, cancellation status/basis/deadline/scope and source-wide signals survive CLI and typed MCP output. These amounts are fixture evidence, not current-price promises.
- Latest live proof projections also retain rate-versus-availability scope and discovery's property-summary scope. Live dorm pricing changed to source 16720 / derived 33440 for two guests, which is consistent with the same unit contract.

## Scoped phase outcomes

- **Phase 14 semantic SKILL: PASS.** Updated cancellation, profile-cache, retention and unknown-local-calendar guidance matches implementation.
- **Phase 15 documents: PASS.** README/SKILL accurately disclose conditional rates, stale saved prices and conservative date acceptance; the payment example no longer requests a filter that excludes its observed conditional rates.
- **Phase 16 output: PASS.** Reviewed fixture/package outputs and latest relevant live projections preserve the critical fields and honest empty-filter result.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---

- **Phase 17 code/security/reliability/data: PASS.** Scoped reads/writes fail closed under selected profiles; retention is atomic and bounded; cancellation does not inherit a rate guarantee from a property/availability signal. No new safety regression found in the reviewed delta. Source transport and booking/payment/account boundaries remain as previously reviewed.

## Frozen artifact identity

Independent SHA256 checks confirm these exact root binaries are embedded byte-for-byte in the final MCPB:

- CLI: `80c56278445937553a379576568f4d59699daaff4c5a4806746642aaab73c736`
- MCP: `0c50a55b6314e6b472ee69d966d96ac6a4b2fbdf9a3f6801b286861b7ab8a8e0`
- MCPB: `6f72f09e8293c0c806b1da04d45373ba5ed0fb4a08b3c32fc68f8f3d1d756929`

This supersedes earlier artifact identities for the reviewed publication-fix payload. It does not assert stage-directory identity. The builder reports the final 133-pass/0-fail matrix and seven shipping gates; those final live/current-head gates and draft PR2253 readiness remain builder-owned. This review independently clears the publication fixes, source semantics and exact packaged fixture behavior, without assuming pending promotion or publication succeeds. Any later source or payload change needs the corresponding scoped recheck.

