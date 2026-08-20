# Phase 4.85 Agentic Output Review Findings

Wave B policy: all findings are `warning` severity, non-blocking. Shipcheck proceeds regardless.

## Findings

1. **[warning] `recognition audit --dept engineering --agent` returns bare `{}` with no in-band reason.** The "department not found locally, run sync" guidance is stderr-only; an `--agent`/`--json` consumer parsing only stdout won't see it. Not fixed this run. Fix: embed the reason in the JSON body alongside `{}`, e.g. `{"department": "engineering", "found_locally": false, "hint": "run sync --resources departments"}`.
2. **[warning] `recognition values --dept engineering --agent` — same issue as #1**, same fix.
3. **[warning → investigated further, see shipcheck report] `balance history --agent` prints a raw API error (404) instead of a graceful empty state.** Investigation upgraded this from "format concern" to "confirmed wrong inferred path" — `GET /users/me/points_balance` genuinely 404s against the real bonus.ly server (verified via direct curl, 8 total candidate-path probes, all 404). This is now the headline entry in the README/shipcheck `## Known Gaps` sections rather than being fixed as a formatting issue, since the real bug is the wrong path, not the error presentation.

## Disposition

Findings #1 and #2 are legitimate, low-cost future polish (not fixed this run — logged for whoever picks this up next, or for a future polish pass with a live token to test against). Finding #3 was investigated immediately given its severity (affects a table-stakes command) and is now documented as a confirmed-wrong path with a concrete remediation path in Known Gaps, per the ship-with-gaps requirements.
