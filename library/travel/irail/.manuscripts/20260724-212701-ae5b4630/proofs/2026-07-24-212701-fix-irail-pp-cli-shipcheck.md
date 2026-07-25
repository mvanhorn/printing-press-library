# irail-pp-cli — Shipcheck

## Final result

```
  LEG                 RESULT  EXIT
  verify              PASS    0
  validate-narrative  PASS    0
  dogfood             PASS    0
  workflow-verify     PASS    0
  apify-audit         PASS    0
  verify-skill        PASS    0
  scorecard           PASS    0

Verdict: PASS (7/7 legs passed)
```

## Scorecard: 96/100 — Grade A

| Dimension | Score |
|---|---|
| Output Modes / Auth / Terminal UX / README / Doctor / Agent Native | 10/10 each |
| MCP Quality / MCP Desc Quality / MCP Remote Transport / Local Cache | 10/10 each |
| Vision / Workflows / Insight | 10/10 each |
| Breadth | 9/10 |
| Agent Workflow | 9/10 |
| Error Handling | 8/10 |
| MCP Token Efficiency | 7/10 |
| Cache Freshness | 5/10 |
| Path Validity / Data Pipeline Integrity / Sync Correctness | 10/10 each |
| Type Fidelity / Dead Code | 5/5 each |

Sample Output Probe: 7/7 live novel-feature samples, 100% pass rate.

**Cache Freshness 5/10 is a deliberate trade-off, not an oversight.** Enabling
generator cache freshness fires a pre-read sync against `board`, `route` and
`train`, which require parameters (`station`, `from`/`to`, `id`) that the
generated syncer does not supply, producing HTTP 500/400 on the CLI's three most
important commands. Station reference data is refreshed explicitly instead with
`sync --resources stations,disruptions`. Correct behaviour was preferred over
five scorecard points.

## Before / after
- Scorecard: 96/100 at first measurement, held at 96/100 after fixes.
- Verify: PASS throughout.
- tools-audit: 2 pending findings -> 1 (the remaining one is in a
  generator-emitted template file and would revert on regeneration).

## Top blockers found and fixed
1. **Pre-read sync broke the three headline commands** (HTTP 500/400). Removed
   `cache.enabled`.
2. **Every response came back as XML.** iRail ignores `Accept: application/json`
   and the syncer discards spec param defaults. Fixed by pinning `format=json`
   into each endpoint path.
3. **Reachable panic** slicing RFC3339 timestamps at a fixed offset when iRail
   omits a time field. Routed through a bounds-safe helper; regression test added.
4. **Duplicate command registration** for `stations facilities` and
   `disruptions route`.
5. **Inaccurate typed-output claim** in the narrative. The raw endpoint commands
   pass iRail's string scalars through unchanged; only the analysis commands emit
   typed values. Claim corrected and a troubleshooting entry added.
6. **Documented invocations that silently did the wrong thing**: examples used
   `board list` where the generator promotes the command to `board`, so the extra
   word was silently ignored.
7. **Help text referencing commands that do not exist** (`route plan`,
   `disruptions list`).
8. **Station listed twice** in `disruptions route` output under its Dutch and
   English names.

## Ship threshold
- shipcheck exits 0, every leg PASS — yes
- verify verdict PASS — yes
- dogfood structural + wiring checks pass — yes
- workflow-verify: workflow-pass — yes
- verify-skill exit 0 — yes
- scorecard >= 65 — yes (96)
- No flagship or approved feature returns wrong/empty output — verified live,
  command by command, including absence-of-correctness cases

## Final recommendation: ship

All 7 approved transcendence features and all 24 absorbed features ship fully
implemented. No stubs, no known functional bugs, no deferred work from the
approved manifest.
