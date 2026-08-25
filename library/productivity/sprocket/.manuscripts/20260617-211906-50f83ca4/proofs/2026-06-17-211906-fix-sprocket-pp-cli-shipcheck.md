# Sprocket Sports CLI — Shipcheck Report

## Verdict: ship (pending Phase 5 live verification)

## Shipcheck umbrella (PASS 6/6)
| Leg | Result | Notes |
|-----|--------|-------|
| verify | PASS | mock-mode runtime checks |
| validate-narrative | PASS | all README/SKILL example commands resolve |
| dogfood | PASS | wiring + novel_features_check (9/9 built) |
| workflow-verify | PASS | |
| verify-skill | PASS | flag/command/positional/canonical-sections all clean |
| scorecard | PASS | 91/100 Grade A |

## Scorecard 91/100 (Grade A)
- 10/10: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native,
  MCP Quality, MCP Remote Transport, Local Cache, Vision, Workflows, Path Validity,
  Auth Protocol, Sync Correctness.
- Below 10: MCP Desc Quality 7, MCP Token Efficiency 7 (→ Phase 5.5 polish), Cache
  Freshness 5 (live data, no cache by design), Insight 7, Agent Workflow 9, Breadth 9,
  Data Pipeline Integrity 7, Type Fidelity 4/5.
- Live API Verification: N/A (no token in env; deferred to Phase 5).

## Code review (Phase 4.95) — 4 correctness bugs found and FIXED
A subagent reviewed the 11 hand-authored novel files. Findings (all fixed + regression-tested):
1. **[ERROR] Timezone skew** — zone-less calendar datetimes parsed as UTC but compared
   to local `now`, skewing `next`/`week`/all schedule commands by the UTC offset for any
   US club. Fix: parse zone-less times in local zone (`ParseInLocation`), normalize to
   `.Local()` for display.
2. **[ERROR] `owed` silent $0** — money as JSON string or unexpected wrapper produced a
   silent $0. Fix: `firstFloat` parses numeric strings; `asObjects` handles any
   single-array wrapper and reports uninterpretable shapes as an error, not $0.
3. **[WARN] `deadlines` off-by-a-day** — UTC/local + integer truncation. Fix: calendar-date
   diff in a single zone.
4. **[WARN] `conflicts` missed cross-location tight gaps** — early break skipped a later
   different-location event inside the gap window. Fix: scan the full gap window.
- Plus a `chunkRange` UTC-truncation bug surfaced by the timezone fix (would send wrong
  dates to the calendar API). Fixed to truncate on the local calendar date.
- Cleared by review: chunk windowing, weekBounds (incl. DST), diffSnapshots, buildICS
  (RFC-5545), snapshot 0600 perms, resource handling, credential safety (no token/PII to disk).

## SKILL/README review (Phase 4.8/4.9) — PASS
- Trigger phrases all map to real commands; anti-triggers correctly exclude standings &
  messages (not in surface) and state read-only + tenant scope; auth narrative accurate
  (SPROCKET_TOKEN/OIDC/DevTools extraction/expiry). No placeholder leaks in executable
  examples. Cosmetic: SKILL description frontmatter is truncated (long headline) — polish.

## Tests
- `go test ./internal/cli/...` PASS (19 table-driven novel-logic tests, incl. regressions
  for all 4 review findings).

## Ship-threshold check
- shipcheck exit 0 ✓ · verify PASS ✓ · dogfood PASS ✓ · workflow-verify PASS ✓ ·
  verify-skill exit 0 ✓ · scorecard 91 ≥ 65 ✓ · no flagship feature returns wrong/empty
  output (the live-sample 401s are correct auth-gating without a token).
- Behavioral correctness of novel features against the live API → Phase 5 dogfood.
