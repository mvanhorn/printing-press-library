# Google Tag Manager CLI — Shipcheck

## Umbrella verdict: PASS (6/6 legs)

| Leg | Result |
|---|---|
| verify | PASS |
| validate-narrative | PASS (10 commands resolved, full examples passed) |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS — **91/100, Grade A** |

## Scorecard highlights
Output Modes 10, Auth 10, Error Handling 10, README 10, Doctor 10, Agent Native 10,
MCP Quality 10, MCP Desc 10, MCP Remote Transport 10, Local Cache 10, Workflows 10,
Path Validity 10, Auth Protocol 10, Sync Correctness 10, Type Fidelity 5/5, Dead Code 5/5.
Lower: MCP Token Efficiency 7, Cache Freshness 5, Insight 4, Vision 6 (read-only scope caps some).

## Behavioral verification (seeded mock mirror + unit tests)
Proven correct against a 3-snapshot / 11-entity seeded mirror and 11 unit tests:
- audit: dead-tag, orphan-trigger, unused-variable, missing-consent (high), custom-html, all-pages — all fire correctly.
- consent-report: gated/ungated split, vendor classification correct.
- fleet: tag/trigger/variable counts, custom-HTML, consent %, GA4-id inventory across 2 containers correct.
- diff: added/removed/changed with field names; volatile fields ignored; container-scoped.
- search: substring across name/type/params, cross-container; correct (earlier blob miss was a seed artifact, real TEXT storage works).
- uses / fires: reference graph both directions correct.
- export: deterministic flattened JSON.

## Sample-probe note (not defects)
The scorecard live-probe showed pull → HTTP 401 (no `GTM_ACCESS_TOKEN` in env — expected for an
auth-gated API) and empty output for diff/search/uses/export (probe seeds no mirror — correct no-data
behavior). SQLITE_BUSY from the first run was fixed by switching read commands to `OpenReadOnly`.

## Code review
pr-review-toolkit:code-reviewer on all hand-written GTM files: **PASS, no high/medium findings.**
Read-only invariant solidly enforced (GET-only client surface, mode=ro reads, stdout-only output);
SQL fully parameterized. One low-severity hardening applied: numeric-id validation in `pull`.

## MCP
tools-audit: no findings. All 9 novel commands carry `mcp:read-only` hints.

## Verdict: ship
All ship-threshold conditions met. No known functional bugs in shipping-scope features.
Live smoke testing deferred (no credential) — verified against realistic mock data + unit tests instead.
